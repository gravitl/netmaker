#!/usr/bin/env bash
# Netmaker CI helper: bring WireGuard up/down and manage ephemeral client lifecycle.
# Subcommands:
#   up   - fetch config, capture Client-ID, bring interface up, save state
#   down - bring interface down, delete local conf, delete client via API
#
# Env vars (can be overridden by flags):
#   NETMAKER_BASE_URL   (required)  e.g. https://nm.example.com   or pass --base-url
#   NETMAKER_NETWORK    (required)  e.g. corpnet                  or pass --network
#   NETMAKER_API_JWT    (required)  Bearer token                  or pass --jwt
#   WG_IFACE            (default netmaker)                           or pass --iface
#   WG_CONF_DIR         (default /etc/wireguard)                  or pass --confdir
#   NETMAKER_STATE_FILE (default RUNNER_TEMP or /tmp)
# You may also pass --client-id on `down` to avoid relying on the state file.

set -euo pipefail

# ---------- defaults ----------
WG_IFACE="${WG_IFACE:-netmaker}"
WG_CONF_DIR="${WG_CONF_DIR:-/etc/wireguard}"
SUBCMD=""
CLIENT_ID_OVERRIDE=""

usage() {
  cat <<USAGE
Usage:
  $0 up   [--iface IFACE] [--confdir DIR] [--base-url URL] [--network NET] [--jwt TOKEN]
  $0 down [--iface IFACE] [--confdir DIR] [--base-url URL] [--network NET] [--jwt TOKEN] [--client-id ID]

Flags override env vars. Env vars documented at top of the script.
Examples:
  NETMAKER_BASE_URL=https://nm.example.com NETMAKER_NETWORK=corpnet NETMAKER_API_JWT=... $0 up
  $0 down --base-url https://nm.example.com --network corpnet --jwt ... --client-id icy-water
USAGE
}

# ---------- arg parse ----------
if [[ $# -lt 1 ]]; then usage; exit 2; fi
SUBCMD="$1"; shift || true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --iface)      WG_IFACE="$2"; shift 2;;
    --confdir)    WG_CONF_DIR="$2"; shift 2;;
    --base-url)   NETMAKER_BASE_URL="$2"; shift 2;;
    --network)    NETMAKER_NETWORK="$2"; shift 2;;
    --jwt)        NETMAKER_API_JWT="$2"; shift 2;;
    --client-id)  CLIENT_ID_OVERRIDE="$2"; shift 2;;
    -h|--help)    usage; exit 0;;
    *) echo "Unknown arg: $1" >&2; usage; exit 2;;
  esac
done

STATE_FILE="${NETMAKER_STATE_FILE:-${RUNNER_TEMP:-/tmp}/netmaker_ci_${WG_IFACE}.env}"

require_env() {
  : "${NETMAKER_BASE_URL:?ERROR: NETMAKER_BASE_URL not set}"
  : "${NETMAKER_NETWORK:?ERROR: NETMAKER_NETWORK not set}"
  : "${NETMAKER_API_JWT:?ERROR: NETMAKER_API_JWT not set}"
}

