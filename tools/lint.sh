#!/usr/bin/env bash
# Runs golangci-lint at exactly the version CI pins (#912).
#
# The pin is read from .github/workflows/validate.yml, so this never needs
# editing when CI moves. Each pinned version is built once into its own folder
# under $(go env GOPATH)/golangci-lint/<version>/, never into the shared
# ~/go/bin: other projects on this machine install other versions there, and a
# drifted binary passed PR #909 locally while CI failed it. An explicit
# GOLANGCI_LINT_CACHE in the environment is overridden on purpose (see below).
#
# Usage: tools/lint.sh [golangci-lint run args]
# With no args it runs the pre-push gate: --new-from-merge-base=origin/master.
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
workflow="$root/.github/workflows/validate.yml"
ver="$(sed -n 's/^[[:space:]]*version:[[:space:]]*\(v[0-9][0-9.]*\)[[:space:]]*$/\1/p' "$workflow")"
if [ "$(printf '%s\n' "$ver" | grep -c .)" != "1" ]; then
	echo "lint.sh: expected exactly one golangci-lint 'version: vX.Y.Z' pin in $workflow, found: '${ver}'" >&2
	exit 2
fi

gopath="$(go env GOPATH)"
if command -v cygpath >/dev/null 2>&1; then
	gopath="$(cygpath -u "$gopath")"
fi
bindir="$gopath/golangci-lint/$ver"
bin="$bindir/golangci-lint"
if [ ! -x "$bin" ] && [ ! -x "$bin.exe" ]; then
	echo "lint.sh: building golangci-lint $ver into $bindir (once per version)" >&2
	GOBIN="$bindir" go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$ver"
fi
if [ -x "$bin.exe" ]; then
	bin="$bin.exe"
fi

have="$("$bin" version)"
case "$have" in
*"has version ${ver#v} "*) ;;
*)
	echo "lint.sh: $bin reports '$have', CI pins $ver" >&2
	exit 2
	;;
esac

# Each version keeps its own analysis cache. The default cache is shared by
# every golangci-lint on the machine, and a v2.12.2 run that reused v2.15.0's
# cached results reported 0 issues on the line CI failed in PR #909.
export GOLANGCI_LINT_CACHE="$bindir/cache"

cd "$root"
if [ "$#" -eq 0 ]; then
	set -- --new-from-merge-base=origin/master
fi
echo "lint.sh: golangci-lint $ver run $*" >&2
exec "$bin" run "$@"
