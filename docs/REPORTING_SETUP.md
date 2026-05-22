# ADLTS Reporting Service Setup

This feature adds a reporting pipeline that:
- fetches test data from Testing Core,
- fetches candidate profile data from the identity endpoints,
- generates deterministic analytics,
- calls Anthropic Claude for narrative text,
- renders HTML,
- converts HTML to PDF,
- caches generated artifacts on disk.

## Environment Variables

Required:
- `TESTING_CORE_BASE_URL` - base URL for Testing Core (default: `https://api.adlts.et/api/v1`)
- `TESTING_CORE_TOKEN` - bearer token for Testing Core calls
- `IDENTITY_BASE_URL` - base URL for identity/user endpoints (default: `https://api.adlts.et/api/v1`)
- `IDENTITY_TOKEN` - bearer token used to fetch candidate profile data
- `ANTHROPIC_API_KEY` - Anthropic API key
- `ANTHROPIC_MODEL` - model name (default: `claude-3-5-sonnet-latest`)
- `REPORT_OUTPUT_DIR` - local cache directory for generated PDF/HTML/JSON files (default: `../generated-reports`)

## Candidate name lookup

The reporting service resolves the candidate profile using the `candidate_id` from Testing Core and calls the identity API:

- `GET /candidates/{id}`

That keeps candidate names and contact details in the user profile system instead of duplicating them in reporting.

## Endpoints

- `POST /api/v1/reports/{testID}/generate`
- `GET /api/v1/reports/{testID}/pdf`

The `generate` endpoint creates the report if it is not already cached. The `pdf` endpoint serves the cached PDF and regenerates it on demand if missing.

## Caching

Generated artifacts are stored under:

- `<REPORT_OUTPUT_DIR>/{testID}/report.pdf`
- `<REPORT_OUTPUT_DIR>/{testID}/report.html`
- `<REPORT_OUTPUT_DIR>/{testID}/analytics.json`
- `<REPORT_OUTPUT_DIR>/{testID}/narrative.json`

## Notes

- The deterministic analytics layer is the source of truth for scores and recommendations.
- Claude only rewrites the structured analytics into polished narrative text.
- The service only generates reports for completed tests.
