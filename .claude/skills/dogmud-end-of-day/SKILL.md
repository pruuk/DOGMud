---
name: dogmud-end-of-day
description: Use at the end of a working day or session, when the owner signs off, says EOD, or asks to wrap up. Covers the daily archive sweep of docs/superpowers/specs and plans (shipped work to completed/, dropped work to abandoned/, every reference repointed), the repo-root tidy that archives old screenshots without touching binaries, the scratch sweep of C:\tmp and C:\gotmp (merged worktrees, stale cargo targets and scratch dirs), and the session handoff memory.
---

This skill is the owner's end-of-day SOP (ruled 2026-09-25). It keeps
`docs/superpowers/specs` and `docs/superpowers/plans` holding only live work,
keeps the repo root free of stale screenshots, keeps the shared scratch
directories from filling C:, and leaves a handoff the next session can start
from. Run all four parts every EOD, even when a part turns out to have nothing
to do.

## 1. Archive sweep: specs and plans

The top level of each folder holds only LIVE work. Everything else moves:

| Destination | What goes there |
|---|---|
| `completed/` | Work whose PR merged. Superseded notes whose successor shipped go here too, beside it |
| `abandoned/` | Work that was dropped or will never be built, per its own banner or an owner ruling |
| stays at top | An arc spec whose arc is still open, a spec or plan in progress, or a spec that covers an open follow-up chunk |

**Prove shipped from git, never from memory.** The first first-parent commit
on master that touches a file is the merge that brought it in:

```bash
git log --first-parent master --reverse --format='%h %ad %s' --date=short -- <path> | head -1
```

A plan can land before its work does (a PR can carry the plans for later
PRs), so check that the PR that shipped the WORK merged too:
`gh pr list --repo pruuk/DOGMud --state merged --search "<n>"`. An arc spec
stays at the top until the arc's LAST piece ships; read its roadmap or its
own open-items section to decide.

**Move with `git mv`, then repoint every reference.** Moved paths are cited
from Go comments, test constants, content YAML comments, tools, skills,
`context.md` files and `docs/README.md`. List the stale ones:

```bash
git grep -nF -f moved.txt -- . ':!docs/superpowers/specs/completed/*.md' ':!docs/superpowers/plans/completed/*.md' \
  | grep -vE "completed/(<moved names, |-joined>)"
```

- `specs/<name>` and `plans/<name>` rewrite to `specs/completed/<name>`. The
  rewrite is idempotent.
- Bare-filename relative links (`](name.md)`) need a hand fix.
- **Links FROM moved files break too.** A moved spec that links a sibling
  which stayed at the top now needs `../`; a moved plan's `../specs/x` becomes
  `../../specs/completed/x`. Run a relative-link check over every tracked
  `.md` and fix only the breaks this move caused; leave pre-existing ones.

⚠️ Git Bash `sed -i` strips every CR from the files it touches, even for a
within-line substitution. That is harmless only where the index stores LF
(`git ls-files --eol` shows `i/lf`), because `core.autocrlf` restores CRLF on
checkout. Check it before trusting a `sed` pass, and use the Edit tool on any
file whose index is CRLF. Either way, `git diff --numstat` must show only the
lines you meant to change.

`_datafiles/config.yaml` is an ordinary tracked file; a moved path cited there
is edited in place like any other doc. Local-only settings live in the
gitignored `config-overrides.yaml` (`dogmud-balance-config`).

Afterwards run the guard tests whose constants name spec paths
(`go test -count=1 -run 'Identifier|WireFreeze|ConsistentAttack' .`) and
`gofmt -l` on any touched Go file.

## 2. Repo-root tidy

- **Screenshots and loose images** older than today move to
  `_archive/screenshots/` (gitignored). TODAY'S stay: the owner drops fresh
  screenshots in the root for Claude to read. Never overwrite an archive entry
  of the same name; skip it and say so.
- **Stray logs or scratch files** in the root move to `_archive/logs/` or
  `_archive/notes/`.
- **Never delete or move a binary** (`*.exe`). They are build outputs the
  owner may be running, and killing or replacing the owner's server is a
  tripwire. List them if they look stale; do not act.
- **Never touch `novel/`**, `.mcp.json` or `.claude/settings.local.json`.
- Anything untracked and unrecognised: report it, do not move it.

## 3. Scratch sweep: C:\tmp and C:\gotmp

