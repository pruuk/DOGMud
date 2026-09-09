---
name: dogmud-refactoring
description: Use when modifying, removing, or restructuring code that already works, as opposed to writing something new. Covers deleting the Go field first and letting the compiler enumerate its consumers, grepping for existing infrastructure before building a parallel mechanism, the shallow-copy trap where a struct copy shares pointers and maps with its template, doing a full sweep on removal rather than a partial hack-around, and enumerating every wiring step when adding an admin command.
---

This skill covers changing code that already works: modifying, removing, or
restructuring it, as distinct from writing something new. Most work in this
repository is this kind of work.

## Grep for how the codebase already does this job

Before inventing a mechanism, grep for how the codebase already does that
job. This is the user's global instruction, not a DOGMud-specific memory
file, and it belongs first because it is the cheapest check available and
the one most often skipped. If the codebase already composes a score,
resolves a contest, picks a winner, or gates an award, use that. A bespoke
variant has to justify itself against the existing one and usually cannot.
A 30-second question here prevents a day of redesign.

`[[feedback_search_for_existing_infrastructure_first]]` is the DOGMud
instance of the same failure: a zone-adjacency crawl was written for
cross-zone quest routing without checking whether one existed. One did,
in `modules/weather/crawler/build.go`, and it was easy to miss because
`internal/` never imports `modules/`, so it never surfaces in the call
paths you are already reading. Grep the domain noun (`adjacency`,
`neighbour`, `graph`, `index`) across both `internal/` AND `modules/`
before writing anything that indexes, caches, crawls, or graphs the
world. Note that in that case building separately turned out to be the
right call for documented reasons (an undirected edge type that
discarded data the new use case needed, an arch boundary, a vendored
package), so the lesson is not "never duplicate": it is that the
duplication should be a documented decision made after looking, not a
discovery the user has to prompt.

## Verify negatives

"I grepped and found nothing" is only evidence if the grep could have
found something. This is also a user global instruction, and it pairs
directly with the section above: the search you run to confirm nothing
already does this job is exactly the search that can silently fail. A
glob that matches no files, a pattern with a typo, and a genuinely
absent symbol all return empty output, and they look identical from the
outside.

Before concluding absence, confirm the search was capable of succeeding:
- Run the same pattern against a term you know exists, to confirm the
  glob or grep scope is not empty or misconfigured.
- Check the pattern against a known false positive or a known true
  positive nearby, not just the target case.
- If searching only `internal/`, also search `modules/` (or vice versa):
  the import graph between them does not run both ways, so a grep scoped
  to one side by habit can return a clean empty result that is simply
  the wrong search.
- Prefer the domain noun or the emitted string (a JSON key, a template
  variable) over the Go identifier alone when checking for downstream
  consumers, since a plain identifier grep does not follow into
  templates or JS.

## The compiler is the dead-code sweep

When removing a struct field or type in Go, delete the declaration first
and let the compiler enumerate the consumers. Do not trust a grep sweep,
and do not trust a followup note's count of affected surfaces.
`[[feedback_compiler_is_the_dead_code_sweep]]` records a removal
(`Room.SkillTraining`) that a scoped grep estimated at 2 surfaces; it was
actually 9, because a Go field also feeds JSON/GMCP payload keys,
template field references, and client-side string literals, none of
which contain the Go identifier.

How to apply:
1. Delete the declaration, run `go build ./...`, and treat the error
   list as the authoritative work list. Fix one, rebuild, repeat.
2. For each Go consumer the compiler finds, ask what it emits (a payload
   key, a tag, a template variable) and grep the templates/JS for that
   emitted string, not the Go name, to catch the non-Go surfaces
   downstream of it.
3. Verify with the full suite, the boot smoke test, and the
   unknown-YAML-key drift gate, which proves the removal introduced no
   silently-ignored authored keys.

## Removal means a full sweep

When asked to remove a feature, do the full removal, not a hack-around
that hides the mechanic while leaving its scaffolding in place.
`[[feedback_remove_downed_fully]]` describes the "downed condition"
being asked for removal twice: the underlying mechanic (bleedout, the
PlayerDrop event, two-tier death) was removed once, but display tags, a
find-flag, room/zone filters, and test fixtures survived. The leftovers
resurfaced as a bug: a mob whose health dropped below the removed
threshold, but whose death-handler had not fired, sat in the room
tagged "(downed)" running idle commands, looking like the removal had
not taken.

