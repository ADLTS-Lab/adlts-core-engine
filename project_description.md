# ADLTS Core Engine — Project Description

## What ADLTS is

**ADLTS (Automated Driving License Testing System)** is a platform designed to modernize how driving tests are booked, conducted, reviewed, and documented.

At its center is the **Core Engine**: a secure backend that coordinates identity, permissions, scheduling, payments, test session records, recordings, appeals, and reporting — so that the overall system is **fairer, more auditable, and easier to operate at scale**.

This repository is intentionally built as a **modular monolith** (one service, many modules) to keep development fast while still separating responsibilities cleanly.

---

## Who it’s for (key personas)

ADLTS supports multiple real-world roles, each with distinct workflows:

- **Candidates**: register, verify identity, book tests, pay, reschedule when allowed, view outcomes, and submit appeals within the appeal window.
- **Institutes**: validate candidate readiness/verification requirements (where applicable) and participate in the operational pipeline.
- **Admins & Super Admins**: manage the system lifecycle — scheduling, approvals, oversight, and platform administration.
- **Experts**: review and resolve appeals; contribute to quality assurance and fairness.
- **Transport Authorities / Regulators**: governance-oriented access, policy enforcement, and accountability support.

These roles are first-class in the system and enforced by authentication + authorization rules.

---

## Core capabilities (what the system does)

### 1) Secure identity, onboarding, and access control

ADLTS provides a complete onboarding and access model built for real institutions:

- **Self-registration with OTP verification** for candidates
- **Invitation-based onboarding** for staff/organizations (e.g., experts, institutes, admins)
- **Password reset + password change** flows
- **JWT-based authentication** for stateless, scalable sessions
- **Role- and entity-based authorization** to ensure each user only sees and performs what they’re allowed to
- **Bootstrap seeding** for a root Super Admin (so deployments are operable from day one)

Why this matters for the frontend: you can confidently build separate dashboards for each persona without duplicating permission logic in the UI.

### 2) Booking and scheduling lifecycle (end-to-end)

The booking system is not just “create a booking” — it models the real operational process:

- Candidate initiates a **booking request**
- Verification steps can be enforced (institutional approval) before scheduling
- Admins assign a **time slot** with capacity controls
- Candidates can **reschedule** within allowed states
- The booking progresses through clear lifecycle states (see “Status-driven UI” below)

This is built for high-volume operations where scheduling conflicts, capacity, and governance matter.

### 3) Payments integrated into the lifecycle (Chapa)

ADLTS integrates payments as a first-class part of the booking workflow:

- Hosted checkout integration via **Chapa** (common in Ethiopia)
- Controlled **retry attempts** (up to a defined max) to prevent ambiguous states
- Verified payment outcome handling (webhook + verification flow)
- A **payment audit trail** per booking attempt

Frontend impact: the UI can guide candidates through a clean “Pay → Confirmed” experience and provide clarity around failure/retry states.

### 4) Test session records (the backbone for automation)

A driving test is represented as a structured **session record** that can hold:

- Session status transitions (scheduled → started → completed/finalized)
- A score and result metadata
- A link to recording artifacts
- Telemetry pointers (for near-real-time updates)

Even when additional automated scoring modules evolve, the Core Engine already models the entities needed to store and explain outcomes.

### 5) Recording storage and playback

For transparency and auditability, the system supports recording artifacts:

- Frame storage (designed for object storage)
- Playback streaming via **MJPEG**
- **Presigned URLs** for securely sharing stored frames without exposing buckets publicly
- Backed by a MinIO-compatible object storage client

Frontend impact: enables “Watch the test” and “Review frames” experiences for authorized roles.

### 6) Appeals for fairness and accountability

ADLTS includes an appeal workflow designed to be strict, explainable, and auditable:

- Candidates can submit appeals **within an appeal window**
- Appeals carry a reason and progress through review states
- Experts can resolve appeals with a written resolution
- If an appeal is accepted, the system updates the corresponding test result status
- Evidence snapshot support exists (so reviews can reference what was known at the time)

Frontend impact: you can build a clear “Appeal timeline” UI and expert review workspace.

### 7) Reporting: analytics + narrative + PDF artifacts

ADLTS generates professional reports that combine:

- **Deterministic analytics** (scores, decisions, findings) as the source of truth
- **AI-assisted narrative text** (for readable explanations) generated from structured analytics
- HTML rendering and **PDF generation**
- Disk caching for performance and repeatability

