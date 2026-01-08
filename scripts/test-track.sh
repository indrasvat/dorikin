#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
#  🏎️ DORIKIN TEST TRACK - Local K8s Drift Testing Environment
# ═══════════════════════════════════════════════════════════════════════════════
#
#  A comprehensive test harness for dorikin that:
#  - Sets up a local k3s cluster via Colima (isolated profile)
#  - Deploys sample Kubernetes resources
#  - Creates intentional DRIFT scenarios to test dorikin
#  - Provides interactive controls for testing
#
#  Requirements: macOS with Homebrew, Colima, kubectl
#
#  Usage:
#    ./scripts/test-track.sh [command]
#
#  Commands:
#    setup      - Setup test environment (colima + k3s + resources)
#    drift      - Apply drift scenarios (interactive menu)
#    reset      - Reset cluster to match manifests (remove drift)  
#    status     - Show current environment status
#    scan       - Run dorikin scan against test environment
#    tui        - Launch dorikin TUI against test environment
#    cleanup    - Destroy test environment completely
#    help       - Show this help message
#
# ═══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ═══════════════════════════════════════════════════════════════════════════════
# 🎨 DORIKIN COLOR PALETTE - AE86 Panda Trueno Aesthetic
# ═══════════════════════════════════════════════════════════════════════════════

NC='\033[0m'           # Reset

# Panda Base
BLACK='\033[0;30m'
WHITE='\033[1;37m'
GRAY='\033[0;90m'

# Tsuchiya Jade (#00A86B approximation)
JADE='\033[38;5;35m'
JADE_BOLD='\033[1;38;5;35m'

# Hazard Yellow (#FFD700 - Shoshinsha mark)
YELLOW='\033[38;5;220m'
YELLOW_BOLD='\033[1;38;5;220m'

# Brake Red (#FF2D2D)
RED='\033[38;5;196m'
RED_BOLD='\033[1;38;5;196m'

# JDM Purple (#9B59B6)
PURPLE='\033[38;5;133m'

# Neon Cyan (#00FFFF - underglow)
CYAN='\033[38;5;51m'
CYAN_BOLD='\033[1;38;5;51m'

# Backgrounds for banners
BG_JADE='\033[48;5;35m'
BG_YELLOW='\033[48;5;220m'
BG_RED='\033[48;5;196m'
BG_BLACK='\033[48;5;233m'

# ═══════════════════════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." 2>/dev/null && pwd || echo "${SCRIPT_DIR}")"

# Colima profile (isolated from other colima instances)
COLIMA_PROFILE="dorikin-ae86"

# Test namespace
TEST_NAMESPACE="dorikin-garage"

# Manifest directories
MANIFEST_DIR="${PROJECT_ROOT}/testdata/track-manifests"
SMART_DIFF_DIR="${PROJECT_ROOT}/testdata/smart-diff"

# Dorikin binary
DORIKIN_BIN="${PROJECT_ROOT}/bin/dorikin"

# ═══════════════════════════════════════════════════════════════════════════════
# 🖨️ Output Functions - Racing Dashboard Style
# ═══════════════════════════════════════════════════════════════════════════════

banner() {
    echo -e "${JADE_BOLD}"
    cat << 'EOF'
    
  ██████╗  ██████╗ ██████╗ ██╗██╗  ██╗██╗███╗   ██╗
  ██╔══██╗██╔═══██╗██╔══██╗██║██║ ██╔╝██║████╗  ██║
  ██║  ██║██║   ██║██████╔╝██║█████╔╝ ██║██╔██╗ ██║
  ██║  ██║██║   ██║██╔══██╗██║██╔═██╗ ██║██║╚██╗██║
  ██████╔╝╚██████╔╝██║  ██║██║██║  ██╗██║██║ ╚████║
  ╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚═╝╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝
    
EOF
    echo -e "  ${WHITE}🏎️  AE86 TEST TRACK${NC} ${GRAY}// Local K8s Drift Testing${NC}"
    echo -e "${NC}"
}

section() {
    echo ""
    echo -e "${BG_BLACK}${JADE_BOLD} ═══ $1 ═══ ${NC}"
    echo ""
}

info()    { echo -e "${JADE}🏁${NC} $1"; }
success() { echo -e "${JADE_BOLD}✓${NC} ${JADE}$1${NC}"; }
warn()    { echo -e "${YELLOW_BOLD}🚨${NC} ${YELLOW}$1${NC}"; }
error()   { echo -e "${RED_BOLD}💥${NC} ${RED}$1${NC}"; }
step()    { echo -e "${CYAN}▶${NC} $1"; }
substep() { echo -e "  ${GRAY}├─${NC} $1"; }
substep_last() { echo -e "  ${GRAY}└─${NC} $1"; }