install_deps() {
  echo "[*] Checking dependencies ..."
  local need=(curl jq wg-quick ip)
  local miss=()
  for b in "${need[@]}"; do command -v "$b" >/dev/null 2>&1 || miss+=("$b"); done
  if [[ ${#miss[@]} -eq 0 ]]; then
    echo "[*] All dependencies present."
    return
  fi
  echo "[*] Installing missing deps: ${miss[*]}"
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -y
    sudo apt-get install -y wireguard-tools jq curl iproute2 resolvconf
  elif command -v yum >/dev/null 2>&1; then
    sudo yum install -y wireguard-tools jq curl iproute iproute-tc
  elif command -v dnf >/dev/null 2>&1; then
    sudo dnf install -y wireguard-tools jq curl iproute
  else
    echo "ERROR: no supported package manager found; install: curl jq wireguard-tools iproute" >&2
    exit 1
  fi
}

do_up() {
  require_env
  install_deps

  local ep="${NETMAKER_BASE_URL}/api/v1/client_conf/${NETMAKER_NETWORK}"
  local tmp_conf="/tmp/${WG_IFACE}.conf"
  local tmp_hdr="/tmp/${WG_IFACE}.headers"

  echo "[*] Requesting client config: ${ep}"
  # Optional headers
  declare -a hdrs
  hdrs=(-H "Authorization: Bearer ${NETMAKER_API_JWT}")
  [[ -n "${NM_CLIENT_LABEL:-}"   ]] && hdrs+=(-H "X-NM-Client-Label: ${NM_CLIENT_LABEL}")
  [[ -n "${NM_REQUESTED_NAME:-}" ]] && hdrs+=(-H "X-NM-Requested-Name: ${NM_REQUESTED_NAME}")

  local code
  code="$(curl -sS -L --dump-header "${tmp_hdr}" -w '%{http_code}' -o "${tmp_conf}" "${hdrs[@]}" "${ep}")"
  if [[ "${code}" != "200" ]]; then
    echo "ERROR: client_conf HTTP ${code}" >&2
    curl -sS -L "${hdrs[@]}" "${ep}" | head -c 400 >&2 || true
    exit 1
  fi
  grep -q "^\[Interface\]" "${tmp_conf}" || { echo "ERROR: not a WireGuard conf"; head -n 20 "${tmp_conf}"; exit 1; }

  # --- Extract Client-ID (one-liner, trim spaces/quotes) ---
  local client_id
  client_id="$(grep -i '^Client-ID:' "${tmp_hdr}" | head -n1 | cut -d: -f2- | tr -d '\r' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^"//; s/"$//' -e "s/^'//; s/'$//")"
  if [[ -z "${client_id}" ]]; then
    echo "ERROR: Client-ID header missing in response; cannot manage lifecycle." >&2
    exit 1
  fi
  echo "[*] Client-ID: ${client_id}"

  # Optional marker
  if ! grep -q "^#interface-name=" "${tmp_conf}"; then
    echo "#interface-name=${WG_IFACE}" | cat - "${tmp_conf}" > "${tmp_conf}.tmp" && mv "${tmp_conf}.tmp" "${tmp_conf}"
  fi

  # Install & bring up
  sudo mkdir -p "${WG_CONF_DIR}"
  sudo mv "${tmp_conf}" "${WG_CONF_DIR}/${WG_IFACE}.conf"
  sudo chmod 600 "${WG_CONF_DIR}/${WG_IFACE}.conf"
  echo "[*] Bringing up ${WG_IFACE} ..."
  sudo wg-quick up "${WG_IFACE}"

  echo "==== ${WG_IFACE} is up ===="
  ip addr show "${WG_IFACE}" || true
  wg show "${WG_IFACE}" || true

  # Persist state
  cat > "${STATE_FILE}" <<EOF
NETMAKER_BASE_URL='${NETMAKER_BASE_URL}'
NETMAKER_NETWORK='${NETMAKER_NETWORK}'
NETMAKER_API_JWT='${NETMAKER_API_JWT}'
WG_IFACE='${WG_IFACE}'
WG_CONF_DIR='${WG_CONF_DIR}'
CLIENT_ID='${client_id}'
EOF
  chmod 600 "${STATE_FILE}"
  echo "[*] Saved state: ${STATE_FILE}"
}

do_down() {
  # Load state if present; flags/env can still override
  if [[ -f "${STATE_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${STATE_FILE}"
  fi

  require_env

  local client_id="${CLIENT_ID_OVERRIDE:-${CLIENT_ID:-}}"
  echo "[*] Bringing down ${WG_IFACE} ..."
  sudo wg-quick down "${WG_IFACE}" || echo "WARN: wg-quick down failed (already down?)."

  # Remove local conf
  if [[ -f "${WG_CONF_DIR}/${WG_IFACE}.conf" ]]; then
    sudo shred -u "${WG_CONF_DIR}/${WG_IFACE}.conf" 2>/dev/null || sudo rm -f "${WG_CONF_DIR}/${WG_IFACE}.conf"
  fi

  # Delete ephemeral client on server (if we know its ID)
  if [[ -n "${client_id}" ]]; then
    local del_ep="${NETMAKER_BASE_URL}/api/extclients/${NETMAKER_NETWORK}/${client_id}"
    echo "[*] Deleting client: DELETE ${del_ep}"
    local http
    http="$(curl -sS -o /dev/null -w '%{http_code}' -X DELETE -H "Authorization: Bearer ${NETMAKER_API_JWT}" "${del_ep}")"
    if [[ "${http}" =~ ^20[0-9]$ ]]; then
      echo "[*] Client deleted (HTTP ${http})."
    else
      echo "WARN: deletion returned HTTP ${http}; verify server state."
    fi
  else
    echo "WARN: client id not known (missing --client-id and state file); skipping server delete."
  fi

  rm -f "${STATE_FILE}" || true
  echo "[*] Teardown finished."
}

case "${SUBCMD}" in
  up)   do_up ;;
  down) do_down ;;
  *)    usage; exit 2 ;;
esac

