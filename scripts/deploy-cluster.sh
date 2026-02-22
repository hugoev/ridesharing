#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
# deploy-cluster.sh — Full cluster setup: Kind + Build + Deploy
# ═══════════════════════════════════════════════════════════════
#
# Usage:
#   ./scripts/deploy-cluster.sh            # Full setup
#   ./scripts/deploy-cluster.sh --rebuild  # Rebuild images only
#   ./scripts/deploy-cluster.sh --destroy  # Tear down cluster
#
set -euo pipefail

CLUSTER_NAME="rideshare"
NAMESPACE="rideshare"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

SERVICES=("gateway" "auth" "user" "ride" "location" "payment")

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[✗]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[→]${NC} $1"; }
header() { echo -e "\n${BOLD}═══ $1 ═══${NC}\n"; }

# ── Destroy ──────────────────────────────────────────────────
destroy() {
  warn "Destroying Kind cluster '$CLUSTER_NAME'..."
  kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
  log "Cluster destroyed."
  exit 0
}

# ── Prerequisites ────────────────────────────────────────────
check_prereqs() {
  header "Checking Prerequisites"

  for cmd in docker kind kubectl; do
    if command -v "$cmd" &>/dev/null; then
      log "$cmd found"
    else
      err "$cmd is required but not installed."
    fi
  done

  if ! docker info &>/dev/null 2>&1; then
    err "Docker is not running. Start Docker Desktop first."
  fi
  log "Docker is running."
}

# ── Create Kind Cluster ─────────────────────────────────────
create_cluster() {
  header "Creating Kind Cluster (1 control-plane + 3 workers)"

  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    log "Cluster '$CLUSTER_NAME' already exists. Skipping creation."
    return
  fi

  kind create cluster --config "$PROJECT_DIR/kind-config.yaml"
  log "Cluster created!"

  # Wait for nodes to be ready
  info "Waiting for nodes to be ready..."
  kubectl wait --for=condition=Ready nodes --all --timeout=120s
  log "All nodes ready."
}

# ── Install metrics-server (required for HPA) ───────────────
install_metrics_server() {
  header "Installing Metrics Server (required for HPA)"

  if kubectl get deployment metrics-server -n kube-system &>/dev/null 2>&1; then
    log "metrics-server already installed."
    return
  fi

  kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

  # Patch for Kind (insecure kubelet TLS)
  kubectl patch deployment metrics-server -n kube-system \
    --type='json' \
    -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

  info "Waiting for metrics-server to be ready..."
  kubectl wait --for=condition=Available deployment/metrics-server \
    -n kube-system --timeout=120s
  log "metrics-server ready."
}

# ── Build Docker Images ──────────────────────────────────────
build_images() {
  header "Building Docker Images (6 microservices)"

  cd "$PROJECT_DIR"
  for svc in "${SERVICES[@]}"; do
    info "Building rideshare-${svc}:latest ..."
    docker build \
      --build-arg SERVICE_NAME="$svc" \
      -t "rideshare-${svc}:latest" \
      -f Dockerfile \
      . \
      --quiet
    log "rideshare-${svc}:latest built."
  done
}

# ── Load Images into Kind ────────────────────────────────────
load_images() {
  header "Loading Images into Kind Cluster"

  for svc in "${SERVICES[@]}"; do
    info "Loading rideshare-${svc}:latest ..."
    kind load docker-image "rideshare-${svc}:latest" --name "$CLUSTER_NAME"
    log "rideshare-${svc}:latest loaded."
  done
}

# ── Deploy via Kustomize ─────────────────────────────────────
deploy_stack() {
  header "Deploying Full Stack via Kustomize"

  # Create namespace if it doesn't exist
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

  # Apply kustomize
  info "Applying k8s/base manifests..."
  kubectl apply -k "$PROJECT_DIR/k8s/base" 2>&1 | head -30
  echo "  ... (truncated)"
  log "Manifests applied."

  # Wait for deployments
  info "Waiting for Gateway deployment to be ready..."
  kubectl rollout status deployment/gateway -n "$NAMESPACE" --timeout=120s 2>/dev/null || {
    warn "Gateway not ready yet — this may be normal if infrastructure (Postgres/Redis/Kafka) is still starting."
  }
}

# ── Summary ──────────────────────────────────────────────────
print_summary() {
  header "DEPLOYMENT COMPLETE"

  echo -e "${BOLD}Cluster:${NC}  $CLUSTER_NAME (Kind)"
  echo -e "${BOLD}Nodes:${NC}    $(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ') nodes"
  echo ""

  echo -e "${BOLD}Pod Status:${NC}"
  kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | \
    awk '{printf "  %-40s %s\n", $1, $3}' || echo "  (waiting for pods to start)"
  echo ""

  echo -e "${BOLD}Services:${NC}"
  kubectl get svc -n "$NAMESPACE" --no-headers 2>/dev/null | \
    awk '{printf "  %-40s %s\n", $1, $5}' || true
  echo ""

  echo "╔══════════════════════════════════════════════════════════╗"
  echo "║  Access Points (via Kind port mapping):                 ║"
  echo "║                                                         ║"
  echo "║  API Gateway:  http://localhost:8080                    ║"
  echo "║  Grafana:      http://localhost:3000                    ║"
  echo "║  Prometheus:   http://localhost:9090                    ║"
  echo "║                                                         ║"
  echo "║  Run load test:                                         ║"
  echo "║    ./scripts/run-loadtest.sh --local                    ║"
  echo "║                                                         ║"
  echo "║  Watch HPA scaling:                                     ║"
  echo "║    kubectl get hpa -n rideshare -w                      ║"
  echo "╚══════════════════════════════════════════════════════════╝"
}

# ── Main ─────────────────────────────────────────────────────
case "${1:-}" in
  --destroy)  destroy ;;
  --rebuild)
    build_images
    load_images
    info "Restarting deployments..."
    for svc in "${SERVICES[@]}"; do
      kubectl rollout restart deployment/"$svc" -n "$NAMESPACE" 2>/dev/null || true
    done
    log "Images rebuilt and reloaded."
    ;;
  --help|-h)
    echo "Usage: $0 [--rebuild|--destroy|--help]"
    echo ""
    echo "  (no args)   Full setup: create cluster, build images, deploy stack"
    echo "  --rebuild   Rebuild images and reload into existing cluster"
    echo "  --destroy   Tear down the Kind cluster completely"
    ;;
  *)
    check_prereqs
    create_cluster
    install_metrics_server
    build_images
    load_images
    deploy_stack
    print_summary
    ;;
esac