Owner, 2026-09-29: `C:\tmp` fills rapidly across projects (DOGMud worktrees,
boot checks, review scratch, and Ballistic's cargo `*-target` dirs, which
alone reached 12 GB in four days). What fills C: and what does not is measured
in the `reference-go-build-cache-fills-c-drive` memory; the Go caches are NOT
the problem, so do not `go clean -cache`.

Several sessions share these directories, some of them live. **Age and git
state decide, never the name.** Record `C:` free space first
(`Get-PSDrive C`).

1. **DOGMud worktrees under `C:\tmp`.** From the main checkout, `git fetch
   origin`, then for each `C:/tmp/...` row of `git worktree list`:
   - remove it (`git worktree remove <path>`, then delete the branch with
     `git branch -d`) only when `git -C <path> status --porcelain` prints
     nothing AND its HEAD is merged (`git merge-base --is-ancestor HEAD
     origin/master`, run in the worktree). A detached-HEAD worktree follows
     the same rule.
   - otherwise leave it and list it in the handoff with its branch: an
     unmerged or dirty worktree is someone's live work, this session's or
     another's.
   - Windows can hold a lock on a removed worktree's folder; if `git worktree
     remove` fails, `Remove-Item -Recurse -Force` the folder in PowerShell,
     then `git worktree prune`. A folder that still will not go is left for
     the next EOD.
2. **Any other directory with a `.git` inside** (another repo's worktree or
   clone, e.g. Ballistic's): leave it and report it. Only that repo's own
   session can judge it.
3. **Cargo target dirs** (a `CACHEDIR.TAG` at the top, no `.git`): delete when
   the folder was not written in the last 24 hours. A newer one may be a
   build in progress in another session.
4. **Everything else in `C:\tmp`** (scratch dirs and loose files with no
   `.git`): delete when last written more than two days ago. Newer ones stay:
   another session may be using them today.
5. **`C:\gotmp`**: delete `go-build*` dirs older than one day (Go failed to
   auto-clean them).

Mechanics, from the traps already hit:
- Files locked by running processes fail to delete; skip them and carry on.
  **Never stop or kill a process to free a lock** (the owner runs their own
  server on this machine).
- Keep the deletion and the size report in separate calls: a `Remove-Item`
  script that also formats sizes (`/1GB,2`) trips a path-safety scanner and
  the whole call is blocked.
- Never delete under `C:\Users\<user>\workspace\`, and never delete the
  current session's own scratchpad or worktree.

Report in the handoff: C: free before and after, how many worktrees were
removed, and every item left with its reason.

## 4. Handoff

Write or update the day's `project-session-handoff-YYYY-MM-DD` memory:
what merged, what is open and on which branch, what is waiting on someone
else, the issues filed or closed today, and every trap the day found.

**Status lives in `status.md`, never in `MEMORY.md`** (owner, 2026-09-28).
The memory index is pointers only, so it stops overflowing:

- Rewrite the "Now" section of the `status` memory (`status.md`): what is
  in flight, what merged, and open owner calls. Retired lines move to
  `STATUS-ARCHIVE.md` under a dated heading.
- **Bugs and planned work live in GitHub issues, not memory** (owner,
  2026-10-05). Every defect or idea found today that is not already an issue
  gets filed (`gh issue create --repo pruuk/DOGMud`, `bug` or `enhancement`
  plus an `area:*` label; `owner-decision` when it needs a ruling; milestone
  `1.0` when it blocks 1.0). Close issues that today's merges fixed, with the
  PR number. `status.md` and the handoff cite issue numbers instead of
  restating bugs. Exploit-class defects live on master stay out of public
  issues: tell the owner and use a private security advisory.
- In `MEMORY.md`, update only the one "Start here" line so it links the new
  handoff. Add a one-line pointer (about 200 characters) for each NEW memory
  file; never paste status, tables or long trap text into the index. Long
  trap text belongs in a topic file such as `reference-standing-traps.md`.
- Check the index before finishing. These must print nothing, and the size
  must stay well under 17 KB:

  ```bash
  awk 'length > 220 {print NR": "length}' MEMORY.md
  wc -c MEMORY.md
  ```

  Shorten any over-long line by moving its detail into the topic file it
  points at.

Other sessions write to the same memory folder. Edit `status.md` and
`MEMORY.md` with the Edit tool on the lines you own; do not rewrite another
session's lines.

## Commit

The archive sweep is its own commit on the working branch (or a
`chore/eod-archive-<date>` branch off master when no branch is open), with
named paths only. The message states how many moved to each destination, why
anything stayed at the top, and that the link check found no new breaks.
