// Extreme load test for validating Kubernetes HPA horizontal scaling.
//
// This script is designed to be run as a DISTRIBUTED test via the k6-operator.
// Each k6 worker pod runs this script independently.
//
// Target: 20,000 req/s per worker × 50 workers = 1,000,000 req/s total
//
// Usage (standalone, for local testing at lower scale):
//   k6 run scripts/loadtest-1m.js
//
// Usage (distributed, via k6-operator):
//   kubectl apply -f k8s/base/loadtest/

import { check } from 'k6';
import http from 'k6/http';
import { Counter, Gauge, Rate, Trend } from 'k6/metrics';

// ── Custom metrics ──────────────────────────────────────────
const errorRate = new Rate('error_rate');
const reqDuration = new Trend('req_duration', true);
const totalRequests = new Counter('total_requests');
const activeVUs = new Gauge('active_vus');

// ── Configuration ───────────────────────────────────────────
// When run via k6-operator with parallelism: 50, each pod gets this config.
// 20,000 req/s × 50 pods = 1,000,000 req/s total sustained.
export const options = {
    scenarios: {
        // ── Phase 1: Warm-up (30s) ──────────────────────────────
        warmup: {
            executor: 'constant-arrival-rate',
            rate: 1000,            // 1,000 req/s per pod during warm-up
            timeUnit: '1s',
            duration: '30s',
            preAllocatedVUs: 200,
            maxVUs: 500,
            startTime: '0s',
            tags: { phase: 'warmup' },
        },

        // ── Phase 2: Ramp to target (60s) ───────────────────────
        ramp: {
            executor: 'ramping-arrival-rate',
            startRate: 1000,
            timeUnit: '1s',
            stages: [
                { duration: '30s', target: 10000 },   // ramp to 10K/s
                { duration: '30s', target: 20000 },   // ramp to 20K/s (full blast)
            ],
            preAllocatedVUs: 2000,
            maxVUs: 5000,
            startTime: '30s',
            tags: { phase: 'ramp' },
        },

        // ── Phase 3: Sustained peak (3 min) ─────────────────────
        // This is where HPA should scale aggressively
        sustained: {
            executor: 'constant-arrival-rate',
            rate: 20000,           // 20,000 req/s per pod = 1M total with 50 pods
            timeUnit: '1s',
            duration: '3m',
            preAllocatedVUs: 5000,
            maxVUs: 10000,
            startTime: '1m30s',
            tags: { phase: 'sustained' },
        },

        // ── Phase 4: Cool-down (60s) ────────────────────────────
        // Verifies clean scale-down behavior
        cooldown: {
            executor: 'ramping-arrival-rate',
            startRate: 20000,
            timeUnit: '1s',
            stages: [
                { duration: '30s', target: 5000 },
                { duration: '30s', target: 0 },
            ],
            preAllocatedVUs: 2000,
            maxVUs: 5000,
            startTime: '4m30s',
            tags: { phase: 'cooldown' },
        },
    },

    thresholds: {
        // SLOs under extreme load
        http_req_duration: ['p(95)<1000', 'p(99)<3000'],  // p95 < 1s, p99 < 3s
        error_rate: ['rate<0.05'],                         // < 5% errors
        http_req_failed: ['rate<0.05'],
    },

    // Disable TLS verification for internal cluster traffic
    insecureSkipTLSVerify: true,
    noConnectionReuse: false,   // Reuse connections for higher throughput
};

// ── Target URL ──────────────────────────────────────────────
// Inside the cluster, k6 pods hit the internal ClusterIP service directly.
// Override via K6_BASE_URL env var if needed.
const BASE_URL = __ENV.BASE_URL || 'http://gateway.rideshare.svc.cluster.local:8080';

// ── Pre-registered test credentials ─────────────────────────
// For 1M req/s, we cannot register users on-the-fly.
// These should be seeded into the database before the test.
const TEST_TOKEN = __ENV.TEST_TOKEN || 'test-bearer-token';

