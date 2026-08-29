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
done < <(
  find ./examples \
    -name go.mod \
    -type f \
    -not -path '*/vendor/*' \
    -print0 2>/dev/null |
    sort -z
)

# The workspace is ephemeral in CI and local-only for development.
GOWORK=off go work init "${modules[@]}"

# Bind example requirements for released xredis versions to the current
# checkout so all Go tooling resolves the root module locally.
while read -r module version; do
  [[ -n "${module}" && -n "${version}" ]] || continue

  GOWORK="${root}/go.work" go work edit \
    -replace="${module}@${version}=."
done < <(
  find ./examples \
    -name go.mod \
    -type f \
    -not -path '*/vendor/*' \
    -print0 2>/dev/null |
  while IFS= read -r -d '' mod; do
    awk '
      $1 == "require" &&
      $2 == "github.com/mkbeh/xredis" &&
      $3 ~ /^v[0-9]/ {
        print $2, $3
        next
      }

      $1 == "github.com/mkbeh/xredis" &&
      $2 ~ /^v[0-9]/ {
        print $1, $2
      }
    ' "${mod}"
  done |
  sort -u
)

echo "Created temporary Go workspace at ${root}/go.work"