drift_alert() {
    echo ""
    echo -e "${BG_YELLOW}${BLACK}  ⚡ DRIFT APPLIED ⚡  ${NC}"
    echo ""
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🔧 Utility Functions
# ═══════════════════════════════════════════════════════════════════════════════

command_exists() { command -v "$1" &>/dev/null; }

check_macos() {
    if [[ "$(uname)" != "Darwin" ]]; then
        error "This script requires macOS. Detected: $(uname)"
        exit 1
    fi
}

colima_running() {
    colima list 2>/dev/null | grep -q "${COLIMA_PROFILE}.*Running"
}

get_context() { echo "colima-${COLIMA_PROFILE}"; }

# kubectl with our context
kc() { kubectl --context="$(get_context)" "$@"; }

wait_ready() {
    local kind=$1 name=$2 ns=${3:-$TEST_NAMESPACE} timeout=${4:-90}
    step "Waiting for ${kind}/${name}..."
    if kc wait "${kind}/${name}" -n "${ns}" --for=condition=available --timeout="${timeout}s" &>/dev/null; then
        substep_last "${JADE}Ready${NC}"
        return 0
    fi
    warn "${kind}/${name} not ready after ${timeout}s"
    return 1
}

# ═══════════════════════════════════════════════════════════════════════════════
# 📋 Pre-flight Checks
# ═══════════════════════════════════════════════════════════════════════════════

preflight() {
    section "Pre-flight Checks"
    
    check_macos
    success "macOS $(sw_vers -productVersion)"
    
    local missing=()
    
    step "Checking required tools..."
    
    if command_exists brew; then
        substep "Homebrew: ${JADE}✓${NC}"
    else
        missing+=("brew")
        substep "Homebrew: ${RED}✗${NC}"
    fi
    
    if command_exists colima; then
        substep "Colima: ${JADE}$(colima version 2>/dev/null | head -1 | awk '{print $NF}')${NC}"
    else
        missing+=("colima")
        substep "Colima: ${RED}✗${NC}"
    fi
    
    if command_exists kubectl; then
        substep "kubectl: ${JADE}$(kubectl version --client -o json 2>/dev/null | grep gitVersion | head -1 | awk -F'"' '{print $4}')${NC}"
    else
        missing+=("kubectl")
        substep "kubectl: ${RED}✗${NC}"
    fi
    
    if [[ -f "${DORIKIN_BIN}" ]]; then
        substep_last "dorikin: ${JADE}${DORIKIN_BIN}${NC}"
    else
        substep_last "dorikin: ${YELLOW}not built (run 'make build')${NC}"
    fi
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        echo ""
        error "Missing: ${missing[*]}"
        echo ""
        echo "Install with:"
        for tool in "${missing[@]}"; do
            case $tool in
                brew)   echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"" ;;
                colima) echo "  brew install colima docker" ;;
                kubectl) echo "  brew install kubectl" ;;
            esac
        done
        exit 1
    fi
    
    success "All checks passed!"
}

# ═══════════════════════════════════════════════════════════════════════════════
# 📝 Create Test Manifests
# ═══════════════════════════════════════════════════════════════════════════════

