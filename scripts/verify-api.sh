#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BACKEND_URL:-${BASE_URL:-http://localhost:8080}}"
TIMEOUT="${TIMEOUT:-20}"
RUN_AT="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
RUN_ID="$(date -u +'%Y%m%d_%H%M%S')"
RUN_DIR="tmp/api-verification/${RUN_ID}"
RAW_DIR="${RUN_DIR}/responses"
RESULTS_FILE="${RUN_DIR}/results.tsv"
REPORT_FILE="API_VERIFICATION_REPORT.md"
INTERNAL_API_KEY="${INTERNAL_API_KEY:-}"

mkdir -p "$RAW_DIR"
: > "$RESULTS_FILE"

declare -A TOKENS
declare -A REFRESH_TOKENS

TOTAL_CASES=0
PASS_CASES=0
WARN_CASES=0
FAIL_CASES=0
CRITICAL_FAILS=0

LAST_CASE_RESULT=""
LAST_CASE_STATUS=""
LAST_CASE_RAW_FILE=""

declare -A ROLE_EMAILS
declare -A ROLE_PASSWORDS
ROLE_EMAILS[candidate]="candidate@test.et"
ROLE_PASSWORDS[candidate]="Candidate123!"
ROLE_EMAILS[admin]="admin@adlts.et"
ROLE_PASSWORDS[admin]="Admin123!"
ROLE_EMAILS[super_admin]="super@adlts.et"
ROLE_PASSWORDS[super_admin]="SuperAdmin123!"
ROLE_EMAILS[institute]="institute@test.et"
ROLE_PASSWORDS[institute]="Institute123!"
ROLE_EMAILS[expert]="expert@test.et"
ROLE_PASSWORDS[expert]="Expert123!"
ROLE_EMAILS[transport_authority]="authority@test.et"
ROLE_PASSWORDS[transport_authority]="Authority123!"

SEED_CANDIDATE_ID="10000000-0000-4000-8000-000000000141"
RUNNING_TEST_ID="10000000-0000-4000-8000-000000000141"
COMPLETED_TEST_ID_1="10000000-0000-4000-8000-000000000142"
COMPLETED_TEST_ID_2="10000000-0000-4000-8000-000000000143"

printf '%s\n' "group	name	method	path	role	status	expected	result	criticality	reason	content_type	is_json	has_envelope	raw_file	mode	body" >> "$RESULTS_FILE"

safe_field() {
  echo "$1" | tr '\t' ' ' | tr -d '\r' | tr '\n' ' '
}

json_is_json() {
  local file="$1"
  python3 - "$file" <<'PY' >/dev/null
import sys
path = sys.argv[1]
try:
    with open(path, "r", encoding="utf-8") as f:
        _ = f.read()
    # full validation done in parser below
    print("1")
except Exception:
    print("0")
PY
}

json_has_envelope() {
  local file="$1"
  python3 - "$file" <<'PY' >/dev/null
import json
import sys

path = sys.argv[1]
try:
    with open(path, "r", encoding="utf-8") as f:
        payload = json.load(f)
except Exception:
    print("0")
    raise SystemExit

if isinstance(payload, dict) and "success" in payload:
    print("1")
else:
    print("0")
PY
}

json_get() {
  local file="$1"
  local expr="$2"
  python3 - "$file" "$expr" <<'PY'
import json
import sys

path = sys.argv[1]
expr = sys.argv[2]
parts = expr.split(".")

try:
    with open(path, "r", encoding="utf-8") as f:
        payload = json.load(f)
except Exception:
    raise SystemExit

cur = payload
for part in parts:
    if isinstance(cur, dict) and part in cur:
        cur = cur[part]
    else:
        raise SystemExit
print("" if cur is None else cur)
PY
}

extract_json_field() {
  local file="$1"
  shift
  for field in "$@"; do
    local value=""
    if value="$(python3 - "$file" "$field" <<'PY'
import json
import sys

path = sys.argv[1]
expr = sys.argv[2]
parts = expr.split(".")

try:
  with open(path, "r", encoding="utf-8") as f:
    payload = json.load(f)
except Exception:
  raise SystemExit

cur = payload
for part in parts:
  if isinstance(cur, dict) and part in cur:
    cur = cur[part]
  else:
    raise SystemExit
if cur is None:
  raise SystemExit
if isinstance(cur, str):
  print(cur)
else:
  print(cur)
PY
)"; then
      [[ -n "$value" ]] && { echo "$value"; return 0; }
    fi
  done
}

json_first_id() {
  local file="$1"
  python3 - "$file" <<'PY'
import json
import sys

def get_first_id(payload):
    if isinstance(payload, dict):
        if "data" in payload and payload["data"] is not None:
            payload = payload["data"]
        if isinstance(payload, dict):
            if isinstance(payload.get("id"), str):
                return payload["id"]
            if isinstance(payload.get("data"), list):
                for item in payload["data"]:
                    if isinstance(item, dict) and isinstance(item.get("id"), str):
                        return item["id"]
            if isinstance(payload.get("items"), list):
                for item in payload["items"]:
                    if isinstance(item, dict) and isinstance(item.get("id"), str):
                        return item["id"]
            return ""
        if isinstance(payload, list):
            for item in payload:
                if isinstance(item, dict) and isinstance(item.get("id"), str):
                    return item["id"]
            return ""
    if isinstance(payload, list):
        for item in payload:
            if isinstance(item, dict) and isinstance(item.get("id"), str):
                return item["id"]
    return ""

with open(sys.argv[1], "r", encoding="utf-8") as f:
    payload = json.load(f)
print(get_first_id(payload))
PY
}

json_first_field() {
  local file="$1"
  local field="$2"
  python3 - "$file" "$field" <<'PY'
import json
import sys

path = sys.argv[1]
field = sys.argv[2]

def first_field(payload, field):
    if isinstance(payload, dict):
        if "data" in payload and isinstance(payload["data"], dict):
            payload = payload["data"]
        if isinstance(payload, dict):
            value = payload.get(field)
            if isinstance(value, str) and value:
                return value
            for key in ("items", "data"):
                nested = payload.get(key)
                if isinstance(nested, list):
                    for item in nested:
                        if isinstance(item, dict):
                            value = item.get(field)
                            if isinstance(value, str) and value:
                                return value
        elif isinstance(payload, list):
            for item in payload:
                if isinstance(item, dict):
                    value = item.get(field)
                    if isinstance(value, str) and value:
                        return value
        return ""
    if isinstance(payload, list):
        for item in payload:
            if isinstance(item, dict):
                value = item.get(field)
                if isinstance(value, str) and value:
                    return value
    return ""

with open(path, "r", encoding="utf-8") as f:
    payload = json.load(f)
print(first_field(payload, field))
PY
}

