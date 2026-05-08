Project Specification: Automated Driving License Testing System (ADLTS)
1. Project Vision
The ADLTS is a machine-learning-based platform designed to automate the evaluation of driving exams.

The Setup: A toy car with an ESP32-CAM streams video of a demo track (S-Curve, Parking, Traffic Lights).

The Intelligence: The backend processes these frames using Lane Detection and Sign Detection models to calculate scores and detect violations in real-time.

2. Technical Architecture (Modular Monolith)
The system uses a Go-inspired Internal/Domain structure. Each module is self-contained with its own transport (handler), logic (service), and data access (repository) layers.

Plaintext
/cmd/api            # Application entry point
/internal           # Private application code
  ├── auth          # Registration, JWT, Invitations, OTP
  ├── users         # Profiles, Roles, Account Status
  ├── bookings      # Exam scheduling & School verification
  ├── exams         # Live test logic, results, and history
  ├── scoring       # ML Model integration & violation logic
  ├── devices       # IoT (ESP32) management & health checks
  ├── appeals       # Candidate appeals & Authority resolution
  └── analytics     # Reporting for Transport Authority
/pkg                # Public libraries (e.g., logger, validator)
Module Component Roles:

Models: Data structures and DB tags.

Repository: Database queries (PostgreSQL).

Service: Business logic (e.g., "Can this candidate book this slot?").

Handler: HTTP/WebSocket request/response handling.

3. Comprehensive Endpoint Clarification
3.1 Authentication & User Management
Method	Endpoint	Actor	Description
POST	/auth/register	Candidate	Self-registration.
POST	/auth/invite	Admin/Authority	Send invite to School/Examiner.
POST	/auth/login	All	Returns JWT & Role.
GET	/users/me	All	Get own profile details.
PATCH	/users/:id	Admin	Activate/Suspend accounts.
3.2 Booking & Institute Verification
Method	Endpoint	Actor	Description
GET	/institutes	Candidate	List registered driving schools.
POST	/bookings	Candidate	Book exam; status starts as pending_verification.
GET	/bookings/verify	Inst. Admin	List candidates claiming to be from this school.
PATCH	/bookings/:id/verify	Inst. Admin	Approve/Reject training record.
3.3 Exam Execution & IoT (ESP32)
Method	Endpoint	Actor	Description
GET	/schedules	Candidate	View available slots for verified users.
POST	/exams/initiate	Candidate	Scan QR code on car to link Exam ID to Device ID.
WS	/ws/iot/stream	ESP32-CAM	Binary/Base64 frame ingestion for ML processing.
GET	/exams/:id/live	Examiner	Real-time monitoring with AI violation overlays.
POST	/exams/:id/stop	Examiner	Manual emergency override.
3.4 Scoring & Results
Method	Endpoint	Actor	Description
POST	/scoring/analyze	Internal	ML Engine triggers score calculation for a frame.
GET	/exams/:id/results	Candidate	View session violations and final score.
GET	/analytics/global	Authority	National pass/fail rates & common failure points.
3.5 Appeals
Method	Endpoint	Actor	Description
POST	/appeals	Candidate	File appeal for a specific Exam ID.
GET	/appeals/pending	Authority	View all appeals requiring resolution.
PATCH	/appeals/:id	Authority	Resolve appeal (Accepted/Rejected).
4. Machine Learning Workflow
Ingestion: internal/devices receives the frame via WebSocket.

Inference: internal/scoring sends the frame to the ML model (YOLO/OpenCV).

Detection:

Lanes: Are coordinates outside of the "Lane Mask"?

Signs: Is a "Stop" sign present while speed > 0?

Update: The internal/exams service updates the live session state in Redis/PostgreSQL.

5. Standard Response Format
JSON
{
  "success": true,
  "data": {}, 
  "error": null,
  "meta": { "page": 1, "limit": 10 }
}
This structure ensures that your Go backend remains clean, the ESP32 stays lightweight, and the Transport Authority has full oversight.