function authHeaders() {
    return {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${TEST_TOKEN}`,
        },
    };
}

// ── Request Pool ────────────────────────────────────────────
// Distribute load across different endpoint types to simulate realistic traffic.
// Weights: Health 10%, NearbyDrivers 35%, GetRide 25%, Location 20%, Auth 10%
const endpoints = [
    { weight: 10, fn: healthCheck },
    { weight: 35, fn: nearbyDrivers },
    { weight: 25, fn: getRide },
    { weight: 20, fn: updateLocation },
    { weight: 10, fn: loginAttempt },
];

// Build a weighted lookup array for O(1) random selection
const weightedEndpoints = [];
endpoints.forEach((ep) => {
    for (let i = 0; i < ep.weight; i++) {
        weightedEndpoints.push(ep.fn);
    }
});

// ── Main test function ──────────────────────────────────────
export default function () {
    activeVUs.add(__VU);
    totalRequests.add(1);

    // Pick a random endpoint based on weight distribution
    const idx = Math.floor(Math.random() * weightedEndpoints.length);
    const selectedFn = weightedEndpoints[idx];
    selectedFn();
}

// ── Endpoint functions ──────────────────────────────────────
function healthCheck() {
    const res = http.get(`${BASE_URL}/health`);
    reqDuration.add(res.timings.duration);
    const ok = check(res, {
        'health: status 200': (r) => r.status === 200,
    });
    errorRate.add(!ok);
}

function nearbyDrivers() {
    // Randomize coordinates slightly to avoid Redis cache hits every time
    const lat = 29.4241 + (Math.random() - 0.5) * 0.1;
    const long = -98.4936 + (Math.random() - 0.5) * 0.1;
    const radius = 5 + Math.floor(Math.random() * 15);

    const res = http.get(
        `${BASE_URL}/v1/locations/nearby?lat=${lat}&long=${long}&radius=${radius}`,
        authHeaders(),
    );
    reqDuration.add(res.timings.duration);
    const ok = check(res, {
        'nearby: status 200 or 401': (r) => r.status === 200 || r.status === 401,
    });
    errorRate.add(!ok);
}

function getRide() {
    // Use a fake UUID — will return 404 but exercises the full gRPC pipeline
    const fakeId = `${randomHex(8)}-${randomHex(4)}-4${randomHex(3)}-${randomHex(4)}-${randomHex(12)}`;
    const res = http.get(
        `${BASE_URL}/v1/rides/${fakeId}`,
        authHeaders(),
    );
    reqDuration.add(res.timings.duration);
    const ok = check(res, {
        'get ride: status 200 or 404 or 401': (r) => r.status === 200 || r.status === 404 || r.status === 401,
    });
    errorRate.add(!ok);
}

function updateLocation() {
    const lat = 29.4241 + (Math.random() - 0.5) * 0.05;
    const long = -98.4936 + (Math.random() - 0.5) * 0.05;

    const res = http.post(
        `${BASE_URL}/v1/locations/update`,
        JSON.stringify({ lat, long }),
        authHeaders(),
    );
    reqDuration.add(res.timings.duration);
    const ok = check(res, {
        'location: status 200 or 401 or 403': (r) => r.status === 200 || r.status === 401 || r.status === 403,
    });
    errorRate.add(!ok);
}

function loginAttempt() {
    const res = http.post(
        `${BASE_URL}/v1/auth/login`,
        JSON.stringify({
            email: `loadtest-${__VU}@k6.io`,
            password: 'loadtest123',
        }),
        { headers: { 'Content-Type': 'application/json' } },
    );
    reqDuration.add(res.timings.duration);
    const ok = check(res, {
        'login: status 200 or 401': (r) => r.status === 200 || r.status === 401,
    });
    errorRate.add(!ok);
}

// ── Helpers ─────────────────────────────────────────────────
function randomHex(len) {
    let result = '';
    const chars = '0123456789abcdef';
    for (let i = 0; i < len; i++) {
        result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
}

// ── Summary ─────────────────────────────────────────────────
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';

export function handleSummary(data) {
    const totalReqs = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;
    const duration = data.state ? data.state.testRunDurationMs / 1000 : 0;
    const rps = duration > 0 ? Math.round(totalReqs / duration) : 0;

    console.log(`\n╔══════════════════════════════════════════╗`);
    console.log(`║   LOAD TEST COMPLETE                     ║`);
    console.log(`║   Total Requests: ${String(totalReqs).padStart(20)}  ║`);
    console.log(`║   Avg RPS:        ${String(rps).padStart(20)}  ║`);
    console.log(`║   Duration:       ${String(Math.round(duration) + 's').padStart(20)}  ║`);
    console.log(`╚══════════════════════════════════════════╝\n`);

    return {
        stdout: textSummary(data, { indent: ' ', enableColors: true }),
    };
}
