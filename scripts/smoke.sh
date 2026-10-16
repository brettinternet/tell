#!/bin/sh
set -eu
# The Go smoke test uses t.TempDir for owned, portable fixture cleanup.
exec mise exec go -- go test -tags smoke -count=1 ./integration
