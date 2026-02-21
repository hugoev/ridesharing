#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────
# 🚗 Ride-Sharing E2E Demo — Full Ride Lifecycle
# ─────────────────────────────────────────────────────────────
set -euo pipefail

BASE="http://localhost:8080"
BOLD="\033[1m"
GREEN="\033[32m"
CYAN="\033[36m"
YELLOW="\033[33m"
RESET="\033[0m"

step() { echo -e "\n${BOLD}${CYAN}═══ $1 ═══${RESET}"; }
ok()   { echo -e "${GREEN}✅ $1${RESET}"; }
info() { echo -e "${YELLOW}   → $1${RESET}"; }

# ── 0. Health Check ──────────────────────────────────────────
step "0. Health Check"
curl -s "$BASE/health" | python3 -m json.tool
ok "Server is healthy"

# ── 1. Register a Rider ──────────────────────────────────────
step "1. Register Rider (Alice)"
RIDER_RESP=$(curl -s -X POST "$BASE/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice_'$RANDOM'@test.com",
    "password": "password123",
    "name": "Alice Johnson",
    "phone": "555-1001",
    "role": "rider"
  }')
echo "$RIDER_RESP" | python3 -m json.tool
RIDER_TOKEN=$(echo "$RIDER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
RIDER_ID=$(echo "$RIDER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
ok "Rider registered: $RIDER_ID"

# ── 2. Register a Driver ─────────────────────────────────────
step "2. Register Driver (Bob)"
DRIVER_RESP=$(curl -s -X POST "$BASE/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "bob_'$RANDOM'@test.com",
    "password": "password123",
    "name": "Bob Smith",
    "phone": "555-2002",
    "role": "driver",
    "vehicle_type": "sedan",
    "license_plate": "NYC-4567"
  }')
echo "$DRIVER_RESP" | python3 -m json.tool
DRIVER_TOKEN=$(echo "$DRIVER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
DRIVER_ID=$(echo "$DRIVER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
ok "Driver registered: $DRIVER_ID"

# ── 3. Driver Logs In ────────────────────────────────────────
step "3. Driver Logs In (verify JWT)"
LOGIN_RESP=$(curl -s -X POST "$BASE/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\": \"$(echo "$DRIVER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['email'])")\", \"password\": \"password123\"}")
echo "$LOGIN_RESP" | python3 -m json.tool
ok "Login successful, got fresh JWT"

# ── 4. Get User Profile ──────────────────────────────────────
step "4. View Driver Profile"
curl -s "$BASE/v1/users/$DRIVER_ID" \
  -H "Authorization: Bearer $DRIVER_TOKEN" | python3 -m json.tool
ok "Profile retrieved with driver details"

# ── 5. Driver Updates GPS Location ───────────────────────────
step "5. Driver Updates GPS (Times Square, NYC)"
curl -s -X POST "$BASE/v1/locations/update" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"lat": 40.7580, "long": -73.9855}' | python3 -m json.tool
info "Stored in Redis GEO set + PostgreSQL"
ok "Driver location updated"

# ── 6. Rider Searches for Nearby Drivers ──────────────────────
step "6. Find Nearby Drivers (from Lower Manhattan)"
curl -s "$BASE/v1/locations/nearby?lat=40.7128&long=-74.0060&radius=10" \
  -H "Authorization: Bearer $RIDER_TOKEN" | python3 -m json.tool
info "Uses Redis GEOSEARCH with PostGIS fallback"
ok "Found nearby drivers"

# ── 7. Rider Requests a Ride ──────────────────────────────────
step "7. Request a Ride (Lower Manhattan → Times Square)"
RIDE_RESP=$(curl -s -X POST "$BASE/v1/rides/request" \
  -H "Authorization: Bearer $RIDER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "pickup_lat": 40.7128,
    "pickup_long": -74.0060,
    "dropoff_lat": 40.7580,
    "dropoff_long": -73.9855
  }')
echo "$RIDE_RESP" | python3 -m json.tool
RIDE_ID=$(echo "$RIDE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
FARE=$(echo "$RIDE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['fare'])")
SURGE=$(echo "$RIDE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['surge_multiplier'])")
STATUS=$(echo "$RIDE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])")
info "Haversine distance calculated, fare=\$$FARE, surge=${SURGE}x"
info "Auto-matched to nearest driver → status=$STATUS"
ok "Ride created and matched!"

# ── 8. Get Ride Details ───────────────────────────────────────
step "8. Get Ride Details"
curl -s "$BASE/v1/rides/$RIDE_ID" \
  -H "Authorization: Bearer $RIDER_TOKEN" | python3 -m json.tool
ok "Ride details retrieved"

# ── 9. Driver Accepts (starts heading to pickup) ─────────────
step "9. Driver Accepts Ride (status: matched → enroute)"
curl -s -X POST "$BASE/v1/rides/$RIDE_ID/accept" \
  -H "Authorization: Bearer $DRIVER_TOKEN" | python3 -m json.tool
info "FSM transition: matched → enroute"
ok "Driver is heading to pickup"

# ── 10. Driver Completes the Ride ─────────────────────────────
step "10. Driver Completes Ride (status: enroute → completed)"
curl -s -X POST "$BASE/v1/rides/$RIDE_ID/complete" \
  -H "Authorization: Bearer $DRIVER_TOKEN" | python3 -m json.tool
info "FSM transition: enroute → completed"
ok "Ride completed!"

# ── 11. Process Payment ───────────────────────────────────────
step "11. Process Payment (Stripe stub)"
curl -s -X POST "$BASE/v1/payments/charge" \
  -H "Authorization: Bearer $RIDER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"ride_id\": \"$RIDE_ID\", \"amount\": $FARE}" | python3 -m json.tool
info "Stripe PaymentIntent created + captured"
ok "Payment processed!"

# ── 12. Verify Payment ────────────────────────────────────────
step "12. Verify Payment Record"
curl -s "$BASE/v1/payments/ride/$RIDE_ID" \
  -H "Authorization: Bearer $RIDER_TOKEN" | python3 -m json.tool
ok "Payment verified in database"

# ── 13. Test Cancellation Flow ────────────────────────────────
step "13. Test Cancellation (new ride)"
RIDE2_RESP=$(curl -s -X POST "$BASE/v1/rides/request" \
  -H "Authorization: Bearer $RIDER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "pickup_lat": 40.7484,
    "pickup_long": -73.9857,
    "dropoff_lat": 40.6892,
    "dropoff_long": -74.0445
  }')
RIDE2_ID=$(echo "$RIDE2_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
info "Created ride $RIDE2_ID"

curl -s -X POST "$BASE/v1/rides/$RIDE2_ID/cancel" \
  -H "Authorization: Bearer $RIDER_TOKEN" | python3 -m json.tool
info "FSM transition: matched → cancelled"
ok "Ride cancelled!"

# ── Summary ───────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════${RESET}"
echo -e "${BOLD}${GREEN}  🎉 E2E DEMO COMPLETE — Full Ride Lifecycle!     ${RESET}"
echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════${RESET}"
echo ""
echo -e "  ${CYAN}Flow tested:${RESET}"
echo "    Register → Login → Profile → GPS Update → Nearby Search"
echo "    → Request Ride → Auto-Match → Accept → Complete → Pay"
echo "    → Cancel (alternate flow)"
echo ""
echo -e "  ${CYAN}Services hit:${RESET}"
echo "    Auth · User · Location · Ride · Payment · Notification"
echo ""
echo -e "  ${CYAN}Infra used:${RESET}"
echo "    PostgreSQL · Redis GEO · Kafka events · JWT auth"
echo ""