A full sweep on removal covers, at minimum: production code, display
tags, find-flags, room/zone filters, btree primitives, condition
constants, help templates, and test fixtures. Audit with grep before
declaring done. A partial hack-around does not delete the concept, it
just hides it, and hidden scaffolding re-arms later as a confusing bug
that looks like the removal never happened.

## Shallow copies share pointers

A struct copy made as `dst := *src` shares every pointer-, map-, and
slice-typed field between `src` and `dst`. `[[feedback_shallow_copy_shared_pointers]]`
traces a real instance: `mobs.newMobByIdInternal` copies a template Mob
with `mob := *m`, and every spawned instance ended up sharing the
template's `Life`, `CombatPhase`, `Position`, `Awareness`, and `Activity`
state machines (all `*state.Machine`), plus a `combatPhaseWired=true`
guard that then suppressed re-initialization for every instance. Deaths
silently failed because the despawn cascade closed over the template
character (`MobInstanceId=0`) and bailed on an instance-id check meant
for real instances.

Symptoms that should raise this as a suspect:
- An observer fires against the wrong instance id.
- A mutation on one mob, item, or character instance leaks into other
  spawns of the same template.
- The first spawned instance behaves correctly and every later one
  behaves like a copy of the first.
- A state-machine cascade does not fire for a specific instance, or a
  callback only ever sees the template, never the instance.

When any of these show up, audit the spawn or clone path for a
`dst := *src` (or an equivalent slice copy) and check every pointer-,
map-, and slice-typed field on the struct for whether it is explicitly
re-initialized after the copy. Anything not explicitly reset is shared
with the template. A useful diagnostic: log the pointer address (via
`fmt.Sprintf("%p", c)`) at both the wire-up site and the fire site; if
the addresses disagree between where a callback was registered and
where it fires, that is a shallow-copy or wrong-instance bug.

## Admin command wiring

When a plan adds a new admin command, or a new subcommand on an
existing one, `[[feedback_admin_command_wiring_checklist]]` requires the
plan to enumerate every wiring step as its own task, because a subagent
executing one slice of a plan only sees that slice and will otherwise
ship an unreachable command. A prior instance shipped a handler that
returned correct output under direct unit test, but was never added to
the command-table registration, so the in-game command resolved to
"unknown command."

Full checklist for a new admin command, each item its own task, not
bundled together:
1. Handler file at `internal/usercommands/admin.<name>.go` (mirror the
   shape of an existing handler such as `admin.opinion.go`).
2. Registration in `internal/usercommands/usercommands.go`'s command
   table (the `map[string]CommandAccess{}` near line 100 to 300). The
   trailing `true` triple marks it admin-only.
3. Helpfile template at
   `_datafiles/world/dogmud/templates/admincommands/help/command.<name>.template`.
4. Unit or integration test exercising at least one subcommand.
5. Pre-push smoke check: `help <name>` returns the template, and the
   command itself returns its top-line usage when invoked with no
   arguments.

Checklist for adding subcommands to an existing admin command:
1. Dispatch case added to the existing handler.
2. Helpfile template updated to document the new subcommands. The plan
   must say "update," not "ensure exists," so a subagent does not no-op
   on seeing the file already present.
3. Unit test for each new subcommand.
4. Pre-push smoke check: invoke each new subcommand at least once.

The granularity is the safety net: "create handler and register it" is
two tasks, not one, because collapsing them is exactly how registration
gets dropped.

## Related preference notes

Two related preferences govern how refactoring and review findings are
handled, and are not restated here because they belong to working-style
guidance rather than a code-change procedure:
`[[feedback-dont-file-your-own-inconsistency-as-followup]]` (a change
that leaves a sibling code path half-converted must finish the sibling,
not log it as a follow-on) and `[[feedback-fix-flaws-dont-revert-the-work]]`
(when review finds flaws in delivered work, fix them rather than
reverting the work).

## Sources

- [[feedback_compiler_is_the_dead_code_sweep]]
- [[feedback_search_for_existing_infrastructure_first]]
- [[feedback_shallow_copy_shared_pointers]]
- [[feedback_remove_downed_fully]]
- [[feedback_admin_command_wiring_checklist]]
- [[feedback-dont-file-your-own-inconsistency-as-followup]]
- [[feedback-fix-flaws-dont-revert-the-work]]
- User global instructions (`C:\Users\Calabe Davis\.claude\CLAUDE.md`):
  grep before inventing a mechanism; verify negatives before concluding
  absence.
