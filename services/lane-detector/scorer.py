"""
ADLTS Maneuver Scorer — Phase 8
Evaluators for all 8 maneuver types (§7.1–§7.7).
"""

from __future__ import annotations

import statistics
from typing import Optional

from pydantic import BaseModel

# ── Constants (§7.1) ──────────────────────────────────────────────────────────

MAX_LANE_HALF_WIDTH = 80
OFFSET_MINOR = 20
OFFSET_MAJOR = 35
OFFSET_CRITICAL = 60
STRAIGHT_RADIUS = 400
MIN_ARC_FRAMES = 10
STOP_MIN_FRAMES = 15
PENALTY = {"minor": 3, "major": 8, "critical": 20}


# ── Pydantic Models (§6.2) ────────────────────────────────────────────────────


class FrameData(BaseModel):
    frame_seq_no: int
    lane_detected: bool
    center_offset_px: float
    curvature_r: float
    curvature_dir: str
    lane_symmetry: float
    motion_dir: str
    iou_score: float
    maneuver_phase: str = ""


class ScoreManeuverRequest(BaseModel):
    maneuver_type: str
    maneuver_config_id: str
    session_id: str
    tolerance_px: int = 20
    pass_threshold: float = 70.0
    frames: list[FrameData]


class ScoreManeuverResponse(BaseModel):
    maneuver_type: str
    session_id: str
    score: float
    passed: bool
    critical_fail: bool
    dimension_scores: dict | None
    phase_scores: dict
    weakest_phase: str
    mean_center_offset_px: float
    offset_variance_px: float
    direction_accuracy: float
    events: list[dict]
    event_count_by_severity: dict


# ── Internal helpers ──────────────────────────────────────────────────────────


def _safe_stdev(values: list[float]) -> float:
    """Standard deviation; returns 0.0 for fewer than 2 values (Rule 6)."""
    if len(values) < 2:
        return 0.0
    return statistics.stdev(values)


def _safe_mean(values: list[float]) -> float:
    if not values:
        return 0.0
    return statistics.mean(values)


def _frame_offset_event(frame: FrameData, tolerance_px: int) -> Optional[dict]:
    """
    Return an event dict if the frame has an offset violation, else None.
    Thresholds (§7.2):
      > OFFSET_CRITICAL → critical
      > OFFSET_MAJOR    → major
      > max(tolerance_px, OFFSET_MINOR) → minor
    """
    offset = abs(frame.center_offset_px)
    if offset > OFFSET_CRITICAL:
        return {
            "frame_seq_no": frame.frame_seq_no,
            "severity": "critical",
            "type": "offset_violation",
            "description": f"Critical offset {offset:.1f}px exceeds {OFFSET_CRITICAL}px limit",
        }
    if offset > OFFSET_MAJOR:
        return {
            "frame_seq_no": frame.frame_seq_no,
            "severity": "major",
            "type": "offset_violation",
            "description": f"Major offset {offset:.1f}px exceeds {OFFSET_MAJOR}px limit",
        }
    if offset > max(tolerance_px, OFFSET_MINOR):
        return {
            "frame_seq_no": frame.frame_seq_no,
            "severity": "minor",
            "type": "offset_violation",
            "description": f"Minor offset {offset:.1f}px exceeds tolerance {tolerance_px}px",
        }
    return None


def _lane_loss_event(frame: FrameData) -> Optional[dict]:
    """Critical event when lane is completely lost on a frame."""
    if not frame.lane_detected:
        return {
            "frame_seq_no": frame.frame_seq_no,
            "severity": "critical",
            "type": "lane_loss",
            "description": "Lane not detected in frame",
        }
    return None


def _direction_event(frame: FrameData, expected_dir: str, phase: str) -> Optional[dict]:
    """Major event when curvature direction doesn't match expected in a scored phase."""
    actual = frame.curvature_dir
    if actual in ("none", ""):
        return None
    if actual != expected_dir:
        return {
            "frame_seq_no": frame.frame_seq_no,
            "severity": "major",
            "type": "direction_mismatch",
            "description": (
                f"Expected curvature '{expected_dir}', "
                f"got '{actual}' in phase '{phase}'"
            ),
        }
    return None


