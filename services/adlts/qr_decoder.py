"""
qr_decoder.py — QR code maneuver decoder with pyzbar + OpenCV fallback.
"""

from __future__ import annotations

import logging

import cv2
import numpy as np

from config import ManeuverResult

logger = logging.getLogger("adlts-service")


class QRDecoder:
    """QR decoder that tries pyzbar first, falls back to OpenCV QRCodeDetector."""

    def __init__(self):
        self._detector = cv2.QRCodeDetector()
        self._pyzbar_ok = False
        try:
            from pyzbar import pyzbar
            self._pyzbar = pyzbar
            self._pyzbar_ok = True
        except ImportError:
            logger.info("pyzbar not available — falling back to OpenCV QRCodeDetector")

    def decode(self, frame_bgr) -> ManeuverResult | None:
        data = None
        bbox = None

        if self._pyzbar_ok:
            result = self._decode_pyzbar(frame_bgr)
            if result is not None:
                data, bbox = result

        if data is None:
            result = self._decode_opencv(frame_bgr)
            if result is not None:
                data, bbox = result

        if data is None:
            return None

        logger.info("QR detected: %s", data)

        return ManeuverResult(
            maneuver_name=data,
            confidence=1.0,
            payload=data,
            bbox=bbox,
        )

    def _decode_pyzbar(self, frame_bgr):
        try:
            decoded = self._pyzbar.decode(frame_bgr)
            if decoded:
                d = decoded[0]
                data = d.data.decode("utf-8").strip()
                poly = d.polygon
                if poly and len(poly) == 4:
                    pts = np.array([(p.x, p.y) for p in poly], dtype=np.float32)
                    x1 = int(np.min(pts[:, 0]))
                    y1 = int(np.min(pts[:, 1]))
                    x2 = int(np.max(pts[:, 0]))
                    y2 = int(np.max(pts[:, 1]))
                    return data, (x1, y1, x2, y2)
                return data, None
        except Exception:
            pass
        return None

    def _decode_opencv(self, frame_bgr):
        try:
            data, points, _ = self._detector.detectAndDecode(frame_bgr)
            if data:
                bbox = self._points_to_bbox(points)
                return data, bbox
        except Exception:
            pass
        return None

    @staticmethod
    def _points_to_bbox(points) -> tuple | None:
        if points is None:
            return None
        pts = np.asarray(points).reshape(-1, 2)
        if pts.size == 0:
            return None
        x1 = int(np.min(pts[:, 0]))
        y1 = int(np.min(pts[:, 1]))
        x2 = int(np.max(pts[:, 0]))
        y2 = int(np.max(pts[:, 1]))
        return (x1, y1, x2, y2)
