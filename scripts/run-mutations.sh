#!/usr/bin/env bash
# Run gremlins mutation testing against high-leverage packages.
# This is slow (minutes per package) and should be run ad-hoc, not in CI.
set -euo pipefail

PKGS=(
    "./internal/config/..."
    "./internal/engine/..."
)

# Output directory (gitignored).
OUTDIR="${OUTDIR:-gremlins-workdir}"
mkdir -p "$OUTDIR"

for pkg in "${PKGS[@]}"; do
    safe=$(echo "$pkg" | tr '/.' '__')
    out="$OUTDIR/report-${safe}.txt"
    echo ">>> Mutating $pkg → $out"
    gremlins unleash \
        --tags="" \
        --output="$out" \
        "$pkg" || true
done

echo "Done. Reports under $OUTDIR/"
