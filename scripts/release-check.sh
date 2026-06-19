#!/usr/bin/env sh
set -eu

mode="${1:-release}"
root="${2:-.}"

fail() {
	printf '%s\n' "release-check: $*" >&2
	exit 1
}

require_file() {
	path="$1"
	[ -f "$root/$path" ] || fail "missing required file: $path"
}

json_string_eq() {
	path="$1"
	filter="$2"
	expected="$3"
	actual="$(jq -r "$filter" "$root/$path")"
	[ "$actual" = "$expected" ] || fail "$path $filter expected $expected, got $actual"
}

json_bool_eq() {
	path="$1"
	filter="$2"
	expected="$3"
	jq -e "$filter == $expected" "$root/$path" >/dev/null || fail "$path $filter expected $expected"
}

json_number_eq() {
	path="$1"
	filter="$2"
	expected="$3"
	jq -e "$filter == $expected" "$root/$path" >/dev/null || fail "$path $filter expected $expected"
}

json_array_empty() {
	path="$1"
	filter="$2"
	jq -e "$filter == []" "$root/$path" >/dev/null || fail "$path $filter expected []"
}

json_matches() {
	path="$1"
	filter="$2"
	pattern="$3"
	actual="$(jq -r "$filter" "$root/$path")"
	printf '%s\n' "$actual" | grep -Eq "$pattern" || fail "$path $filter expected to match $pattern, got $actual"
}

json_profile_pass() {
	path="$1"
	profile="$2"
	jq -e --arg profile "$profile" '.profile_status[$profile] == "pass"' "$root/$path" >/dev/null || fail "$path profile $profile is not pass"
}

case "$mode" in
release | factory) ;;
*) fail "usage: sh scripts/release-check.sh [release|factory] [repo-root]" ;;
esac

command -v jq >/dev/null 2>&1 || fail "jq is required for structured evidence validation"

readiness=".agent/evidence/decision/release-readiness.json"
manifest=".agent/evidence/manifest.json"
trace=".agent/evidence/trace/traceability-matrix.json"
retrospective=".agent/evidence/retrospective.json"

require_file "$readiness"
require_file "$manifest"
require_file "$trace"

json_string_eq "$readiness" ".module" "clickhousex"
json_string_eq "$readiness" ".target_release_level" "L2-T4"
json_matches "$readiness" ".release_level_actual" "^L2-T[34]$"
json_bool_eq "$readiness" ".release_allowed" "true"
json_number_eq "$readiness" ".hard_failure_count" "0"

for profile in unit contract integration chaos benchmark adoption; do
	json_profile_pass "$readiness" "$profile"
done

for evidence in \
	.agent/evidence/raw/unit-test.json \
	.agent/evidence/raw/contract-test.json \
	.agent/evidence/raw/integration-test.json \
	.agent/evidence/raw/chaos-test.json \
	.agent/evidence/raw/adoption-test.json \
	.agent/evidence/raw/benchmark.txt \
	.agent/evidence/normalized/contract-check.json \
	.agent/evidence/normalized/integration-check.json \
	.agent/evidence/normalized/chaos-check.json \
	.agent/evidence/normalized/adoption-check.json \
	.agent/evidence/normalized/layer-guard.json \
	.agent/evidence/normalized/secret-scan.json \
	.agent/evidence/decision/test-plan.json \
	.agent/evidence/trace/traceability-matrix.json; do
	require_file "$evidence"
done

json_string_eq "$manifest" ".module" "clickhousex"
json_string_eq "$manifest" ".schema_version" "1.0"
json_string_eq "$trace" ".traceability_status" "complete"

if [ "$mode" = "factory" ]; then
	require_file "$retrospective"
	json_string_eq "$readiness" ".release_level_actual" "L2-T4"
	json_bool_eq "$readiness" ".factory_grade" "true"
	json_array_empty "$readiness" ".factory_blockers"
	json_string_eq "$readiness" ".profile_status.retrospective" "pass"
	json_string_eq "$retrospective" ".status" "pass"
fi

printf '%s\n' "release-check: $mode gate passed"
