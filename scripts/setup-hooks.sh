#!/bin/sh
# Point git at the versioned hooks in .githooks/.
#
#   sh scripts/setup-hooks.sh
set -eu

cd "$(git rev-parse --show-toplevel)"
git config core.hooksPath .githooks
echo "git hooks enabled: .githooks (pre-commit: gofmt, vet, test; pre-push: test -race, build)"
