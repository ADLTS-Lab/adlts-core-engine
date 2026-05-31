"""
ADLTS Driving Test Service — FastAPI entry point.

Endpoints:
  GET  /health                     — Service health
  POST /calibrate                  — Calibrate lane detection from stream
  POST /start_test                 — Start a new test with device IP + maneuver list
  GET  /test/{test_id}/status      — Current test state
  GET  /test/{test_id}/result      — Final test result JSON
  POST /test/{test_id}/webhook     — Push result notification to Go API
"""
from __future__ import annotations

import asyncio
import io
import json
import logging
import os
import time as _time
from typing import Optional

import cv2
import numpy as np
from fastapi import FastAPI, HTTPException
from fastapi.responses import HTMLResponse, StreamingResponse
from pydantic import BaseModel

from config import (
    MANEUVER_WEIGHTS,
    PASS_THRESHOLD,
    MINIO_ENDPOINT,
    MINIO_ACCESS,
    MINIO_SECRET,
    MINIO_BUCKET,
    MINIO_SECURE,
    LANE_WIDTH_CM,
    TestState,
)
from lane_detector import LaneDetector
from motion_detector import MotionDetector
from qr_decoder import QRDecoder
from scoring_engine import ScoringEngine
from test_controller import TestController
from recorder import Recorder

logging.basicConfig(level=logging.INFO, format="[%(asctime)s] %(name)s — %(levelname)s — %(message)s")
logger = logging.getLogger("adlts-service")

app = FastAPI(title="ADLTS Driving Test Service", version="1.0.0")

# ── Globals (initialized at startup / per test) ─────────────────────────────

_lane_detector = LaneDetector()
_motion_detector = MotionDetector()
_qr_decoder = QRDecoder()
_scoring_engine = ScoringEngine(_lane_detector)
_test_controller = TestController(_scoring_engine)
_recorder: Optional[Recorder] = None
_recording_active = False

# Active test state
_running_test = False
_test_task: Optional[asyncio.Task] = None
_go_webhook_url: Optional[str] = None
_go_webhook_token: str = ""
_device_ip: str = ""


# ── Pydantic models ──────────────────────────────────────────────────────────

class CalibrateResponse(BaseModel):
    ok: bool
    pixels_per_cm: Optional[float] = None
    message: str = ""


class ManeuverDef(BaseModel):
    name: str
    weight: float = 1.0
    tolerance_px: int = 20
    pass_threshold: float = 70.0


class StartTestRequest(BaseModel):
    test_id: str
    candidate_id: str
    device_ip: str = "http://192.168.1.100"
    go_webhook_url: str = ""  # POST result to this Go endpoint when test finishes
    internal_api_key: str = ""  # API key to authenticate webhook POSTs
    minio_endpoint: str = MINIO_ENDPOINT
    minio_access: str = MINIO_ACCESS
    minio_secret: str = MINIO_SECRET
    minio_bucket: str = MINIO_BUCKET


class StartTestResponse(BaseModel):
    ok: bool
    test_id: str
    message: str


class StatusResponse(BaseModel):
    state: str
    test_id: str
    candidate_id: str
    current_maneuver: Optional[str] = None
    maneuver_index: int = 0
    total_maneuvers: int = 0
    scores: list = []
    frames_in_buffer: int = 0


class ResultResponse(BaseModel):
    test_id: str
    candidate_id: str
    total_score: float
    passed: bool
    pass_threshold: float
    recording_prefix: str
    maneuvers: list


class WebhookResponse(BaseModel):
    ok: bool
    message: str


# ── Lifecycle ────────────────────────────────────────────────────────────────

@app.on_event("startup")
def startup():
    global _recorder
    _recorder = Recorder(
        endpoint=MINIO_ENDPOINT,
        access_key=MINIO_ACCESS,
        secret_key=MINIO_SECRET,
        bucket=MINIO_BUCKET,
        secure=MINIO_SECURE,
    )
    logger.info("ADLTS service started — MinIO recording %s",
                "enabled" if _recorder.enabled else "disabled (no minio lib)")


# ── Endpoints ────────────────────────────────────────────────────────────────

@app.get("/health")
def health():
    return {
        "status": "ok",
        "test_state": _test_controller.state.value,
        "recorder_enabled": _recorder.enabled if _recorder else False,
    }


