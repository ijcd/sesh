#!/usr/bin/env bash
# Fail if total Go test coverage drops below the floor.
set -euo pipefail

FLOOR=${COVERAGE_FLOOR:-75}
PROFILE=${COVERAGE_PROFILE:-/tmp/sesh-cover.out}

go test -coverprofile="$PROFILE" ./... > /dev/null
TOTAL=$(go tool cover -func="$PROFILE" | tail -1 | awk '{print $3}' | sed 's/%//')

echo "Total coverage: ${TOTAL}% (floor: ${FLOOR}%)"

# Bash arithmetic doesn't do floats; use awk for comparison.
if awk "BEGIN {exit !(${TOTAL} < ${FLOOR})}"; then
    echo "FAIL: coverage ${TOTAL}% is below floor ${FLOOR}%"
    exit 1
fi
echo "OK: coverage ${TOTAL}% >= floor ${FLOOR}%"
