# DOGMud - Codex Project Instructions

This is the compact, always-loaded operating memory for Codex. `CLAUDE.md`
retains the detailed historical memory; the documents linked below are the
authoritative sources for subsystem detail and backlog status. Read only the
references relevant to the task instead of loading every historical plan.

## Workflow and Skills

- Check the available skills before any response or action. If a skill applies,
  read and follow it before exploring or asking questions.
- Use the Superpowers workflow: brainstorm and obtain approval before creative
  changes; use `systematic-debugging` before fixing unexpected behavior;
  implement features and bug fixes test-first; use `writing-plans` for approved
  multi-step designs; use `executing-plans` or `subagent-driven-development`
  when executing an implementation plan; request review for substantial work;
  and run `verification-before-completion` before claiming success.
- Any task complex enough to use the full brainstorm -> spec -> plan -> execute
  -> validate lifecycle has a mandatory independent adversarial plan-review
  gate after the plan is written and before execution begins. Dispatch a
  reviewer subagent with the approved spec, canonical roadmap/backlog notes,
  implementation plan, and current repository evidence, but not the planner's
  conversation history. The reviewer must try to falsify file/symbol
  assumptions, task ordering, test commands, scope boundaries, and spec
  coverage. Resolve every valid Critical and Important finding in the plan, or
  bring a disputed finding to the user explicitly, before offering or starting
  execution. A planner self-review does not satisfy this gate. If subagents are
  unavailable, stop and disclose that the gate has not run rather than calling
  the plan ready.
- Do not treat an existing plan as proof that its assumptions still match the
  repository. Verify symbols, config values, and shipped behavior first.
- Delegate only when the user, an applicable skill, or these instructions call
  for it. Parallelize independent read-heavy work. Avoid concurrent edits to the
  same files. For content agents, allocate non-overlapping IDs first.
- Codex model intent mapping: `gpt-5.6-luna` for trivial mechanical lookups,
  `gpt-5.6-terra` for normal exploration/implementation, and `gpt-5.6-sol` for
  architecture or the hardest reasoning. Default upward for non-trivial work.
- Prefer codegraph tools for Go symbol discovery when available; otherwise use
  `rg`. Verify newly edited symbols from the files/tests because indexes lag.

## Project and Source Routing

DOGMud (Delusions of Grandeur) is a MUD built on the GoMud engine.

- World and tone: `docs/world.md`
- Current compact backlog: `docs/roadmaps/CURRENT_BACKLOG.md`
- Legacy broad plan: `docs/roadmaps/DEVELOPMENT_PLAN.md`
- Current remediation: `docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`
- Resolution/cost arc: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`
- NPC systems: `docs/roadmaps/MOB_ALIVENESS_ROADMAP.md`
- Combat state machines: `docs/roadmaps/COMBAT_STATE_ROADMAP.md`
- Content schemas: `docs/schemas/`
- Package contracts: `internal/*/context.md`, `modules/*/context.md`
- Git procedure: `docs/guides/github_guide.md`
- Detailed imported historical memory: `CLAUDE.md`

Before planning new work, read `CURRENT_BACKLOG.md` and the relevant canonical
roadmap. Explicit tracker status and merged code outrank filenames, unchecked
boxes in archived plans, or stale prose.

## Repository and Git Safety

- `master` is integration and production. `development` is legacy.
- `origin` is `pruuk/DOGMud`.
- **Never interact with `GoMudEngine/GoMud` upstream:** no pushes, branches,
  PRs, issues, comments, or other writes. Cherry-pick from upstream only.
  Module work under `modules/` is the sole possible upstream-facing exception;
  propose it and obtain explicit approval before any upstream action.
- Every `gh` command that can select a repository must include
  `--repo pruuk/DOGMud`; bare `gh` defaults to the fork parent.
- Ship through a PR unless the user explicitly directs otherwise. Use merge
  commits (`--merge`), not squash. A green summary is insufficient: confirm
  which runs reran and inspect failed logs/annotations.
- Branches: `feature/stage-X.Y-description` or `fix/description`. Use
  conventional commits (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`).