@app.post("/calibrate", response_model=CalibrateResponse)
def calibrate(device_ip: str = "http://192.168.1.100"):
    """
    Calibrate lane detection by grabbing a frame from the device stream.
    The car must be centred on a straight lane.
    """
    stream_url = f"{device_ip.rstrip('/')}/stream"
    cap = cv2.VideoCapture(stream_url)
    cap.set(cv2.CAP_PROP_OPEN_TIMEOUT_MSEC, 5000)
    cap.set(cv2.CAP_PROP_READ_TIMEOUT_MSEC, 5000)

    ret, frame = cap.read()
    cap.release()

    if not ret or frame is None:
        raise HTTPException(status_code=502, detail="Could not read frame from device stream")

    result = _lane_detector.detect(frame)
    if result.left_line and result.right_line:
        _lane_detector.calibrate(result.left_line, result.right_line, LANE_WIDTH_CM)
        logger.info("Calibration done — pixels_per_cm=%.2f", _lane_detector.pixels_per_cm)
        return CalibrateResponse(
            ok=True,
            pixels_per_cm=_lane_detector.pixels_per_cm,
            message=f"Calibrated at {_lane_detector.pixels_per_cm:.2f} px/cm",
        )
    else:
        raise HTTPException(
            status_code=400,
            detail="Could not find both lanes in frame. Centre the car on a straight lane.",
        )


@app.post("/start_test", response_model=StartTestResponse)
async def start_test(req: StartTestRequest):
    """Start a new driving test. Runs asynchronously in a background task."""
    global _running_test, _test_task, _go_webhook_url, _device_ip

    if _running_test:
        raise HTTPException(status_code=409, detail="A test is already running")

    _go_webhook_url = req.go_webhook_url
    _go_webhook_token = req.internal_api_key
    _device_ip = req.device_ip

    # Start the test controller
    recording_prefix = f"recordings/{req.test_id}/full"
    _test_controller.start_test(
        test_id=req.test_id,
        candidate_id=req.candidate_id,
        recording_prefix=recording_prefix,
    )
    _running_test = True

    # Launch background test runner
    _test_task = asyncio.create_task(
        _run_test(
            test_id=req.test_id,
            device_ip=req.device_ip,
            candidate_id=req.candidate_id,
            go_webhook_url=req.go_webhook_url,
            minio_endpoint=req.minio_endpoint,
            minio_access=req.minio_access,
            minio_secret=req.minio_secret,
            minio_bucket=req.minio_bucket,
        )
    )

    return StartTestResponse(
        ok=True,
        test_id=req.test_id,
        message=f"Test {req.test_id} started for candidate {req.candidate_id}",
    )


async def _run_test(
    test_id: str,
    device_ip: str,
    candidate_id: str,
    go_webhook_url: str = "",
    minio_endpoint: str = "",
    minio_access: str = "",
    minio_secret: str = "",
    minio_bucket: str = "",
):
    """Background task: ingest stream, detect lanes, score, record to MinIO."""
    global _running_test, _recording_active, _recorder

    stream_url = f"{device_ip.rstrip('/')}/stream"
    frame_count = 0
    maneuver_frame_counters: dict[str, int] = {}

    _motion_detector.reset()

    # Create a per-test recorder with the supplied creds
    if minio_endpoint:
        _recorder = Recorder(
            endpoint=minio_endpoint,
            access_key=minio_access,
            secret_key=minio_secret,
            bucket=minio_bucket,
            secure=MINIO_SECURE,
        )

    try:
        cap = cv2.VideoCapture(stream_url)
        cap.set(cv2.CAP_PROP_OPEN_TIMEOUT_MSEC, 5000)
        cap.set(cv2.CAP_PROP_BUFFERSIZE, 3)

        while _running_test:
            ret, frame = cap.read()
            if not ret or frame is None:
                await asyncio.sleep(0.05)
                continue

            frame_count += 1

            # ── Record raw frame to MinIO ─────────────────────────────────
            if _recorder and _recorder.enabled:
                ok, buf = cv2.imencode(".jpg", frame, [cv2.IMWRITE_JPEG_QUALITY, 80])
                if ok:
                    jpeg_data = buf.tobytes()
                    _recorder.write_frame(test_id, frame_count, jpeg_data)

                    # Also record to per-maneuver prefix if inside a maneuver
                    current_m = _test_controller.current_maneuver
                    if current_m:
                        maneuver_frame_counters.setdefault(current_m, 0)
                        maneuver_frame_counters[current_m] += 1
                        _recorder.write_maneuver_frame(
                            test_id, current_m, maneuver_frame_counters[current_m], jpeg_data
                        )

            # ── Lane detection ──────────────────────────────────────────
            lane = _lane_detector.detect(frame)

            # ── QR detection ────────────────────────────────────────────
            qr_result = _qr_decoder.decode(frame)

            # ── Motion detection ────────────────────────────────────────
            motion = _motion_detector.detect(frame)

            # ── Per-frame scoring ───────────────────────────────────────
            fs = _scoring_engine.score_frame(lane.left_line, lane.right_line)
            if fs is not None:
                _scoring_engine.add_frame_score(fs)

            # ── Test controller update ──────────────────────────────────
            result = _test_controller.update(
                maneuver_result=qr_result,
                frame_score=fs,
                motion_result=motion,
            )

            # ── Check if test finished ──────────────────────────────────
            if result is not None:
                # Push result to Go webhook
                if go_webhook_url:
                    await _push_result(go_webhook_url, result, _go_webhook_token)
                _running_test = False
                break

            # Yield control to event loop periodically
            if frame_count % 10 == 0:
                await asyncio.sleep(0.001)

        cap.release()

    except Exception as exc:
        logger.exception("Test run failed: %s", exc)
        _test_controller.abort()
        _running_test = False


