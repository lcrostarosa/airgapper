#!/bin/bash
# Security check script for Airgapper production deployments
# Run this before deploying to verify security configuration

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

# Helper functions
pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ERRORS=$((ERRORS + 1))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    WARNINGS=$((WARNINGS + 1))
}

info() {
    echo -e "[INFO] $1"
}

header() {
    echo ""
    echo "=========================================="
    echo "$1"
    echo "=========================================="
}

# Get the base directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(dirname "$SCRIPT_DIR")"
DOCKER_DIR="$BASE_DIR/docker"

header "Airgapper Security Check"
echo "Base directory: $BASE_DIR"
echo "Docker directory: $DOCKER_DIR"

# ==========================================
# Check 1: htpasswd file exists
# ==========================================
header "1. Authentication Configuration"

if [ -f "$DOCKER_DIR/auth/.htpasswd" ]; then
    # Check permissions
    PERMS=$(stat -f "%Lp" "$DOCKER_DIR/auth/.htpasswd" 2>/dev/null || stat -c "%a" "$DOCKER_DIR/auth/.htpasswd" 2>/dev/null)
    if [ "$PERMS" = "600" ] || [ "$PERMS" = "400" ]; then
        pass "htpasswd file exists with secure permissions ($PERMS)"
    else
        warn "htpasswd file permissions are $PERMS, should be 600 or 400"
    fi
else
    fail "htpasswd file not found at $DOCKER_DIR/auth/.htpasswd"
    info "  Generate with: htpasswd -Bc $DOCKER_DIR/auth/.htpasswd <username>"
fi

# ==========================================
# Check 2: TLS certificates exist
# ==========================================
header "2. TLS Certificate Configuration"

if [ -f "$DOCKER_DIR/certs/server.crt" ] && [ -f "$DOCKER_DIR/certs/server.key" ]; then
    # Check certificate expiry
    if command -v openssl &> /dev/null; then
        EXPIRY=$(openssl x509 -enddate -noout -in "$DOCKER_DIR/certs/server.crt" 2>/dev/null | cut -d= -f2)
        EXPIRY_EPOCH=$(date -j -f "%b %d %T %Y %Z" "$EXPIRY" +%s 2>/dev/null || date -d "$EXPIRY" +%s 2>/dev/null)
        NOW_EPOCH=$(date +%s)
        DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))

        if [ $DAYS_LEFT -lt 0 ]; then
            fail "TLS certificate has expired!"
        elif [ $DAYS_LEFT -lt 30 ]; then
            warn "TLS certificate expires in $DAYS_LEFT days"
        else
            pass "TLS certificate valid for $DAYS_LEFT more days"
        fi
    else
        pass "TLS certificates exist (openssl not available for expiry check)"
    fi

    # Check key permissions
    KEY_PERMS=$(stat -f "%Lp" "$DOCKER_DIR/certs/server.key" 2>/dev/null || stat -c "%a" "$DOCKER_DIR/certs/server.key" 2>/dev/null)
    if [ "$KEY_PERMS" = "600" ] || [ "$KEY_PERMS" = "400" ]; then
        pass "Private key has secure permissions ($KEY_PERMS)"
    else
        warn "Private key permissions are $KEY_PERMS, should be 600 or 400"
    fi
else
    fail "TLS certificates not found at $DOCKER_DIR/certs/"
    info "  Generate self-signed cert:"
    info "    openssl req -x509 -newkey rsa:4096 -keyout $DOCKER_DIR/certs/server.key -out $DOCKER_DIR/certs/server.crt -days 365 -nodes"
fi

# ==========================================
# Check 3: No --no-auth in production config
# ==========================================
header "3. Docker Compose Configuration"