Key principle: the AI component rewrites structured analytics into human-friendly language — it does not replace deterministic scoring.

Frontend impact: a polished report experience for institutes/admins/experts, suitable for printing and compliance.

### 8) Media & document handling

- Controlled upload directory
- Size limits and safe static serving

Frontend impact: profile photos/logos and supporting documents can be handled consistently.

---

## How it works (high-level flow)

A typical lifecycle looks like this:

1. **Onboarding**
   - Candidate registers → OTP verifies → becomes active
   - Staff users can be created through invitations and governance flows

2. **Booking**
   - Candidate selects institute/test center context and submits a booking
   - Optional verification/approval occurs (institution/admin flow)

3. **Scheduling**
   - Admin assigns a time slot (capacity-controlled)
   - Candidate can reschedule in specific states

4. **Payment**
   - Candidate pays via hosted checkout
   - Outcome is verified and booking becomes confirmed

5. **Testing session + artifacts**
   - Session record holds status, score, and links to artifacts
   - Recording frames can be stored and later reviewed

6. **Outcome, reporting, and accountability**
   - A report can be generated (analytics + narrative + PDF)
   - Candidate may submit an appeal within the configured window
   - Experts resolve appeals with written decisions

---

## Status-driven UI (important for frontend builders)

A lot of ADLTS UX becomes simple when you treat the platform as **state machines**.

### Booking statuses (candidate/admin/institute UX)

Bookings move through defined states such as:

- `drafted`
- `pending_verification`
- `verified`
- `scheduled`
- `payment_pending`
- `payment_failed`
- `confirmed`
- `archived` / `cancelled` / `rejected`

Frontend guidance:

- Design each major screen around “What can the user do in this state?”
- Always show the **current state** and the **next expected action**
- Make failure states explicit (e.g., payment failed → retry)

### Appeal statuses (fairness workflow)

Appeals are tracked as a review process (e.g., pending → accepted/rejected) and should be shown as a timeline with reasons and resolutions.

---

## Trust, safety, and auditability (what makes it credible)

ADLTS is built to make testing **defensible**:

- Clear separation of roles and permissions
- Recording + evidence snapshots for review
- Deterministic analytics as the source of truth
- Reports that are reproducible and cached (consistency)
- Data model prepared for telemetry and detection events (for proctoring/scoring evolution)

This is a system designed for **institutional accountability**, not just convenience.

---

## Technical architecture (what the Core Engine is)

- **Language/runtime**: Go
- **Architecture**: modular monolith (single HTTP process; clean module boundaries)
- **Persistence**: PostgreSQL (migrations-managed schema)
- **Auth**: JWT bearer tokens
- **Email**: SMTP-backed delivery for OTP, invitations, and reset flows (safe to skip in dev)
- **Object storage**: MinIO-compatible client for recording frames
- **Reporting pipeline**: external data fetch + analytics + AI narrative + HTML/PDF rendering + caching
- **Containerization**: Docker + Compose for local/dev deployment

---

## Integrations (what the platform connects to)

- **Chapa**: payment initialization and verification (hosted checkout)
- **Anthropic**: narrative generation for reports (based on deterministic analytics)
- **External “Testing Core” service**: provides test/session/event data used in reporting
- **Identity/profile source**: candidate profile resolution for reporting and governance

---

## What frontend teams should highlight on the homepage

If you’re building a landing page or product homepage, strong sections to lead with:

- **Fairness + transparency**: recording, appeals, explainable outcomes
- **Operational efficiency**: streamlined booking → schedule → pay pipeline
- **Institution-ready governance**: roles, permissions, auditability
- **Report-grade outputs**: printable PDFs with analytics + readable narrative
- **Scalable foundation**: modular architecture designed to grow into full proctoring/scoring workflows

Suggested homepage structure (copy-friendly):

- Hero: “Automated, auditable driving license testing.”
- Problem: manual processes, inconsistent scoring, scheduling chaos, weak accountability
- Solution: ADLTS lifecycle + governance + evidence
- Feature blocks: identity, booking/scheduling, payments, recording, appeals, reporting
- How it works: 6-step lifecycle diagram
- Trust: deterministic analytics + evidence + permissions
- CTA: candidate onboarding / institution onboarding / request a demo

---

## Summary

**ADLTS Core Engine** is the system of record and orchestration layer for an automated driving test ecosystem — designed to ensure that every test can be **scheduled cleanly**, **paid reliably**, **recorded transparently**, **reviewed fairly**, and **reported professionally**.