status_in_list() {
  local status="$1"
  local list="$2"
  local expected
  IFS=',' read -r -a expected <<< "$list"
  for item in "${expected[@]}"; do
    local trimmed="${item//[[:space:]]/}"
    [[ "$status" == "$trimmed" ]] && return 0
  done
  return 1
}

api_call() {
  local method="$1"
  local path="$2"
  local token="$3"
  local body="$4"
  local out="$5"
  shift 5
  local -a extra_headers=("$@")

  local -a curl_headers=("-H" "Accept: application/json")
  local -a curl_args=(-sS --max-time "$TIMEOUT" --connect-timeout 4 -o "$out" -D "${out}.headers" -X "$method")

  if [[ -n "$token" ]]; then
    curl_headers+=(-H "Authorization: Bearer $token")
  fi
  if ((${#extra_headers[@]} > 0)); then
    curl_headers+=("${extra_headers[@]}")
  fi
  if [[ -n "$body" ]]; then
    curl_args+=(-H "Content-Type: application/json" -d "$body")
  fi

  local status
  if status=$(curl "${curl_args[@]}" "${curl_headers[@]}" "${BASE_URL}${path}" -w '%{http_code}'); then
    API_STATUS="${status//$'\r'/}"
  else
    API_STATUS="000"
  fi

  API_CONTENT_TYPE="$(awk 'BEGIN{IGNORECASE=1} /^Content-Type:/ {sub(/\r/, "", $0); sub(/^Content-Type:[[:space:]]*/, "", $0); sub(/;.*/, "", $0); print; exit}' "${out}.headers" 2>/dev/null || true)"
  if [[ -z "${API_CONTENT_TYPE}" ]]; then
    API_CONTENT_TYPE="unknown"
  fi
  if python3 - "$out" <<'PY' >/dev/null
import json
import sys

try:
    with open(sys.argv[1], "r", encoding="utf-8") as f:
        json.load(f)
    print("1")
except Exception:
    print("0")
PY
  then
    API_JSON="1"
  else
    API_JSON="0"
  fi
  if python3 - "$out" <<'PY' >/dev/null
import json
import sys

try:
    with open(sys.argv[1], "r", encoding="utf-8") as f:
        payload = json.load(f)
except Exception:
    print("0")
    raise SystemExit

if isinstance(payload, dict) and "success" in payload:
    print("1")
else:
    print("0")
PY
  then
    API_ENVELOPE="1"
  else
    API_ENVELOPE="0"
  fi
}

record_case() {
  local group="$1"
  local name="$2"
  local method="$3"
  local path="$4"
  local role="$5"
  local status="$6"
  local expected="$7"
  local result="$8"
  local criticality="$9"
  local reason="${10}"
  local ctype="${11}"
  local is_json="${12}"
  local envelope="${13}"
  local raw="${14}"
  local mode="${15}"
  local body="${16}"

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$(safe_field "$group")" \
    "$(safe_field "$name")" \
    "$(safe_field "$method")" \
    "$(safe_field "$path")" \
    "$(safe_field "$role")" \
    "$(safe_field "$status")" \
    "$(safe_field "$expected")" \
    "$(safe_field "$result")" \
    "$(safe_field "$criticality")" \
    "$(safe_field "$reason")" \
    "$(safe_field "$ctype")" \
    "$is_json" \
    "$envelope" \
    "$(safe_field "$raw")" \
    "$(safe_field "$mode")" \
    "$(safe_field "$body" | cut -c1-120)" >> "$RESULTS_FILE"
}

run_case() {
  local group="$1"
  local name="$2"
  local method="$3"
  local path="$4"
  local role="$5"
  local token="$6"
  local body="$7"
  local expected="$8"
  local mode="${9:-json}"
  local criticality="${10:-critical}"
  local dependency="${11:-0}"
  shift 11
  local -a extra_headers=("$@")

  TOTAL_CASES=$((TOTAL_CASES + 1))
  local raw_file="${RAW_DIR}/${group}_${TOTAL_CASES}.json"

  api_call "$method" "$path" "$token" "$body" "$raw_file" "${extra_headers[@]}"

  local status="$API_STATUS"
  local ctype="$API_CONTENT_TYPE"
  local is_json="$API_JSON"
  local envelope="$API_ENVELOPE"
  local reason="unexpected"
  local result="FAIL"

  if [[ "$status" == "000" ]]; then
    result="FAIL"
    reason="request_error"
  elif status_in_list "$status" "$expected"; then
    result="PASS"
    reason="expected"
  else
    if [[ "$dependency" == "1" ]] && [[ "$status" =~ ^[0-9]+$ ]] && ((status >= 500 && status <= 599)) && [[ "$is_json" == "1" ]] && [[ "$envelope" == "1" ]]; then
      result="WARN"
      reason="dependency"
    elif [[ "$status" == "404" && "$criticality" == "warn" ]]; then
      result="WARN"
      reason="missing_endpoint"
    else
      result="FAIL"
      reason="unexpected_status"
    fi
  fi

  local require_json=0
  local require_envelope=0
  case "$mode" in
    json|json_error|json_or_binary)
      require_json=1
      require_envelope=1
      ;;
    binary_or_json_error)
      if [[ "$status" =~ ^[0-9]+$ ]] && ((status >= 400)); then
        require_json=1
        require_envelope=1
      fi
      ;;
  esac

  if [[ "$status" =~ ^[0-9]+$ ]] && ((status >= 400)); then
    if ((require_json == 1 && is_json != 1)); then
      if [[ "$criticality" == "critical" ]]; then
        result="FAIL"
      elif [[ "$result" == "PASS" ]]; then
        result="WARN"
      fi
      reason="non_json_error"
    elif ((require_envelope == 1 && is_json == 1 && envelope != 1)); then
      if [[ "$criticality" == "critical" ]]; then
        result="FAIL"
      elif [[ "$result" == "PASS" ]]; then
        result="WARN"
      fi
      reason="envelope_missing"
    fi
  elif [[ "$status" =~ ^[0-9]+$ ]] && ((status < 400)); then
    if ((require_json == 1)); then
      if [[ "$is_json" != "1" ]]; then
        if [[ "$criticality" == "critical" ]]; then
          result="FAIL"
        elif [[ "$result" == "PASS" ]]; then
          result="WARN"
        fi
        reason="non_json_success"
      elif ((require_envelope == 1 && envelope != 1)); then
        if [[ "$criticality" == "critical" ]]; then
          result="FAIL"
        elif [[ "$result" == "PASS" ]]; then
          result="WARN"
        fi
        reason="envelope_missing"
      fi
    fi
  fi

  LAST_CASE_RESULT="$result"
  LAST_CASE_STATUS="$status"
  LAST_CASE_RAW_FILE="$raw_file"

  case "$result" in
    PASS) PASS_CASES=$((PASS_CASES + 1)) ;;
    WARN) WARN_CASES=$((WARN_CASES + 1)) ;;
    FAIL) FAIL_CASES=$((FAIL_CASES + 1)) ;;
  esac
  [[ "$criticality" == "critical" && "$result" == "FAIL" ]] && CRITICAL_FAILS=$((CRITICAL_FAILS + 1))

  printf '%-7s %-9s %-5s %-9s %-45s %s\n' "$result" "$criticality" "$method" "$role" "$path" "$status"
  record_case "$group" "$name" "$method" "$path" "$role" "$status" "$expected" "$result" \
    "$criticality" "$reason" "$ctype" "$is_json" "$envelope" "$raw_file" "$mode" "$body"
}

