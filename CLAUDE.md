# DOGMud - Claude Code Project Memory

## Project Context
- DOGMud (Delusions of Grandeur) is a MUD built on the GoMud engine
- World design document: `docs/world.md`
- Development roadmap: `docs/roadmaps/DEVELOPMENT_PLAN.md`
- Remote origin: https://github.com/pruuk/DOGMud
- Remote upstream: https://github.com/GoMudEngine/GoMud

## Working style

Cross-project rules (subagents and model choice, PowerShell vs Bash, no
console popups, no dashes, `git add` by name, no Python file edits, kill by
PID, the `grep -c` trap) live in `~/.claude/CLAUDE.md` and its hooks. This
file holds only what is DOGMud's own.

- Design proposals of roughly 30 to 40 lines; longer belongs in a file.
- The brainstorming visual companion is a standing yes.
- A flaw found mid-task widens scope, it does not cut it.
- Fix flaws in delivered work rather than reverting the work.
- Finish sibling code paths you made inconsistent; do not file your own
  inconsistency as a follow-up.
- The owner runs all deploys; Claude prepares and merges but never deploys.
- Known bugs and planned work live in GitHub issues on `pruuk/DOGMud`, not
  in memory. Search before filing; exploit-class bugs live on master stay out
  of public issues.
- Every EOD runs `dogmud-end-of-day`: archive shipped specs and plans, tidy
  the repo root, sweep merged worktrees and stale scratch from `C:\tmp`,
  write the handoff.

## Tripwires

These fire unprompted, before any skill would normally load, which is why
they live here instead of only in a skill.

- Every `gh` command carries `--repo pruuk/DOGMud`. This repo is a fork of
  `GoMudEngine/GoMud` and `gh` defaults to the parent; a bare `gh pr create`
  once opened a PR on upstream. See `dogmud-shipping`.
- The owner's own server may be running on this machine. Test servers use
  private ports and are killed by PID; see `dogmud-playtesting`.
- `_datafiles/config.yaml` is an ordinary tracked file (its skip-worktree bit
  was retired on this machine 2026-10-10). Local-only settings (dev port, log
  level) live in the gitignored `_datafiles/world/dogmud/config-overrides.yaml`,
  which the engine overlays at boot. Never put a local value in `config.yaml`.
  See `dogmud-balance-config`.
- Balance numbers come from `config.yaml`, never from a Go default. Several
  shipped values differ sharply. See `dogmud-balance-config`.
- New files: state the full path and add them to `docs/README.md`.

## Codegraph MCP

The `codegraph` MCP server indexes every Go symbol in the repo into a local
SQLite knowledge graph (~4.6k files, ~18k nodes, ~60k edges). Sub-millisecond
queries return signatures, sources, callers, callees, and trails. Use it
BEFORE writing code, not during.

**Tool selection by intent:**
- "What's the deal with this task/feature/area?" -> `codegraph_context`.
- "What is/calls/triggers this symbol?" -> `codegraph_node`.
- "Find a symbol by name" -> `codegraph_search`.
- "Trace from X to Y" -> `codegraph_trace`.

## Package `context.md` Convention

Every package under `internal/` and `modules/` carries a `context.md`, a
developer/agent-facing description of what the package is and how to use it
correctly. Any work that creates a new package MUST ship one; any work that
reshapes an existing package's API, data model, or file list MUST update it.

**Verify before you document.** Every symbol you name must exist. Check it
with `codegraph_search` / `codegraph_node`, or extract the real surface with
`Select-String -Path internal\<pkg>\*.go -Pattern '^(func|type|const|var)\s'`.
`tools/context_md_audit.py` finds `context.md` files that document symbols
their package no longer defines. This phase proved the point:
`internal/items/context.md` carried three phantom symbols (`UseItem`,
`ItemPower`, `IsUpgrade`) until this work cleared them.

## Skills

Project procedure lives in `.claude/skills/` as thirteen on-demand skills; the
harness lists them with descriptions, so no index is repeated here.
Subsystem detail lives in each package's `context.md`.