def _motion_event(frame: FrameData, expected_motion: str) -> Optional[dict]:
    """Minor event when motion direction doesn't match expected (ignores stopped)."""
    if frame.motion_dir == "stopped":
        return None
    if frame.motion_dir != expected_motion:
        return {
            "frame_seq_no": frame.frame_seq_no,
            "severity": "minor",
            "type": "motion_mismatch",
            "description": (
                f"Expected motion '{expected_motion}', got '{frame.motion_dir}'"
            ),
        }
    return None


def _compute_lane_keeping_score(
    frames: list[FrameData], tolerance_px: int
) -> tuple[float, list[dict]]:
    """
    §7.3 Lane-keeping sub-score (0–100) with per-frame offset and lane-loss events.

    Formula:
        penalty_rate = Σ PENALTY[sev] / (N × PENALTY["critical"])
        score        = max(0, (1 − penalty_rate) × 100)

    Worst case (every frame critical) → score = 0.
    Every frame perfect              → score = 100.
    """
    if not frames:
        return 0.0, []

    events: list[dict] = []
    total_penalty = 0.0

    for f in frames:
        ev = _frame_offset_event(f, tolerance_px)
        if ev:
            events.append(ev)
            total_penalty += PENALTY[ev["severity"]]

        ll = _lane_loss_event(f)
        if ll:
            events.append(ll)
            total_penalty += PENALTY["critical"]

    max_penalty = len(frames) * PENALTY["critical"]
    penalty_rate = min(1.0, total_penalty / max(max_penalty, 1))
    score = round((1.0 - penalty_rate) * 100.0, 2)
    return score, events


def _compute_direction_accuracy(frames: list[FrameData], expected_dir: str) -> float:
    """
    §7.4 Fraction of frames where curvature_dir matches expected_dir.
    Frames with curvature_dir == 'none' or '' are excluded (no measurement).
    """
    relevant = [f for f in frames if f.curvature_dir not in ("none", "")]
    if not relevant:
        return 1.0
    correct = sum(1 for f in relevant if f.curvature_dir == expected_dir)
    return round(correct / len(relevant), 4)


def _compute_motion_accuracy(frames: list[FrameData], expected_motion: str) -> float:
    """
    Fraction of non-stopped frames where motion_dir matches expected.
    """
    relevant = [f for f in frames if f.motion_dir != "stopped"]
    if not relevant:
        return 1.0
    correct = sum(1 for f in relevant if f.motion_dir == expected_motion)
    return round(correct / len(relevant), 4)


def _phase_score_map(frames: list[FrameData], tolerance_px: int) -> dict[str, float]:
    """Group frames by maneuver_phase and compute per-phase lane-keeping scores."""
    phases: dict[str, list[FrameData]] = {}
    for f in frames:
        phases.setdefault(f.maneuver_phase or "body", []).append(f)
    result: dict[str, float] = {}
    for phase_name, phase_frames in phases.items():
        lane_score, _ = _compute_lane_keeping_score(phase_frames, tolerance_px)
        result[phase_name] = lane_score
    return result


def _event_count_by_severity(events: list[dict]) -> dict:
    counts: dict[str, int] = {"minor": 0, "major": 0, "critical": 0}
    for ev in events:
        sev = ev.get("severity", "minor")
        if sev in counts:
            counts[sev] += 1
    return counts


def _has_critical_fail(events: list[dict]) -> bool:
    return any(ev.get("severity") == "critical" for ev in events)


def _global_stats(frames: list[FrameData]) -> tuple[float, float]:
    """Returns (mean_center_offset_px, offset_stdev_px)."""
    offsets = [f.center_offset_px for f in frames]
    mean = _safe_mean(offsets)
    stdev = _safe_stdev(offsets)
    return round(mean, 3), round(stdev, 3)


def _weakest_phase(phase_scores: dict) -> str:
    if not phase_scores:
        return ""
    return min(phase_scores, key=lambda k: phase_scores[k])


