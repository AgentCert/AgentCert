#!/usr/bin/env bash
set -euo pipefail
COMPONENT="${1:-${directory:-}}"
case "${COMPONENT}" in
  frontend|web) context=web; image=agentcert-web; dockerfile=Dockerfile ;;
  graphql-server|graphql) context=graphql; image=agentcert-graphql; dockerfile=server/Dockerfile ;;
  authentication|auth) context=authentication; image=agentcert-auth; dockerfile=Dockerfile ;;
  -h|--help) echo 'Usage: filter_and_build.sh frontend|graphql-server|authentication'; exit 0 ;;
  *) echo 'ERROR: select frontend, graphql-server or authentication' >&2; exit 2 ;;
esac
SOURCE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILDER="${SOURCE_ROOT}/../scripts/build-and-push.sh"
if [[ -f "${BUILDER}" ]]; then
  bash "${BUILDER}" --local --only "^agentcert/${image}:"
else
  docker build --network "${ACE_BUILD_NETWORK:-default}" -f "${SOURCE_ROOT}/chaoscenter/${context}/${dockerfile}" \
    -t "agentcert/${image}:latest" "${SOURCE_ROOT}/chaoscenter/${context}"
fi
mkdir -p Images
docker save -o "Images/${COMPONENT}.tar" "agentcert/${image}:latest"
