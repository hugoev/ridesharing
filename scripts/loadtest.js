// k6 load test for the RideShare API gateway.
//
// Usage:
//   k6 run scripts/loadtest.js
//   k6 run scripts/loadtest.js --vus 50 --duration 2m
//
// Prerequisites:
//   - Gateway running at http://localhost:8080
//   - Database seeded with at least one rider and driver

import { check, group, sleep } from 'k6';
import http from 'k6/http';
import { Rate, Trend } from 'k6/metrics';

// ── Custom metrics ──────────────────────────────────────────
const errorRate = new Rate('errors');
const loginDuration = new Trend('login_duration', true);
const rideRequestDuration = new Trend('ride_request_duration', true);
const nearbyDriversDuration = new Trend('nearby_drivers_duration', true);

// ── Config ──────────────────────────────────────────────────
export const options = {
    scenarios: {
        // Smoke test — quick sanity check
        smoke: {
            executor: 'constant-vus',
            vus: 1,
            duration: '10s',
            tags: { scenario: 'smoke' },
            startTime: '0s',
        },
        // Load test — maximum sustained load for M1 16GB
        load: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '30s', target: 200 },  // ramp up
                { duration: '2m', target: 200 },  // hold
                { duration: '30s', target: 0 },   // ramp down
            ],
            tags: { scenario: 'load' },
            startTime: '15s',
        },
        // Spike test — sudden burst
        spike: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '10s', target: 300 },  // spike
                { duration: '30s', target: 300 },  // hold
                { duration: '10s', target: 0 },    // recover
            ],
            tags: { scenario: 'spike' },
            startTime: '3m30s',
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<2000', 'p(99)<5000'],  // SLO: p95 < 2s, p99 < 5s for stress
        errors: ['rate<0.10'],                            // SLO: < 10% error rate
        http_req_failed: ['rate<0.10'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// ── Helper functions ────────────────────────────────────────
function registerUser(email, password, role, name) {
    const payload = JSON.stringify({
        email: email,
        password: password,
        name: name,
        phone: '555-0000',
        role: role,
        vehicle_type: role === 'driver' ? 'sedan' : undefined,
        license_plate: role === 'driver' ? `K6-${__VU}` : undefined,
    });

    return http.post(`${BASE_URL}/v1/auth/register`, payload, {
        headers: { 'Content-Type': 'application/json' },
    });
}

function loginUser(email, password) {
    const payload = JSON.stringify({ email, password });
    const res = http.post(`${BASE_URL}/v1/auth/login`, payload, {
        headers: { 'Content-Type': 'application/json' },
    });
    loginDuration.add(res.timings.duration);
    return res;
}

function authHeaders(token) {
    return {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`,
        },
    };
}

// ── Main test scenario ──────────────────────────────────────
export default function () {
    const vuEmail = `loadtest-rider-${__VU}-${__ITER}@k6.io`;
    const driverEmail = `loadtest-driver-${__VU}-${__ITER}@k6.io`;
    const password = 'loadtest123';

    // ── Health check ──
    group('Health Check', () => {
        const res = http.get(`${BASE_URL}/health`);
        check(res, {
            'health: status 200': (r) => r.status === 200,
        }) || errorRate.add(1);
    });

    // ── Register rider ──
    let riderToken;
    group('Register Rider', () => {
        const res = registerUser(vuEmail, password, 'rider', `Rider ${__VU}`);
        const success = check(res, {
            'register rider: status 201': (r) => r.status === 201,
        });
        if (success) {
            riderToken = res.json('access_token');
        }
        errorRate.add(!success);
    });

    if (!riderToken) {
        // Try login in case user already exists
        const loginRes = loginUser(vuEmail, password);
        if (loginRes.status === 200) {
            riderToken = loginRes.json('access_token');
        }
    }

    if (!riderToken) return;

    // ── Register driver ──
    let driverToken;
    group('Register Driver', () => {
        const res = registerUser(driverEmail, password, 'driver', `Driver ${__VU}`);
        const success = check(res, {
            'register driver: status 201': (r) => r.status === 201,
        });
        if (success) {
            driverToken = res.json('access_token');
        }
        errorRate.add(!success);
    });

    if (!driverToken) {
        const loginRes = loginUser(driverEmail, password);
        if (loginRes.status === 200) {
            driverToken = loginRes.json('access_token');
        }
    }

    // ── Update driver location ──
    if (driverToken) {
        group('Update Driver Location', () => {
            const lat = 29.4241 + (Math.random() - 0.5) * 0.01;
            const long = -98.4936 + (Math.random() - 0.5) * 0.01;
            const res = http.post(
                `${BASE_URL}/v1/locations/update`,
                JSON.stringify({ lat, long }),
                authHeaders(driverToken),
            );
            check(res, {
                'location update: status 200': (r) => r.status === 200,
            }) || errorRate.add(1);
        });
    }

    // ── Nearby drivers ──
    group('Nearby Drivers', () => {
        const res = http.get(
            `${BASE_URL}/v1/locations/nearby?lat=29.4241&long=-98.4936&radius=10`,
            authHeaders(riderToken),
        );
        nearbyDriversDuration.add(res.timings.duration);
        check(res, {
            'nearby: status 200': (r) => r.status === 200,
        }) || errorRate.add(1);
    });

    // ── Request ride ──
    let rideId;
    group('Request Ride', () => {
        const res = http.post(
            `${BASE_URL}/v1/rides/request`,
            JSON.stringify({
                pickup_lat: 29.4241,
                pickup_long: -98.4936,
                dropoff_lat: 29.4500 + (Math.random() - 0.5) * 0.02,
                dropoff_long: -98.5200 + (Math.random() - 0.5) * 0.02,
            }),
            authHeaders(riderToken),
        );
        rideRequestDuration.add(res.timings.duration);
        const success = check(res, {
            'ride request: status 201 or 409': (r) => r.status === 201 || r.status === 409,
        });
        if (res.status === 201) {
            rideId = res.json('id');
        }
        errorRate.add(!success);
    });

    // ── Get ride ──
    if (rideId) {
        group('Get Ride', () => {
            const res = http.get(
                `${BASE_URL}/v1/rides/${rideId}`,
                authHeaders(riderToken),
            );
            check(res, {
                'get ride: status 200': (r) => r.status === 200,
            }) || errorRate.add(1);
        });

        // ── Cancel ride (cleanup) ──
        group('Cancel Ride', () => {
            const res = http.post(
                `${BASE_URL}/v1/rides/${rideId}/cancel`,
                null,
                authHeaders(riderToken),
            );
            check(res, {
                'cancel ride: status 200': (r) => r.status === 200 || r.status === 400,
            });
        });
    }

    sleep(0.5 + Math.random());
}

// ── Summary ─────────────────────────────────────────────────
export function handleSummary(data) {
    return {
        stdout: textSummary(data, { indent: ' ', enableColors: true }),
    };
}

// Inline text summary (k6 built-in)
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
