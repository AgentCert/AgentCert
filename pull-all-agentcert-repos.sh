#!/usr/bin/env bash
set -euo pipefail

# Pull (and optionally clone) all repositories for a GitHub organization.
# Defaults are tuned for running this script from inside the AgentCert repo.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_BASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

ORG="AgentCert"
BASE_DIR="${DEFAULT_BASE_DIR}"
CLONE_MISSING=true

usage() {
  cat <<'EOF'
Usage:
  ./pull-all-agentcert-repos.sh [options]

Options:
  --org <name>         GitHub organization name (default: AgentCert)
  --base-dir <path>    Directory where repos exist / should be cloned
                       (default: parent of this script)
  --no-clone           Do not clone missing repositories; only pull existing
  -h, --help           Show help

Environment:
  GITHUB_TOKEN         Optional token for GitHub API fallback (useful for private repos)

Examples:
  ./pull-all-agentcert-repos.sh
  ./pull-all-agentcert-repos.sh --base-dir "$HOME/projects"
  ./pull-all-agentcert-repos.sh --no-clone
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --org)
      ORG="${2:-}"
      [[ -n "$ORG" ]] || { echo "[ERROR] --org requires a value" >&2; exit 1; }
      shift 2
      ;;
    --base-dir)
      BASE_DIR="${2:-}"
      [[ -n "$BASE_DIR" ]] || { echo "[ERROR] --base-dir requires a value" >&2; exit 1; }
      shift 2
      ;;
    --no-clone)
      CLONE_MISSING=false
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[ERROR] Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

mkdir -p "${BASE_DIR}"

get_repos_via_gh() {
  command -v gh >/dev/null 2>&1 || return 1
  gh repo list "${ORG}" --limit 1000 --json name -q '.[].name' 2>/dev/null
}

get_repos_via_api() {
  command -v curl >/dev/null 2>&1 || {
    echo "[ERROR] curl is required for API fallback" >&2
    return 1
  }
  command -v jq >/dev/null 2>&1 || {
    echo "[ERROR] jq is required for API fallback" >&2
    return 1
  }

  local page=1
  local auth_header=()
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    auth_header=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi

  while true; do
    local url="https://api.github.com/orgs/${ORG}/repos?per_page=100&page=${page}"
    local json
    json="$(curl -fsSL -H "Accept: application/vnd.github+json" "${auth_header[@]}" "$url")" || return 1
    local count
    count="$(jq 'length' <<<"$json")"
    [[ "$count" -gt 0 ]] || break
    jq -r '.[].name' <<<"$json"
    page=$((page + 1))
  done
}

echo "[INFO] Organization: ${ORG}"
echo "[INFO] Base directory: ${BASE_DIR}"
echo "[INFO] Clone missing repos: ${CLONE_MISSING}"

if repos="$(get_repos_via_gh)" && [[ -n "${repos}" ]]; then
  echo "[INFO] Repository list source: gh CLI"
elif repos="$(get_repos_via_api)" && [[ -n "${repos}" ]]; then
  echo "[INFO] Repository list source: GitHub API"
else
  echo "[ERROR] Unable to fetch repository list for org '${ORG}'." >&2
  echo "        Tip: authenticate 'gh auth login' or set GITHUB_TOKEN for API access." >&2
  exit 1
fi

ok=0
fail=0
skip=0

while IFS= read -r repo; do
  [[ -n "${repo}" ]] || continue
  repo_dir="${BASE_DIR}/${repo}"

  echo ""
  echo "[INFO] Processing: ${repo}"

  if [[ -d "${repo_dir}/.git" ]]; then
    if git -C "${repo_dir}" pull --ff-only; then
      echo "[OK] Pulled ${repo}"
      ok=$((ok + 1))
    else
      echo "[ERROR] Pull failed: ${repo}"
      fail=$((fail + 1))
    fi
    continue
  fi

  if [[ -e "${repo_dir}" && ! -d "${repo_dir}/.git" ]]; then
    echo "[WARN] Path exists but is not a git repo: ${repo_dir}"
    skip=$((skip + 1))
    continue
  fi

  if [[ "${CLONE_MISSING}" == "true" ]]; then
    if git clone "https://github.com/${ORG}/${repo}.git" "${repo_dir}"; then
      echo "[OK] Cloned ${repo}"
      ok=$((ok + 1))
    else
      echo "[ERROR] Clone failed: ${repo}"
      fail=$((fail + 1))
    fi
  else
    echo "[WARN] Missing repo skipped (clone disabled): ${repo}"
    skip=$((skip + 1))
  fi
done <<<"${repos}"

echo ""
echo "[INFO] Completed"
echo "[INFO] Success: ${ok}"
echo "[INFO] Failed:  ${fail}"
echo "[INFO] Skipped: ${skip}"

if [[ "${fail}" -gt 0 ]]; then
  exit 1
fi
