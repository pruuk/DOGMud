---
name: dogmud-shipping
description: Use when committing, pushing, opening or merging a PR, running pre-push checks, or booting the server to verify a change. Covers the gh --repo pinning that stops PRs landing on the upstream fork parent, the pre-push gate order, the detached-worktree boot check, and the instance-save wipe before a smoke test.
---

This skill covers shipping a DOGMud change: git and `gh` hygiene around the
upstream fork, the pre-push gate order, the isolated boot check, and the
instance-save wipe before a local smoke test. It goes first among the shipping
concerns because it guards against the single most expensive mistake in the
old CLAUDE.md: a bare `gh pr create` once opened a PR against
`GoMudEngine/GoMud` and had to be closed immediately.

## The upstream hazard

Lifted verbatim from CLAUDE.md's "Git Workflow" section.

## Git Workflow
Follow the branch strategy in `docs/guides/github_guide.md`:
- `master` is the main integration branch + production. `origin` = pruuk/DOGMud
- **HARD RULE: do not touch `GoMudEngine/GoMud` at all.** No pushes, no
  branches, no PRs, no issues, no comments. Cherry-pick *from* upstream only.
  The single exception is work that **builds or modifies a module**
  (`modules/`), which is legitimately upstream-facing. Even then: propose it,
  get explicit approval first, and never open anything on upstream as a side
  effect of ordinary DOGMud work.
  Note `gh` defaults to the fork **parent**, so always pass
  `--repo pruuk/DOGMud` (see the gh section below).
- `development` is legacy from when the project still pulled from upstream
  (no longer used as the integration branch)
- Feature branches: `feature/stage-X.Y-description`, fixes: `fix/description`
- Use conventional commit messages (feat:, fix:, refactor:, docs:, chore:)

### `gh` is installed, always pin `--repo pruuk/DOGMud`

**WARNING: THIS REPO IS A FORK OF `GoMudEngine/GoMud`. `gh` DEFAULTS TO THE
PARENT.** A bare `gh pr create` opened a PR against **upstream** on
2026-08-08 and had to be closed immediately. Every `gh` command that can
target a repo MUST carry `--repo pruuk/DOGMud` explicitly. Do not rely on the
default, ever.

```bash
gh pr create --repo pruuk/DOGMud --base master --head <branch> ...
gh pr checks <n> --repo pruuk/DOGMud
gh run view <id> --repo pruuk/DOGMud --log-failed
```

### Ship via PR, not direct-to-master

`.github/workflows/run-tests.yml` (PR) runs **lint + a coverage gate**.
`build-and-release.yml` (master/tag) runs **neither** (review Finding 10). So a
direct push to master gets strictly weaker validation than a PR does. Until
roadmap Chunk 1.1 unifies them, ship through a PR and merge from the terminal.
No browser needed:

```bash
git push -u origin <branch>
gh pr create --repo pruuk/DOGMud --base master --head <branch> --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
gh pr merge  <n> --repo pruuk/DOGMud --merge --delete-branch
```

Use `--merge` (a `--no-ff` merge commit), **not** `--squash`. The project
convention is `--no-ff`, and per-commit messages carry the finding evidence and
verification notes that a squash would flatten into one blob.

**A green check is not a merge signal on its own.** `notify-discord` only
triggers on `pull_request: opened`, so a fix pushed to an existing PR is never
re-executed by that workflow. Check *which* runs actually re-ran before
concluding a workflow fix works.

Once Chunk 1.1 lands and both pipelines enforce the same contract, direct
merges to master become equivalent and this preference can relax.

### Two memory files, one hazard, folded together

Two separate memory entries cover this hazard and both are folded here rather
than picking one:

- [[feedback_gh_defaults_to_upstream_fork_parent]] documents the 2026-08-08
  incident (PR #643 opened against upstream by a bare `gh pr create`) and the
  rule to always read the URL `gh` prints back to confirm it says
  `pruuk/DOGMud`.
- [[feedback-gh-defaults-to-upstream-fork-parent]] extends the same hazard
  with two more leak vectors: pushing a brand-new branch to the fork makes
  GitHub render a "Compare & pull request" banner on **upstream's own** Pull
  Requests tab, even though nothing is created there, so a push to origin is
  not a free action; and the `upstream` remote's push URL has been
  deliberately set to `DISABLED` and must never be repaired.

Also folded here: [[feedback_master_is_main_branch]] (master is the
integration branch AND the production branch; upstream is cherry-pick only,
never merged), [[feedback_dev_branch_local_only]] (only `master` is pushed to
`origin`; `development` stays local and is not synced), and
[[feedback_merge_to_master_means_shipped]] (a `--no-ff` merge to master IS the
ship decision; a feature-specific "owed before prod" note in a backlog cannot
hold a commit back, because the droplet deploys the whole branch on every
pull). [[reference_gh_cli_now_installed]] confirms `gh` is installed and
authenticated as `pruuk`, superseding any older note that it was not
available.

## Pre-push gate order

Lifted verbatim from CLAUDE.md's "Pre-Push SOP" section.

## Pre-Push SOP

Local gates first, then let CI do the rest. The point of the local list is to
catch what CI cannot see or what wastes a CI round-trip.

1. **`gofmt -l internal/ modules/`** must print nothing. This has its own CI
   gate and has broken a push before. Cheapest possible check; run it first.
2. **`go build ./...`** and the tests for every package you touched.
3. **Update `docs/PATCH_NOTES.md`** with a dated entry. Player-facing framing,
   no raw numbers, no em dashes.
4. **`Logging.LogToFile: false`** in `_datafiles/config.yaml` (the droplet has
   limited disk). Note this file has `skip-worktree` set.
5. **Boot the server and confirm `Server Ready`.** `go build` only checks
   compilation. YAML data files (mobs, items, quests, dialogues, rooms, schedules,
   patrols) panic at *startup* on a filename/name-field mismatch, an invalid
   trigger event, an ID collision, or an unresolved reference. Nothing but a
   real boot catches these.

   Use an **isolated detached worktree** so you never disturb the user's running
   server, and copy the skip-worktree config in by hand:

   ```bash
   git worktree add --detach C:/tmp/dogmud-boot-check HEAD
   cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
   cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
   timeout 180 ./boot-check.exe > boot.log 2>&1
   grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log  # want 0
   grep -c "Server Ready" boot.log                                          # want 1
   ```

   **Build to `boot-check.exe`, do not `go run .`.** `go run` links the binary
   into a randomly-named temp directory on every invocation, and Windows
   Firewall keys its rules on the executable *path*, so every boot test looked
   like a brand-new unknown app and popped a "Windows Security Alert" dialog at
   whoever was at the keyboard, forever, because the approval could never
   apply to the next run. Building to a fixed path inside the (fixed) worktree
   means one `New-NetFirewallRule` for
   `C:\tmp\dogmud-boot-check\boot-check.exe` silences it permanently, and a
   rebuild to the same path keeps the rule, since rules match path and not
   hash. It also keeps compile time out of the 180s budget, which used to eat
   into the window the server had to actually come up in.

   **Exit code 124 is the success case**: it means the timeout fired because
   the server stayed up. Do not grep for the bare word `panic`: the config key
   `GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic` and
   will produce false hits. Clean up with `git worktree remove --force`, and if
   Windows holds a lock, `rm -rf` then `git worktree prune`.

   The same reasoning applies to running the game locally: prefer
   `go build -o dogmud.exe . && ./dogmud.exe` over `go run .` (both `/dogmud.exe`
   and `/*.exe` are already gitignored). One firewall rule for that path and the
   prompt stops for good.

6. **Push, open the PR, watch the checks.** A green check is **not** proof: a
   run can pass while emitting annotations, and the lint gate is configured
   `only-new-issues`. Confirm with `gh run view <id> --repo pruuk/DOGMud
   --log-failed` rather than trusting the summary.

7. After merge, delete the stray `refs/tags/master` if it re-seeds on origin.

## Boot check in an isolated worktree

The boot-check recipe above (step 5 of the Pre-Push SOP), repeated on its own
because it is a distinct, reusable procedure: build to a fixed-path
`boot-check.exe` inside a detached worktree, never `go run .`, treat exit code
124 as success, and do not grep for the bare word `panic` because
`GamePlay.MapConsistencyEnforce: panic` is a legitimate config value.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log  # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

[[reference_boot_test_in_isolated_worktree]] adds the earlier version of this
same recipe: a detached worktree plus `CONFIG_PATH` port overrides plus a
PID-scoped kill, so the boot test never collides with the user's live server.
It also records the bug class only a fresh-clone boot test catches (four
world folders were gitignored down to nothing, since git cannot track an
empty directory, and a fresh clone died before loading one room until tracked
`.gitkeep`s were added) and a teardown gotcha: Windows can hold a lock on the
freshly built exe immediately after killing the server, so `git worktree
remove --force` and MSYS `rm -rf` can both fail; PowerShell's `Remove-Item
-Recurse -Force` succeeds where they do not.

[[feedback_go_run_dot_not_main]] is the reason the recipe insists on `go run
.` (never `go run main.go`) whenever a boot check is run without building a
binary first: the repo root has multiple files in package `main`, so running
just `main.go` skips the others and fails with `undefined:` errors.

## Instance saves before a smoke test

Lifted verbatim from CLAUDE.md's "Instance Saves & Smoke-Test SOP" section.

## Instance Saves & Smoke-Test SOP (Important!)

The engine loads YAML templates first, then overwrites with instance
data from `_datafiles/world/dogmud/mobs.instances/` and
`_datafiles/world/dogmud/rooms.instances/` if present. **Stale instance
saves silently shadow template edits**, including new
`schedule_id:`, `patrol_id:`, `maxwander:`, idle commands, exits,
etc. This has been a recurring source of "my change isn't taking
effect" frustration.

**EXCEPTION: fields tagged `instance:"skip"` are NOT shadowed.**
`SaveRoomInstance` skips them when writing, and
`restoreSkipTaggedFields` (`internal/rooms/save_and_load.go`) copies
them back from the template after the instance overlay is applied,
so a stale save cannot override them. **Room spawn lists
(`Room.SpawnInfo`) are in this category** and were wrongly listed
above until 2026-07-25; a spawn-list edit takes effect on the next
room load with no wipe needed. Check the struct tag before assuming
a field is shadowed.

**SOP: nuke instance saves before every local smoke test.** Mirror
the prod policy where these directories are not deployed. Run:

```bash
rm -rf _datafiles/world/dogmud/mobs.instances \
       _datafiles/world/dogmud/rooms.instances
```

This removes the two directories themselves, not just their contents: the
loaders recreate them (`internal/rooms/save_and_load.go:522` builds each zone
folder at load; `internal/mobs/instance_save.go:171` builds it before every
write). Because the rooms side only does this at zone load, wipe with the
server **down**.

Then restart the server. The engine will re-spawn mobs and re-build
rooms from the (updated) templates. **Do NOT also wipe
`_datafiles/world/dogmud/shops/` or `_datafiles/world/dogmud/guilds/`**, those
are persistent living state (shop economy; player guilds), not
instance overrides (see Shop Persistence below). Guild files are
runtime-generated per-guild YAML (`guilds/<tag>.yaml`); a malformed one
logs+skips at boot rather than panicking (unlike authored content).

When making content changes you intend to smoke-test, run the wipe
as part of your pre-smoke ritual before the user is involved. When
the user reports "my change isn't taking effect," instance saves
should be your first guess.

[[feedback_no_instance_saves_to_prod]] adds the matching rule for the other
direction: never commit or push `mobs.instances/`, `shops/`, or
`rooms.instances/`. They are runtime data generated by the live server, and
pushing them to prod overwrites live server state; if `git status` shows
changes in these directories, leave them unstaged.

## Traps that exit 0 while doing the wrong thing

Four gotchas that report success while doing something other than what you
asked:

- **`git checkout <ref> -- <path>` stages the reversion.** It writes the file
  AND stages it; restoring the file in the working tree afterwards does not
  unstage anything, so a later `git add` on only the file you meant to change
  can still ship the reversion in the commit. Always run `git status` (or
  `git diff --cached --stat`) after any such checkout, before committing.
  [[feedback-git-checkout-pathspec-stages-the-revert]]
- **`git stash push -- <path>` with nothing to stash is a silent no-op that
  still exits 0.** A paired `git stash pop` then pops whatever was already at
  `stash@{0}`, which in this repo is one of several long-lived stashes. Prefer
  a detached worktree over a stash when the goal is comparing against an
  unmodified tree. [[feedback-git-stash-pathspec-noop-then-pop-hits-another-stash]]
- **`gh pr checks <n> --repo pruuk/DOGMud --watch` can return green before a
  slower, path-filtered job has even registered.** It reports on the checks it
  currently knows about, not the full expected set. Before merging anything
  touching a path-filtered trigger, confirm the expected workflows actually
  ran with `gh run list`. [[feedback-gh-pr-checks-can-return-early]]
- **A red `validate / lint` check on a large PR can be meaningless.** The
  `only-new-issues` gate asks GitHub's API for the PR patch, and that API
  refuses any diff over 20,000 lines; when it fails, the action cannot tell
  new findings from old and reports the entire grandfathered backlog as if the
  branch introduced it. Verify locally with `golangci-lint run
  --new-from-rev=master` before trusting a red check on a big PR.
  [[reference-lint-gate-inverts-on-large-prs]]

## Sources

Lifted verbatim from CLAUDE.md:
- "Git Workflow" (lines 44-98)
- "Pre-Push SOP" (lines 99-158)
- "Instance Saves & Smoke-Test SOP" (lines 159-205)

Folded memory files (rule stated inline above, cited here):
- [[feedback_gh_defaults_to_upstream_fork_parent]]
- [[feedback-gh-defaults-to-upstream-fork-parent]]

  Note: these two filenames (underscore vs. hyphen) cover the same rule and
  both exist in the memory store. This skill folds the union of both rather
  than picking one; resolving the duplicate file itself is later work, not
  this task's.
- [[feedback_dev_branch_local_only]]
- [[feedback_merge_to_master_means_shipped]]
- [[feedback_no_instance_saves_to_prod]]
- [[feedback_go_run_dot_not_main]]
- [[feedback_master_is_main_branch]]
- [[feedback-gh-pr-checks-can-return-early]]
- [[feedback-git-checkout-pathspec-stages-the-revert]]
- [[feedback-git-stash-pathspec-noop-then-pop-hits-another-stash]]
- [[reference_boot_test_in_isolated_worktree]]
- [[reference_gh_cli_now_installed]]
- [[reference-lint-gate-inverts-on-large-prs]]

Cited, not folded (dated incident records rather than standing rules):
- [[reference_reusable_workflow_permissions]]
- [[reference_advertising_listings_kit]]