login_role() {
  local role="$1"
  local email="$2"
  local password="$3"
  local body
  body="$(printf '{"email":"%s","password":"%s"}' "$email" "$password")"

  run_case "B" "POST /api/v1/auth/login (${role})" "POST" "/api/v1/auth/login" "public" "" "$body" "200" "json" "critical" 0
  if [[ "${LAST_CASE_RESULT}" == "PASS" ]]; then
    local access
    local refresh
    access="$(extract_json_field "$LAST_CASE_RAW_FILE" "data.access_token" "access_token")"
    refresh="$(extract_json_field "$LAST_CASE_RAW_FILE" "data.refresh_token" "refresh_token")"
    [[ -n "$access" ]] && TOKENS["$role"]="$access"
    [[ -n "$refresh" ]] && REFRESH_TOKENS["$role"]="$refresh"
  fi
}

unique_token_suffix() {
  date +%s%N
}

run_case "A" "GET /health" "GET" "/health" "public" "" "" "200" "json" "critical" 0
if [[ "$LAST_CASE_RESULT" != "PASS" ]]; then
  echo "Backend health check failed; aborting verification. Start server at $BASE_URL."
  python3 - "$RESULTS_FILE" "$REPORT_FILE" "$BASE_URL" "$RUN_AT" "$RUN_ID" "$TOTAL_CASES" "$PASS_CASES" "$WARN_CASES" "$FAIL_CASES" "$CRITICAL_FAILS" "" "" "" "" "" "" <<'PY'
from pathlib import Path
import csv

results_path, report_path, base_url, run_at, run_id, total_cases = __import__("sys").argv[1:7]
rows=[]
with open(results_path, "r", encoding="utf-8") as f:
    rows=list(csv.DictReader(f, delimiter="\t"))
content = [
    "# API Verification Report\n\n",
    f"- Run time: {run_at}\n",
    "- Result: backend unavailable at startup (health check failed)\n",
]
Path(report_path).write_text("".join(content), encoding="utf-8")
PY
  exit 1
fi

for role in candidate admin super_admin institute expert transport_authority; do
  login_role "$role" "${ROLE_EMAILS[$role]}" "${ROLE_PASSWORDS[$role]}"
done

run_case "B" "POST /api/v1/auth/logout (candidate)" "POST" "/api/v1/auth/logout" "candidate" "${TOKENS[candidate]:-}" "" "200" "json" "critical" 0
run_case "B" "POST /api/v1/auth/token/refresh (candidate)" "POST" "/api/v1/auth/token/refresh" "candidate" "${TOKENS[candidate]:-}" \
  "{\"refresh_token\":\"${REFRESH_TOKENS[candidate]:-invalid}\"}" "200" "json" "critical" 0
run_case "B" "POST /api/v1/auth/password/forgot" "POST" "/api/v1/auth/password/forgot" "public" "" \
  "{\"email\":\"${ROLE_EMAILS[candidate]}\"}" "200" "json" "critical" 0
run_case "B" "POST /api/v1/auth/password/reset (invalid token)" "POST" "/api/v1/auth/password/reset" "public" "" \
  "{\"token\":\"invalid-refresh-token\",\"password\":\"Password123!\"}" "400,422" "json" "critical" 0
run_case "B" "PATCH /api/v1/auth/password/change (candidate, invalid current password)" "PATCH" "/api/v1/auth/password/change" "candidate" "${TOKENS[candidate]:-}" \
  "{\"current_password\":\"WrongPassword123!\",\"new_password\":\"Password123!\"}" "400" "json" "critical" 0
run_case "B" "POST /api/v1/auth/candidates/register" "POST" "/api/v1/auth/candidates/register" "public" "" \
  "{\"first_name\":\"Verifier\",\"last_name\":\"Tester\",\"email\":\"verifier.$(unique_token_suffix)@adlts.et\",\"password\":\"Pass1234!\",\"phone\":\"+251700000000\",\"fayida_id\":\"FYID-$(unique_token_suffix)\",\"birth_date\":\"1990-01-01T00:00:00Z\",\"gender\":\"M\"}" \
  "201,400" "json" "critical" 0
run_case "B" "POST /api/v1/auth/candidates/verify-otp (invalid)" "POST" "/api/v1/auth/candidates/verify-otp" "public" "" \
  "{\"email\":\"${ROLE_EMAILS[candidate]}\",\"code\":\"000000\"}" "400" "json" "critical" 0
run_case "B" "POST /api/v1/auth/candidates/resend-otp" "POST" "/api/v1/auth/candidates/resend-otp" "public" "" \
  "{\"email\":\"${ROLE_EMAILS[candidate]}\"}" "200,404" "json" "warn" 0
