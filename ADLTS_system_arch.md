ADLTS: Complete Backend System Architecture & API Specification
The Automated Driving License Testing System (ADLTS) is a high-integrity platform that replaces manual testing with an AI-driven computer vision pipeline. The system uses a Go-inspired modular monolith architecture to handle the complex lifecycle of a driver’s test.

1. System Governance & Actor Model
The system operates on a hierarchical invitation-based trust model to prevent fraudulent entities from joining.

Transport Authority (Supreme Actor): * Acts as the system owner.

Invites Institutes (Driving Schools) via secure email tokens.

Acts as the final judge for Appeals.

Views global analytics and national pass/fail trends.

System Admin (Technical Operator):

Manages IoT Device registration (pairing ESP32-CAM IDs to the system).

Monitors system health and ML engine latency.

Does not interfere with exam results but manages user account statuses (Suspend/Activate).

Institute Admin (The Verifier):

Validates that a Candidate has completed the required training hours.

Must upload a "Certificate of Completion" or training log to verify a booking.

Examiner (Human-in-the-loop):

Monitors the live AI stream.

Can trigger an Emergency Stop if the toy car (or vehicle) is at risk.

Candidate:

Registers, books exams, selects their school, and views digital results.

2. The Modular Monolith Structure (/internal)
The backend is divided into domain-driven modules. Each module contains its own handler.go (HTTP/WS), service.go (Logic), and repository.go (Database).

2.1 /auth & /users
Handles the complex onboarding flow.

POST /auth/invite: Authority sends an invitation to an institute email.

POST /auth/register/institute: Institute uses the token from the invite to create their admin account.

POST /auth/register/candidate: Public endpoint for students.

POST /auth/verify-otp: Multi-factor authentication for high-stakes actions.

2.2 /bookings
Manages the "Trust Handshake" between student and school.

POST /bookings: Candidate creates a request, selecting their Institute. Status: PENDING_VERIFICATION.

PATCH /bookings/{id}/verify: Institute uploads training data and signs off. Status: VERIFIED.

GET /bookings/available-slots: Only visible to VERIFIED candidates.

2.3 /devices & /scoring
The IoT and AI core.

POST /devices/register: Admin whitelists an ESP32-CAM MAC address.

WS /iot/stream/{device_id}: Raw frame ingestion.

Scoring Logic: A separate service within the monolith that consumes frames. It runs:

Lane Detection: Checks if car coordinates cross the boundary polygon.

Object Detection: Identifies Stop Signs and Traffic Lights.

State Logic: If Sign=Stop and Speed > 0, trigger VIOLATION_SIGNAL_DISREGARD.

3. The Exam Lifecycle (State Machine)
An exam is not just a record; it is a state machine that ensures data integrity.

SCHEDULED: Booking is paid and time-slot assigned.

INITIATING: Candidate scans the QR code on the vehicle. Backend links ExamID + DeviceID + CandidateID.

ACTIVE: WebSocket is open. Frames are being processed.

COMPLETED: AI aggregates all violations.

REVIEW_REQUIRED: If the score is near the fail-threshold, an Examiner must review the video.

FINALIZED: Result is immutable and pushed to the Transport Authority.

4. Required & Refined Endpoints for Implementation
4.1 Authority & Admin Operations
GET /admin/analytics/map: Heatmap of where most violations occur (e.g., "S-Curve" has a 60% failure rate).

POST /admin/devices/heartbeat: Monitoring endpoint for ESP32 connectivity.

PATCH /authority/appeals/{id}/resolve: Authority overrides or upholds a failed result.

4.2 The Live Scoring Engine (Internal/Private)
POST /internal/scoring/frame-process: Receives a frame, returns detected objects and violation flags.

GET /exams/{id}/telemetry: Provides a real-time JSON stream of the car's current "Health" and "Score" for the Examiner dashboard.

4.3 Candidate Results
GET /exams/{id}/result-overlay: Returns a video URL or sequence of frames where violations were detected (Proof for the candidate).

5. Technical Elements for the "Code-Ready" State
Database: PostgreSQL for relational data (Users, Bookings, Exams); Redis for real-time WebSocket state.

Storage: MinIO or S3 to store the recorded exam video for later appeal review.

Concurrency: Go routines to handle simultaneous ML inference requests without blocking the WebSocket stream.

Security: RBAC (Role-Based Access Control) middleware to ensure a Candidate cannot hit /auth/invite.

This documentation provides the "Why" and the "How," allowing a developer to begin building the internal/ folders with a clear understanding of the interdependent roles and the high-stakes nature of the automated scoring.