def _dominant_dir(frames: list[FrameData]) -> str:
    """Most-common curvature_dir, ignoring 'none' and ''."""
    dirs = [f.curvature_dir for f in frames if f.curvature_dir not in ("none", "")]
    if not dirs:
        return "straight"
    return max(set(dirs), key=dirs.count)


def _dedup_events(events: list[dict]) -> list[dict]:
    """Remove duplicate (frame_seq_no, type) pairs, keep first occurrence."""
    seen: set[tuple] = set()
    unique: list[dict] = []
    for ev in events:
        key = (ev.get("frame_seq_no"), ev.get("type", ""))
        if key not in seen:
            seen.add(key)
            unique.append(ev)
    unique.sort(key=lambda e: e.get("frame_seq_no", 0))
    return unique


# ── Phase Assignment (§6.3) ───────────────────────────────────────────────────


def assign_phases(frames: list[FrameData], maneuver_type: str) -> None:
    """
    Labels each frame's maneuver_phase field in-place.

    straight_line                             → all "body"
    left/right curve (forward/reverse)        → 15% entry / 70% body / 15% exit
    figure_8                                  → arc1 / crossover / arc2  (via RLE)
    parking / reverse_parking                 → "approach" until sustained stop, then "stop"
    """
    n = len(frames)
    if n == 0:
        return

    if maneuver_type == "straight_line":
        for f in frames:
            f.maneuver_phase = "body"

    elif maneuver_type in (
        "left_curve",
        "right_curve",
        "left_curve_reverse",
        "right_curve_reverse",
    ):
        entry_end = int(n * 0.15)
        exit_start = int(n * 0.85)
        for i, f in enumerate(frames):
            if i < entry_end:
                f.maneuver_phase = "entry"
            elif i >= exit_start:
                f.maneuver_phase = "exit"
            else:
                f.maneuver_phase = "body"

    elif maneuver_type == "figure_8":
        _assign_figure8_phases(frames)

    elif maneuver_type in ("parking", "reverse_parking"):
        _assign_parking_phases(frames)

    else:
        # Unknown maneuver: treat everything as "body"
        for f in frames:
            f.maneuver_phase = "body"


def _assign_figure8_phases(frames: list[FrameData]) -> None:
    """
    Applies arc1 / crossover / arc2 / entry / exit labels using run-length
    encoding on curvature_dir.

    Algorithm:
      1. Build RLE runs on curvature_dir.
      2. Collect "significant" arc runs (non-straight, non-none, ≥ MIN_ARC_FRAMES).
      3. First significant arc run → arc1.
      4. Gap between arc1 end and arc2 start → crossover.
      5. Second significant arc run → arc2.
      6. Frames before arc1 → entry; frames after arc2 → exit.
      7. Any other frames (e.g. between arc2 and end) default to "exit".
    """
    n = len(frames)
    if n == 0:
        return

    phases = ["body"] * n  # safe default

    # Build RLE: list of (direction, start_idx, end_idx) inclusive
    runs: list[tuple[str, int, int]] = []
    cur_dir = frames[0].curvature_dir
    run_start = 0
    for i in range(1, n):
        d = frames[i].curvature_dir
        if d != cur_dir:
            runs.append((cur_dir, run_start, i - 1))
            cur_dir = d
            run_start = i
    runs.append((cur_dir, run_start, n - 1))

    # Significant arc runs (curved, long enough)
    arc_runs = [
        r
        for r in runs
        if r[0] not in ("straight", "none", "") and (r[2] - r[1] + 1) >= MIN_ARC_FRAMES
    ]

    if len(arc_runs) >= 2:
        arc1 = arc_runs[0]
        arc2 = arc_runs[1]

        # Frames before arc1 → entry
        for i in range(0, arc1[1]):
            phases[i] = "entry"
        # arc1 body
        for i in range(arc1[1], arc1[2] + 1):
            phases[i] = "arc1"
        # Gap between arcs → crossover
        for i in range(arc1[2] + 1, arc2[1]):
            phases[i] = "crossover"
        # arc2 body
        for i in range(arc2[1], arc2[2] + 1):
            phases[i] = "arc2"
        # Frames after arc2 → exit
        for i in range(arc2[2] + 1, n):
            phases[i] = "exit"

    elif len(arc_runs) == 1:
        arc1 = arc_runs[0]
        for i in range(0, arc1[1]):
            phases[i] = "entry"
        for i in range(arc1[1], arc1[2] + 1):
            phases[i] = "arc1"
        # Treat remaining frames as incomplete arc2
        for i in range(arc1[2] + 1, n):
            phases[i] = "arc2"

    # else: all remain "body" — insufficient arc data

    for i, f in enumerate(frames):
        f.maneuver_phase = phases[i]


