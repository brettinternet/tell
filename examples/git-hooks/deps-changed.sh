#!/bin/sh
# Usage: deps-changed.sh OLD NEW
# Warns when dependency, toolchain, or migration files differ between two commits,
# so you reinstall before chasing a "works on main" failure.
set -u
old=$1 new=$2
[ "$old" = "$new" ] && exit 0

changed=$(git diff --name-only "$old" "$new" -- \
    package.json bun.lock bun.lockb package-lock.json pnpm-lock.yaml yarn.lock \
    go.mod go.sum Cargo.lock uv.lock poetry.lock 'requirements*.txt' Gemfile.lock \
    mise.toml .tool-versions flake.lock 'migrations/*' '*/migrations/*' 2>/dev/null) || exit 0
[ -n "$changed" ] || exit 0

count=$(printf '%s\n' "$changed" | wc -l | tr -d ' ')
shown=$(printf '%s\n' "$changed" | head -n 5)
tell notify --type warning --title "Dependencies changed" -- "$count changed:
$shown
Reinstall tools/dependencies or run migrations before testing." </dev/null || true
