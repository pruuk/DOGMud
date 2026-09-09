# DOGMud - Claude Code Project Memory

## Subagent Model Preference
Pick the model that fits the task — don't reflexively pin everything to haiku.

- **haiku** — trivial mechanical work: a single-file grep/glob, a one-shot
  symbol lookup, a fixed-recipe edit. Cheap and fine when there's no judgment
  involved.
- **sonnet / opus** — exploration or implementation that benefits from
  reasoning: tracing how a subsystem fits together, multi-file searches where
  the answer requires synthesis, refactoring, architectural decisions,
  multi-step code writing, or executing a plan task with real logic.

We added the **codegraph MCP** specifically to cut token use on code
intelligence (sub-millisecond symbol/caller/callee queries instead of grep +
many Reads). That headroom means using a stronger exploration agent is usually
the right call when the task warrants deeper reasoning — instruct those agents
to prefer codegraph tools for symbol verification so the stronger model spends
its budget on thinking, not file-spelunking. When in doubt for a non-trivial
task, default up (sonnet), not down.

## Project Context
- DOGMud (Delusions of Grandeur) is a MUD built on the GoMud engine
- World design document: `docs/world.md`
- Development roadmap: `docs/roadmaps/DEVELOPMENT_PLAN.md`
- Remote origin: https://github.com/pruuk/DOGMud
- Remote upstream: https://github.com/GoMudEngine/GoMud

## Codegraph MCP — Code Intelligence

The `codegraph` MCP server indexes every Go symbol in the repo into a
local SQLite knowledge graph (~4.6k files, ~18k nodes, ~60k edges).
Sub-millisecond queries return signatures, sources, callers, callees,
and trails. Use it BEFORE writing code, not during.

**Use it for:**
- **Pre-dispatch verification.** Before sending a subagent off, run 2–3
  `codegraph_node` / `codegraph_search` calls to confirm the struct
  shapes, function signatures, and field names the plan references.
  Cheaper than letting a subagent waste turns rediscovering or, worse,
  shipping code against a stale plan. (Caught a real `Engine` field
  rename during 4.2 — plan said `mobTrees`/`noMobTree`, actual is
  `trees`/`noTree`.)
- **Symbol-trail navigation.** `codegraph_node Foo` with `includeCode:true`
  returns the source + callers/callees with file:line. Replaces
  grep + 5–10 Reads.
- **Disambiguation.** Code-base has many `Add` / `Remove` / `Clear` /
  `ClearCache` symbols across packages. `codegraph_node` lists all
  matches and shows the one you asked for, so you don't accidentally
  Read the wrong file.
- **Front-loading subagent prompts.** Paste the verified struct/signature
  into the prompt's "context I've already verified for you" block so
  the subagent skips exploration.

**Don't use it for:**
- File authoring that doesn't reference Go symbols — YAML data files,
  templates, prose docs.
- "Find me the test helper that looks similar to X" — codegraph models
  structure, not similarity. Glob + Read is right for that.
- Confirming code you JUST edited — the index lags ~1s; trust your edit
  + file state over the index for symbols you touched this turn.

**Tool selection by intent (lifted from the codegraph server docs):**
- "What's the deal with this task/feature/area?" → `codegraph_context`
  (composes search + node + callers + callees in one call).
- "What is/calls/triggers this symbol?" → `codegraph_node` (with
  `includeCode:true` for source).
- "Find a symbol by name" → `codegraph_search`.
- "Trace from X to Y" → `codegraph_trace`.

**Subagent guidance.** When dispatching a subagent that needs to touch
unfamiliar code, instruct it to prefer codegraph MCP tools over Read/Grep
for symbol verification — saves their context window and reduces
back-and-forth.

## Package `context.md` Convention

Every package under `internal/` and `modules/` carries a `context.md` — a
developer/agent-facing description of what the package is and how to use it
correctly. **Any work that creates a new package MUST ship one; any work that
reshapes an existing package's API, data model, or file list MUST update it.**
(This rule previously lived only in `docs/roadmaps/MOB_ALIVENESS_ROADMAP.md`,
which is why coverage drifted — 37 packages had none and several documented
functions that did not exist.)

**Verify before you document.** Every symbol you name must exist. Check it with
`codegraph_search` / `codegraph_node`, or extract the real surface with:

```powershell
Select-String -Path internal\<pkg>\*.go -Pattern '^(func|type|const|var)\s'
```

A `context.md` that describes an invented API is worse than no file at all — an
agent will code against it and the mistake surfaces at compile time or, worse,
at runtime.

**Structure** (adapt, don't pad):

- `## Purpose` — what it does and why it exists, 2–4 sentences. Say what it
  deliberately does *not* do.
- `## Files` — one line per file.
- Core types with real field names, in a `go` block.
- `## Public API` — verified signatures, grouped by job.
- `## Gotchas` — the things that bite. Nil-return contracts, panics,
  comparison hazards, ordering requirements, deliberate-looking-wrong code.
- `## Dependencies` and `## Consumers`.

**Do not write** "Future Enhancements," "Security Considerations,"
"Performance Characteristics," "Administrative Features," or "Scalability"
sections unless the package genuinely has something specific to say. The
upstream-generated files are full of that filler and it is being removed, not
copied.

Good exemplars (verified 2026-07-31): `internal/term/context.md` (small,
declarative), `internal/mutators/context.md` (medium, lifecycle-heavy),
`internal/mapper/context.md` (large, multi-subsystem).

## Spell Duration System
All spell durations use `calcSpellDuration(baseFolds, skill, willpower)`:
`duration = baseFolds × (10 + wil/20 + skill/2)`. Effect-specific scaling:
shield = full, heal = ÷2, DoT = ÷3.

