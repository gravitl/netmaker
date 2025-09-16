#!/usr/bin/env bash
# Fetch WireGuard config from Netmaker via /api/v1/client_conf/{network} and bring it up.
# Required env:
#   NETMAKER_BASE_URL, NETMAKER_API_JWT, NETMAKER_NETWORK
#   WG_IFACE (default: netmaker), WG_CONF_DIR (default: /etc/wireguard)

set -euo pipefail

# --- Fail fast if mandatory variables missing ---
: "${NETMAKER_BASE_URL:?ERROR: NETMAKER_BASE_URL not set}"
: "${NETMAKER_NETWORK:?ERROR: NETMAKER_NETWORK not set}"
: "${NETMAKER_API_JWT:?ERROR: NETMAKER_API_JWT not set}"

# --- Ensure required packages are present ---
echo "[*] Checking dependencies ..."
DEPS=(curl jq wg-quick ip)
MISSING=()
for bin in "${DEPS[@]}"; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    MISSING+=("$bin")
  fi
done

if [[ ${#MISSING[@]} -gt 0 ]]; then
  echo "[*] Installing missing deps: ${MISSING[*]} ..."
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -y
    sudo apt-get install -y wireguard-tools jq curl iproute2 resolvconf
  elif command -v yum >/dev/null 2>&1; then
    sudo yum install -y wireguard-tools jq curl iproute iproute-tc
  elif command -v dnf >/dev/null 2>&1; then
    sudo dnf install -y wireguard-tools jq curl iproute
  else
    echo "ERROR: Package manager not found. Install ${MISSING[*]} manually." >&2
    exit 1
  fi
else
  echo "[*] All dependencies found."
fi

# --- Inputs & defaults ---
BASE_URL="${NETMAKER_BASE_URL:?NETMAKER_BASE_URL not set}"
NETWORK="${NETMAKER_NETWORK:?NETMAKER_NETWORK not set}"
JWT="${NETMAKER_API_JWT:?NETMAKER_API_JWT not set}"
WG_IFACE="${WG_IFACE:-netmaker}"
WG_CONF_DIR="${WG_CONF_DIR:-/etc/wireguard}"
TMP_CONF="/tmp/${WG_IFACE}.conf"

EP="${BASE_URL}/api/v1/client_conf/${NETWORK}"

echo "[*] Requesting client configuration from: ${EP}"

HDRS=(-H "Authorization: Bearer ${JWT}")
[[ -n "${NM_CLIENT_LABEL:-}" ]]    && HDRS+=(-H "X-NM-Client-Label: ${NM_CLIENT_LABEL}")
[[ -n "${NM_REQUESTED_NAME:-}" ]]  && HDRS+=(-H "X-NM-Requested-Name: ${NM_REQUESTED_NAME}")

# --- Fetch config ---
HTTP_STATUS="$(curl -sS -L -w '%{http_code}' -o "${TMP_CONF}" "${HDRS[@]}" "${EP}")"

if [[ "${HTTP_STATUS}" != "200" ]]; then
  echo "ERROR: client_conf returned HTTP ${HTTP_STATUS}" >&2
  curl -sS -L "${HDRS[@]}" "${EP}" | head -c 400 >&2 || true
  exit 1
fi

# --- Sanity check ---
if ! grep -q "^\[Interface\]" "${TMP_CONF}"; then
  echo "ERROR: Response does not look like a WireGuard config." >&2
  head -n 20 "${TMP_CONF}" >&2 || true
  exit 1
fi

# --- Add interface-name for traceability ---
if ! grep -q "^#interface-name=" "${TMP_CONF}"; then
  echo "#interface-name=${WG_IFACE}" | cat - "${TMP_CONF}" > "${TMP_CONF}.tmp" && mv "${TMP_CONF}.tmp" "${TMP_CONF}"
fi

# --- Move into place ---
sudo mkdir -p "${WG_CONF_DIR}"
sudo mv "${TMP_CONF}" "${WG_CONF_DIR}/${WG_IFACE}.conf"
sudo chmod 600 "${WG_CONF_DIR}/${WG_IFACE}.conf"

# --- Bring it up ---
echo "[*] Bringing up ${WG_IFACE} ..."
sudo wg-quick up "${WG_IFACE}"

echo "==== ${WG_IFACE} is up ===="
ip addr show "${WG_IFACE}" || true
wg show "${WG_IFACE}" || true
