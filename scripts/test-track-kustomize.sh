#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
#  🏎️ DORIKIN KUSTOMIZE TEST TRACK - Kustomize Overlay Drift Testing Environment
# ═══════════════════════════════════════════════════════════════════════════════
#
#  A test harness for dorikin's Kustomize integration that:
#  - Uses the existing Colima k3s cluster (from test-track.sh setup)
#  - Deploys Kustomize overlays to dorikin-tuner namespace
#  - Creates intentional DRIFT scenarios to test Kustomize mode
#  - Provides interactive controls for testing
#
#  Requirements: Colima running (via test-track.sh setup), kustomize, kubectl
#
#  Usage:
#    ./scripts/test-track-kustomize.sh [command]
#
#  Commands:
#    deploy     - Deploy Kustomize overlay to cluster
#    drift      - Apply drift scenarios (interactive menu)
#    reset      - Reset namespace (delete and redeploy)
#    status     - Show current environment status
#    scan       - Run dorikin scan with --kustomize flag
#    tui        - Launch dorikin TUI with --kustomize flag
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

# Colima profile (shared with main test track)
COLIMA_PROFILE="dorikin-ae86"

# Kustomize test namespace (tuner = custom modifications)
TEST_NAMESPACE="dorikin-tuner"

# Kustomize overlay directory
KUSTOMIZE_DIR="${PROJECT_ROOT}/testdata/track-kustomize/overlays/prod"

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
    echo -e "  ${WHITE}🏎️  KUSTOMIZE TEST TRACK${NC} ${GRAY}// Tuner Drift Testing${NC}"
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

    local missing=()

    step "Checking required tools..."

    if command_exists kustomize; then
        substep "Kustomize: ${JADE}$(kustomize version --short 2>/dev/null || kustomize version 2>/dev/null | head -1)${NC}"
    else
        missing+=("kustomize")
        substep "Kustomize: ${RED}✗${NC}"
    fi

    if command_exists kubectl; then
        substep "kubectl: ${JADE}$(kubectl version --client -o json 2>/dev/null | grep gitVersion | head -1 | awk -F'"' '{print $4}')${NC}"
    else
        missing+=("kubectl")
        substep "kubectl: ${RED}✗${NC}"
    fi

    if colima_running; then
        substep "Colima: ${JADE}Running${NC} (${COLIMA_PROFILE})"
    else
        missing+=("colima-running")
        substep "Colima: ${RED}Not running${NC}"
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
                kustomize) echo "  brew install kustomize" ;;
                kubectl) echo "  brew install kubectl" ;;
                colima-running) echo "  Run: ./scripts/test-track.sh setup" ;;
            esac
        done
        exit 1
    fi

    success "All checks passed!"
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🚀 Deploy Functions
# ═══════════════════════════════════════════════════════════════════════════════

deploy() {
    banner
    section "Deploying Kustomize Overlay to Tuner"

    preflight

    step "Creating namespace..."
    kc create namespace "${TEST_NAMESPACE}" --dry-run=client -o yaml | kc apply -f - &>/dev/null
    substep_last "Namespace: ${TEST_NAMESPACE}"

    step "Applying Kustomize overlay..."
    kustomize build "${KUSTOMIZE_DIR}" | kc apply -f - 2>&1 | while IFS= read -r line; do
        substep "$line"
    done

    # Wait for deployment
    wait_ready deployment nginx "${TEST_NAMESPACE}" 120 || true

    section "Deployment Complete! 🏁"

    echo -e "${JADE}The Kustomize test track is ready!${NC}"
    echo ""
    echo -e "  Context:   ${CYAN}$(get_context)${NC}"
    echo -e "  Namespace: ${CYAN}${TEST_NAMESPACE}${NC}"
    echo -e "  Overlay:   ${CYAN}${KUSTOMIZE_DIR}${NC}"
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
    echo -e "  ${GRAY}(Note: HPA will manage this, so it may not show as drift)${NC}"
}