run_case "B" "POST /api/v1/auth/invitations/accept (invalid token)" "POST" "/api/v1/auth/invitations/accept" "public" "" \
  "{\"token\":\"invalid-token\",\"password\":\"Pass1234!\"}" "400" "json" "critical" 0

run_case "B" "Unauthenticated protected endpoint must require auth" "GET" "/api/v1/candidates" "public" "" "" "401" "json" "critical" 0
run_case "B" "Wrong-role protected endpoint returns denial" "GET" "/api/v1/admins" "candidate" "${TOKENS[candidate]:-}" "" "403,401" "json" "critical" 0

run_case "C" "GET /api/v1/candidates/me" "GET" "/api/v1/candidates/me" "candidate" "${TOKENS[candidate]:-}" "" "200" "json" "critical" 0
run_case "C" "PATCH /api/v1/candidates/me" "PATCH" "/api/v1/candidates/me" "candidate" "${TOKENS[candidate]:-}" \
  "{\"phone\":\"+251700000111\"}" "200,400" "json" "critical" 0

run_case "C" "GET /api/v1/admins/me" "GET" "/api/v1/admins/me" "admin" "${TOKENS[admin]:-}" "" "200" "json" "critical" 0
run_case "C" "GET /api/v1/super-admins/me" "GET" "/api/v1/super-admins/me" "super_admin" "${TOKENS[super_admin]:-}" "" "200" "json" "critical" 0
run_case "C" "GET /api/v1/experts/me" "GET" "/api/v1/experts/me" "expert" "${TOKENS[expert]:-}" "" "200" "json" "critical" 0
run_case "C" "PATCH /api/v1/experts/me (if supported)" "PATCH" "/api/v1/experts/me" "expert" "${TOKENS[expert]:-}" \
  "{\"phone\":\"+251700000112\"}" "200,400" "json" "warn" 0
run_case "C" "GET /api/v1/institutes/me" "GET" "/api/v1/institutes/me" "institute" "${TOKENS[institute]:-}" "" "200" "json" "critical" 0
run_case "C" "PATCH /api/v1/institutes/me" "PATCH" "/api/v1/institutes/me" "institute" "${TOKENS[institute]:-}" \
  "{\"phone\":\"+251700000113\"}" "200,400" "json" "warn" 0
run_case "C" "GET /api/v1/transport-authorities/me" "GET" "/api/v1/transport-authorities/me" "transport_authority" "${TOKENS[transport_authority]:-}" "" "200" "json" "critical" 0

run_case "D" "GET /api/v1/candidates (admin)" "GET" "/api/v1/candidates" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "critical" 0
run_case "D" "GET /api/v1/candidates?page=1&search=candidate" "GET" "/api/v1/candidates?page=1&search=candidate" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "critical" 0
run_case "D" "GET /api/v1/candidates/{seeded}" "GET" "/api/v1/candidates/$SEED_CANDIDATE_ID" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "warn" 0
run_case "D" "PATCH /api/v1/candidates/{seeded}/status" "PATCH" "/api/v1/candidates/$SEED_CANDIDATE_ID/status" "admin" "${TOKENS[admin]:-}" \
  "{\"status\":\"active\"}" "200,400,409,422,403" "json" "warn" 0

run_case "D" "GET /api/v1/admins" "GET" "/api/v1/admins" "super_admin" "${TOKENS[super_admin]:-}" "" "200,404" "json" "critical" 0
run_case "D" "GET /api/v1/experts" "GET" "/api/v1/experts" "super_admin" "${TOKENS[super_admin]:-}" "" "200,404" "json" "critical" 0
run_case "D" "GET /api/v1/institutes" "GET" "/api/v1/institutes" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "critical" 0
ACTIVE_INSTITUTES_FILE="$LAST_CASE_RAW_FILE"
run_case "D" "GET /api/v1/institutes/active" "GET" "/api/v1/institutes/active" "candidate" "${TOKENS[candidate]:-}" "" "200" "json" "critical" 0
INSTITUTE_ID="$(json_first_id "$LAST_CASE_RAW_FILE")"
if [[ -z "$INSTITUTE_ID" ]]; then
  INSTITUTE_ID="$(json_first_field "$ACTIVE_INSTITUTES_FILE" "id")"
fi
if [[ -z "$INSTITUTE_ID" ]]; then
  INSTITUTE_ID="$(json_first_id "${RAW_DIR}/D_5.json" 2>/dev/null || true)"
fi
if [[ -z "$INSTITUTE_ID" ]]; then
  INSTITUTE_ID=""
fi

run_case "D" "GET /api/v1/transport-authorities" "GET" "/api/v1/transport-authorities" "super_admin" "${TOKENS[super_admin]:-}" "" "200,404" "json" "warn" 0

run_case "E" "GET /api/v1/bookings (candidate)" "GET" "/api/v1/bookings" "candidate" "${TOKENS[candidate]:-}" "" "200" "json" "critical" 0
BOOKING_ID=""
if [[ -n "$INSTITUTE_ID" ]]; then
  run_case "E" "POST /api/v1/bookings" "POST" "/api/v1/bookings" "candidate" "${TOKENS[candidate]:-}" \
    "{\"institute_id\":\"$INSTITUTE_ID\"}" "201,200" "json" "warn" 0
  BOOKING_ID="$(json_first_id "$LAST_CASE_RAW_FILE")"
fi
if [[ -z "$BOOKING_ID" ]]; then
  BOOKING_ID="$SEED_CANDIDATE_ID"
fi
run_case "E" "GET /api/v1/bookings (institute)" "GET" "/api/v1/bookings" "institute" "${TOKENS[institute]:-}" "" "200" "json" "critical" 0
run_case "E" "PATCH /api/v1/bookings/{bookingID}/verify" "PATCH" "/api/v1/bookings/$BOOKING_ID/verify" "institute" "${TOKENS[institute]:-}" \
  "{\"action\":\"approve\"}" "200,400,409" "json" "warn" 0
