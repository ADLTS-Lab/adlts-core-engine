"""
Classical lane detection pipeline for controlled toy-car track.
Primary mode: works on white/yellow tape lanes in controlled lighting.
"""

from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from typing import Optional

import cv2
import numpy as np


@dataclass
class LaneResult:
    lane_detected: bool = False
    detector_mode: str = "classical"
    center_offset_px: float = 0.0
    curvature_r: float = 0.0
    curvature_dir: str = "none"  # straight | left | right | none
    lane_symmetry: float = 0.0
    iou_score: float = 0.0
    left_xs: list = field(default_factory=list)
    left_ys: list = field(default_factory=list)
    right_xs: list = field(default_factory=list)
    right_ys: list = field(default_factory=list)


# ── Calibration constants ──────────────────────────────────────────────────────
# TODO: calibrate these values for your specific track and camera mounting.
# Run calibrate_perspective.py once with a chessboard or straight-lane image.
# For now, defaults work for a typical 640×480 wide-angle mounting.

FRAME_W, FRAME_H = 640, 480
ROI_TOP_PCT = 0.40  # ignore top 40 % of frame (sky / ceiling)
WARP_SRC = np.float32(
    [  # trapezoid in original image
        [FRAME_W * 0.10, FRAME_H * 0.95],
        [FRAME_W * 0.90, FRAME_H * 0.95],
        [FRAME_W * 0.60, FRAME_H * ROI_TOP_PCT],
        [FRAME_W * 0.40, FRAME_H * ROI_TOP_PCT],
    ]
)
WARP_DST = np.float32(
    [  # rectangle in bird's-eye view
        [FRAME_W * 0.10, FRAME_H],
        [FRAME_W * 0.90, FRAME_H],
        [FRAME_W * 0.90, 0],
        [FRAME_W * 0.10, 0],
    ]
)

CALIB_FILE = "models/perspective_M.json"

def load_perspective_M():
    if os.path.exists(CALIB_FILE):
        try:
            with open(CALIB_FILE, 'r') as f:
                data = json.load(f)
                return np.array(data, dtype=np.float32)
        except Exception as e:
            print(f"Failed to load {CALIB_FILE}: {e}")
    return cv2.getPerspectiveTransform(WARP_SRC, WARP_DST)

M = load_perspective_M()


def calibrate_perspective_from_chessboard(image_paths, nx=9, ny=6):
    """Calibrate camera using chessboard images and compute perspective warp M.

    image_paths: iterable of file paths to chessboard images taken from the camera.
    nx, ny: internal corners in the chessboard pattern (columns, rows).
    Returns (ret, camera_matrix, dist_coefs, M) where M is the perspective transform.
    """
    objp = np.zeros((ny * nx, 3), np.float32)
    objp[:, :2] = np.mgrid[0:nx, 0:ny].T.reshape(-1, 2)

    objpoints = []
    imgpoints = []
    found_image = None

    for p in image_paths:
        img = cv2.imread(p)
        if img is None:
            continue
        gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
        ret, corners = cv2.findChessboardCorners(gray, (nx, ny), None)
        if not ret:
            continue
        # refine corners
        corners2 = cv2.cornerSubPix(gray, corners, (11, 11), (-1, -1),
                                    (cv2.TermCriteria_EPS + cv2.TermCriteria_MAX_ITER, 30, 0.001))
        objpoints.append(objp)
        imgpoints.append(corners2)
        found_image = img

    if len(objpoints) == 0:
        return False, None, None, None

    # calibrate
    ret, mtx, dist, rvecs, tvecs = cv2.calibrateCamera(objpoints, imgpoints, (FRAME_W, FRAME_H), None, None)

    # Use the first successful image to compute a perspective transform mapping the
    # chessboard quad to the desired bird's-eye rectangle. We'll take the 4 outer
    # corners from the detected chessboard corners ordering.
    if found_image is None:
        return ret, mtx, dist, None

    # Undistort then detect corners again to get reliable image coordinates
    und = cv2.undistort(found_image, mtx, dist, None, mtx)
    grayu = cv2.cvtColor(und, cv2.COLOR_BGR2GRAY)
    ok, corners = cv2.findChessboardCorners(grayu, (nx, ny), None)
    if not ok:
        return ret, mtx, dist, None
    corners = corners.reshape(-1, 2)

    # select outer four corners in order: bottom-left, bottom-right, top-right, top-left
    tl = corners[0]
    tr = corners[nx - 1]
    br = corners[-1]
    bl = corners[-nx]
    src = np.float32([bl, br, tr, tl])

    dst = np.float32([
        [FRAME_W * 0.10, FRAME_H],
        [FRAME_W * 0.90, FRAME_H],
        [FRAME_W * 0.90, 0],
        [FRAME_W * 0.10, 0],
    ])

    M_calib = cv2.getPerspectiveTransform(src, dst)
    return ret, mtx, dist, M_calib


