"""Motion direction detector using Farneback dense optical flow."""

import cv2
import numpy as np


class MotionDirectionDetector:
    STOPPED_THRESHOLD = 0.5  # mean flow magnitude below this → "stopped"
    FORWARD_BIAS = 0.3  # net downward flow → "forward" (camera facing forward)

    def __init__(self):
        self._prev_gray = None

    def detect(self, frame_bgr) -> str:
        """Returns 'forward' | 'backward' | 'stopped'."""
        gray = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2GRAY)
        gray = cv2.resize(gray, (160, 120))  # small for speed

        if self._prev_gray is None:
            self._prev_gray = gray
            return "stopped"

        flow = cv2.calcOpticalFlowFarneback(
            self._prev_gray,
            gray,
            None,
            pyr_scale=0.5,
            levels=3,
            winsize=15,
            iterations=3,
            poly_n=5,
            poly_sigma=1.2,
            flags=0,
        )
        self._prev_gray = gray

        magnitude = np.sqrt(flow[..., 0] ** 2 + flow[..., 1] ** 2)
        mean_mag = float(np.mean(magnitude))

        if mean_mag < self.STOPPED_THRESHOLD:
            return "stopped"

        # Vertical (y) flow: positive = downward on screen = forward motion
        mean_vy = float(np.mean(flow[..., 1]))
        return "forward" if mean_vy > self.FORWARD_BIAS else "backward"

    def reset(self):
        self._prev_gray = None
