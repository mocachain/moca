#!/usr/bin/env bash
# Runs govulncheck and compares the advisories it reports with vulncheck_known.txt.
# Fails on an advisory that is not listed, on a listed advisory that is no longer
# reported, and when govulncheck itself fails.
# Usage: scripts/vulncheck.sh <govulncheck binary> [govulncheck arguments]
set -euo pipefail

govulncheck=${1:?usage: vulncheck.sh <govulncheck binary> [govulncheck arguments]}
shift
known_file=${VULNCHECK_KNOWN:-$(dirname -- "${BASH_SOURCE[0]}")/vulncheck_known.txt}

problems=0
note() { # level message
	if [ "${GITHUB_ACTIONS:-}" = "true" ]; then
		echo "::$1 title=vulncheck::$2"
	else
		echo "vulncheck: $1: $2" >&2
	fi
}
problem() {
	note error "$1"
	problems=$((problems + 1))
}

output=$(mktemp)
trap 'rm -f "$output"' EXIT

# govulncheck exits 0 when it reports no advisory and 3 when it reports at least one
status=0
"$govulncheck" "$@" 2>&1 | tee "$output" || status=${PIPESTATUS[0]}
if [ "$status" -ne 0 ] && [ "$status" -ne 3 ]; then
	problem "govulncheck exited $status"
	exit 1
fi

# only the symbol results count: the advisories whose affected code this repository calls
reported=$(awk '
    /^=== .* ===$/ { symbols = ($0 == "=== Symbol Results ===") }
    symbols && /^Vulnerability #[0-9]+: GO-[0-9]+-[0-9]+$/ { print $3 }
' "$output" | LC_ALL=C sort -u)
known=$(sed -E 's/[[:space:]]*#.*//; /^[[:space:]]*$/d' "$known_file" | LC_ALL=C sort -u)

if [ "$status" -eq 3 ] && [ -z "$reported" ]; then
	problem "govulncheck exited 3 without naming an advisory"
fi
for id in $reported; do
	if grep -Fxq -- "$id" <<<"$known"; then
		note warning "known advisory: $id"
	else
		problem "advisory is not listed in $known_file: $id"
	fi
done
for id in $known; do
	if ! grep -Fxq -- "$id" <<<"$reported"; then
		problem "listed advisory is no longer reported, remove it from $known_file: $id"
	fi
done

if [ "$problems" -gt 0 ]; then
	exit 1
fi