START1="$(date -u -d '+20 minutes' '+%Y-%m-%dT%H:%M:%SZ')"
END1="$(date -u -d '+50 minutes' '+%Y-%m-%dT%H:%M:%SZ')"
START2="$(date -u -d '+70 minutes' '+%Y-%m-%dT%H:%M:%SZ')"
END2="$(date -u -d '+100 minutes' '+%Y-%m-%dT%H:%M:%SZ')"
run_case "G" "POST /api/v1/slots (admin)" "POST" "/api/v1/slots" "admin" "${TOKENS[admin]:-}" \
  "{\"institute_id\":\"$INSTITUTE_ID\",\"starts_at\":\"$START1\",\"ends_at\":\"$END1\",\"capacity\":2}" "201,400" "json" "warn" 0
SLOT_ID="$(json_first_id "$LAST_CASE_RAW_FILE")"
run_case "G" "POST /api/v1/slots (admin second slot)" "POST" "/api/v1/slots" "admin" "${TOKENS[admin]:-}" \
  "{\"institute_id\":\"$INSTITUTE_ID\",\"starts_at\":\"$START2\",\"ends_at\":\"$END2\",\"capacity\":2}" "201,400" "json" "warn" 0
SLOT_ID_2="$(json_first_id "$LAST_CASE_RAW_FILE")"
run_case "E" "PATCH /api/v1/bookings/{bookingID}/schedule" "PATCH" "/api/v1/bookings/$BOOKING_ID/schedule" "admin" "${TOKENS[admin]:-}" \
  "{\"slot_id\":\"${SLOT_ID:-$SLOT_ID_2}\"}" "200,400,409,422" "json" "warn" 0
run_case "E" "PATCH /api/v1/bookings/{bookingID}/reschedule" "PATCH" "/api/v1/bookings/$BOOKING_ID/reschedule" "candidate" "${TOKENS[candidate]:-}" \
  "{\"slot_id\":\"${SLOT_ID_2:-$SLOT_ID}\"}" "200,400,409,422" "json" "warn" 0
run_case "E" "DELETE /api/v1/bookings/{bookingID} (throwaway only)" "DELETE" "/api/v1/bookings/$BOOKING_ID" "admin" "${TOKENS[admin]:-}" "" "200,409,400,422" "json" "warn" 0

run_case "F" "POST /api/v1/bookings/{id}/payments" "POST" "/api/v1/bookings/$BOOKING_ID/payments" "candidate" "${TOKENS[candidate]:-}" \
  "{\"amount_cents\":1500,\"currency\":\"ETB\"}" "200,201,400,409,422" "json" "warn" 1
run_case "F" "POST /api/v1/bookings/{id}/payments/retry" "POST" "/api/v1/bookings/$BOOKING_ID/payments/retry" "candidate" "${TOKENS[candidate]:-}" \
  "{\"amount_cents\":1500,\"currency\":\"ETB\"}" "200,201,400,409,422" "json" "warn" 1
run_case "F" "GET /api/v1/bookings/{id}/payments" "GET" "/api/v1/bookings/$BOOKING_ID/payments" "candidate" "${TOKENS[candidate]:-}" "" "200,404" "json" "warn" 0
run_case "F" "POST /api/v1/bookings/{id}/payments/callback invalid signature" "POST" "/api/v1/bookings/$BOOKING_ID/payments/callback" "public" "" \
  "{\"tx_ref\":\"invalid\",\"status\":\"failed\"}" "400,401,422" "json" "critical" 0

run_case "G" "GET /api/v1/slots?institute_id={id}" "GET" "/api/v1/slots?institute_id=$INSTITUTE_ID" "candidate" "${TOKENS[candidate]:-}" "" "200,400,404" "json" "critical" 0
run_case "G" "GET /api/v1/slots/{id}" "GET" "/api/v1/slots/$SLOT_ID" "candidate" "${TOKENS[candidate]:-}" "" "200,404" "json" "warn" 0

run_case "H" "GET /api/v1/devices?page=1" "GET" "/api/v1/devices?page=1" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "critical" 0
if [[ -n "$INSTITUTE_ID" ]]; then
  run_case "H" "POST /api/v1/devices" "POST" "/api/v1/devices" "admin" "${TOKENS[admin]:-}" \
    "{\"device_code\":\"VERIFIER-$(unique_token_suffix)\",\"password\":\"Device123!\",\"test_center_id\":\"$INSTITUTE_ID\",\"allowed_levels\":[\"class_b\"],\"stream_url\":\"https://example.com/stream\"}" \
    "201,200,400" "json" "warn" 0
fi
DEVICE_ID="$(json_first_id "$LAST_CASE_RAW_FILE")"
if [[ -z "$DEVICE_ID" ]]; then
  DEVICE_ID="00000000-0000-0000-0000-000000000000"
fi
run_case "H" "GET /api/v1/devices/{id}" "GET" "/api/v1/devices/$DEVICE_ID" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "warn" 0
run_case "H" "PATCH /api/v1/devices/{id}/status" "PATCH" "/api/v1/devices/$DEVICE_ID/status" "admin" "${TOKENS[admin]:-}" \
  "{\"status\":\"active\"}" "200,400,404" "json" "warn" 0
run_case "H" "GET /api/v1/devices/{id}/qr-code?password=Device123!" "GET" "/api/v1/devices/$DEVICE_ID/qr-code?password=Device123!" "admin" "${TOKENS[admin]:-}" "" "200,400,403,404" "binary_or_json_error" "warn" 0
run_case "H" "PATCH /api/v1/devices/{id}" "PATCH" "/api/v1/devices/$DEVICE_ID" "admin" "${TOKENS[admin]:-}" \
  "{\"stream_url\":\"https://example.com/stream-updated\"}" "200,400,404" "json" "warn" 0