def _assign_parking_phases(frames: list[FrameData]) -> None:
    """
    Scans for the first run of 'stopped' motion_dir with length ≥ STOP_MIN_FRAMES.
    Frames before it → "approach"; from the stop start onward → "stop".
    If no qualifying stop is found, all frames are labelled "approach".
    """
    n = len(frames)
    stop_start = n  # sentinel: no stop found

    i = 0
    while i < n:
        if frames[i].motion_dir == "stopped":
            run_end = i
            while run_end < n and frames[run_end].motion_dir == "stopped":
                run_end += 1
            if run_end - i >= STOP_MIN_FRAMES:
                stop_start = i
                break
            else:
                i = run_end  # skip this short stop run
        else:
            i += 1

    for j, f in enumerate(frames):
        f.maneuver_phase = "approach" if j < stop_start else "stop"


# ── Evaluators (§7.3–§7.7) ───────────────────────────────────────────────────


class _EvalConfig:
    """Thin carrier for per-request scoring configuration."""

    def __init__(self, tolerance_px: int, pass_threshold: float) -> None:
        self.tolerance_px = tolerance_px
        self.pass_threshold = pass_threshold


def score_straight_line(
    frames: list[FrameData],
    events: list[dict],
    config: _EvalConfig,
) -> tuple[float, dict, dict]:
    """
    §7.3 Straight-line evaluator.

    Dimensions (weights):
      lane_keeping      70 %  — offset + lane-loss penalties
      direction_accuracy 20 %  — fraction of frames with curvature_dir == "straight"
      motion_accuracy   10 %  — fraction of non-stopped frames with motion_dir == "forward"

    Composite score = Σ(dim × weight).
    """
    lane_score, lane_events = _compute_lane_keeping_score(frames, config.tolerance_px)
    events.extend(lane_events)

    dir_acc = _compute_direction_accuracy(frames, "straight")
    motion_acc = _compute_motion_accuracy(frames, "forward")

    for f in frames:
        ev = _motion_event(f, "forward")
        if ev:
            events.append(ev)

    score = round(
        lane_score * 0.70 + dir_acc * 100.0 * 0.20 + motion_acc * 100.0 * 0.10,
        2,
    )
    dimension_scores = {
        "lane_keeping": round(lane_score, 2),
        "direction_accuracy": round(dir_acc * 100.0, 2),
        "motion_accuracy": round(motion_acc * 100.0, 2),
    }
    phase_scores = _phase_score_map(frames, config.tolerance_px)
    return score, dimension_scores, phase_scores


def score_curve(
    frames: list[FrameData],
    events: list[dict],
    expected_dir: str,
    is_reverse: bool,
    config: _EvalConfig,
) -> tuple[float, dict, dict]:
    """
    §7.4 Curve evaluator — handles left/right, forward/reverse variants.

    Dimensions (weights):
      lane_keeping      60 %
      direction_accuracy 30 %  — measured on body-phase frames only
      motion_accuracy   10 %  — "backward" if is_reverse else "forward"

    Direction events are raised only for body-phase frames to avoid
    penalising natural entry/exit geometry.
    """
    lane_score, lane_events = _compute_lane_keeping_score(frames, config.tolerance_px)
    events.extend(lane_events)

    body_frames = [f for f in frames if f.maneuver_phase == "body"]
    dir_acc = _compute_direction_accuracy(body_frames, expected_dir)

    for f in body_frames:
        ev = _direction_event(f, expected_dir, "body")
        if ev:
            events.append(ev)

    expected_motion = "backward" if is_reverse else "forward"
    motion_acc = _compute_motion_accuracy(frames, expected_motion)

    for f in frames:
        ev = _motion_event(f, expected_motion)
        if ev:
            events.append(ev)

    score = round(
        lane_score * 0.60 + dir_acc * 100.0 * 0.30 + motion_acc * 100.0 * 0.10,
        2,
    )
    dimension_scores = {
        "lane_keeping": round(lane_score, 2),
        "direction_accuracy": round(dir_acc * 100.0, 2),
        "motion_accuracy": round(motion_acc * 100.0, 2),
    }
    phase_scores = _phase_score_map(frames, config.tolerance_px)
    return score, dimension_scores, phase_scores


