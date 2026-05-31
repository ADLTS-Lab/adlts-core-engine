"""
lane_hough.py — Proven Hough-lines lane detector.

Ported from adlts/backend/lane_detector.py (tested on the actual ADLTS track).
No perspective calibration required.  Works directly on the raw camera image.

Algorithm
---------
1. Grayscale → GaussianBlur → Canny edges
2. Trapezoid ROI mask  (bottom-wide, top-narrow, tuned for forward-facing camera)
3. HoughLinesP — collect candidate line segments
4. Separate into left / right by slope sign
5. Average all left segments into one line; same for right
6. Extrapolate both lines from frame bottom to ~60 % height
7. One-side fallback: estimate missing side from last-known lane width
8. Compute all fields required by scorer.py and main.py

Output: LaneResult (same dataclass as lane_classical.py)
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from typing import Optional

import cv2
import numpy as np
from lane_classical import FRAME_H, FRAME_W, STRAIGHT_R, LaneResult

# ── Tunable parameters ─────────────────────────────────────────────────────────
# These match the tested values from adlts/backend/lane_detector.py.
# Adjust only if the track surface changes significantly.

CANNY_LOW = 50
CANNY_HIGH = 150
HOUGH_RHO = 1
HOUGH_THETA = math.pi / 180
HOUGH_THRESHOLD = 20
HOUGH_MIN_LEN = 20
HOUGH_MAX_GAP = 10

# Minimum absolute slope to be considered a lane line (rejects near-horizontal noise)
MIN_SLOPE = 0.3

# How far up the ROI reaches (fraction of frame height from top)
ROI_TOP_FRAC = 0.45  # ignore top 45 % — avoids background clutter
ROI_SIDE_FRAC = 0.10  # inset from sides at the top of the trapezoid

# Bottom y for the extrapolated line, top y
LINE_Y_BOTTOM = FRAME_H
LINE_Y_TOP = int(FRAME_H * 0.60)

# Lane-width memory: if one side goes missing, last good width is used for estimation
_last_lane_width_px: float = FRAME_W * 0.45  # sensible default for ~30 cm track


def detect(frame_bgr: np.ndarray) -> LaneResult:
    """
    Run the Hough-lines pipeline on one BGR frame.

    Returns a fully-populated LaneResult.  Falls back gracefully when lanes
    cannot be found (lane_detected=False, all numeric fields = 0).
    """
    global _last_lane_width_px

    h, w = frame_bgr.shape[:2]

    # ── 1. Edge detection ──────────────────────────────────────────────────────
    gray = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2GRAY)
    blurred = cv2.GaussianBlur(gray, (5, 5), 0)
    edges = cv2.Canny(blurred, CANNY_LOW, CANNY_HIGH)

    # ── 2. ROI mask — trapezoid ────────────────────────────────────────────────
    roi_vertices = np.array(
        [
            [
                (0, h),
                (int(w * ROI_SIDE_FRAC), int(h * ROI_TOP_FRAC)),
                (int(w * (1 - ROI_SIDE_FRAC)), int(h * ROI_TOP_FRAC)),
                (w, h),
            ]
        ],
        dtype=np.int32,
    )
    mask = np.zeros_like(edges)
    cv2.fillPoly(mask, roi_vertices, 255)
    masked = cv2.bitwise_and(edges, mask)

    # ── 3. Hough line detection ────────────────────────────────────────────────
    raw_lines = cv2.HoughLinesP(
        masked,
        HOUGH_RHO,
        HOUGH_THETA,
        HOUGH_THRESHOLD,
        minLineLength=HOUGH_MIN_LEN,
        maxLineGap=HOUGH_MAX_GAP,
    )

    # ── 4. Split into left / right by slope ────────────────────────────────────
    left_segs: list[tuple] = []
    right_segs: list[tuple] = []
    if raw_lines is not None:
        for seg in raw_lines:
            x1, y1, x2, y2 = seg[0]
            if x2 == x1:
                continue
            slope = (y2 - y1) / (x2 - x1)
            if slope < -MIN_SLOPE:  # negative slope in image coords → left
                left_segs.append((x1, y1, x2, y2))
            elif slope > MIN_SLOPE:  # positive slope → right
                right_segs.append((x1, y1, x2, y2))

    # ── 5. Average + extrapolate ───────────────────────────────────────────────
    left_line = _fit_line(left_segs, h)
    right_line = _fit_line(right_segs, h)

    # ── 6. One-side fallback ───────────────────────────────────────────────────
    if left_line is None and right_line is not None:
        rx_bottom = right_line[0]
        left_line = (
            int(rx_bottom - _last_lane_width_px),
            LINE_Y_BOTTOM,
            int(rx_bottom - _last_lane_width_px * 0.5),
            LINE_Y_TOP,
        )
    elif right_line is None and left_line is not None:
        lx_bottom = left_line[0]
        right_line = (
            int(lx_bottom + _last_lane_width_px),
            LINE_Y_BOTTOM,
            int(lx_bottom + _last_lane_width_px * 0.5),
            LINE_Y_TOP,
        )

    # ── 7. Build LaneResult ────────────────────────────────────────────────────
    if left_line is None or right_line is None:
        return LaneResult(lane_detected=False, detector_mode="hough")

    lx_b, _, lx_t, _ = left_line
    rx_b, _, rx_t, _ = right_line

    # Update lane-width memory
    current_width = abs(rx_b - lx_b)
    if current_width > 20:  # ignore implausibly narrow readings
        _last_lane_width_px = current_width

    # Center offset — signed: positive = car is right of centre
    lane_centre_x = (lx_b + rx_b) / 2.0
    frame_centre_x = w / 2.0
    center_offset = lane_centre_x - frame_centre_x

    # Curvature direction — from how much the lines converge left/right at the top
    # If top-centre shifts left of bottom-centre → curving left, and vice versa
    top_centre = (lx_t + rx_t) / 2.0
    centre_shift = top_centre - lane_centre_x  # positive = shifts right toward top
    if abs(centre_shift) < 10:
        curv_dir = "straight"
        curv_r = STRAIGHT_R * 2
    elif centre_shift < 0:
        curv_dir = "left"
        curv_r = float(STRAIGHT_R * abs(frame_centre_x / max(abs(centre_shift), 1)))
    else:
        curv_dir = "right"
        curv_r = float(STRAIGHT_R * abs(frame_centre_x / max(abs(centre_shift), 1)))

    # Lane symmetry — how well left and right mirror the frame centre
    sym_score = 1.0 - abs(lane_centre_x - frame_centre_x) / (frame_centre_x + 1e-6)
    sym_score = float(max(0.0, min(1.0, sym_score)))

    # IoU proxy — use ratio of detected edge pixels in the ROI vs total ROI area
    lane_pixels = float(np.count_nonzero(masked))
    roi_pixels = float(np.count_nonzero(mask))
    iou_proxy = float(min(lane_pixels / max(roi_pixels * 0.1, 1.0), 1.0))

    # Populate left/right curves as sampled points from extrapolated lines
    # (scorer.py uses these for polynomial re-fitting when needed)
    n_pts = 20
    ys = np.linspace(LINE_Y_BOTTOM, LINE_Y_TOP, n_pts)
    lx_arr = np.linspace(lx_b, lx_t, n_pts)
    rx_arr = np.linspace(rx_b, rx_t, n_pts)

    return LaneResult(
        lane_detected=True,
        detector_mode="hough",
        center_offset_px=round(center_offset, 3),
        curvature_r=round(curv_r, 2),
        curvature_dir=curv_dir,
        lane_symmetry=round(sym_score, 4),
        iou_score=round(iou_proxy, 4),
        left_xs=lx_arr.tolist(),
        left_ys=ys.tolist(),
        right_xs=rx_arr.tolist(),
        right_ys=ys.tolist(),
    )


# ── Internal helpers ───────────────────────────────────────────────────────────


def _fit_line(
    segments: list[tuple], img_height: int
) -> Optional[tuple[int, int, int, int]]:
    """
    Fit a single representative line through a list of (x1,y1,x2,y2) segments.
    Returns (x_bottom, LINE_Y_BOTTOM, x_top, LINE_Y_TOP) or None.
    """
    if not segments:
        return None

    slopes: list[float] = []
    intercepts: list[float] = []
    for x1, y1, x2, y2 in segments:
        dx = x2 - x1
        if dx == 0:
            continue
        m = (y2 - y1) / dx
        b = y1 - m * x1
        slopes.append(m)
        intercepts.append(b)

    if not slopes:
        return None

    m_avg = float(np.mean(slopes))
    b_avg = float(np.mean(intercepts))

    # Avoid nearly-horizontal lines surviving slope filter
    if abs(m_avg) < 1e-6:
        m_avg = 1e-6

    x_bottom = int((LINE_Y_BOTTOM - b_avg) / m_avg)
    x_top = int((LINE_Y_TOP - b_avg) / m_avg)

    return (x_bottom, LINE_Y_BOTTOM, x_top, LINE_Y_TOP)
