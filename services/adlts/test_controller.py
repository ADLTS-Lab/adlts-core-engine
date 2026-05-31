"""
test_controller.py — Test state machine (IDLE → RUNNING → FINISHED).

Ported from adlts/backend/test_controller.py.
Orchestrates the driving test lifecycle: starts test, processes frames,
closes maneuvers on QR events, and computes final score.
"""

from __future__ import annotations

import json
import logging
import os
import time
from dataclasses import asdict
from enum import Enum
from typing import Optional

from config import (
    MANEUVER_SEQUENCE,
    MANEUVER_WEIGHTS,
    PASS_THRESHOLD,
    RESULTS_DIR,
    FrameScore,
    ManeuverResult,
    ManeuverScore,
    MotionResult,
    TestResult,
    TestState,
    TrafficLightResult,
)

logger = logging.getLogger(__name__)


class TestController:
    """
    Drives a complete driving test lifecycle.

    Parameters
    ----------
    scoring_engine : ScoringEngine
        The live scoring engine — TestController calls aggregate_maneuver()
        at each maneuver boundary.
    """

    def __init__(self, scoring_engine):
        self._se = scoring_engine
        self.state: TestState = TestState.IDLE
        self.test_id: str = ""
        self.candidate_id: str = ""
        self.started_at: float = 0.0
        self._maneuver_index: int = 0
        self._scores: list[ManeuverScore] = []
        self._recording_prefix: str = ""

        os.makedirs(RESULTS_DIR, exist_ok=True)

    # ── Public lifecycle API ──────────────────────────────────────────────────

    def start_test(self, test_id: str, candidate_id: str, recording_prefix: str = "") -> None:
        if self.state == TestState.RUNNING:
            raise RuntimeError("A test is already in progress.")

        self.test_id = test_id
        self.candidate_id = candidate_id
        self.started_at = time.monotonic()
        self._maneuver_index = 0
        self._scores = []
        self._recording_prefix = recording_prefix or f"recordings/{test_id}/full"
        self._se._reset_buffer()
        self.state = TestState.RUNNING

        logger.info("Test STARTED  test=%s candidate=%s first_maneuver=%s",
                     test_id, candidate_id, self.current_maneuver)

    def abort(self) -> None:
        if self.state == TestState.RUNNING:
            logger.warning("Test ABORTED at maneuver %d/%d (%s)",
                           self._maneuver_index, len(MANEUVER_SEQUENCE), self.current_maneuver)
        self.state = TestState.IDLE
        self._se._reset_buffer()

    # ── Per-frame update ─────────────────────────────────────────────────────

    def update(
        self,
        maneuver_result: Optional[ManeuverResult],
        frame_score: Optional[FrameScore],
        traffic_light_result: Optional[TrafficLightResult] = None,
        motion_result: Optional[MotionResult] = None,
    ) -> Optional[TestResult]:
        """
        Called every frame by the main loop.

        Returns None — test still in progress.
        Returns TestResult — the single moment the test finishes.
        """
        if self.state != TestState.RUNNING:
            return None

        detected_name = maneuver_result.maneuver_name if maneuver_result else None

        if detected_name is None:
            return None  # normal mid-maneuver frame

        # ── Maneuver boundary detected via QR code ─────────────────────────

        closing_name = self.current_maneuver
        if closing_name is None:
            return None

        # Validate QR matches the current maneuver (ignore stale QR codes)
        expected_qr = f"maneuver:{closing_name}"
        if detected_name != expected_qr:
            logger.warning("QR '%s' does not match current maneuver '%s' — ignoring",
                           detected_name, closing_name)
            return None

        # Close the maneuver that just ended
        ms = self._se.aggregate_maneuver(closing_name)
        self._scores.append(ms)
        logger.info("Maneuver '%s' closed  final=%.1f  frames=%d",
                    ms.name, ms.final_score, ms.frame_count)

        # Check for stop sentinel or all done
        if detected_name == "stop" or self._all_done():
            return self._finish_test()

        # Advance to the next maneuver
        self._maneuver_index += 1
        if self._maneuver_index < len(MANEUVER_SEQUENCE):
            logger.info("Advancing to maneuver %d/%d — '%s'",
                        self._maneuver_index + 1, len(MANEUVER_SEQUENCE), self.current_maneuver)
        else:
            return self._finish_test()

        return None

    # ── Properties ─────────────────────────────────────────────────────────

    @property
    def current_maneuver(self) -> Optional[str]:
        if self._maneuver_index < len(MANEUVER_SEQUENCE):
            return MANEUVER_SEQUENCE[self._maneuver_index]
        return None

    @property
    def scores_so_far(self) -> list[ManeuverScore]:
        return list(self._scores)

    @property
    def progress(self) -> dict:
        return {
            "state": self.state.value,
            "test_id": self.test_id,
            "candidate_id": self.candidate_id,
            "current_maneuver": self.current_maneuver,
            "maneuver_index": self._maneuver_index,
            "total_maneuvers": len(MANEUVER_SEQUENCE),
            "scores_so_far": [asdict(s) for s in self._scores],
            "frames_in_buffer": len(self._se._frame_scores),
        }

    @property
    def result_json(self) -> Optional[dict]:
        """Return the final result dict if finished, else None."""
        if self.state != TestState.FINISHED:
            return None
        return self._build_result_dict()

    # ── Private ──────────────────────────────────────────────────────────────

    def _all_done(self) -> bool:
        return len(self._scores) >= len(MANEUVER_SEQUENCE)

    def _finish_test(self) -> TestResult:
        finished_at = time.monotonic()
        total_score = self._weighted_mean()
        passed = total_score >= PASS_THRESHOLD

        result = TestResult(
            test_id=self.test_id,
            candidate_id=self.candidate_id,
            started_at=self.started_at,
            finished_at=finished_at,
            maneuvers=list(self._scores),
            total_score=round(total_score, 2),
            passed=passed,
            recording_prefix=self._recording_prefix,
        )

        self._persist(result)
        self.state = TestState.FINISHED

        logger.info("Test FINISHED  test=%s  candidate=%s  total=%.1f  passed=%s",
                     self.test_id, self.candidate_id, total_score, passed)
        return result

    def _build_result_dict(self) -> dict:
        total_score = self._weighted_mean()
        passed = total_score >= PASS_THRESHOLD
        return {
            "test_id": self.test_id,
            "candidate_id": self.candidate_id,
            "total_score": round(total_score, 2),
            "passed": passed,
            "pass_threshold": PASS_THRESHOLD,
            "recording_prefix": self._recording_prefix,
            "maneuvers": [asdict(ms) for ms in self._scores],
        }

    def _weighted_mean(self) -> float:
        if not self._scores:
            return 0.0
        weighted_sum = 0.0
        weight_total = 0.0
        for ms in self._scores:
            w = MANEUVER_WEIGHTS.get(ms.name, 1.0)
            weighted_sum += ms.final_score * w
            weight_total += w
        return weighted_sum / weight_total if weight_total > 0 else 0.0

    def _persist(self, result: TestResult) -> None:
        ts_ms = int(result.finished_at * 1000)
        fname = f"{result.test_id}_{ts_ms}.json"
        fpath = os.path.join(RESULTS_DIR, fname)
        tmp = fpath + ".tmp"

        payload = {
            "test_id": result.test_id,
            "candidate_id": result.candidate_id,
            "started_at": result.started_at,
            "finished_at": result.finished_at,
            "total_score": result.total_score,
            "passed": result.passed,
            "pass_threshold": PASS_THRESHOLD,
            "recording_prefix": result.recording_prefix,
            "maneuvers": [asdict(ms) for ms in result.maneuvers],
        }

        try:
            with open(tmp, "w") as f:
                json.dump(payload, f, indent=2)
            os.replace(tmp, fpath)
            logger.info("Result persisted → %s", fpath)
        except OSError as exc:
            logger.error("Failed to persist result: %s", exc)