if [ -f "$DOCKER_DIR/docker-compose.production.yml" ]; then
    if grep -q "\-\-no-auth" "$DOCKER_DIR/docker-compose.production.yml"; then
        fail "Production docker-compose contains --no-auth flag"
    else
        pass "No --no-auth flag in production config"
    fi

    if grep -q '"8000:8000"' "$DOCKER_DIR/docker-compose.production.yml" && ! grep -q '"127.0.0.1:8000:8000"' "$DOCKER_DIR/docker-compose.production.yml"; then
        warn "Storage port may be exposed to all interfaces (consider 127.0.0.1:8000:8000)"
    else
        pass "Ports bound to localhost only"
    fi
else
    warn "Production docker-compose.yml not found"
fi

# Check the development compose too
if [ -f "$DOCKER_DIR/docker-compose.yml" ]; then
    if grep -q "\-\-no-auth" "$DOCKER_DIR/docker-compose.yml"; then
        warn "Development docker-compose contains --no-auth (OK for dev, ensure not used in production)"
    fi
fi

# ==========================================
# Check 4: API Key configuration
# ==========================================
header "4. API Key Configuration"

CONFIG_FILE="$HOME/.airgapper/config.json"
if [ -f "$CONFIG_FILE" ]; then
    if grep -q '"api_key"' "$CONFIG_FILE"; then
        pass "API key is configured"
    else
        warn "API key not configured in config.json"
        info "  Add api_key field to enable API authentication"
    fi

    # Check config file permissions
    CONFIG_PERMS=$(stat -f "%Lp" "$CONFIG_FILE" 2>/dev/null || stat -c "%a" "$CONFIG_FILE" 2>/dev/null)
    if [ "$CONFIG_PERMS" = "600" ] || [ "$CONFIG_PERMS" = "400" ]; then
        pass "Config file has secure permissions ($CONFIG_PERMS)"
    else
        warn "Config file permissions are $CONFIG_PERMS, should be 600"
    fi
else
    info "Config file not found at $CONFIG_FILE (not initialized yet)"
fi

# ==========================================
# Check 5: Environment variables
# ==========================================
header "5. Environment Variables"

if [ -n "$RESTIC_PASSWORD" ]; then
    warn "RESTIC_PASSWORD is set in environment (passwords should be in config)"
fi

if [ -n "$AIRGAPPER_API_KEY" ]; then
    pass "AIRGAPPER_API_KEY is set"
else
    info "AIRGAPPER_API_KEY not set in environment (can be set in config instead)"
fi

# ==========================================
# Check 6: Network configuration
# ==========================================
header "6. Network Configuration"

# Check if common ports are exposed
if command -v netstat &> /dev/null; then
    for port in 8000 8081; do
        if netstat -an 2>/dev/null | grep -q "0.0.0.0:$port"; then
            warn "Port $port is bound to 0.0.0.0 (all interfaces)"
        elif netstat -an 2>/dev/null | grep -q "127.0.0.1:$port"; then
            pass "Port $port is bound to localhost only"
        fi
    done
elif command -v ss &> /dev/null; then
    for port in 8000 8081; do
        if ss -tuln 2>/dev/null | grep -q "0.0.0.0:$port"; then
            warn "Port $port is bound to 0.0.0.0 (all interfaces)"
        elif ss -tuln 2>/dev/null | grep -q "127.0.0.1:$port"; then
            pass "Port $port is bound to localhost only"
        fi
    done
else
    info "Cannot check port bindings (netstat/ss not available)"
fi

# ==========================================
# Summary
# ==========================================
header "Security Check Summary"

echo ""
if [ $ERRORS -gt 0 ]; then
    echo -e "${RED}ERRORS: $ERRORS${NC}"
fi
if [ $WARNINGS -gt 0 ]; then
    echo -e "${YELLOW}WARNINGS: $WARNINGS${NC}"
fi

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}All security checks passed!${NC}"
    exit 0
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}Security check completed with warnings.${NC}"
    exit 0
else
    echo -e "${RED}Security check failed. Please fix the errors above before deploying.${NC}"
    exit 1
fi
