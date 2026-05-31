"""
recorder.py — MinIO frame recording during test execution.

Writes frames from the MJPEG stream to MinIO under a test-specific prefix.
Both full-test recording and per-maneuver session recording are supported.
"""

from __future__ import annotations

import io
import logging
from typing import Optional

logger = logging.getLogger(__name__)

try:
    from minio import Minio
    HAS_MINIO = True
except ImportError:
    Minio = None
    HAS_MINIO = False


class Recorder:
    """
    Records JPEG frames to MinIO during a test.

    Structure:
      recordings/{test_id}/full/frame_{seq_no:08d}.jpg   — all frames
      recordings/{test_id}/maneuvers/{maneuver}/{seq:08d}.jpg — per-maneuver

    Parameters
    ----------
    endpoint : str
        MinIO endpoint (e.g. "minio:9000")
    access_key : str
    secret_key : str
    bucket : str
    secure : bool
    """

    def __init__(
        self,
        endpoint: str,
        access_key: str,
        secret_key: str,
        bucket: str,
        secure: bool = False,
    ):
        self.bucket = bucket
        self._minio = None
        self._enabled = False

        if not HAS_MINIO:
            logger.warning("minio library not installed — recording disabled")
            return

        try:
            self._minio = Minio(
                endpoint=endpoint,
                access_key=access_key,
                secret_key=secret_key,
                secure=secure,
            )
            # Ensure bucket exists
            if not self._minio.bucket_exists(bucket):
                self._minio.make_bucket(bucket)
                logger.info("Created MinIO bucket: %s", bucket)
            else:
                logger.info("MinIO bucket %s already exists", bucket)
            self._enabled = True
        except Exception as exc:
            logger.warning("MinIO connection failed (%s) — recording disabled", exc)

    @property
    def enabled(self) -> bool:
        return self._enabled

    def write_frame(self, test_id: str, seq_no: int, data: bytes) -> bool:
        """Write a single JPEG frame to the full-test prefix."""
        if not self._enabled or self._minio is None:
            return False
        key = f"recordings/{test_id}/full/frame_{seq_no:08d}.jpg"
        try:
            self._minio.put_object(
                bucket_name=self.bucket,
                object_name=key,
                data=io.BytesIO(data),
                length=len(data),
                content_type="image/jpeg",
            )
            return True
        except Exception as exc:
            logger.warning("MinIO write failed for frame %d: %s", seq_no, exc)
            return False

    def write_maneuver_frame(self, test_id: str, maneuver: str, seq_no: int, data: bytes) -> bool:
        """Write a frame to a per-maneuver prefix."""
        if not self._enabled or self._minio is None:
            return False
        key = f"recordings/{test_id}/maneuvers/{maneuver}/frame_{seq_no:08d}.jpg"
        try:
            self._minio.put_object(
                bucket_name=self.bucket,
                object_name=key,
                data=io.BytesIO(data),
                length=len(data),
                content_type="image/jpeg",
            )
            return True
        except Exception as exc:
            logger.warning("MinIO write failed for maneuver frame %s/%d: %s", maneuver, seq_no, exc)
            return False