create_manifests() {
    step "Creating test manifests..."
    mkdir -p "${MANIFEST_DIR}"

    # ─────────────────────────────────────────────────────────────────────────
    # nginx Deployment - Primary test target
    # ─────────────────────────────────────────────────────────────────────────
    cat > "${MANIFEST_DIR}/01-nginx-deployment.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: dorikin-garage
  labels:
    app: nginx
    team: ae86
    track: touge
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
        team: ae86
    spec:
      containers:
      - name: nginx
        image: nginx:1.25.0
        ports:
        - containerPort: 80
          name: http
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
        env:
        - name: DRIVER
          value: "Tsuchiya"
        - name: CAR
          value: "AE86"
        livenessProbe:
          httpGet:
            path: /
            port: 80
          initialDelaySeconds: 5
          periodSeconds: 10
YAML
    substep "01-nginx-deployment.yaml"

    # ─────────────────────────────────────────────────────────────────────────
    # nginx Service
    # ─────────────────────────────────────────────────────────────────────────
    cat > "${MANIFEST_DIR}/02-nginx-service.yaml" << 'YAML'
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: dorikin-garage
  labels:
    app: nginx
    team: ae86
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
    name: http
  type: ClusterIP
YAML
    substep "02-nginx-service.yaml"

    # ─────────────────────────────────────────────────────────────────────────
    # ConfigMap - Racing config
    # ─────────────────────────────────────────────────────────────────────────
    cat > "${MANIFEST_DIR}/03-racing-config.yaml" << 'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: racing-config
  namespace: dorikin-garage
  labels:
    app: racing
    team: ae86
data:
  TRACK: "Akina"
  TECHNIQUE: "Gutter Run"
  DRIFT_MODE: "disabled"
  MAX_RPM: "8000"
  TIRES: "Potenza RE-71RS"
YAML
    substep "03-racing-config.yaml"

    # ─────────────────────────────────────────────────────────────────────────
    # Redis - Secondary deployment for variety
    # ─────────────────────────────────────────────────────────────────────────
    cat > "${MANIFEST_DIR}/04-redis.yaml" << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: dorikin-garage
  labels:
    app: redis
    team: ae86
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
        team: ae86
    spec:
      containers:
      - name: redis
        image: redis:7.2-alpine
        ports:
        - containerPort: 6379
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: dorikin-garage
  labels:
    app: redis
    team: ae86
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
  type: ClusterIP
YAML
    substep "04-redis.yaml"

    # ─────────────────────────────────────────────────────────────────────────
    # HPA - Autoscaler
    # ─────────────────────────────────────────────────────────────────────────
    cat > "${MANIFEST_DIR}/05-nginx-hpa.yaml" << 'YAML'
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: nginx-hpa
  namespace: dorikin-garage
  labels:
    app: nginx
    team: ae86
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: nginx
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
YAML
    substep_last "05-nginx-hpa.yaml"

    success "Manifests created in ${MANIFEST_DIR}"
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🚀 Setup Functions
# ═══════════════════════════════════════════════════════════════════════════════

start_colima() {
    step "Starting Colima k3s cluster..."
    
    if colima_running; then
        substep_last "Already running"
        return 0
    fi
    
    colima start \
        --profile "${COLIMA_PROFILE}" \
        --kubernetes \
        --kubernetes-version "v1.29.0+k3s1" \
        --cpu 2 \
        --memory 4 \
        --disk 20 \
        --runtime containerd \
        2>&1 | while IFS= read -r line; do substep "$line"; done
    
    # Wait for k8s
    step "Waiting for Kubernetes API..."
    local i=0
    while ! kc cluster-info &>/dev/null && [[ $i -lt 30 ]]; do
        sleep 2
        ((i++))
    done
    
    if [[ $i -ge 30 ]]; then
        error "Kubernetes did not start"
        exit 1
    fi
    
    substep_last "Kubernetes ready"
    success "Colima k3s running (profile: ${COLIMA_PROFILE})"
}

deploy_resources() {
    step "Deploying test resources..."
    
    # Namespace
    kc create namespace "${TEST_NAMESPACE}" --dry-run=client -o yaml | kc apply -f - &>/dev/null
    substep "Namespace: ${TEST_NAMESPACE}"
    
    # Apply manifests
    kc apply -f "${MANIFEST_DIR}/" -n "${TEST_NAMESPACE}" 2>&1 | while IFS= read -r line; do
        substep "$line"
    done
    
    # Wait for deployments
    wait_ready deployment nginx "${TEST_NAMESPACE}" 120 || true
    wait_ready deployment redis "${TEST_NAMESPACE}" 120 || true
    
    success "Resources deployed!"
}

setup() {
    banner
    section "Setting Up AE86 Test Track"
    
    preflight
    create_manifests
    start_colima
    deploy_resources
    
    section "Setup Complete! 🏁"
    
    echo -e "${JADE}The test track is ready!${NC}"
    echo ""
    echo -e "  Context:   ${CYAN}$(get_context)${NC}"
    echo -e "  Namespace: ${CYAN}${TEST_NAMESPACE}${NC}"
    echo -e "  Manifests: ${CYAN}${MANIFEST_DIR}${NC}"
    echo ""
    echo -e "${WHITE}Quick Start:${NC}"
    echo -e "  ${JADE}1.${NC} ${CYAN}$0 scan${NC}   - Verify baseline (all ${JADE}IN_SYNC${NC})"
    echo -e "  ${JADE}2.${NC} ${CYAN}$0 drift${NC}  - Apply drift scenarios"
    echo -e "  ${JADE}3.${NC} ${CYAN}$0 scan${NC}   - See drift detected! 🚨"
    echo -e "  ${JADE}4.${NC} ${CYAN}$0 tui${NC}    - Interactive exploration"
    echo ""
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🚨 DRIFT SCENARIOS - Intentional configuration drift!
# ═══════════════════════════════════════════════════════════════════════════════

drift_replicas() {
    info "Drift: Scaling nginx replicas 3 → 5"
    kc scale deployment/nginx -n "${TEST_NAMESPACE}" --replicas=5
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} spec.replicas: ${JADE}3${NC} → ${YELLOW}5${NC}"
}

drift_image() {
    info "Drift: Updating nginx image 1.25.0 → 1.26.0"
    kc set image deployment/nginx nginx=nginx:1.26.0 -n "${TEST_NAMESPACE}"
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} image: ${JADE}nginx:1.25.0${NC} → ${YELLOW}nginx:1.26.0${NC}"
}

drift_configmap() {
    info "Drift: Modifying racing-config"
    kc patch configmap racing-config -n "${TEST_NAMESPACE}" --type merge \
        -p '{"data":{"DRIFT_MODE":"TOKYO_DRIFT","TRACK":"Hakone"}}'
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} DRIFT_MODE: ${JADE}disabled${NC} → ${YELLOW}TOKYO_DRIFT${NC}"
    echo -e "  ${YELLOW}⚡${NC} TRACK: ${JADE}Akina${NC} → ${YELLOW}Hakone${NC}"
}

