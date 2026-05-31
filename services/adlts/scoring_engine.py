"""
scoring_engine.py — Per-frame lateral scoring & traffic-light violation tracking.

Ported from adlts/backend/scoring_engine.py.
"""

from __future__ import annotations

import logging
from typing import Optional

import numpy as np

from config import (
    IMAGE_WIDTH,
    LANE_WIDTH_CM,
    FrameScore,
    ManeuverScore,
    MotionResult,
    TrafficLightResult,
    TrafficLightState,
)

logger = logging.getLogger(__name__)

VIOLATION_PENALTY = 20.0
VIOLATION_MIN_FRAMES = 5
TRIM_RATIO = 0.10
TRIM_MIN_FRAMES = 10
MAX_ERROR_CM = LANE_WIDTH_CM / 2.0


class ScoringEngine:
    def __init__(self, pixels_per_cm_provider):
        """
        Parameters
        ----------
        pixels_per_cm_provider : object
            Must expose a ``pixels_per_cm`` property (Optional[float]).
            Typically a LaneDetector or a simple adapter.
        """
        self._provider = pixels_per_cm_provider
        self._frame_scores: list[float] = []
        self._penalty: float = 0.0
        self._violations: int = 0
        self._red_moving_streak: int = 0

    @property
    def is_calibrated(self) -> bool:
        return self._provider.pixels_per_cm is not None

    @property
    def pixels_per_cm(self) -> Optional[float]:
        return self._provider.pixels_per_cm

    def score_frame(
        self,
        left_line: Optional[tuple],
        right_line: Optional[tuple],
    ) -> Optional[FrameScore]:
        if not self.is_calibrated:
            return None
        if left_line is None or right_line is None:
            return FrameScore(score=0.0, error_cm=999.0, error_px=999.0)
        lane_centre_x = (left_line[0] + right_line[0]) / 2.0
        car_centre_x = IMAGE_WIDTH / 2.0
        error_px = abs(car_centre_x - lane_centre_x)
        error_cm = error_px / self.pixels_per_cm
        score = 100.0 * max(0.0, 1.0 - error_cm / MAX_ERROR_CM)
        return FrameScore(score=score, error_cm=error_cm, error_px=error_px)

    def record_traffic_event(self, tl_result: Optional[TrafficLightResult],
                             motion_result: Optional[MotionResult]) -> bool:
        if tl_result is None or motion_result is None:
            self._red_moving_streak = 0
            return False
        if not self.is_calibrated:
            return False
        if (tl_result.state == TrafficLightState.RED and motion_result.is_moving):
            self._red_moving_streak += 1
            if self._red_moving_streak % VIOLATION_MIN_FRAMES == 0:
                self._penalty += VIOLATION_PENALTY
                self._violations += 1
                return True
        else:
            self._red_moving_streak = 0
        return False

    def add_frame_score(self, fs: FrameScore) -> None:
        self._frame_scores.append(fs.score)

    def aggregate_maneuver(self, name: str) -> ManeuverScore:
        scores = list(self._frame_scores)
        if not scores:
            raw = 0.0
        elif len(scores) < TRIM_MIN_FRAMES:
            raw = float(np.mean(scores))
        else:
            trim = max(1, int(len(scores) * TRIM_RATIO))
            trimmed = sorted(scores)[trim: len(scores) - trim]
            raw = float(np.mean(trimmed)) if trimmed else float(np.mean(scores))
        final = float(np.clip(raw - self._penalty, 0.0, 100.0))
        result = ManeuverScore(
            name=name,
            raw_score=round(raw, 2),
            penalty=round(self._penalty, 2),
            final_score=round(final, 2),
            frame_count=len(scores),
            violations=self._violations,
        )
        self._reset_buffer()
        return result

    def _reset_buffer(self) -> None:
        self._frame_scores = []
        self._penalty = 0.0
        self._violations = 0
        self._red_moving_streak = 0