if __name__ == "__main__":
    import glob
    import sys

    # Usage: python lane_classical.py /path/to/chessboard_images/*.jpg
    if len(sys.argv) < 2:
        print("Provide glob path to chessboard images, e.g. python lane_classical.py imgs/*.jpg")
        sys.exit(1)
    paths = glob.glob(sys.argv[1])
    ok, mtx, dist, M_calib = calibrate_perspective_from_chessboard(paths)
    if not ok or M_calib is None:
        print("Calibration failed or no chessboard corners found")
        sys.exit(2)
        
    os.makedirs(os.path.dirname(CALIB_FILE), exist_ok=True)
    with open(CALIB_FILE, 'w') as f:
        json.dump(M_calib.tolist(), f)
        
    print(f"Calibration successful. Perspective matrix M saved to {CALIB_FILE}:")
    print(M_calib)

NWINDOWS = 9
MARGIN = 60  # sliding window half-width px
MINPIX = 30  # min pixels to re-centre window
STRAIGHT_R = 400.0  # radius above which direction is "straight"


def _hsl_mask(frame_bgr: np.ndarray) -> np.ndarray:
    """Binary mask isolating white and yellow tape."""
    hls = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2HLS)
    # White: high lightness regardless of hue
    white_mask = cv2.inRange(hls, (0, 180, 0), (255, 255, 255))
    # Yellow: hue 15-35 in OpenCV HLS (0-180 scale)
    yellow_mask = cv2.inRange(hls, (15, 100, 100), (35, 255, 255))
    return cv2.bitwise_or(white_mask, yellow_mask)