drift_image() {
    info "Drift: Updating nginx image 1.25.0 → 1.24.0"
    kc set image deployment/nginx nginx=nginx:1.24.0 -n "${TEST_NAMESPACE}"
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} image: ${JADE}nginx:1.25.0${NC} → ${YELLOW}nginx:1.24.0${NC}"
}

drift_configmap() {
    info "Drift: Modifying racing-config"
    kc patch configmap racing-config -n "${TEST_NAMESPACE}" --type merge \
        -p '{"data":{"drift-mode":"enabled","track":"Tsukuba"}}'
    drift_alert
    echo -e "  ${YELLOW}⚡${NC} drift-mode: ${JADE}disabled${NC} → ${YELLOW}enabled${NC}"
    echo -e "  ${YELLOW}⚡${NC} track: ${JADE}Akina${NC} → ${YELLOW}Tsukuba${NC}"
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
    info "Drift: Adding rogue deployment (not in overlay)"
    kc create deployment rival-car --image=busybox:latest -n "${TEST_NAMESPACE}" -- sleep infinity 2>/dev/null || true
    kc label deployment/rival-car -n "${TEST_NAMESPACE}" team=unknown --overwrite 2>/dev/null || true
    drift_alert
    echo -e "  ${PURPLE}➕${NC} deployment/rival-car ${GRAY}(exists in cluster only)${NC}"
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
    drift_extra

    section "Maximum Drift Achieved! 🚨"
    echo -e "${BG_YELLOW}${BLACK}  ⚡⚡⚡ TUNER CHAOS MODE ⚡⚡⚡  ${NC}"
    echo ""
    echo -e "Run ${CYAN}$0 scan${NC} to see dorikin detect all this drift!"
}

drift_menu() {
    banner
    section "Drift Scenario Menu"

    if ! colima_running; then
        error "Cluster not running. Run './scripts/test-track.sh setup' first."
        exit 1
    fi

    echo -e "Select a drift scenario:\n"
    echo -e "  ${JADE}1${NC}) Replicas     ${GRAY}nginx: 3 → 5${NC}"
    echo -e "  ${JADE}2${NC}) Image        ${GRAY}nginx:1.25.0 → 1.24.0${NC}"
    echo -e "  ${JADE}3${NC}) ConfigMap    ${GRAY}Enable drift mode${NC}"
    echo -e "  ${JADE}4${NC}) Env Vars     ${GRAY}Modify DRIVER, CAR${NC}"
    echo -e "  ${JADE}5${NC}) Resources    ${GRAY}Memory: 128Mi → 256Mi${NC}"
    echo -e "  ${JADE}6${NC}) Service      ${GRAY}ClusterIP → NodePort${NC}"
    echo -e "  ${JADE}7${NC}) Extra        ${GRAY}Add deployment not in overlay${NC}"
    echo -e "  ${YELLOW}A${NC}) ALL          ${GRAY}⚡ Apply everything! ⚡${NC}"
    echo -e "  ${GRAY}0${NC}) Cancel"
    echo ""

    read -rp "$(echo -e "${JADE}Select [1-7, A, 0]: ${NC}")" choice

    case $choice in
        1) drift_replicas ;;
        2) drift_image ;;
        3) drift_configmap ;;
        4) drift_env ;;
        5) drift_resources ;;
        6) drift_service ;;
        7) drift_extra ;;
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
    section "Resetting Tuner to Baseline"

    if ! colima_running; then
        error "Cluster not running."
        exit 1
    fi

    step "Deleting namespace..."
    kc delete namespace "${TEST_NAMESPACE}" --ignore-not-found --wait=true

    step "Redeploying overlay..."
    kc create namespace "${TEST_NAMESPACE}"
    kustomize build "${KUSTOMIZE_DIR}" | kc apply -f - &>/dev/null

    wait_ready deployment nginx "${TEST_NAMESPACE}" 120 || true

    section "Reset Complete! 🏁"
    success "Cluster matches Kustomize overlay - zero drift"
}