- Preserve dirty/untracked user work. Do not alter `.agents/`, `.codex/`, or
  unrelated changes unless the task explicitly includes them.

Typical release commands:

```text
git push -u origin <branch>
gh pr create --repo pruuk/DOGMud --base master --head <branch> --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
gh run view <id> --repo pruuk/DOGMud --log-failed
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

After merge, remove a stray `refs/tags/master` if it reseeds on origin.

## Required Verification and Pre-Push SOP

Run checks proportional to the change. Before pushing:

1. `gofmt -l internal/ modules/` must print nothing.
2. Run `go build ./...` and tests for every touched package.
3. Update `docs/PATCH_NOTES.md` with a dated, player-facing entry. No raw
   balance numbers and no em dashes in player-facing prose.
4. Confirm `Logging.LogToFile: false` in `_datafiles/config.yaml`; it is
   skip-worktree and production disk is limited.
5. Boot the real server and confirm exactly one `Server Ready` and no actual
   panic/runtime crash. Compilation and YAML parsing alone do not prove boot.
6. For player-facing content, complete the adversarial playtest gate below.
7. Push, open a PR, and inspect the actual CI runs.

Use an isolated detached worktree for boot checks so the user's server and
working tree remain untouched. Copy the skip-worktree config into it. Build to
the fixed path `C:/tmp/dogmud-boot-check/boot-check.exe`; never use `go run .`
for boot tests because changing temp executable paths repeatedly trigger Windows
Firewall prompts. A timeout exit (historically 124) is success when the server
remained running. Search for `^panic:`, running goroutines, or runtime errors,
not the bare word `panic` (`MapConsistencyEnforce: panic` is valid config).
Clean up the verified worktree afterward.

## Player-Facing Content Gate

Any task or plan that authors rooms, mobs, items, quests, dialogue, tutorials,
onboarding, help prose, or other player-facing content must end with a fresh,
critical in-game playtest before handoff. Boot-clean verifies the system, not
the experience.

Use the `source-command-playtest` skill for a local or production session and
`source-command-playtest-scenario` only when actors truly require one shared
world. Local sessions require an absolute checkout and a goals file containing
`ephemeral:`. Spawn a fresh character, drive the complete real flow, read every
line, identify confusing prompts, buried instructions, broken gates, dead ends,
voice problems, and pacing defects, fix them, and rerun as needed. Never claim
content is done from a clean boot alone.

Example intent:

```text
/playtest local --checkout <abs> bug-finder 2026-08-03-prepush-sweep.yaml
```

`tools/playtest/targets.yaml` contains production credentials, is gitignored,
and must never be committed, displayed, or pasted. Local ephemeral tests do not
use it. The retired `tools/mud_bridge.py` / `tools/ai_player.py` path is not the
current harness.

## Content Authoring Rules

- Before creating YAML, run `python tools/id_inventory.py` (optionally
  `--zone`, `--type`, or `--alloc`). Sequential content agents are the default.
  For parallel content creation, pre-allocate a distinct block for each agent,
  put the exact range in its prompt, and rerun inventory after merge.
- Read `docs/world.md`, the relevant `docs/schemas/` document, and existing
  nearby examples before authoring. Restart/boot after any data-file change.
- Derive filenames from the loader's `Filepath()` contract. Zone folders use
  underscores. `ConvertForFilename` lowercases, preserves a-z/0-9, drops
  apostrophes, and maps other characters to underscores. Buff, item, and mob
  filenames use converted names; spell filenames use `spellid`. A mismatch can
  panic at startup.
- Keep multi-word player nouns and `component_tag`s space-separated unless the
  phrase is naturally hyphenated. Matching already supports multi-word input.
- Wrap all player-visible text at about 78-80 columns.
- Do not expose raw damage, healing, armor, duration, or balance values in
  player-facing messages. Use descriptive helpers such as
  `combat.GetDamageDescription` and `combat.GetHealDescription`. The `status`
  stat sheet is the deliberate exception.
- For map edits, preserve Cartesian consistency. `oneway: true` exempts only
  reciprocity; `non_cartesian: true` belongs on intentionally toroidal/maze
  zones. Named/portal exits are non-spatial. Run `cartcheck` when relevant.

### Quest and dialogue invariants

- Every `grantsQuest` node/pattern lists both the start token and the quest's
  `{id}-end` token in `questExcluded` so completed quests cannot be re-granted.
- Quest-granting nodes include `quest` and `task` triggers; introducing patterns
  include those keywords. Every trigger must be discoverable in dialogue,
  hints, room prose, or the quest log.
- NPC `text` is first-person NPC speech. `hints` are narrator/player-facing.
- Prefer `questRequired` over expiring per-player `requires`. Use
  `expiryPeriod` only when urgency is intentional.
- `give.go` transfers an item before handlers fire. Use quest `item_give` and/or
  mob `player_give`; use `return_item` when the NPC should not retain it. Any NPC
  granting a physical quest item needs a loss-recovery dialogue path.
- Declare every quest flag and all allowed values in quest YAML; undeclared
  references panic at boot. Use `{questId}-{flagName}` references. Branching
  quests need a declared flag, branch setters, required/excluded gates,
  wrong-path dismissal nodes first, path-specific roots, and cross-NPC
  mid-quest variants.

## Runtime Data and Smoke-Test Safety

Templates load first; mob/room instance saves can then shadow template fields.
Before local smoke tests, clear only:

```text
_datafiles/world/dogmud/mobs.instances/*
_datafiles/world/dogmud/rooms.instances/*
```

Never clear `shops/`, `guilds/`, or `moderation/`; these are persistent living
state. Deleting a shop file resets its economy. Moderation holds petitions and
account/IP bans. Fields tagged `instance:"skip"` are restored from templates and
are not shadowed; `Room.SpawnInfo` is one of them. Check the struct tag before
assuming stale state is responsible.

Schedules and patrols live under their zone folders and are boot-validated.
Schedules cover all 24 hours. Patrols support strict and yo-yo loops, dwell
rounds, combat interruption, and path retry/fallback. Read
`docs/schemas/schedule.md` or `docs/schemas/patrol.md` before editing them.
Conversation pair overrides must remain role-agnostic: A/B roles are randomized.
Mobs marked `non_combatant: true` cannot be attacked, stolen from, or targeted
by harmful spells; preserve all three enforcement paths.

Sleep is shared actor behavior. Use `actions.Sleep(actor, opts)` and query
`HasBuffFlag(buffs.Sleeping)` instead of adding player/mob-specific paths.
Damage, failed theft, room shouts, arriving light, standing, and scheduled
segment changes can wake sleepers; scheduled wake grace prevents immediate
re-sleep. Preserve the first-round vulnerability of sleeping targets.

## Go and Package Conventions

- Every package under `internal/` and `modules/` needs an accurate `context.md`.
  New packages ship one; API/data/file-list changes update it. Verify every
  documented symbol. Include purpose, files, real core types/API, gotchas,
  dependencies, and consumers; omit generic filler sections.
- `internal/parser` is the shared seam for composition-heavy multi-slot input.
  Use `SplitTrailingContainer`, `SplitLeadingMatch`, or `Resolve*` as appropriate.
  Keep authorization/ownership/visibility gates in commands. Do not add parser
  plumbing where existing fuzzy matching already resolves one multi-word slot.
- Before hardcoding a balance number, inspect `internal/configs/config.balance.go`,
  the subsystem defaulting/validation file, and `_datafiles/config.yaml`.
  Retuning is normally config, not code. Go defaults are not shipped values;
  missing keys fall back to defaults, while explicit zero is valid.

## Combat, Progression, and Resources

- Stats are centered at 100 and progress by use. There is no player hard cap or
  stat compression; `ValueAdj == Value`. Never reintroduce compression in
  `StatInfo.Recalculate()` because resource maxima use the same type.
- `StatProgressionSoftCap` and skill soft cap are chance-curve concepts, not
  ceilings. A progression roll happens every use; `UsesPerRank` converts use
  count to virtual rank and does not set the check cadence. Player
  `IncreaseStat`/`IncreaseSkill` are uncapped; mob caps are explicit mob-only
  guards.
- For every opposed contest, use `combat.RunContest`. Production must not call
  unfloored `contest.Run` / `AgainstDifficulty` or deprecated
  `dice.OpposedRollStat*` directly. For a single non-contested roll,
  `dice.RollStat(mean)` is correct. `RollSpread` is the master proportional
  variance knob.
- Damage uses the five-factor pipeline in `internal/combat/damage_pipeline.go`:
  stat, skill multiplier, item multiplier, channel scale, and global multiplier,
  followed by mitigation and variance. Read live config before quoting values.
  Physical, magical, and conviction channels use their matching mitigation.
  `shoot` uses Perception for hit and damage; melee basics use Strength.
- Prefer multiplicative combat modifiers. Resource depletion uses the shared
  smooth multiplier curve. Defense rolls every available defense and keeps the
  best margin.
- All regeneration is percentage-of-max. Mutations use regen multipliers; heal
  spells store a multiplier; direct heal buffs compute a max-pool fraction.
  Never add flat regeneration/healing without an approved design change.
- Costs cannot reduce a pool below zero. Harm may take health below zero for
  death processing but floors stamina/conviction. Route through the centralized
  cost/harm APIs and preserve killer attribution.

## Items, Spells, and Crafting Gotchas

- Item mitigation fields are `physical_mitigation`, `magical_mitigation`, and
  `conviction_mitigation`; legacy `DamageReduction` is gone.
- Item disambiguation supports `N.item`, `item#N`, and where applicable
  `all.item`. Inventory stacking is display-only; do not infer storage merging.
- Equipment includes paired wrist/ring slots plus mutation-gated extra arms,
  wrists, and tail behavior. Read the item/equipment package contracts before
  altering slot or reservation logic.
- Spell durations use `calcSpellDuration`; shield/heal/DoT apply their documented
  full/half/third duration scaling. Caster weapons use
  `spell_damage_multiplier`, independent from melee `damage_multiplier`.
- Shield strength uses `effect_magnitude`; magical and conviction mitigation
  buffs must flow through their matching mitigation accessors. Hidden mob
  detection is an opposed Perception/Search versus Dexterity/Skullduggery path.
- Mob `archetype` controls physical/mental stat distribution (`fighting`,
  `casting`, or uniform default). Do not infer archetype from equipment alone.
- Potions have aging, bottle multipliers, toxicity, craft scaling, and bandolier
  routing. Salvage always consumes the item and rolls ingredients independently.
  Read the relevant package `context.md`, config, and schema rather than copying
  historical numeric tables from `CLAUDE.md`.

## Backlog Memory Maintenance

- `docs/roadmaps/CURRENT_BACKLOG.md` is the compact cross-roadmap index, not a
  replacement for the canonical trackers.
- Update it when work starts, ships, is parked, or is invalidated. Include the
  source link and last-reviewed date; do not duplicate detailed implementation
  steps.
- Merged code and explicit tracker status outrank plan/spec filenames. Anything
  ambiguous belongs under “Needs triage,” never silently under active work.
- When importing a durable new lesson from `CLAUDE.md` or a completed task,
  add the smallest behavioral rule or routing pointer that prevents recurrence.
  Do not restore large shipped-feature histories to this always-loaded file.
