"""
lane_detector.py — Classical OpenCV lane detection for white lines.

Ported from adlts/backend/lane_detector.py.
"""

from __future__ import annotations

from typing import Optional

import cv2
import numpy as np

from config import LaneResult, IMAGE_WIDTH, IMAGE_HEIGHT


class LaneDetector:
    """Classical OpenCV lane detection with calibration support."""

    CANNY_LOW = 50
    CANNY_HIGH = 150
    HOUGH_RHO = 1
    HOUGH_THETA = np.pi / 180
    HOUGH_THRESHOLD = 20
    HOUGH_MIN_LEN = 20
    HOUGH_MAX_GAP = 10

    lane_width_px: float | None = None
    pixels_per_cm: float | None = None

    def calibrate(self, left_line: tuple, right_line: tuple, lane_width_cm: float):
        lx = left_line[0]
        rx = right_line[0]
        self.lane_width_px = abs(rx - lx)
        self.pixels_per_cm = self.lane_width_px / lane_width_cm

    def detect(self, frame: np.ndarray) -> LaneResult:
        h, w = frame.shape[:2]

        gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
        blurred = cv2.GaussianBlur(gray, (5, 5), 0)
        edges = cv2.Canny(blurred, self.CANNY_LOW, self.CANNY_HIGH)

        roi_vertices = np.array([[
            (0, h),
            (int(w * 0.1), int(h * 0.55)),
            (int(w * 0.9), int(h * 0.55)),
            (w, h),
        ]], dtype=np.int32)
        mask = np.zeros_like(edges)
        cv2.fillPoly(mask, roi_vertices, 255)
        masked = cv2.bitwise_and(edges, mask)

        lines = cv2.HoughLinesP(
            masked,
            self.HOUGH_RHO, self.HOUGH_THETA, self.HOUGH_THRESHOLD,
            minLineLength=self.HOUGH_MIN_LEN,
            maxLineGap=self.HOUGH_MAX_GAP,
        )

        left_lines, right_lines = [], []
        if lines is not None:
            for line in lines:
                x1, y1, x2, y2 = line[0]
                if x2 == x1:
                    continue
                slope = (y2 - y1) / (x2 - x1)
                if slope < -0.3:
                    left_lines.append(line[0])
                elif slope > 0.3:
                    right_lines.append(line[0])

        left_line = self._average_lines(left_lines, h) if left_lines else None
        right_line = self._average_lines(right_lines, h) if right_lines else None

        # Fallback when one side is missing
        if left_line is None and right_line is not None and self.lane_width_px:
            rx1 = right_line[0]
            left_line = (
                int(rx1 - self.lane_width_px), h,
                int(rx1 - self.lane_width_px * 0.5), int(h * 0.6)
            )
        elif right_line is None and left_line is not None and self.lane_width_px:
            lx1 = left_line[0]
            right_line = (
                int(lx1 + self.lane_width_px), h,
                int(lx1 + self.lane_width_px * 0.5), int(h * 0.6)
            )

        centre_x = None
        if left_line and right_line:
            centre_x = (left_line[0] + right_line[0]) / 2.0

        overlay = frame.copy()
        if left_line:
            cv2.line(overlay, left_line[:2], left_line[2:], (0, 255, 0), 2)
        if right_line:
            cv2.line(overlay, right_line[:2], right_line[2:], (0, 255, 0), 2)
        if centre_x is not None:
            cv2.circle(overlay, (int(centre_x), h - 10), 5, (0, 0, 255), -1)

        return LaneResult(
            left_line=left_line,
            right_line=right_line,
            centre_x=centre_x,
            raw_frame=overlay,
        )

    @staticmethod
    def _average_lines(lines: list, img_height: int) -> Optional[tuple]:
        if not lines:
            return None
        slopes, intercepts = [], []
        for x1, y1, x2, y2 in lines:
            if x2 == x1:
                continue
            slope = (y2 - y1) / (x2 - x1)
            intercept = y1 - slope * x1
            slopes.append(slope)
            intercepts.append(intercept)
        if not slopes:
            return None
        avg_slope = float(np.mean(slopes))
        avg_intercept = float(np.mean(intercepts))
        y1 = img_height
        y2 = int(img_height * 0.6)
        if abs(avg_slope) < 1e-6:
            avg_slope = 1e-6 if avg_slope >= 0 else -1e-6
        x1 = int((y1 - avg_intercept) / avg_slope)
        x2 = int((y2 - avg_intercept) / avg_slope)
        return (x1, y1, x2, y2)