drift_env() {
    info "Drift: Modifying nginx env vars"
    kc set env deployment/nginx -n "${TEST_NAMESPACE}" DRIVER="Keiichi_Tsuchiya" CAR="AE86_Trueno"
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} DRIVER: ${JADE}Tsuchiya${NC} → ${YELLOW}Keiichi_Tsuchiya${NC}"
    echo -e "  ${YELLOW}⚡${NC} CAR: ${JADE}AE86${NC} → ${YELLOW}AE86_Trueno${NC}"
}

drift_resources() {
    info "Drift: Changing nginx memory limits"
    kc patch deployment/nginx -n "${TEST_NAMESPACE}" --type json \
        -p '[{"op":"replace","path":"/spec/template/spec/containers/0/resources/limits/memory","value":"256Mi"}]'
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} limits.memory: ${JADE}128Mi${NC} → ${YELLOW}256Mi${NC}"
}

drift_service() {
    info "Drift: Changing nginx service type"
    kc patch service/nginx -n "${TEST_NAMESPACE}" -p '{"spec":{"type":"NodePort"}}'
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} type: ${JADE}ClusterIP${NC} → ${YELLOW}NodePort${NC}"
}

drift_extra() {
    info "Drift: Adding rogue deployment (not in manifests)"
    kc create deployment rival-car --image=busybox:latest -n "${TEST_NAMESPACE}" -- sleep infinity 2>/dev/null || true
    kc label deployment/rival-car -n "${TEST_NAMESPACE}" team=unknown --overwrite 2>/dev/null || true
    drift_alert
    echo -e "  ${PURPLE}➕${NC} deployment/rival-car ${GRAY}(exists in cluster only)${NC}"
}

drift_missing() {
    info "Drift: Deleting redis (will show as MISSING)"
    kc delete deployment/redis service/redis -n "${TEST_NAMESPACE}" --ignore-not-found
    drift_alert
    echo -e "  ${RED}🛑${NC} deployment/redis ${GRAY}(exists in manifests only)${NC}"
    echo -e "  ${RED}🛑${NC} service/redis ${GRAY}(exists in manifests only)${NC}"
}

drift_quantity_equiv() {
    info "Drift: Changing nginx resources to equivalent values (should be NO drift)"
    # Change 128Mi to equivalent bytes (134217728), 200m to 0.2
    kc patch deployment/nginx -n "${TEST_NAMESPACE}" --type json \
        -p '[{"op":"replace","path":"/spec/template/spec/containers/0/resources/limits/memory","value":"134217728"},{"op":"replace","path":"/spec/template/spec/containers/0/resources/limits/cpu","value":"0.2"}]'
    echo ""
    echo -e "  ${JADE}ℹ${NC} limits.memory: ${JADE}128Mi${NC} → ${JADE}134217728 (equivalent bytes)${NC}"
    echo -e "  ${JADE}ℹ${NC} limits.cpu: ${JADE}200m${NC} → ${JADE}0.2 (equivalent decimal)${NC}"
    echo -e "  ${JADE}✓${NC} Dorikin should detect ${JADE}NO drift${NC} (quantity normalization)"
}