def _sliding_window(binary_warped: np.ndarray):
    """
    Returns (left_xs, left_ys, right_xs, right_ys) lane pixel coordinates
    in the bird's-eye image.
    """
    h, w = binary_warped.shape
    hist = np.sum(binary_warped[h // 2 :, :], axis=0)
    mid = w // 2
    left_x = int(np.argmax(hist[:mid]))
    right_x = int(np.argmax(hist[mid:]) + mid)

    win_h = h // NWINDOWS
    nonzero = binary_warped.nonzero()
    nzy, nzx = nonzero[0], nonzero[1]

    left_inds, right_inds = [], []
    for w_idx in range(NWINDOWS):
        y_lo = h - (w_idx + 1) * win_h
        y_hi = h - w_idx * win_h
        for x_cur, inds in [(left_x, left_inds), (right_x, right_inds)]:
            good = (
                (nzy >= y_lo)
                & (nzy < y_hi)
                & (nzx >= x_cur - MARGIN)
                & (nzx < x_cur + MARGIN)
            ).nonzero()[0]
            inds.extend(good)
            if len(good) >= MINPIX:
                x_cur = int(np.mean(nzx[good]))  # noqa: assigned but not carried between iterations

        # recompute centres using the found inds
        if len(left_inds) >= MINPIX:
            left_x = int(np.mean(nzx[left_inds[-MINPIX:]]))
        if len(right_inds) >= MINPIX:
            right_x = int(np.mean(nzx[right_inds[-MINPIX:]]))

    lx = nzx[left_inds] if left_inds else np.array([], dtype=int)
    ly = nzy[left_inds] if left_inds else np.array([], dtype=int)
    rx = nzx[right_inds] if right_inds else np.array([], dtype=int)
    ry = nzy[right_inds] if right_inds else np.array([], dtype=int)
    return lx, ly, rx, ry


def detect(frame_bgr: np.ndarray) -> LaneResult:
    """Run the full classical pipeline on one BGR frame."""
    # Resize if needed
    if frame_bgr.shape[1] != FRAME_W or frame_bgr.shape[0] != FRAME_H:
        frame_bgr = cv2.resize(frame_bgr, (FRAME_W, FRAME_H))

    mask = _hsl_mask(frame_bgr)
    edges = cv2.Canny(mask, 50, 150)
    warped = cv2.warpPerspective(edges, M, (FRAME_W, FRAME_H))

    lx, ly, rx, ry = _sliding_window(warped)
    MIN_POINTS = 5

    if len(lx) < MIN_POINTS and len(rx) < MIN_POINTS:
        return LaneResult(lane_detected=False)

    result = LaneResult(lane_detected=True, detector_mode="classical")

    # Fit polynomials
    h = FRAME_H
    ploty = np.linspace(0, h - 1, h)

    if len(lx) >= MIN_POINTS:
        left_fit = np.polyfit(ly, lx, 2)
        result.left_xs = np.polyval(left_fit, ploty).tolist()
        result.left_ys = ploty.tolist()
    if len(rx) >= MIN_POINTS:
        right_fit = np.polyfit(ry, rx, 2)
        result.right_xs = np.polyval(right_fit, ploty).tolist()
        result.right_ys = ploty.tolist()

    # Center offset (at bottom of frame)
    left_x_bottom = result.left_xs[-1] if result.left_xs else (FRAME_W * 0.20)
    right_x_bottom = result.right_xs[-1] if result.right_xs else (FRAME_W * 0.80)
    result.center_offset_px = (left_x_bottom + right_x_bottom) / 2.0 - FRAME_W / 2.0

    # Curvature radius (average of the two fits at mid-height)
    y_eval = h // 2
    radii = []
    for fit_pts in [result.left_xs, result.right_xs]:
        if len(fit_pts) > 2:
            # Use finite differences on the polynomial curve
            ys = np.array(ploty)
            xs = np.array(fit_pts)
            dy = np.gradient(ys)
            dx = np.gradient(xs)
            d2x = np.gradient(dx)
            idx = min(y_eval, len(xs) - 1)
            denom = abs(d2x[idx])
            if denom > 1e-6:
                radii.append((1 + (dx[idx] / dy[idx]) ** 2) ** 1.5 / denom)
    if radii:
        result.curvature_r = float(np.mean(radii))

    # Curvature direction
    if abs(result.curvature_r) > STRAIGHT_R:
        result.curvature_dir = "straight"
    elif result.curvature_r < 0:
        result.curvature_dir = "left"
    else:
        result.curvature_dir = "right"

    # Lane symmetry: how well left and right mirror each other
    if result.left_xs and result.right_xs:
        lx_arr = np.array(result.left_xs)
        rx_arr = np.array(result.right_xs)
        sym = 1.0 - float(np.mean(np.abs(lx_arr + rx_arr - FRAME_W) / FRAME_W))
        result.lane_symmetry = float(np.clip(sym, 0.0, 1.0))

    # IoU proxy: fraction of bottom-half pixels classified as lane
    lane_area = np.count_nonzero(warped[h // 2 :, :])
    total_area = (h // 2) * FRAME_W
    result.iou_score = float(np.clip(lane_area / max(total_area, 1), 0.0, 1.0))

    return result
