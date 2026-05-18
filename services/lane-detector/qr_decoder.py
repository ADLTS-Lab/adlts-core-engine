"""QR code decoder with 3-frame confirmation debounce."""

import cv2


class QRDecoder:
    CONFIRM_FRAMES = 3
    PREFIX_START = "ADLTS:S:"
    PREFIX_END = "ADLTS:E:"

    def __init__(self):
        self.detector = cv2.QRCodeDetector()
        self._last = None
        self._count = 0

    def decode(self, frame_bgr) -> dict | None:
        """
        Returns dict or None.
        dict keys: action ("start"|"end"), maneuver_type, config_id, raw
        Requires 3 consecutive identical detections to confirm.
        """
        data, bbox, _ = self.detector.detectAndDecode(frame_bgr)
        if not data:
            self._last = None
            self._count = 0
            return None

        if data == self._last:
            self._count += 1
        else:
            self._last = data
            self._count = 1

        if self._count < self.CONFIRM_FRAMES:
            return None

        if data.startswith(self.PREFIX_START):
            parts = data.split(":", 3)  # ["ADLTS","S","left_curve","<uuid>"]
            if len(parts) == 4:
                return {
                    "action": "start",
                    "maneuver_type": parts[2],
                    "config_id": parts[3],
                    "raw": data,
                }
        if data.startswith(self.PREFIX_END):
            parts = data.split(":", 3)
            if len(parts) == 4:
                return {
                    "action": "end",
                    "maneuver_type": parts[2],
                    "config_id": parts[3],
                    "raw": data,
                }
        return None