drift_reorder_env() {
    info "Drift: Reordering nginx env vars (should be NO drift)"
    # Get current env, reorder, and patch
    kc patch deployment/nginx -n "${TEST_NAMESPACE}" --type json \
        -p '[{"op":"replace","path":"/spec/template/spec/containers/0/env","value":[{"name":"CAR","value":"AE86"},{"name":"DRIVER","value":"Tsuchiya"}]}]'
    echo ""
    echo -e "  ${JADE}ℹ${NC} env order: ${JADE}[DRIVER, CAR]${NC} → ${JADE}[CAR, DRIVER]${NC}"
    echo -e "  ${JADE}✓${NC} Dorikin should detect ${JADE}NO drift${NC} (content-based array matching)"
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🎯 SMART DIFF SCENARIOS - Test different render modes
# ═══════════════════════════════════════════════════════════════════════════════

drift_configmap_yaml() {
    info "Drift: Modifying embedded YAML in app-config ConfigMap"

    # First, deploy if not present
    if ! kc get configmap app-config -n "${TEST_NAMESPACE}" &>/dev/null; then
        step "Deploying app-config ConfigMap..."
        kc apply -f "${PROJECT_ROOT}/testdata/smart-diff/03-configmap-yaml.yaml" -n "${TEST_NAMESPACE}" 2>/dev/null || true
    fi

    # Patch with significant YAML changes
    kc patch configmap app-config -n "${TEST_NAMESPACE}" --type merge \
        -p '{"data":{"application.yml":"# Spring Boot Application Configuration\n# Environment: PRODUCTION - MODIFIED!\n\nspring:\n  application:\n    name: myapp-prod\n    version: 2.0.0\n\n  profiles:\n    active: production\n\n  datasource:\n    url: jdbc:postgresql://db-prod:5432/myapp_prod\n    username: ${DB_USER}\n    password: ${DB_PASSWORD}\n    hikari:\n      minimum-idle: 10\n      maximum-pool-size: 50\n      idle-timeout: 60000\n\n  redis:\n    host: redis-prod\n    port: 6379\n    cluster:\n      enabled: true\n      nodes:\n        - redis-prod-0:6379\n        - redis-prod-1:6379\n        - redis-prod-2:6379\n\n  kafka:\n    bootstrap-servers: kafka-prod-0:9092,kafka-prod-1:9092\n    consumer:\n      group-id: myapp-prod-consumer\n\nserver:\n  port: 8443\n  ssl:\n    enabled: true\n    key-store: /etc/ssl/keystore.p12\n\nmanagement:\n  endpoints:\n    web:\n      exposure:\n        include: health,info,metrics,prometheus\n\ncache:\n  type: redis\n  redis:\n    host: redis-prod\n    ttl: 3600\n\nlogging:\n  level:\n    root: WARN\n    com.myapp: INFO\n"}}' 2>/dev/null || true

    drift_alert
    echo -e "  ${YELLOW}⚡${NC} application.yml modified (staging → production config)"
    echo -e "  ${CYAN}→${NC} Tests ${CYAN}ModeUnifiedDiff${NC} for embedded YAML"
}

drift_sidecar_inject() {
    info "Drift: Simulating Istio sidecar injection (adding container)"

    # First, deploy if not present
    if ! kc get deployment sidecar-test -n "${TEST_NAMESPACE}" &>/dev/null; then
        step "Deploying sidecar-test Deployment..."
        kc apply -f "${PROJECT_ROOT}/testdata/smart-diff/04-sidecar-base.yaml" -n "${TEST_NAMESPACE}" 2>/dev/null || true
    fi

    # Inject a simulated sidecar container
    kc patch deployment sidecar-test -n "${TEST_NAMESPACE}" --type json \
        -p '[{"op":"add","path":"/spec/template/spec/containers/-","value":{"name":"istio-proxy","image":"docker.io/istio/proxyv2:1.20.0","ports":[{"containerPort":15001,"name":"envoy","protocol":"TCP"},{"containerPort":15006,"name":"envoy-mtls","protocol":"TCP"},{"containerPort":15090,"name":"http-envoy-prom","protocol":"TCP"}],"args":["proxy","sidecar","--domain","$(POD_NAMESPACE).svc.cluster.local","--proxyLogLevel","warning","--proxyComponentLogLevel","misc:error"],"env":[{"name":"POD_NAME","valueFrom":{"fieldRef":{"fieldPath":"metadata.name"}}},{"name":"POD_NAMESPACE","valueFrom":{"fieldRef":{"fieldPath":"metadata.namespace"}}},{"name":"ISTIO_META_MESH_ID","value":"cluster.local"}],"resources":{"requests":{"cpu":"10m","memory":"40Mi"},"limits":{"cpu":"2000m","memory":"1Gi"}},"securityContext":{"runAsUser":1337,"runAsGroup":1337}}}]' 2>/dev/null || true

    drift_alert
    echo -e "  ${PURPLE}➕${NC} Container ${CYAN}istio-proxy${NC} injected"
    echo -e "  ${CYAN}→${NC} Tests ${CYAN}ModeStructuralAdd${NC} rendering"
}

drift_replace_template() {
    info "Drift: Complete template replacement (substantially different)"

    # First, deploy if not present
    if ! kc get deployment replacement-test -n "${TEST_NAMESPACE}" &>/dev/null; then
        step "Deploying replacement-test Deployment..."
        kc apply -f "${PROJECT_ROOT}/testdata/smart-diff/06-replacement-deploy.yaml" -n "${TEST_NAMESPACE}" 2>/dev/null || true
    fi

    # Replace with completely different template
    kc patch deployment replacement-test -n "${TEST_NAMESPACE}" --type strategic \
        -p '{"spec":{"replicas":3,"strategy":{"type":"RollingUpdate","rollingUpdate":{"maxSurge":"25%","maxUnavailable":"25%"}},"template":{"metadata":{"labels":{"version":"v3.0.0","architecture":"microservices"},"annotations":{"config-hash":"completely-new-hash"}},"spec":{"containers":[{"name":"app","image":"totally-different-image:v3.0.0","command":["/bin/new-entrypoint","--mode=production","--workers=4"],"ports":[{"containerPort":8443,"name":"https"},{"containerPort":9090,"name":"metrics"},{"containerPort":5000,"name":"grpc"}],"env":[{"name":"MODE","value":"production"},{"name":"WORKERS","value":"4"},{"name":"ENABLE_TLS","value":"true"},{"name":"LOG_FORMAT","value":"structured"}],"resources":{"requests":{"cpu":"500m","memory":"1Gi"},"limits":{"cpu":"2000m","memory":"4Gi"}},"volumeMounts":[{"name":"tls-certs","mountPath":"/etc/tls","readOnly":true}]}],"volumes":[{"name":"tls-certs","secret":{"secretName":"app-tls"}}]}}}}' 2>/dev/null || true

    drift_alert
    echo -e "  ${YELLOW}⚡${NC} Template substantially rewritten (>75% different)"
    echo -e "  ${CYAN}→${NC} Tests ${CYAN}ModeReplacement${NC} rendering"
}

drift_all() {
    section "Applying ALL Drift Scenarios"
    warn "Maximum drift incoming!"
    echo ""

    drift_replicas; sleep 1
    drift_image; sleep 1
    drift_configmap; sleep 1
    drift_env; sleep 1
    drift_resources; sleep 1
    drift_service; sleep 1
    drift_extra; sleep 1
    drift_missing; sleep 1
    # Smart diff scenarios
    drift_configmap_yaml; sleep 1
    drift_sidecar_inject; sleep 1
    drift_replace_template

    section "Maximum Drift Achieved! 🚨"
    echo -e "${BG_YELLOW}${BLACK}  ⚡⚡⚡ TOKYO DRIFT MODE ACTIVATED ⚡⚡⚡  ${NC}"
    echo ""
    echo -e "Run ${CYAN}$0 scan${NC} to see dorikin detect all this drift!"
}

drift_menu() {
    banner
    section "Drift Scenario Menu"

    if ! colima_running; then
        error "Cluster not running. Run '$0 setup' first."
        exit 1
    fi

    echo -e "Select a drift scenario:\n"
    echo -e "  ${JADE}1${NC}) Replicas     ${GRAY}nginx: 3 → 5${NC}"
    echo -e "  ${JADE}2${NC}) Image        ${GRAY}nginx:1.25.0 → 1.26.0${NC}"
    echo -e "  ${JADE}3${NC}) ConfigMap    ${GRAY}Enable TOKYO_DRIFT mode${NC}"
    echo -e "  ${JADE}4${NC}) Env Vars     ${GRAY}Modify DRIVER, CAR${NC}"
    echo -e "  ${JADE}5${NC}) Resources    ${GRAY}Memory: 128Mi → 256Mi${NC}"
    echo -e "  ${JADE}6${NC}) Service      ${GRAY}ClusterIP → NodePort${NC}"
    echo -e "  ${JADE}7${NC}) Extra        ${GRAY}Add deployment not in manifests${NC}"
    echo -e "  ${JADE}8${NC}) Missing      ${GRAY}Delete redis (shows MISSING)${NC}"
    echo ""
    echo -e "  ${CYAN}Q${NC}) Quantity     ${GRAY}✓ Equivalent values (128Mi → bytes, should be NO drift)${NC}"
    echo -e "  ${CYAN}R${NC}) Reorder      ${GRAY}✓ Reorder env vars (should be NO drift)${NC}"
    echo ""
    echo -e "  ${PURPLE}Smart Diff Scenarios:${NC}"
    echo -e "  ${PURPLE}10${NC}) ConfigMap YAML ${GRAY}Modify embedded YAML (ModeUnifiedDiff)${NC}"
    echo -e "  ${PURPLE}11${NC}) Sidecar       ${GRAY}Inject sidecar container (ModeStructuralAdd)${NC}"
    echo -e "  ${PURPLE}12${NC}) Replace       ${GRAY}Complete template replacement (ModeReplacement)${NC}"
    echo ""
    echo -e "  ${YELLOW}A${NC}) ALL          ${GRAY}⚡ Apply everything! ⚡${NC}"
    echo -e "  ${GRAY}0${NC}) Cancel"
    echo ""

    read -rp "$(echo -e "${JADE}Select [1-8, 10-12, Q, R, A, 0]: ${NC}")" choice

    case $choice in
        1) drift_replicas ;;
        2) drift_image ;;
        3) drift_configmap ;;
        4) drift_env ;;
        5) drift_resources ;;
        6) drift_service ;;
        7) drift_extra ;;
        8) drift_missing ;;
        [Qq]) drift_quantity_equiv ;;
        [Rr]) drift_reorder_env ;;
        10) drift_configmap_yaml ;;
        11) drift_sidecar_inject ;;
        12) drift_replace_template ;;
        [Aa]) drift_all ;;
        0) info "Cancelled" ;;
        *) error "Invalid choice" ;;
    esac
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🔄 Reset & Status
# ═══════════════════════════════════════════════════════════════════════════════

