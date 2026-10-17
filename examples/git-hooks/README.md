# Git hooks: dependencies changed under you

You pull or switch branches, run the tests, and spend ten minutes on a failure
caused by a lockfile you never reinstalled. The hooks here warn when
dependency, toolchain, or migration files differ between the old and new
commit.

```text
Dependencies changed
3 changed:
bun.lock
go.sum
migrations/0042_add_index.sql
Reinstall tools/dependencies or run migrations before testing.
```

## Install

```sh
mkdir -p scripts
cp examples/git-hooks/deps-changed.sh scripts/
cp examples/git-hooks/post-merge examples/git-hooks/post-checkout .git/hooks/
chmod +x scripts/deps-changed.sh .git/hooks/post-merge .git/hooks/post-checkout
```

With [Lefthook](https://lefthook.dev), skip the hook files:

```yaml
# lefthook.yaml
post-merge:
  jobs:
    - run: scripts/deps-changed.sh ORIG_HEAD HEAD
post-checkout:
  jobs:
    - run: test {3} = 1 && scripts/deps-changed.sh {1} {2} || true
```

Edit the file list in [deps-changed.sh](deps-changed.sh) for your stack.
Patterns are Git pathspecs: `go.sum` matches only at the root; use
`'*/go.sum'` for nested modules.

## Notes

- The default stderr route prints the warning right under `git pull` output,
  which is often the best place for it. Desktop works too.
- Hooks never fail the Git operation: delivery errors are ignored.
- Agents in worktrees get the same warning in their command output.