async def _push_result(webhook_url: str, result, api_token: str = "") -> None:
    """POST the final test result JSON to the Go webhook."""
    import httpx
    headers = {"Content-Type": "application/json"}
    if api_token:
        headers["X-Internal-Token"] = api_token
    payload = {
        "test_id": result.test_id,
        "candidate_id": result.candidate_id,
        "total_score": result.total_score,
        "passed": result.passed,
        "pass_threshold": PASS_THRESHOLD,
        "recording_prefix": result.recording_prefix,
        "maneuvers": [{
            "name": m.name,
            "raw_score": m.raw_score,
            "penalty": m.penalty,
            "final_score": m.final_score,
            "frame_count": m.frame_count,
            "violations": m.violations,
        } for m in result.maneuvers],
    }
    try:
        async with httpx.AsyncClient(timeout=10) as client:
            resp = await client.post(webhook_url, json=payload, headers=headers)
            logger.info("Webhook result pushed to %s — status=%d", webhook_url, resp.status_code)
    except Exception as exc:
        logger.warning("Webhook push failed: %s", exc)


@app.get("/test/{test_id}/status", response_model=StatusResponse)
def get_test_status(test_id: str):
    """Return current test progress."""
    p = _test_controller.progress
    return StatusResponse(
        state=p["state"],
        test_id=p.get("test_id", test_id),
        candidate_id=p.get("candidate_id", ""),
        current_maneuver=p.get("current_maneuver"),
        maneuver_index=p.get("maneuver_index", 0),
        total_maneuvers=p.get("total_maneuvers", 0),
        scores=p.get("scores_so_far", []),
        frames_in_buffer=p.get("frames_in_buffer", 0),
    )


@app.get("/test/{test_id}/result", response_model=ResultResponse)
def get_test_result(test_id: str):
    """Return the final test result. Only available when test is FINISHED."""
    if _test_controller.state != TestState.FINISHED:
        raise HTTPException(status_code=400, detail="Test is not yet finished")
    rj = _test_controller.result_json
    if rj is None:
        raise HTTPException(status_code=404, detail="No result available")

    return ResultResponse(
        test_id=rj["test_id"],
        candidate_id=rj["candidate_id"],
        total_score=rj["total_score"],
        passed=rj["passed"],
        pass_threshold=rj["pass_threshold"],
        recording_prefix=rj["recording_prefix"],
        maneuvers=rj["maneuvers"],
    )


@app.post("/test/{test_id}/webhook")
def test_webhook_response(req: WebhookResponse):
    """Acknowledge webhook delivery of result."""
    return {"ok": True, "message": "Acknowledged"}


@app.post("/test/{test_id}/stop")
def stop_test(test_id: str):
    """Force-stop a running test."""
    global _running_test
    if not _running_test:
        return {"ok": False, "message": "No test is running"}
    _test_controller.abort()
    _running_test = False
    logger.info("Test %s force-stopped", test_id)
    return {"ok": True, "message": "Test stopped"}