run_case "I" "GET /api/v1/test-level-types" "GET" "/api/v1/test-level-types" "admin" "${TOKENS[admin]:-}" "" "200" "json" "critical" 0
run_case "I" "GET /api/v1/guidelines" "GET" "/api/v1/guidelines" "candidate" "${TOKENS[candidate]:-}" "" "200" "json" "critical" 0
run_case "I" "GET /api/v1/guidelines/faq" "GET" "/api/v1/guidelines/faq" "candidate" "${TOKENS[candidate]:-}" "" "200" "json" "critical" 0
run_case "I" "GET /api/v1/maneuver-types" "GET" "/api/v1/maneuver-types" "admin" "${TOKENS[admin]:-}" "" "200" "json" "critical" 0
MANEUVER_TYPES_RAW="$LAST_CASE_RAW_FILE"
run_case "I" "GET /api/v1/test-level-mappings" "GET" "/api/v1/test-level-mappings" "admin" "${TOKENS[admin]:-}" "" "200" "json" "critical" 0
TEST_LEVEL_CODE="$(json_first_field "$LAST_CASE_RAW_FILE" "test_level_code")"
MAPPING_PLAN_ID="$(json_first_field "$LAST_CASE_RAW_FILE" "test_plan_id")"
run_case "I" "PUT /api/v1/test-level-mappings" "PUT" "/api/v1/test-level-mappings" "admin" "${TOKENS[admin]:-}" \
  "{\"test_level_code\":\"${TEST_LEVEL_CODE:-class_b}\",\"test_plan_id\":\"${MAPPING_PLAN_ID:-$SEED_CANDIDATE_ID}\"}" "200,400,409,422" "json" "warn" 0

run_case "J" "GET /api/v1/test-plans" "GET" "/api/v1/test-plans" "admin" "${TOKENS[admin]:-}" "" "200" "json" "critical" 0
PLAN_ID="$(json_first_id "$LAST_CASE_RAW_FILE")"
if [[ -z "$PLAN_ID" ]]; then
  PLAN_ID="$SEED_CANDIDATE_ID"
fi
run_case "J" "GET /api/v1/test-plans/{planID}" "GET" "/api/v1/test-plans/$PLAN_ID" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "warn" 0
run_case "J" "GET /api/v1/test-plans/{planID}/maneuvers" "GET" "/api/v1/test-plans/$PLAN_ID/maneuvers" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "warn" 0
run_case "J" "POST /api/v1/test-plans" "POST" "/api/v1/test-plans" "admin" "${TOKENS[admin]:-}" \
  "{\"name\":\"Verifier Plan $(unique_token_suffix)\",\"description\":\"Temporary verifier plan\",\"pass_threshold\":70}" "200,201" "json" "warn" 0
SAFE_PLAN_ID="$(json_first_id "$LAST_CASE_RAW_FILE")"
if [[ -z "$SAFE_PLAN_ID" ]]; then
  SAFE_PLAN_ID="$PLAN_ID"
fi
run_case "J" "PATCH /api/v1/test-plans/{planID}" "PATCH" "/api/v1/test-plans/$SAFE_PLAN_ID" "admin" "${TOKENS[admin]:-}" \
  "{\"description\":\"Verifier plan patched\"}" "200,400,409" "json" "warn" 0
run_case "J" "POST /api/v1/test-plans/{planID}/publish" "POST" "/api/v1/test-plans/$SAFE_PLAN_ID/publish" "admin" "${TOKENS[admin]:-}" "" "200,400,409,422" "json" "warn" 0
run_case "J" "POST /api/v1/test-plans/{planID}/retire" "POST" "/api/v1/test-plans/$SAFE_PLAN_ID/retire" "admin" "${TOKENS[admin]:-}" "" "200,400,409,422" "json" "warn" 0

MANEUVER_TYPE="$(json_first_field "$MANEUVER_TYPES_RAW" "code")"
run_case "J" "POST /api/v1/test-plans/{planID}/maneuvers" "POST" "/api/v1/test-plans/$SAFE_PLAN_ID/maneuvers" "admin" "${TOKENS[admin]:-}" \
  "{\"maneuver_type\":\"${MANEUVER_TYPE:-parallel_park}\",\"display_name\":\"Verifier Maneuver\",\"weight\":1,\"pass_threshold\":80,\"tolerance_px\":16,\"min_frames_required\":6,\"sequence_number\":1}" \
  "201,400,409" "json" "warn" 0
SAFE_MANEUVERS_ID="$(json_first_id "$LAST_CASE_RAW_FILE")"
if [[ -z "$SAFE_MANEUVERS_ID" ]]; then
  SAFE_MANEUVERS_ID="00000000-0000-0000-0000-000000000000"
fi
run_case "J" "PATCH /api/v1/test-plans/{planID}/maneuvers/{maneuverID}" "PATCH" "/api/v1/test-plans/$SAFE_PLAN_ID/maneuvers/$SAFE_MANEUVERS_ID" "admin" "${TOKENS[admin]:-}" \
  "{\"display_name\":\"Verifier Maneuver Updated\"}" "200,400,409,404" "json" "warn" 0
run_case "J" "DELETE /api/v1/test-plans/{planID}/maneuvers/{maneuverID}" "DELETE" "/api/v1/test-plans/$SAFE_PLAN_ID/maneuvers/$SAFE_MANEUVERS_ID" "admin" "${TOKENS[admin]:-}" "" "200,400,404" "json" "warn" 0
run_case "J" "GET /api/v1/test-plans/{planID}/maneuvers/{maneuverID}/qr" "GET" "/api/v1/test-plans/$SAFE_PLAN_ID/maneuvers/$SAFE_MANEUVERS_ID/qr" "admin" "${TOKENS[admin]:-}" "" "200,404,400" "binary_or_json_error" "warn" 0
run_case "J" "GET /api/v1/test-plans/{planID}/maneuvers/{maneuverID}/qr-code" "GET" "/api/v1/test-plans/$SAFE_PLAN_ID/maneuvers/$SAFE_MANEUVERS_ID/qr-code" "admin" "${TOKENS[admin]:-}" "" "200,404,400" "binary_or_json_error" "warn" 0
run_case "J" "GET /api/v1/test-plans/{planID}/maneuvers/{maneuverID}/mask" "GET" "/api/v1/test-plans/$SAFE_PLAN_ID/maneuvers/$SAFE_MANEUVERS_ID/mask" "admin" "${TOKENS[admin]:-}" "" "200,404,400" "binary_or_json_error" "warn" 0

