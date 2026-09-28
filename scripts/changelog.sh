#!/usr/bin/env bash
# Print a markdown changelog for the commits reachable from a tag (or any rev)
# since the previous vX.Y.Z tag. The first release lists every commit.
set -euo pipefail

end="${1:-}"
if [[ -z "${end}" ]]; then
	echo "usage: changelog.sh <tag>" >&2
	exit 2
fi
if ! git rev-parse --verify --quiet "${end}^{commit}" >/dev/null; then
	echo "unknown revision: ${end}" >&2
	exit 2
fi

prev=""
if git rev-parse --verify --quiet "${end}^" >/dev/null; then
	prev="$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' "${end}^" 2>/dev/null || true)"
fi

if [[ -n "${prev}" ]]; then
	range="${prev}..${end}"
else
	range="${end}"
fi

git log --no-merges --reverse --pretty=format:'%h%x09%s' "${range}" | awk '
function title(name) {
	if (names[name]) {
		return
	}
	names[name] = 1
	order[++norder] = name
}
function bucket(subject) {
	if (subject ~ /^(feat|fix|docs|perf|refactor|test|ci|build|chore)(\([^)]+\))?!:/) {
		return "Breaking changes"
	}
	if (subject ~ /^feat(\([^)]+\))?:/) return "Features"
	if (subject ~ /^fix(\([^)]+\))?:/) return "Fixes"
	if (subject ~ /^perf(\([^)]+\))?:/) return "Performance"
	if (subject ~ /^refactor(\([^)]+\))?:/) return "Refactoring"
	if (subject ~ /^docs(\([^)]+\))?:/) return "Documentation"
	if (subject ~ /^(test|ci|build|chore)(\([^)]+\))?:/) return "Maintenance"
	return "Other"
}
function text(subject) {
	sub(/^(feat|fix|docs|perf|refactor|test|ci|build|chore)(\([^)]+\))?!:[[:space:]]*/, "", subject)
	sub(/^(feat|fix|docs|perf|refactor|test|ci|build|chore)(\([^)]+\))?:[[:space:]]*/, "", subject)
	return subject
}
BEGIN { FS = "\t" }
NF < 2 { next }
{
	name = bucket($2)
	title(name)
	items[name] = items[name] sprintf("- %s (`%s`)\n", text($2), $1)
}
END {
	if (norder == 0) {
		print "No changes recorded."
		exit
	}
	print "## Changelog"
	print ""
	wanted[1] = "Breaking changes"
	wanted[2] = "Features"
	wanted[3] = "Fixes"
	wanted[4] = "Performance"
	wanted[5] = "Refactoring"
	wanted[6] = "Documentation"
	wanted[7] = "Maintenance"
	wanted[8] = "Other"
	for (i = 1; i <= 8; i++) {
		name = wanted[i]
		if (!names[name]) continue
		print "### " name
		print ""
		printf "%s", items[name]
		print ""
	}
}
'
