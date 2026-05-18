"""
UFLD-v1 TuSimple ResNet-18 ONNX fallback for lane detection.
Used when the classical pipeline cannot find enough lane points.
Model: models/tusimple_res18.onnx  (input 800×288, output shape varies by model version)
Download: https://s3.ap-northeast-2.wasabisys.com/pinto-model-zoo/140_Ultra-Fast-Lane-Detection/resources_tusimple.tar.gz
"""

from __future__ import annotations

import os
from typing import Optional

import cv2
import numpy as np

try:
    import onnxruntime as ort

    _ORT_AVAILABLE = True
except ImportError:
    _ORT_AVAILABLE = False

from lane_classical import FRAME_H, FRAME_W, STRAIGHT_R, LaneResult

# Model path resolution (in priority order):
#   1. ADLTS_LANE_MODEL env var — set this to your custom toy-car model when ready
#   2. ultra_falst_lane_detection_tusimple_288x800.onnx — file placed by user
#   3. tusimple_res18.onnx — legacy name from PINTO model zoo
_BASE_DIR = os.path.dirname(__file__)


def _resolve_model_path() -> str:
    env_override = os.environ.get("ADLTS_LANE_MODEL")
    if env_override:
        return env_override
    candidates = [
        "ultra_falst_lane_detection_tusimple_288x800.onnx",  # file placed by user
        "tusimple_res18.onnx",  # legacy PINTO name
    ]
    for name in candidates:
        p = os.path.join(_BASE_DIR, "models", name)
        if os.path.exists(p):
            return p
    # Return first candidate path even if missing — _get_session() handles the miss
    return os.path.join(_BASE_DIR, "models", candidates[0])


_MODEL_PATH = _resolve_model_path()

# UFLD-v1 TuSimple config
INPUT_W, INPUT_H = 800, 288
ROW_ANCHORS = [
    64,
    68,
    72,
    76,
    80,
    84,
    88,
    92,
    96,
    100,
    104,
    108,
    112,
    116,
    120,
    124,
    128,
    132,
    136,
    140,
    144,
    148,
    152,
    156,
    160,
    164,
    168,
    172,
    176,
    180,
    184,
    188,
    192,
    196,
    200,
    204,
    208,
    212,
    216,
    220,
    224,
    228,
    232,
    236,
    240,
    244,
    248,
    252,
    256,
    260,
    264,
    268,
    272,
    276,
    280,
    284,
]  # 56 rows
IMAGENET_MEAN = np.array([0.485, 0.456, 0.406], dtype=np.float32)
IMAGENET_STD = np.array([0.229, 0.224, 0.225], dtype=np.float32)
COL_SAMPLE_W = 800 / 100  # column sample width

_session: Optional["ort.InferenceSession"] = None


def _get_session():
    global _session
    if _session is None:
        if not _ORT_AVAILABLE:
            return None
        if not os.path.exists(_MODEL_PATH):
            return None
        try:
            _session = ort.InferenceSession(
                _MODEL_PATH, providers=["CPUExecutionProvider"]
            )
        except Exception:
            return None
    return _session


def detect(frame_bgr: np.ndarray) -> Optional[LaneResult]:
    """
    Run UFLD-v1 inference. Returns LaneResult or None if model unavailable.
    """
    sess = _get_session()
    if sess is None:
        return None

    # Preprocess
    img = cv2.resize(frame_bgr, (INPUT_W, INPUT_H))
    img = cv2.cvtColor(img, cv2.COLOR_BGR2RGB).astype(np.float32) / 255.0
    img = (img - IMAGENET_MEAN) / IMAGENET_STD
    img = np.transpose(img, (2, 0, 1))[np.newaxis, ...]  # (1, 3, 288, 800)

    try:
        out = sess.run(None, {sess.get_inputs()[0].name: img})[0]  # (1, 101, 56, 4)
    except Exception:
        return None

    # Postprocess: argmax per (row, lane) → x coordinate
    out = out[0]  # (101, 56, 4)
    loc = np.argmax(out, axis=0)  # (56, 4) — index 0..100; index 100 = not detected

    result = LaneResult(lane_detected=False, detector_mode="onnx_fallback")
    left_xs_raw, left_ys_raw = [], []
    right_xs_raw, right_ys_raw = [], []

    for lane_idx in range(4):
        xs_img, ys_img = [], []
        for row_idx, row_y in enumerate(ROW_ANCHORS):
            col_idx = int(loc[row_idx, lane_idx])
            if col_idx == 100:  # not detected
                continue
            x_in_input = col_idx * COL_SAMPLE_W
            # Map back to original frame coords
            x_orig = x_in_input * FRAME_W / INPUT_W
            y_orig = row_y * FRAME_H / INPUT_H
            xs_img.append(x_orig)
            ys_img.append(y_orig)

        if len(xs_img) >= 5:
            if lane_idx in (1, 2):  # inner lanes → left and right
                if lane_idx == 1:
                    left_xs_raw, left_ys_raw = xs_img, ys_img
                else:
                    right_xs_raw, right_ys_raw = xs_img, ys_img

    if not left_xs_raw and not right_xs_raw:
        return result  # lane_detected=False

    result.lane_detected = True
    result.left_xs = left_xs_raw
    result.left_ys = left_ys_raw
    result.right_xs = right_xs_raw
    result.right_ys = right_ys_raw

    # Compute center offset
    lb = result.left_xs[-1] if result.left_xs else FRAME_W * 0.20
    rb = result.right_xs[-1] if result.right_xs else FRAME_W * 0.80
    result.center_offset_px = (lb + rb) / 2.0 - FRAME_W / 2.0

    # Lane symmetry
    if result.left_xs and result.right_xs:
        la = np.array(result.left_xs)
        ra = np.array(result.right_xs[: len(la)])
        result.lane_symmetry = float(
            np.clip(1.0 - np.mean(np.abs(la + ra - FRAME_W) / FRAME_W), 0.0, 1.0)
        )

    # Curvature (simplified from left lane polyfit if available)
    if len(result.left_xs) >= 5:
        fit = np.polyfit(result.left_ys, result.left_xs, 2)
        a = fit[0]
        if abs(a) > 1e-6:
            result.curvature_r = float(1.0 / (2.0 * abs(a)))
            result.curvature_dir = (
                ("left" if a < 0 else "right")
                if abs(result.curvature_r) < STRAIGHT_R
                else "straight"
            )
        else:
            result.curvature_r = STRAIGHT_R * 10
            result.curvature_dir = "straight"

    # IoU proxy
    result.iou_score = float(
        np.clip((len(result.left_xs) + len(result.right_xs)) / (56 * 2), 0.0, 1.0)
    )

    return result
