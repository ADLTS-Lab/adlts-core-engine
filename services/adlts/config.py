"""
config.py — Data contracts and constants for the ADLTS test service.

Combines contracts from:
  - adlts/backend/config.py (prototype)
  - adlts-core-engine/services/lane-detector/config.py
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import Optional
from enum import Enum

# ─── Camera / scoring constants ───────────────────────────────────────────────
IMAGE_WIDTH = 320
IMAGE_HEIGHT = 240
LANE_WIDTH_CM = 30.0

# ─── Manoeuvre sequence (matches migration 006 seed data) ─────────────────────
MANEUVER_SEQUENCE = [
    "straight_1",
    "left_curve_1",
    "left_curve_2",
    "straight_2",
    "parallel_parking",
]

# Relative weights per maneuver
MANEUVER_WEIGHTS: dict = {
    "straight_1":        1.0,
    "left_curve_1":      1.5,
    "left_curve_2":      1.5,
    "straight_2":        1.0,
    "parallel_parking":  2.0,
}

PASS_THRESHOLD = float(os.getenv("PASS_THRESHOLD", "60.0"))
RESULTS_DIR = os.getenv("RESULTS_DIR", "test_results")

# MinIO env vars (passed at /start_test or from env)
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "minio:9000")
MINIO_ACCESS = os.getenv("MINIO_ACCESS", "minioadmin")
MINIO_SECRET = os.getenv("MINIO_SECRET", "minioadmin")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "recordings")
MINIO_SECURE = os.getenv("MINIO_SECURE", "false").lower() == "true"


# ─── Enums ─────────────────────────────────────────────────────────────────────

class TrafficLightState(str, Enum):
    RED = "red"
    GREEN = "green"
    NONE = "none"


class TestState(str, Enum):
    IDLE = "idle"
    RUNNING = "running"
    FINISHED = "finished"
    FAILED = "failed"


# ─── Data contracts ───────────────────────────────────────────────────────────

@dataclass
class LaneResult:
    """Output of LaneDetector.detect()."""
    left_line: Optional[tuple] = None
    right_line: Optional[tuple] = None
    centre_x: Optional[float] = None
    raw_frame: Optional[object] = None  # np.ndarray with overlay


@dataclass
class TrafficLightResult:
    state: TrafficLightState
    confidence: float
    bbox: Optional[tuple] = None


@dataclass
class MotionResult:
    is_moving: bool
    pixel_change_ratio: float
    changed_pixels: int
    total_pixels: int
    roi: Optional[tuple] = None


@dataclass
class ManeuverResult:
    maneuver_name: Optional[str] = None
    confidence: float = 0.0
    payload: Optional[str] = None
    bbox: Optional[tuple] = None


@dataclass
class FrameScore:
    score: float = 0.0      # 0–100
    error_cm: float = 0.0
    error_px: float = 0.0


@dataclass
class ManeuverScore:
    name: str = ""
    raw_score: float = 0.0
    penalty: float = 0.0
    final_score: float = 0.0
    frame_count: int = 0
    violations: int = 0


@dataclass
class TestResult:
    """Final result from one driving test."""
    test_id: str = ""
    candidate_id: str = ""
    started_at: float = 0.0
    finished_at: float = 0.0
    maneuvers: list = field(default_factory=list)
    total_score: float = 0.0
    passed: bool = False
    recording_prefix: str = ""


# ─── Pydantic models for HTTP requests/responses ─────────────────────────────
# (used by FastAPI endpoints in main.py)