status() {
    banner
    section "Kustomize Test Track Status"

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
    step "Namespace: ${TEST_NAMESPACE}"
    step "Overlay: ${KUSTOMIZE_DIR}"

    # Resources
    echo ""
    echo -e "  ${CYAN}Deployments:${NC}"
    kc get deployments -n "${TEST_NAMESPACE}" 2>/dev/null | sed 's/^/    /' || echo "    None"
    echo ""
    echo -e "  ${CYAN}HPAs:${NC}"
    kc get hpa -n "${TEST_NAMESPACE}" 2>/dev/null | sed 's/^/    /' || echo "    None"
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
    section "Running Dorikin Kustomize Scan"

    if ! colima_running; then
        error "Cluster not running."
        exit 1
    fi

    ensure_built

    echo -e "Scanning Kustomize overlay ${CYAN}${KUSTOMIZE_DIR}${NC}..."
    echo ""

    "${DORIKIN_BIN}" scan \
        --kustomize "${KUSTOMIZE_DIR}" \
        -n "${TEST_NAMESPACE}" \
        --context "$(get_context)"
}

run_tui() {
    banner
    section "Launching Dorikin Kustomize TUI"

    if ! colima_running; then
        error "Cluster not running."
        exit 1
    fi

    ensure_built

    info "Press 'q' to quit, '?' for help"
    echo ""

    "${DORIKIN_BIN}" ui \
        --kustomize "${KUSTOMIZE_DIR}" \
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
    ${CYAN}deploy${NC}     Deploy Kustomize overlay to cluster
    ${CYAN}drift${NC}      Apply drift scenarios (interactive)
    ${CYAN}reset${NC}      Reset namespace (delete and redeploy)
    ${CYAN}status${NC}     Show environment status
    ${CYAN}scan${NC}       Run dorikin scan with --kustomize
    ${CYAN}tui${NC}        Launch dorikin TUI with --kustomize
    ${CYAN}help${NC}       Show this help

${JADE_BOLD}PREREQUISITES${NC}
    Run ${CYAN}./scripts/test-track.sh setup${NC} first to start the Colima cluster.

${JADE_BOLD}QUICK START${NC}
    $0 deploy             ${GRAY}# Apply Kustomize overlay${NC}
    $0 scan               ${GRAY}# Verify baseline (all green!)${NC}
    $0 drift              ${GRAY}# Apply drift scenarios${NC}
    $0 scan               ${GRAY}# See drift detected!${NC}
    $0 tui                ${GRAY}# Interactive exploration${NC}
    $0 reset              ${GRAY}# Remove all drift${NC}

${JADE_BOLD}DRIFT SCENARIOS${NC}
    ${YELLOW}1${NC}) Replicas     ${GRAY}Scale deployment${NC}
    ${YELLOW}2${NC}) Image        ${GRAY}Change container image${NC}
    ${YELLOW}3${NC}) ConfigMap    ${GRAY}Modify config data${NC}
    ${YELLOW}4${NC}) Env Vars     ${GRAY}Change environment${NC}
    ${YELLOW}5${NC}) Resources    ${GRAY}Modify limits${NC}
    ${YELLOW}6${NC}) Service      ${GRAY}Change service type${NC}
    ${YELLOW}7${NC}) Extra        ${GRAY}Add untracked resource${NC}
    ${YELLOW}A${NC}) ALL          ${GRAY}Apply everything!${NC}

${JADE_BOLD}NAMESPACE${NC}
    ${CYAN}dorikin-tuner${NC} - Isolated from main test track (dorikin-garage)

EOF
}

# ═══════════════════════════════════════════════════════════════════════════════
# 🎬 Main
# ═══════════════════════════════════════════════════════════════════════════════

main() {
    case "${1:-help}" in
        deploy)  deploy ;;
        drift)   drift_menu ;;
        reset)   reset ;;
        status)  status ;;
        scan)    run_scan ;;
        tui)     run_tui ;;
        help|--help|-h) show_help ;;
        *) error "Unknown: $1"; show_help; exit 1 ;;
    esac
}

main "$@"