@app.post("/test/{test_id}/complete")
def complete_test(test_id: str):
    """Force-finish the test with current accumulated scores (bypass QR detection)."""
    global _running_test
    if not _running_test:
        return {"ok": False, "message": "No test is running"}
    if _test_controller.state.value != "running":
        return {"ok": False, "message": "Test is not running"}

    closing_name = _test_controller.current_maneuver
    if closing_name is not None:
        ms = _scoring_engine.aggregate_maneuver(closing_name)
        _test_controller._scores.append(ms)
        logger.info("Maneuver '%s' force-closed  final=%.1f", ms.name, ms.final_score)

    result = _test_controller._finish_test()
    _running_test = False

    if _go_webhook_url and result:
        import httpx
        headers = {"Content-Type": "application/json"}
        if _go_webhook_token:
            headers["X-Internal-Token"] = _go_webhook_token
        try:
            payload = {
                "test_id": result.test_id,
                "candidate_id": result.candidate_id,
                "total_score": result.total_score,
                "passed": result.passed,
                "pass_threshold": PASS_THRESHOLD,
                "recording_prefix": result.recording_prefix,
                "maneuvers": [{
                    "name": m.name,
                    "raw_score": m.raw_score,
                    "penalty": m.penalty,
                    "final_score": m.final_score,
                    "frame_count": m.frame_count,
                    "violations": m.violations,
                } for m in result.maneuvers],
            }
            httpx.post(_go_webhook_url, json=payload, headers=headers, timeout=10)
        except Exception as exc:
            logger.warning("Webhook push after force-complete failed: %s", exc)

    return {"ok": True, "message": f"Test completed with score {result.total_score:.1f}", "passed": result.passed}


# ── Live MJPEG stream with lane overlay + telemetry ──────────────────────


@app.get("/test/{test_id}/stream")
async def stream_test(test_id: str):
    """MJPEG stream with lane detection overlay and live score/maneuver info."""
    device_ip = _device_ip if _device_ip else "http://192.168.1.12"
    stream_url = f"{device_ip.rstrip('/')}/stream"

    async def generate():
        cap = cv2.VideoCapture(stream_url)
        cap.set(cv2.CAP_PROP_OPEN_TIMEOUT_MSEC, 5000)
        try:
            while True:
                ret, frame = cap.read()
                if not ret or frame is None:
                    await asyncio.sleep(0.05)
                    continue

                lane = _lane_detector.detect(frame)
                fs = _scoring_engine.score_frame(lane.left_line, lane.right_line)

                overlay = lane.raw_frame if lane.raw_frame is not None else frame.copy()

                y = 25
                if fs is not None:
                    cv2.putText(overlay, f"Score: {fs.score:.1f}  Drift: {fs.error_cm:.1f}cm",
                                (10, y), cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 255), 1)
                    y += 22
                else:
                    cv2.putText(overlay, "Score: --  Drift: -- (not calibrated)",
                                (10, y), cv2.FONT_HERSHEY_SIMPLEX, 0.5, (100, 100, 100), 1)
                    y += 22

                maneuver = _test_controller.current_maneuver
                if maneuver:
                    cv2.putText(overlay, f"Maneuver: {maneuver.replace('_', ' ').title()}",
                                (10, y), cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 255), 1)
                    y += 22

                state = _test_controller.state.value
                cv2.putText(overlay, f"State: {state}",
                            (10, y), cv2.FONT_HERSHEY_SIMPLEX, 0.5, (200, 200, 200), 1)

                _, buf = cv2.imencode('.jpg', overlay, [cv2.IMWRITE_JPEG_QUALITY, 65])
                yield (b'--frame\r\n'
                       b'Content-Type: image/jpeg\r\n\r\n' + buf.tobytes() + b'\r\n')

                await asyncio.sleep(0.03)
        except asyncio.CancelledError:
            pass
        finally:
            cap.release()

    return StreamingResponse(generate(), media_type='multipart/x-mixed-replace; boundary=frame')


MONITOR_HTML = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>ADLTS Live Monitor</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { background: #0a0a0a; color: #e0e0e0; font-family: 'Segoe UI', system-ui, sans-serif;
       display: flex; flex-direction: column; align-items: center; min-height: 100vh; padding: 20px; }