def score_figure_8(
    frames: list[FrameData],
    events: list[dict],
    config: _EvalConfig,
) -> tuple[float, dict, dict]:
    """
    §7.5 Figure-8 evaluator.

    Dimensions (weights):
      lane_keeping     50 %  — whole-maneuver offset penalties
      arc1_accuracy    20 %  — direction accuracy in arc1
      arc2_accuracy    20 %  — direction accuracy in arc2 (must be opposite of arc1)
      crossover_score  10 %  — direction accuracy in crossover (expect "straight")

    A critical event is raised if arc2 direction matches arc1 (figure-8 not completed).
    """
    lane_score, lane_events = _compute_lane_keeping_score(frames, config.tolerance_px)
    events.extend(lane_events)

    arc1_frames = [f for f in frames if f.maneuver_phase == "arc1"]
    arc2_frames = [f for f in frames if f.maneuver_phase == "arc2"]
    crossover_frames = [f for f in frames if f.maneuver_phase == "crossover"]

    # Determine arc directions from observed dominant direction
    arc1_dir = _dominant_dir(arc1_frames) if arc1_frames else "left"
    arc2_dir_expected = "right" if arc1_dir == "left" else "left"

    arc1_acc = _compute_direction_accuracy(arc1_frames, arc1_dir)
    arc2_acc = _compute_direction_accuracy(arc2_frames, arc2_dir_expected)
    crossover_acc = _compute_direction_accuracy(crossover_frames, "straight")

    for f in arc1_frames:
        ev = _direction_event(f, arc1_dir, "arc1")
        if ev:
            events.append(ev)

    for f in arc2_frames:
        ev = _direction_event(f, arc2_dir_expected, "arc2")
        if ev:
            events.append(ev)

    for f in crossover_frames:
        ev = _direction_event(f, "straight", "crossover")
        if ev:
            events.append(ev)

    # Critical failure: arc2 turned the same way as arc1
    if arc1_frames and arc2_frames:
        arc2_actual_dir = _dominant_dir(arc2_frames)
        if arc2_actual_dir == arc1_dir:
            events.append(
                {
                    "frame_seq_no": arc2_frames[0].frame_seq_no,
                    "severity": "critical",
                    "type": "figure8_direction_error",
                    "description": (
                        f"arc2 dominant direction '{arc2_actual_dir}' matches arc1 "
                        f"'{arc1_dir}'; figure-8 not completed"
                    ),
                }
            )

    score = round(
        lane_score * 0.50
        + arc1_acc * 100.0 * 0.20
        + arc2_acc * 100.0 * 0.20
        + crossover_acc * 100.0 * 0.10,
        2,
    )
    dimension_scores = {
        "lane_keeping": round(lane_score, 2),
        "arc1_accuracy": round(arc1_acc * 100.0, 2),
        "arc2_accuracy": round(arc2_acc * 100.0, 2),
        "crossover_score": round(crossover_acc * 100.0, 2),
    }
    phase_scores = _phase_score_map(frames, config.tolerance_px)
    return score, dimension_scores, phase_scores


