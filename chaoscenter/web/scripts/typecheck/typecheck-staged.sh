#!/usr/bin/env bash
set -euo pipefail
[[ $# -gt 0 ]] || exit 0
# Keep the temporary config beside tsconfig.json so relative paths still resolve.
config="$(mktemp "${PWD}/tsconfig.XXXXXX.json")"
trap 'rm -f "$config"' EXIT
node - "$config" "$@" <<'JS'
const fs = require('fs');
const [config, ...files] = process.argv.slice(2);
fs.writeFileSync(config, JSON.stringify({
  extends: './tsconfig.json',
  include: ['src/global.d.ts', ...files]
}));
JS
tsc --pretty -p "$config"