h1 { color: #00e676; font-weight: 300; margin-bottom: 12px; font-size: 1.6rem; letter-spacing: 1px; }
.controls { margin-bottom: 16px; display: flex; gap: 8px; flex-wrap: wrap; justify-content: center; }
.controls input { background: #1e1e1e; border: 1px solid #333; color: #e0e0e0; padding: 8px 14px;
                  border-radius: 6px; font-size: 14px; width: 320px; font-family: monospace; }
.controls button { background: #00e676; color: #000; border: none; padding: 8px 20px;
                   border-radius: 6px; font-weight: 600; cursor: pointer; font-size: 14px; }
.controls button:hover { background: #00c853; }
.controls button.danger { background: #e53935; color: #fff; }
.controls button.danger:hover { background: #c62828; }
.controls button.warning { background: #ffa726; color: #000; }
.controls button.warning:hover { background: #f57c00; }
.stream-wrapper { border: 2px solid #333; border-radius: 8px; overflow: hidden;
                  max-width: 800px; width: 100%; background: #000; }
.stream-wrapper img { width: 100%; display: block; }
.status-bar { margin-top: 10px; display: flex; gap: 20px; flex-wrap: wrap; justify-content: center;
              font-size: 13px; color: #888; }
.status-bar a { color: #00e676; text-decoration: none; }
.status-bar a:hover { text-decoration: underline; }
#actions { margin-top: 12px; display: flex; gap: 10px; flex-wrap: wrap; justify-content: center; }
#status-json { margin-top: 12px; padding: 10px; background: #111; border: 1px solid #333;
               border-radius: 6px; font-family: monospace; font-size: 12px; color: #aaa;
               max-width: 800px; width: 100%; white-space: pre-wrap; display: none; }
.footer { margin-top: 24px; font-size: 12px; color: #444; }
</style>
</head>
<body>
<h1>ADLTS &#x25B6; Live Monitor</h1>
<div class="controls">
  <input id="tid" placeholder="Test UUID (e.g. c3148a47-ba6f-490b-940e-f9b988a81445)">
  <button onclick="connect()">View Stream</button>
</div>
<div id="viewer" style="display:none">
  <div class="stream-wrapper"><img id="stream-img" alt="Live stream"></div>
  <div class="status-bar">
    <span id="stream-url-display"></span>
    <a id="status-link" target="_blank">Status JSON</a>
  </div>
  <div id="actions">
    <button class="warning" onclick="completeTest()">&#x2714; Complete Test (force)</button>
    <button class="danger" onclick="stopTest()">&#x25A0; Stop Test</button>
  </div>
  <pre id="status-json">Loading...</pre>
</div>
<div id="placeholder" style="margin-top:40px;color:#555;text-align:center">
  <p style="font-size:48px;margin-bottom:10px">&#x1F4F7;</p>
  <p>Enter a test ID above and click <strong>View Stream</strong></p>
  <p style="font-size:12px;margin-top:8px">Tip: Start a test first via <code>POST /start_test</code></p>
</div>
<div class="footer">ADLTS Driving Test System &mdash; Live Stream Monitor</div>
<script>
let currentTid = '';
function connect() {
  const tid = document.getElementById('tid').value.trim();
  if (!tid) return;
  currentTid = tid;
  const base = '/test/' + tid;
  document.getElementById('stream-img').src = base + '/stream?' + Date.now();
  document.getElementById('stream-url-display').textContent = 'Stream: ' + base + '/stream';
  document.getElementById('status-link').href = base + '/status';
  document.getElementById('viewer').style.display = 'block';
  document.getElementById('placeholder').style.display = 'none';
  pollStatus();
}
async function pollStatus() {
  if (!currentTid) return;
  try {
    const r = await fetch('/test/' + currentTid + '/status');
    const data = await r.json();
    document.getElementById('status-json').textContent = JSON.stringify(data, null, 2);
    document.getElementById('status-json').style.display = 'block';
  } catch(e) {
    document.getElementById('status-json').textContent = 'Error: ' + e.message;
  }
  setTimeout(pollStatus, 2000);
}
async function completeTest() {
  if (!currentTid) return;
  if (!confirm('Force-complete this test with current scores?')) return;
  try {
    const r = await fetch('/test/' + currentTid + '/complete', { method: 'POST' });
    const data = await r.json();
    alert(data.message || JSON.stringify(data));
    pollStatus();
  } catch(e) {
    alert('Error: ' + e.message);
  }
}
async function stopTest() {
  if (!currentTid) return;
  if (!confirm('Force-stop this test?')) return;
  try {
    const r = await fetch('/test/' + currentTid + '/stop', { method: 'POST' });
    const data = await r.json();
    alert(data.message || JSON.stringify(data));
    pollStatus();
  } catch(e) {
    alert('Error: ' + e.message);
  }
}
document.getElementById('tid').addEventListener('keydown', function(e) { if (e.key === 'Enter') connect(); });
</script>
</body>
</html>"""


@app.get("/monitor", response_class=HTMLResponse)
async def monitor_page():
    return MONITOR_HTML