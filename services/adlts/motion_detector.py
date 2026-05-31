"""
motion_detector.py — Frame-difference movement detector.

Ported from adlts/backend/motion_detector.py and
adlts-core-engine/services/lane-detector/motion_detector.py.
"""

from __future__ import annotations

from typing import Optional

import cv2
import numpy as np

from config import MotionResult


class MotionDetector:
    """Simple pixel-change based movement detector using frame differencing."""

    def __init__(self, movement_threshold_ratio: float = 0.01, diff_threshold: int = 25):
        self.movement_threshold_ratio = movement_threshold_ratio
        self.diff_threshold = diff_threshold
        self._prev_gray: np.ndarray | None = None

    def detect(self, frame: np.ndarray) -> MotionResult:
        h, w = frame.shape[:2]
        # Default ROI = lower half
        y1 = h // 2
        roi_frame = frame[y1:h, 0:w]
        roi_bbox = (0, y1, w, h)

        gray = cv2.cvtColor(roi_frame, cv2.COLOR_BGR2GRAY)
        gray = cv2.GaussianBlur(gray, (5, 5), 0)

        if self._prev_gray is None:
            self._prev_gray = gray
            return MotionResult(is_moving=False, pixel_change_ratio=0.0,
                                changed_pixels=0, total_pixels=gray.size, roi=roi_bbox)

        diff = cv2.absdiff(self._prev_gray, gray)
        _, binary = cv2.threshold(diff, self.diff_threshold, 255, cv2.THRESH_BINARY)
        changed_pixels = int(np.count_nonzero(binary))
        total_pixels = int(binary.size)
        ratio = (changed_pixels / total_pixels) if total_pixels else 0.0
        is_moving = ratio >= self.movement_threshold_ratio
        self._prev_gray = gray

        return MotionResult(is_moving=is_moving, pixel_change_ratio=ratio,
                            changed_pixels=changed_pixels, total_pixels=total_pixels, roi=roi_bbox)

    def reset(self):
        self._prev_gray = None