def score_parking(
    frames: list[FrameData],
    events: list[dict],
    config: _EvalConfig,
) -> tuple[float, dict, dict]:
    """
    §7.6 Parking evaluator (forward approach).

    Dimensions (weights):
      approach_score  60 %  — lane keeping (80%) + forward motion accuracy (20%)
      stop_score      40 %  — lane keeping during stop; 0 if stop insufficient

    A critical event is raised if no sustained stop is detected.
    A major event is raised if stop duration < STOP_MIN_FRAMES.
    """
    approach_frames = [f for f in frames if f.maneuver_phase == "approach"]
    stop_frames = [f for f in frames if f.maneuver_phase == "stop"]

    # ── Approach ──
    approach_lane, ap_events = _compute_lane_keeping_score(
        approach_frames, config.tolerance_px
    )
    events.extend(ap_events)

    for f in approach_frames:
        ev = _motion_event(f, "forward")
        if ev:
            events.append(ev)

    approach_motion_acc = _compute_motion_accuracy(approach_frames, "forward")
    approach_score = round(approach_lane * 0.80 + approach_motion_acc * 100.0 * 0.20, 2)

    # ── Stop ──
    stop_score = _evaluate_stop(stop_frames, frames, events, config.tolerance_px)

    score = round(approach_score * 0.60 + stop_score * 0.40, 2)
    dimension_scores = {
        "approach_lane_keeping": round(approach_lane, 2),
        "approach_motion": round(approach_motion_acc * 100.0, 2),
        "stop_score": round(stop_score, 2),
    }
    phase_scores = _phase_score_map(frames, config.tolerance_px)
    return score, dimension_scores, phase_scores


def score_reverse_parking(
    frames: list[FrameData],
    events: list[dict],
    config: _EvalConfig,
) -> tuple[float, dict, dict]:
    """
    §7.7 Reverse parking evaluator.

    Identical to score_parking except the approach motion must be 'backward'.
    """
    approach_frames = [f for f in frames if f.maneuver_phase == "approach"]
    stop_frames = [f for f in frames if f.maneuver_phase == "stop"]

    # ── Approach (backward) ──
    approach_lane, ap_events = _compute_lane_keeping_score(
        approach_frames, config.tolerance_px
    )
    events.extend(ap_events)

    for f in approach_frames:
        ev = _motion_event(f, "backward")
        if ev:
            events.append(ev)

    approach_motion_acc = _compute_motion_accuracy(approach_frames, "backward")
    approach_score = round(approach_lane * 0.80 + approach_motion_acc * 100.0 * 0.20, 2)

    # ── Stop ──
    stop_score = _evaluate_stop(stop_frames, frames, events, config.tolerance_px)

    score = round(approach_score * 0.60 + stop_score * 0.40, 2)
    dimension_scores = {
        "approach_lane_keeping": round(approach_lane, 2),
        "approach_motion": round(approach_motion_acc * 100.0, 2),
        "stop_score": round(stop_score, 2),
    }
    phase_scores = _phase_score_map(frames, config.tolerance_px)
    return score, dimension_scores, phase_scores


def _evaluate_stop(
    stop_frames: list[FrameData],
    all_frames: list[FrameData],
    events: list[dict],
    tolerance_px: int,
) -> float:
    """
    Shared stop-quality evaluation used by both parking evaluators.

    Returns a 0–100 stop score.  Side-effect: appends events for insufficient
    or missing stops.
    """
    if len(stop_frames) >= STOP_MIN_FRAMES:
        stop_lane, stop_ev = _compute_lane_keeping_score(stop_frames, tolerance_px)
        events.extend(stop_ev)
        return stop_lane
    else:
        if stop_frames:
            events.append(
                {
                    "frame_seq_no": stop_frames[0].frame_seq_no,
                    "severity": "major",
                    "type": "stop_too_short",
                    "description": (
                        f"Stop duration {len(stop_frames)} frames is below "
                        f"required minimum of {STOP_MIN_FRAMES} frames"
                    ),
                }
            )
        else:
            last_seq = all_frames[-1].frame_seq_no if all_frames else 0
            events.append(
                {
                    "frame_seq_no": last_seq,
                    "severity": "critical",
                    "type": "no_stop_detected",
                    "description": "No sustained stop phase detected during parking",
                }
            )
        return 0.0


# ── Evaluator Registry ────────────────────────────────────────────────────────

