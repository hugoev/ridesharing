#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
# run-loadtest.sh — One-command distributed 1M req/s load test
# ═══════════════════════════════════════════════════════════════
#
# Usage:
#   ./scripts/run-loadtest.sh              # Full 1M req/s test (needs k8s cluster)
#   ./scripts/run-loadtest.sh --local      # Local test at ~1K req/s (no cluster needed)
#   ./scripts/run-loadtest.sh --watch      # Just watch HPA scaling (no new test)
#   ./scripts/run-loadtest.sh --cleanup    # Remove load test resources
#
set -euo pipefail

NAMESPACE="rideshare"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="$(cd "$SCRIPT_DIR/../k8s/base/loadtest" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[✗]${NC} $1"; }
info() { echo -e "${CYAN}[→]${NC} $1"; }

# ── Local mode ───────────────────────────────────────────────
run_local() {
  log "Running LOCAL load test (standalone k6, ~1K req/s)"
  warn "This will NOT test horizontal scaling. Use a k8s cluster for real scaling tests."
  echo ""

  if ! command -v k6 &>/dev/null; then
    info "Installing k6..."
    brew install k6 2>/dev/null || {
      err "Failed to install k6. Install manually: https://k6.io/docs/getting-started/installation/"
      exit 1
    }
  fi

  k6 run \
    --vus 100 \
    --duration 2m \
    --env BASE_URL=http://localhost:8080 \
    "$SCRIPT_DIR/loadtest-1m.js"
}

# ── Watch mode ───────────────────────────────────────────────
watch_scaling() {
  log "Watching HPA scaling in real-time..."
  echo ""
  echo "╔══════════════════════════════════════════════════════════╗"
  echo "║  Press Ctrl+C to stop watching                         ║"
  echo "╚══════════════════════════════════════════════════════════╝"
  echo ""

  # Show HPA status every 2 seconds
  watch -n 2 "kubectl get hpa -n $NAMESPACE -o wide && echo '' && kubectl get pods -n $NAMESPACE --sort-by=.metadata.creationTimestamp | tail -20"
}

# ── Cleanup ──────────────────────────────────────────────────
cleanup() {
  warn "Cleaning up load test resources..."
  kubectl delete testrun loadtest-1m -n "$NAMESPACE" --ignore-not-found
  kubectl delete configmap k6-loadtest-script -n "$NAMESPACE" --ignore-not-found
  log "Cleanup complete."
}

# ── Full distributed test ────────────────────────────────────
run_distributed() {
  echo ""
  echo "╔══════════════════════════════════════════════════════════╗"
  echo "║       🔥 1,000,000 req/s DISTRIBUTED LOAD TEST 🔥      ║"
  echo "╠══════════════════════════════════════════════════════════╣"
  echo "║  50 k6 worker pods × 20,000 req/s each                 ║"
  echo "║  Duration: ~5.5 minutes (warmup → ramp → sustained)    ║"
  echo "╚══════════════════════════════════════════════════════════╝"
  echo ""

  # Step 1: Check prerequisites
  info "Checking prerequisites..."
  if ! command -v kubectl &>/dev/null; then
    err "kubectl not found. Install it first."
    exit 1
  fi

  if ! kubectl cluster-info &>/dev/null 2>&1; then
    err "No Kubernetes cluster connection. Configure kubectl first."
    exit 1
  fi
  log "Kubernetes cluster connected."

  # Step 2: Install k6-operator if not present
  if ! kubectl get crd testruns.k6.io &>/dev/null 2>&1; then
    info "Installing k6-operator..."
    kubectl apply -f https://github.com/grafana/k6-operator/releases/latest/download/bundle.yaml
    info "Waiting for k6-operator to be ready..."
    kubectl wait --for=condition=Available deployment/k6-operator-controller-manager \
      -n k6-operator-system --timeout=120s
    log "k6-operator installed."
  else
    log "k6-operator already installed."
  fi

  # Step 3: Deploy the k6 script ConfigMap
  info "Deploying k6 test script ConfigMap..."
  kubectl apply -f "$K8S_DIR/k6-script-configmap.yaml"
  log "ConfigMap deployed."

  # Step 4: Clean up any previous test run
  kubectl delete testrun loadtest-1m -n "$NAMESPACE" --ignore-not-found 2>/dev/null || true

  # Step 5: Create the TestRun
  info "Starting distributed load test (50 workers)..."
  kubectl apply -f "$K8S_DIR/testrun-1m.yaml"
  log "TestRun created!"

  echo ""
  echo "╔══════════════════════════════════════════════════════════╗"
  echo "║  TEST IS RUNNING! Monitor with these commands:          ║"
  echo "╠══════════════════════════════════════════════════════════╣"
  echo "║                                                         ║"
  echo "║  # Watch k6 worker pods spawn                           ║"
  echo "║  kubectl get pods -n rideshare -l k6_cr=loadtest-1m -w  ║"
  echo "║                                                         ║"
  echo "║  # Watch HPA scaling in real-time                       ║"
  echo "║  kubectl get hpa -n rideshare -w                        ║"
  echo "║                                                         ║"
  echo "║  # Tail k6 worker logs                                  ║"
  echo "║  kubectl logs -n rideshare -l k6_cr=loadtest-1m -f      ║"
  echo "║                                                         ║"
  echo "║  # Open Grafana dashboard                               ║"
  echo "║  kubectl port-forward svc/grafana 3000:3000 -n rideshare║"
  echo "║  → http://localhost:3000 (Dashboard: HPA Scaling)       ║"
  echo "║                                                         ║"
  echo "╚══════════════════════════════════════════════════════════╝"
  echo ""

  # Step 6: Auto-watch HPA
  info "Auto-watching HPA scaling (Ctrl+C to stop)..."
  echo ""
  kubectl get hpa -n "$NAMESPACE" -w
}

# ── Main ─────────────────────────────────────────────────────
case "${1:-}" in
  --local)   run_local ;;
  --watch)   watch_scaling ;;
  --cleanup) cleanup ;;
  --help|-h)
    echo "Usage: $0 [--local|--watch|--cleanup|--help]"
    echo ""
    echo "  (no args)   Run full 1M req/s distributed test in Kubernetes"
    echo "  --local     Run standalone k6 test (~1K req/s, no k8s needed)"
    echo "  --watch     Watch HPA scaling without starting a new test"
    echo "  --cleanup   Remove all load test resources"
    ;;
  *)         run_distributed ;;
esac
