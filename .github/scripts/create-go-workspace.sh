#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${GITHUB_WORKSPACE:-}" ]]; then
  root="${GITHUB_WORKSPACE}"
else
  root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

cd "${root}"

rm -f go.work go.work.sum

modules=(.)
while IFS= read -r -d '' mod; do
  modules+=("$(dirname "${mod}")")
done < <(find ./examples -name go.mod -type f -print0 | sort -z)

# The workspace is ephemeral in CI and local-only for development. It makes
# standalone examples resolve github.com/mkbeh/xredis to the current checkout.
GOWORK=off go work init "${modules[@]}"

echo "Created temporary Go workspace at ${root}/go.work"
if [[ "${WORKSPACE_DEBUG:-false}" == "true" ]]; then
  GOWORK="${root}/go.work" go work edit -json
fi