EVALUATOR_REGISTRY: dict = {
    "straight_line": score_straight_line,
    "figure_8": score_figure_8,
    "left_curve": lambda f, e, c: score_curve(f, e, "left", False, c),
    "right_curve": lambda f, e, c: score_curve(f, e, "right", False, c),
    "left_curve_reverse": lambda f, e, c: score_curve(f, e, "left", True, c),
    "right_curve_reverse": lambda f, e, c: score_curve(f, e, "right", True, c),
    "parking": score_parking,
    "reverse_parking": score_reverse_parking,
}


# ── Top-level entry point ─────────────────────────────────────────────────────


def score_maneuver(req: ScoreManeuverRequest) -> ScoreManeuverResponse:
    """
    Main scoring entry point called by the /score_maneuver endpoint.

    Steps:
      1. Assign phases to each frame via assign_phases().
      2. Route to the appropriate evaluator from EVALUATOR_REGISTRY.
      3. Aggregate global stats and assemble ScoreManeuverResponse.
    """
    # Work on mutable copies so we can set maneuver_phase without mutating the
    # original Pydantic models (model_copy is preferred, but list rebuild is safe).
    frames = [f.model_copy() for f in req.frames]

    # ── Step 1: Phase assignment ──────────────────────────────────────────────
    assign_phases(frames, req.maneuver_type)

    # ── Step 2: Evaluate ─────────────────────────────────────────────────────
    evaluator = EVALUATOR_REGISTRY.get(req.maneuver_type)
    events: list[dict] = []
    config = _EvalConfig(req.tolerance_px, req.pass_threshold)

    if evaluator is None:
        # Unknown maneuver type — fall back to pure lane-keeping
        score_val, lane_events = _compute_lane_keeping_score(
            frames, config.tolerance_px
        )
        events.extend(lane_events)
        dimension_scores: dict | None = {"lane_keeping": round(score_val, 2)}
        phase_scores = _phase_score_map(frames, config.tolerance_px)
    else:
        score_val, dimension_scores, phase_scores = evaluator(frames, events, config)

    # ── Step 3: Global statistics ─────────────────────────────────────────────
    mean_offset, offset_var = _global_stats(frames)

    # Global direction accuracy — mapped from maneuver type
    _expected_dir_map: dict[str, Optional[str]] = {
        "straight_line": "straight",
        "left_curve": "left",
        "left_curve_reverse": "left",
        "right_curve": "right",
        "right_curve_reverse": "right",
        "figure_8": None,  # computed below
        "parking": "straight",
        "reverse_parking": "straight",
    }
    exp_dir = _expected_dir_map.get(req.maneuver_type)

    if exp_dir is not None:
        direction_accuracy = _compute_direction_accuracy(frames, exp_dir)
    else:
        # figure_8: average of arc1 and arc2 direction accuracies
        arc1_frames = [f for f in frames if f.maneuver_phase == "arc1"]
        arc2_frames = [f for f in frames if f.maneuver_phase == "arc2"]
        if arc1_frames:
            arc1_dir = _dominant_dir(arc1_frames)
            arc2_dir_exp = "right" if arc1_dir == "left" else "left"
            da1 = _compute_direction_accuracy(arc1_frames, arc1_dir)
            da2 = (
                _compute_direction_accuracy(arc2_frames, arc2_dir_exp)
                if arc2_frames
                else 0.0
            )
            direction_accuracy = round((da1 + da2) / 2.0, 4)
        else:
            direction_accuracy = 1.0

    # ── Finalise events ───────────────────────────────────────────────────────
    unique_events = _dedup_events(events)
    ev_counts = _event_count_by_severity(unique_events)
    critical_fail = _has_critical_fail(unique_events)
    weakest = _weakest_phase(phase_scores)

    final_score = round(max(0.0, min(100.0, score_val)), 2)
    passed = (final_score >= req.pass_threshold) and not critical_fail

    return ScoreManeuverResponse(
        maneuver_type=req.maneuver_type,
        session_id=req.session_id,
        score=final_score,
        passed=passed,
        critical_fail=critical_fail,
        dimension_scores=dimension_scores,
        phase_scores=phase_scores,
        weakest_phase=weakest,
        mean_center_offset_px=mean_offset,
        offset_variance_px=offset_var,
        direction_accuracy=direction_accuracy,
        events=unique_events,
        event_count_by_severity=ev_counts,
    )