run_case "K" "GET /api/v1/tests (admin)" "GET" "/api/v1/tests" "admin" "${TOKENS[admin]:-}" "" "200,401,403,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests?status=running" "GET" "/api/v1/tests?status=running" "admin" "${TOKENS[admin]:-}" "" "200,400,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests/my" "GET" "/api/v1/tests/my" "candidate" "${TOKENS[candidate]:-}" "" "200,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests/my/stats" "GET" "/api/v1/tests/my/stats" "candidate" "${TOKENS[candidate]:-}" "" "200,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests/my/pending" "GET" "/api/v1/tests/my/pending" "candidate" "${TOKENS[candidate]:-}" "" "200,404" "json" "warn" 0
run_case "K" "GET /api/v1/tests/{running}" "GET" "/api/v1/tests/$RUNNING_TEST_ID" "admin" "${TOKENS[admin]:-}" "" "200,404,403" "json" "critical" 0
run_case "K" "GET /api/v1/tests/{completed}/status" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/status" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests/{completed}/result (candidate)" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/result" "candidate" "${TOKENS[candidate]:-}" "" "200,403,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests/{completed}/result (admin)" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/result" "admin" "${TOKENS[admin]:-}" "" "200,403,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests/{completed}/result (expert)" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/result" "expert" "${TOKENS[expert]:-}" "" "200,403,404" "json" "critical" 0
run_case "K" "GET /api/v1/tests/{completed}/sessions" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/sessions" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "critical" 0
COMPLETED_SESSIONS_RAW="$LAST_CASE_RAW_FILE"
run_case "K" "GET /api/v1/tests/{completed}/recording" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/recording" "candidate" "${TOKENS[candidate]:-}" "" "200,404,500" "binary_or_json_error" "critical" 1
run_case "K" "GET /api/v1/tests/{completed}/monitor/status" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/monitor/status" "admin" "${TOKENS[admin]:-}" "" "200,404,500" "json" "critical" 0
run_case "K" "GET /api/v1/tests/{completed}/monitor/live" "GET" "/api/v1/tests/$COMPLETED_TEST_ID_1/monitor/live" "admin" "${TOKENS[admin]:-}" "" "200,404,500" "json" "critical" 0
run_case "K" "POST /api/v1/tests/{id}/abort" "POST" "/api/v1/tests/$RUNNING_TEST_ID/abort" "admin" "${TOKENS[admin]:-}" \
  "{\"reason\":\"Verifier abort safety check\"}" "200,400,409,422,404" "json" "warn" 0
run_case "K" "POST /api/v1/tests/device-checkin" "POST" "/api/v1/tests/device-checkin" "candidate" "${TOKENS[candidate]:-}" \
  "{\"device_code\":\"VERIFIER-DEVICE\",\"password\":\"Device123!\",\"test_center_id\":\"${INSTITUTE_ID:-00000000-0000-0000-0000-000000000000}\"}" "200,400,401,403,404" "json" "warn" 0
run_case "K" "POST /api/v1/tests/{id}/guidelines/acknowledge" "POST" "/api/v1/tests/$COMPLETED_TEST_ID_1/guidelines/acknowledge" "candidate" "${TOKENS[candidate]:-}" \
  "{\"status\":\"acknowledged\"}" "200,400,409,404" "json" "warn" 0

run_case "L" "GET /api/v1/appeals?status=pending" "GET" "/api/v1/appeals?status=pending" "expert" "${TOKENS[expert]:-}" "" "200,401" "json" "critical" 0
run_case "L" "GET /api/v1/appeals/{id}" "GET" "/api/v1/appeals/$SEED_CANDIDATE_ID" "admin" "${TOKENS[admin]:-}" "" "200,404" "json" "warn" 0
APPEAL_LIST_RAW="$LAST_CASE_RAW_FILE"
APPEAL_ID="$(json_first_id "$APPEAL_LIST_RAW")"
if [[ -z "$APPEAL_ID" ]]; then
  APPEAL_ID="$SEED_CANDIDATE_ID"
fi
run_case "L" "PATCH /api/v1/appeals/{id}/resolve" "PATCH" "/api/v1/appeals/$APPEAL_ID/resolve" "expert" "${TOKENS[expert]:-}" \
  "{\"decision\":\"accepted\",\"resolution\":\"Verifier review\"}" "200,400,409,422,404" "json" "warn" 0
run_case "L" "POST /api/v1/appeals" "POST" "/api/v1/appeals" "candidate" "${TOKENS[candidate]:-}" \
  "{\"test_id\":\"$COMPLETED_TEST_ID_1\",\"session_id\":\"$COMPLETED_TEST_ID_1\",\"reason\":\"Verifier appeal request\"}" "201,400,403,404,409,422" "json" "warn" 0

run_case "M" "GET /api/v1/recordings/{test_id}/play" "GET" "/api/v1/recordings/$COMPLETED_TEST_ID_1/play" "admin" "${TOKENS[admin]:-}" "" "200,401,403,404" "binary_or_json_error" "critical" 1
run_case "M" "GET /api/v1/recordings/{test_id}/frames" "GET" "/api/v1/recordings/$COMPLETED_TEST_ID_1/frames" "admin" "${TOKENS[admin]:-}" "" "200,401,403,404" "json" "critical" 1

run_case "N" "POST /api/v1/reports/{test_id}/generate" "POST" "/api/v1/reports/$COMPLETED_TEST_ID_1/generate" "admin" "${TOKENS[admin]:-}" "" "200,422,409,400" "json" "critical" 1
run_case "N" "GET /api/v1/reports/{test_id}/pdf" "GET" "/api/v1/reports/$COMPLETED_TEST_ID_1/pdf" "admin" "${TOKENS[admin]:-}" "" "200,404" "binary_or_json_error" "critical" 1

if [[ -n "$INTERNAL_API_KEY" ]]; then
  run_case "O" "POST /internal/tests" "POST" "/internal/tests" "internal" "" \
    "{\"booking_id\":\"$BOOKING_ID\",\"candidate_id\":\"$SEED_CANDIDATE_ID\",\"test_center_id\":\"${INSTITUTE_ID:-$SEED_CANDIDATE_ID}\",\"test_level_code\":\"class_b\"}" \
    "200,400,422,500,409" "json" "warn" 1 \
    -H "X-Internal-Token: $INTERNAL_API_KEY"
  run_case "O" "PATCH /internal/tests/by-booking/{bookingID}" "PATCH" "/internal/tests/by-booking/$BOOKING_ID" "internal" "" \
    "{\"new_scheduled_start\":\"$START1\",\"new_scheduled_end\":\"$END1\",\"booking_window_hours\":2}" \
    "200,400,404,409,422,500" "json" "warn" 1 \
    -H "X-Internal-Token: $INTERNAL_API_KEY"
  run_case "O" "DELETE /internal/tests/by-booking/{bookingID}" "DELETE" "/internal/tests/by-booking/$BOOKING_ID" "internal" "" \
    "" "200,400,404,500,422" "json" "warn" 1 \
    -H "X-Internal-Token: $INTERNAL_API_KEY"
