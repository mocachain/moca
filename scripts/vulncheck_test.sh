#!/usr/bin/env bash
# Unit tests for vulncheck.sh, with a stub in place of govulncheck.
# Run: bash scripts/vulncheck_test.sh
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/govulncheck" <<'STUB'
#!/usr/bin/env bash
cat "$STUB_OUTPUT"
exit "$STUB_STATUS"
STUB
chmod +x "$tmp/govulncheck"

# scan_output <symbol-level IDs> <package-level IDs>: what govulncheck prints
scan_output() {
	local n=0 id
	echo "=== Symbol Results ==="
	echo
	for id in $1; do
		n=$((n + 1))
		echo "Vulnerability #$n: $id"
		echo "  More info: https://pkg.go.dev/vuln/$id"
		echo "    Found in: example.com/mod@v1.0.0"
		echo
	done
	[ "$n" -gt 0 ] || echo "No vulnerabilities found."
	if [ -n "$2" ]; then
		n=0
		echo "=== Package Results ==="
		echo
		for id in $2; do
			n=$((n + 1))
			echo "Vulnerability #$n: $id"
			echo
		done
	fi
}

failures=0
# check <label> <scanner exit> <symbol IDs> <package IDs> <listed IDs> <want exit> <want lines>
check() {
	local label=$1 want_exit=$6 want=$7 got_exit=0 got id
	scan_output "$3" "$4" >"$tmp/output"
	{
		echo "# header comment"
		echo
		for id in $5; do echo "$id  # why"; done
	} >"$tmp/known"
	got=$(VULNCHECK_KNOWN="$tmp/known" STUB_OUTPUT="$tmp/output" STUB_STATUS=$2 GITHUB_ACTIONS=true \
		bash "$SCRIPT_DIR/vulncheck.sh" "$tmp/govulncheck" ./... 2>&1) || got_exit=$?
	got=$(grep '^::' <<<"$got" | sed -E "s|$tmp/known|LIST|; s/^::([a-z]+) title=vulncheck::/\1: /" || true)
	if [ "$got_exit" = "$want_exit" ] && [ "$got" = "$want" ]; then
		echo "ok   $label"
	else
		echo "FAIL $label: exit $got_exit, want $want_exit; got:"
		echo "$got"
		failures=$((failures + 1))
	fi
}

check "listed advisories only" 3 "GO-2026-0001 GO-2026-0002" "" "GO-2026-0001 GO-2026-0002" 0 \
	"warning: known advisory: GO-2026-0001
warning: known advisory: GO-2026-0002"
check "an unlisted advisory" 3 "GO-2026-0001 GO-2026-0003" "" "GO-2026-0001" 1 \
	"warning: known advisory: GO-2026-0001
error: advisory is not listed in LIST: GO-2026-0003"
check "a listed advisory is gone" 3 "GO-2026-0001" "" "GO-2026-0001 GO-2026-0002" 1 \
	"warning: known advisory: GO-2026-0001
error: listed advisory is no longer reported, remove it from LIST: GO-2026-0002"
check "nothing reported, one listed" 0 "" "" "GO-2026-0001" 1 \
	"error: listed advisory is no longer reported, remove it from LIST: GO-2026-0001"
check "nothing reported, nothing listed" 0 "" "" "" 0 ""
check "package-level results do not count" 0 "" "GO-2026-0004" "" 0 ""
check "govulncheck fails" 1 "" "" "GO-2026-0001" 1 \
	"error: govulncheck exited 1"
check "exit 3 without an advisory" 3 "" "" "" 1 \
	"error: govulncheck exited 3 without naming an advisory"

if [ "$failures" -gt 0 ]; then
	echo "$failures test(s) failed"
	exit 1
fi
echo "all vulncheck.sh tests passed"
