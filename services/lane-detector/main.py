"""
ADLTS Lane Detector Microservice
"""
from __future__ import annotations

import base64
import logging
import time as _time
from typing import Optional

import cv2
import lane_classical
import lane_hough
import lane_onnx
import numpy as np
from fastapi import FastAPI, HTTPException
from motion_detector import MotionDirectionDetector
from pydantic import BaseModel
from qr_decoder import QRDecoder
from scorer import ScoreManeuverRequest, ScoreManeuverResponse, score_maneuver

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("lane-detector")

app = FastAPI(title="ADLTS Lane Detector", version="2.0.0")

_qr_decoder = QRDecoder()
_motion_det = MotionDirectionDetector()


class DetectRequest(BaseModel):
    frame_b64: str
    frame_seq_no: int = 0
    test_id: str = ""
    session_id: str = ""
    maneuver_type: str = ""
    prev_frame_b64: Optional[str] = None
    timestamp_ms: int = 0


class RawLanes(BaseModel):
    left_xs: list[float] = []
    left_ys: list[float] = []
    right_xs: list[float] = []
    right_ys: list[float] = []


class DetectResponse(BaseModel):
    frame_seq_no: int
    timestamp_ms: int
    lane_detected: bool
    lane_detector_mode: str
    center_offset_px: float
    curvature_r: float
    curvature_dir: str
    lane_symmetry: float
    motion_dir: str
    iou_score: float
    qr_event: Optional[str]
    maneuver_phase: str
    raw_lanes: Optional[RawLanes]
    is_mocked: bool


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/detect", response_model=DetectResponse)
def detect(req: DetectRequest):
    try:
        img_bytes = base64.b64decode(req.frame_b64)
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid base64")

    nparr = np.frombuffer(img_bytes, np.uint8)
    frame = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    if frame is None:
        return _mocked_response(req)

    # ── Lane detection — priority: Hough > ONNX > classical bird's-eye ─────────
    lane = lane_hough.detect(frame)

    if not lane.lane_detected:
        onnx_lane = lane_onnx.detect(frame)
        if onnx_lane is not None and onnx_lane.lane_detected:
            lane = onnx_lane

    if not lane.lane_detected:
        classical_lane = lane_classical.detect(frame)
        if classical_lane.lane_detected:
            lane = classical_lane

    # ── QR detection ─────────────────────────────────────────────────────────
    qr_raw: Optional[str] = None
    qr_event = _qr_decoder.decode(frame)
    if qr_event:
        qr_raw = qr_event.get("raw")

    # ── Motion direction ──────────────────────────────────────────────────────
    motion = _motion_det.detect(frame)

    return DetectResponse(
        frame_seq_no=req.frame_seq_no,
        timestamp_ms=req.timestamp_ms or int(_time.time() * 1000),
        lane_detected=lane.lane_detected,
        lane_detector_mode=lane.detector_mode,
        center_offset_px=round(lane.center_offset_px, 3),
        curvature_r=round(lane.curvature_r, 2),
        curvature_dir=lane.curvature_dir,
        lane_symmetry=round(lane.lane_symmetry, 4),
        motion_dir=motion,
        iou_score=round(lane.iou_score, 4),
        qr_event=qr_raw,
        maneuver_phase="",
        raw_lanes=RawLanes(
            left_xs=lane.left_xs[:20],
            left_ys=lane.left_ys[:20],
            right_xs=lane.right_xs[:20],
            right_ys=lane.right_ys[:20],
        )
        if lane.lane_detected
        else None,
        is_mocked=False,
    )


@app.post("/score_maneuver", response_model=ScoreManeuverResponse)
def score_maneuver_endpoint(req: ScoreManeuverRequest):
    try:
        return score_maneuver(req)
    except Exception as e:
        logger.exception("score_maneuver failed")
        raise HTTPException(status_code=500, detail=str(e))


def _mocked_response(req: DetectRequest) -> DetectResponse:
    return DetectResponse(
        frame_seq_no=req.frame_seq_no,
        timestamp_ms=req.timestamp_ms,
        lane_detected=False,
        lane_detector_mode="classical",
        center_offset_px=0.0,
        curvature_r=0.0,
        curvature_dir="none",
        lane_symmetry=0.0,
        motion_dir="stopped",
        iou_score=0.0,
        qr_event=None,
        maneuver_phase="",
        raw_lanes=None,
        is_mocked=True,
    )