#!/usr/bin/env bash
set -euo pipefail
[[ $# -gt 0 ]] || exit 0
exec eslint --ext .ts --ext .tsx --fix -- "$@"