reset() {
    banner
    section "Resetting to Baseline"
    
    if ! colima_running; then
        error "Cluster not running."
        exit 1
    fi
    
    step "Deleting namespace..."
    kc delete namespace "${TEST_NAMESPACE}" --ignore-not-found --wait=true
    
    step "Recreating..."
    kc create namespace "${TEST_NAMESPACE}"
    kc apply -f "${MANIFEST_DIR}/" -n "${TEST_NAMESPACE}" &>/dev/null
    
    wait_ready deployment nginx "${TEST_NAMESPACE}" 120 || true
    wait_ready deployment redis "${TEST_NAMESPACE}" 120 || true
    
    section "Reset Complete! 🏁"
    success "Cluster matches manifests - zero drift"
}

status() {
    banner
    section "Test Track Status"
    
    # Colima
    step "Colima:"
    if colima_running; then
        substep_last "${JADE}Running${NC} (${COLIMA_PROFILE})"
    else
        substep_last "${RED}Stopped${NC}"
        return
    fi
    
    # Context
    step "Context: $(get_context)"
    
    # Resources
    echo ""
    echo -e "  ${CYAN}Deployments:${NC}"
    kc get deployments -n "${TEST_NAMESPACE}" 2>/dev/null | sed 's/^/    /' || echo "    None"
    echo ""
    echo -e "  ${CYAN}Services:${NC}"
    kc get services -n "${TEST_NAMESPACE}" 2>/dev/null | sed 's/^/    /' || echo "    None"
    echo ""
    echo -e "  ${CYAN}ConfigMaps:${NC}"
    kc get configmaps -n "${TEST_NAMESPACE}" 2>/dev/null | sed 's/^/    /' || echo "    None"
    
    # Dorikin
    step "Dorikin:"
    if [[ -f "${DORIKIN_BIN}" ]]; then
        substep_last "${JADE}Ready${NC} (${DORIKIN_BIN})"
    else
        substep_last "${YELLOW}Not built${NC} - run 'make build'"
    fi
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🏎️ Run Dorikin
# ═══════════════════════════════════════════════════════════════════════════════

ensure_built() {
    if [[ ! -f "${DORIKIN_BIN}" ]]; then
        warn "Building dorikin..."
        (cd "${PROJECT_ROOT}" && make build)
    fi
}

run_scan() {
    banner
    section "Running Dorikin Scan"
    
    if ! colima_running; then
        error "Cluster not running."
        exit 1
    fi
    
    ensure_built
    
    echo -e "Scanning ${CYAN}${MANIFEST_DIR}${NC}..."
    echo ""
    
    "${DORIKIN_BIN}" scan \
        -f "${MANIFEST_DIR}" \
        -n "${TEST_NAMESPACE}" \
        --context "$(get_context)" \
        -R
}

run_tui() {
    banner
    section "Launching Dorikin TUI"
    
    if ! colima_running; then
        error "Cluster not running."
        exit 1
    fi
    
    ensure_built
    
    info "Press 'q' to quit, '?' for help"
    echo ""
    
    "${DORIKIN_BIN}" ui \
        -f "${MANIFEST_DIR}" \
        -n "${TEST_NAMESPACE}" \
        --context "$(get_context)" \
        -R
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🧹 Cleanup
# ═══════════════════════════════════════════════════════════════════════════════

cleanup() {
    banner
    section "Cleanup"
    
    step "Stopping Colima..."
    colima stop --profile "${COLIMA_PROFILE}" 2>/dev/null || true
    
    step "Deleting profile..."
    colima delete --profile "${COLIMA_PROFILE}" --force 2>/dev/null || true
    
    section "Cleanup Complete! 🏁"
    success "Test track demolished"
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🎨 Smart Diff Testing
# ═══════════════════════════════════════════════════════════════════════════════

smart_diff_setup() {
    banner
    section "Smart Diff Test Setup"

    if ! colima_running; then
        error "Cluster not running. Run '$0 setup' first."
        exit 1
    fi

    ensure_built

    # Ensure namespace exists
    kc create namespace "${TEST_NAMESPACE}" 2>/dev/null || true

    step "Applying smart-diff test manifests..."
    # Apply each file individually to handle missing CRDs gracefully
    for f in "${SMART_DIFF_DIR}"/*.yaml; do
        if kc apply -f "$f" -n "${TEST_NAMESPACE}" 2>&1 | grep -q "no matches for kind"; then
            warn "Skipping $(basename "$f") - CRD not installed"
        fi
    done 2>/dev/null || true

    # Wait for deployments
    step "Waiting for deployments..."
    wait_ready deployment simple-nginx "${TEST_NAMESPACE}" 60 || true
    wait_ready deployment sidecar-test "${TEST_NAMESPACE}" 60 || true
    wait_ready deployment replacement-test "${TEST_NAMESPACE}" 60 || true

    section "Smart Diff Setup Complete! 🏁"
    success "Test manifests deployed - ready for drift scenarios 10-13"
    echo ""
    echo -e "  Run: ${CYAN}$0 smart-diff-tui${NC} to launch TUI with smart-diff manifests"
    echo -e "  Run: ${CYAN}$0 drift${NC} and select 10-13 to apply smart diff scenarios"
}

smart_diff_reset() {
    banner
    section "Resetting Smart Diff Resources"

    if ! colima_running; then
        error "Cluster not running."
        exit 1
    fi

    step "Re-applying smart-diff manifests..."
    kc apply -f "${SMART_DIFF_DIR}/" -n "${TEST_NAMESPACE}"

    # Wait for deployments to stabilize
    wait_ready deployment simple-nginx "${TEST_NAMESPACE}" 60 || true
    wait_ready deployment sidecar-test "${TEST_NAMESPACE}" 60 || true
    wait_ready deployment replacement-test "${TEST_NAMESPACE}" 60 || true

    section "Reset Complete! 🏁"
    success "Smart diff resources restored to baseline"
}

smart_diff_tui() {
    banner
    section "Launching Smart Diff TUI"

    if ! colima_running; then
        error "Cluster not running. Run '$0 setup' first."
        exit 1
    fi

    ensure_built

    info "Launching dorikin TUI with smart-diff manifests..."
    echo ""

    "${DORIKIN_BIN}" ui \
        --refresh-interval 2 \
        -f "${SMART_DIFF_DIR}" \
        -n "${TEST_NAMESPACE}" \
        --context "$(get_context)"
}

smart_diff_scan() {
    banner
    section "Running Smart Diff Scan"

    if ! colima_running; then
        error "Cluster not running. Run '$0 setup' first."
        exit 1
    fi

    ensure_built

    info "Scanning smart-diff manifests..."
    echo ""

    "${DORIKIN_BIN}" scan \
        -f "${SMART_DIFF_DIR}" \
        -n "${TEST_NAMESPACE}" \
        --context "$(get_context)"
}

# ═══════════════════════════════════════════════════════════════════════════════
# 📖 Help
# ═══════════════════════════════════════════════════════════════════════════════

show_help() {
    banner
    cat << EOF
${JADE_BOLD}USAGE${NC}
    $0 <command>

${JADE_BOLD}COMMANDS${NC}
    ${CYAN}setup${NC}      Create test environment (Colima k3s + resources)
    ${CYAN}drift${NC}      Apply drift scenarios (interactive)
    ${CYAN}reset${NC}      Reset cluster to match manifests
    ${CYAN}status${NC}     Show environment status
    ${CYAN}scan${NC}       Run dorikin scan
    ${CYAN}tui${NC}        Launch dorikin TUI
    ${CYAN}cleanup${NC}    Destroy test environment
    ${CYAN}help${NC}       Show this help

${JADE_BOLD}SMART DIFF TESTING${NC}
    ${PURPLE}smart-diff-setup${NC}   Deploy smart-diff test manifests
    ${PURPLE}smart-diff-reset${NC}   Reset smart-diff resources to baseline
    ${PURPLE}smart-diff-tui${NC}     Launch TUI with smart-diff manifests
    ${PURPLE}smart-diff-scan${NC}    Scan smart-diff manifests

${JADE_BOLD}QUICK START${NC}
    $0 setup              ${GRAY}# Start cluster, deploy resources${NC}
    $0 scan               ${GRAY}# Verify baseline (all green!)${NC}
    $0 drift              ${GRAY}# Apply drift scenarios${NC}
    $0 scan               ${GRAY}# See drift detected!${NC}
    $0 tui                ${GRAY}# Interactive exploration${NC}
    $0 reset              ${GRAY}# Remove all drift${NC}
    $0 cleanup            ${GRAY}# Tear down when done${NC}

${JADE_BOLD}DRIFT SCENARIOS${NC}
    ${YELLOW}1${NC}) Replicas     ${GRAY}Scale deployment${NC}
    ${YELLOW}2${NC}) Image        ${GRAY}Change container image${NC}
    ${YELLOW}3${NC}) ConfigMap    ${GRAY}Modify config data${NC}
    ${YELLOW}4${NC}) Env Vars     ${GRAY}Change environment${NC}
    ${YELLOW}5${NC}) Resources    ${GRAY}Modify limits${NC}
    ${YELLOW}6${NC}) Service      ${GRAY}Change service type${NC}
    ${YELLOW}7${NC}) Extra        ${GRAY}Add untracked resource${NC}
    ${YELLOW}8${NC}) Missing      ${GRAY}Delete tracked resource${NC}
    ${CYAN}Q${NC}) Quantity     ${GRAY}Equivalent values (should be NO drift)${NC}
    ${CYAN}R${NC}) Reorder      ${GRAY}Reorder env vars (should be NO drift)${NC}
    ${PURPLE}10${NC}) Argo Script  ${GRAY}Modify inline Python script${NC}
    ${PURPLE}11${NC}) ConfigMap YAML ${GRAY}Modify embedded YAML${NC}
    ${PURPLE}12${NC}) Sidecar      ${GRAY}Inject sidecar container${NC}
    ${PURPLE}13${NC}) Replace      ${GRAY}Complete template replacement${NC}
    ${YELLOW}A${NC}) ALL          ${GRAY}Apply everything!${NC}

${JADE_BOLD}REQUIREMENTS${NC}
    macOS, Homebrew, Colima, kubectl

EOF
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🎬 Main
# ═══════════════════════════════════════════════════════════════════════════════

main() {
    case "${1:-help}" in
        setup)   setup ;;
        drift)   drift_menu ;;
        reset)   reset ;;
        status)  status ;;
        scan)    run_scan ;;
        tui)     run_tui ;;
        cleanup) cleanup ;;
        smart-diff-setup) smart_diff_setup ;;
        smart-diff-reset) smart_diff_reset ;;
        smart-diff-tui)   smart_diff_tui ;;
        smart-diff-scan)  smart_diff_scan ;;
        help|--help|-h) show_help ;;
        *) error "Unknown: $1"; show_help; exit 1 ;;
    esac
}

main "$@"