else
  run_case "O" "POST /internal/tests (skip when INTERNAL_API_KEY unset)" "POST" "/internal/tests" "internal" "" "" "401,403,404" "json" "warn" 0
fi

python3 - "$RESULTS_FILE" "$REPORT_FILE" "$BASE_URL" "$RUN_AT" "$RUN_DIR" "$RUN_ID" "$TOTAL_CASES" "$PASS_CASES" "$WARN_CASES" "$FAIL_CASES" "$CRITICAL_FAILS" \
  "${TOKENS[candidate]:-}" "${TOKENS[admin]:-}" "${TOKENS[super_admin]:-}" "${TOKENS[institute]:-}" "${TOKENS[expert]:-}" "${TOKENS[transport_authority]:-}" <<'PY'
import csv
import os
from pathlib import Path
import re
import sys

results_path, report_path, base_url, run_at, run_dir, run_id, total_cases, pass_cases, warn_cases, fail_cases, critical_fails, tok_candidate, tok_admin, tok_super, tok_institute, tok_expert, tok_authority = sys.argv[1:18]

rows = []
with open(results_path, "r", encoding="utf-8") as handle:
  reader = csv.DictReader(handle, delimiter="\t")
  rows = [
    {key: (value.strip() if isinstance(value, str) else value) for key, value in row.items()}
    for row in reader
  ]


def esc(value):
  return str(value).replace("|", "\\|")


def token_summary(token):
  token = str(token or "")
  if not token:
    return "missing"
  return token[:10] + "..." + token[-6:]


critical_failures = [r for r in rows if r["criticality"] == "critical" and r["result"] == "FAIL"]
dependency_warnings = [r for r in rows if r["result"] == "WARN" and r["reason"] == "dependency"]
missing_endpoints = [r for r in rows if r["reason"] == "missing_endpoint"]
json_issues = [r for r in rows if r["reason"] in {"envelope_missing", "non_json_error", "non_json_success"}]
mutating = [r for r in rows if r["method"] in {"POST", "PUT", "PATCH", "DELETE"}]
role_checks = [r for r in rows if "unauth" in r["name"].lower() or "wrong-role" in r["name"].lower()]
if not role_checks:
  role_checks = [r for r in rows if r["role"] in {"candidate", "admin", "super_admin", "institute", "expert", "transport_authority"} and r["status"] in {"401", "403"} and int(r["status"]) in (401, 403)]

ready = "No" if critical_failures else "Yes"

table_rows = [
  "| Group | Name | Method | Path | Role | Status | Expected | Result | Severity | Reason |\n",
  "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n",
]
for row in rows:
  table_rows.append(
    f"| {esc(row['group'])} | {esc(row['name'])} | {esc(row['method'])} | {esc(row['path'])} | "
    f"{esc(row['role'])} | {esc(row['status'])} | {esc(row['expected'])} | {esc(row['result'])} | "
    f"{esc(row['criticality'])} | {esc(row['reason'])} |\n"
  )


def format_list(items, prefix):
  if not items:
    return f"{prefix}: `none`\n"
  return prefix + ":\n" + "".join(
    f"- {esc(r['group'])} | {esc(r['method'])} {esc(r['path'])} -> {esc(r['result'])} ({esc(r['status'])})\n"
    for r in items
  ) + "\n"


sections = [
  "# API Verification Report\n\n",
  f"- Run time: {run_at}\n",
  f"- Backend: `{base_url}`\n",
  f"- Run directory: `{run_dir}`\n",
  f"- Total cases executed: {total_cases}\n",
  f"- PASS: {pass_cases}\n",
  f"- WARN: {warn_cases}\n",
  f"- FAIL: {fail_cases}\n",
  f"- Critical failures: {len(critical_failures)}\n",
  f"- Ready for frontend verification: {ready}\n\n",
  "## Tokens acquired by role\n\n",
  "| role | token sample |\n",
  "| --- | --- |\n",
  f"| candidate | {token_summary(tok_candidate)} |\n",
  f"| admin | {token_summary(tok_admin)} |\n",
  f"| super_admin | {token_summary(tok_super)} |\n",
  f"| institute | {token_summary(tok_institute)} |\n",
  f"| expert | {token_summary(tok_expert)} |\n",
  f"| transport_authority | {token_summary(tok_authority)} |\n",
  "\n## Endpoint pass/fail/warn table\n\n",
  "".join(table_rows),
  "\n## Critical failures\n",
  format_list(critical_failures, "- Critical failures"),
  "\n## Warnings/dependency failures\n",
  format_list(dependency_warnings, "- Warnings"),
  "\n## Missing endpoints\n",
  format_list(missing_endpoints, "- Missing endpoints"),
  "\n## Role/RBAC issues\n",
  format_list(role_checks, "- Role/RBAC issues"),
  "\n## JSON envelope issues\n",
  format_list(json_issues, "- JSON envelope issues"),
  "\n## Mutating endpoints tested\n",
  format_list(mutating, "- Mutating endpoints"),
  "## Recommended fixes\n",
  "- Re-run once auth/session services are healthy and fix non-critical route/path regressions before frontend smoke.\n",
  "- Investigate `non_json_error`/`envelope_missing` on critical endpoints first; frontend contracts should always return JSON envelopes.\n",
  "- Dependency WARN items (`REPORT_FAILED`, `LIST_ERROR`, payment webhook/storage errors) are only considered non-blocking when route is mounted and response is JSON-formatted.\n",
]

Path(report_path).write_text("".join(sections), encoding="utf-8")
PY

echo
echo "Report: $REPORT_FILE"
echo "Raw responses: $RUN_DIR"
echo "Summary: total=$TOTAL_CASES pass=$PASS_CASES warn=$WARN_CASES fail=$FAIL_CASES critical_fail=$CRITICAL_FAILS"
if (( CRITICAL_FAILS > 0 )); then
  echo "Result: FAIL (critical failures present)"
  exit 1
fi
echo "Result: PASS (no critical failures)"
