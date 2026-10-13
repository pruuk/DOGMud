# Messaging M6 rows 26 and 27: combat lines as one record per blow

Issue #370, ledger rows 26 (tier-bleed orphans) and 27 (no human read of the
984 padded lines). Owner rulings 2026-10-13: both rows in one slice; fixes may
reorder, move a line between tiers, or rewrite a line that has no partner; the
store moves to one record per blow, plus a banner-word test.

## Problem

Each attack draws one index and shows line N of each role's list to the
attacker, the defender and the room. The roles are separate lists that pair
only by position, so a reorder, a pad or a tier move in one list silently
breaks a pairing. The validator checks only that the lists are the same length
per tier; it cannot see whether line N describes the same blow in every role.

A word-overlap scan of master `48469f17d` flags about 235 slots across 19 of
the 20 files (noisy; the true count is lower). Three shapes:

- **Cross-tier orphan** (row 26): bite `critical/together/master` index 1
  shows the defender and room "reads every move and delivers a FLAWLESS BITE!"
  while the attacker reads "Your {stance} approach culminates in a DEVASTATING
  CRITICAL strike!"; the attacker's FLAWLESS BITE line is in expert.
- **Misordered within a tier:** all roles exist in the tier at different
  positions.
- **No partner:** expert already holds a complete FLAWLESS BITE triad, so the
  master attacker line "DEVASTATING CRITICAL" has no defender or room line at
  all; only a rewrite fixes it.

## Facts verified against source (master `48469f17d`)

| Fact | Where |
|---|---|
| `TogetherMessages{ToAttacker, ToDefender, ToRoom SkillTieredMessages}` with yaml keys `actor`, `actee`, `observer`; `SeparateMessages` adds `ToAttackerRoom` (`observer`) and `ToDefenderRoom` (`remote_observer`) | `internal/items/attack_messages.go:44-61` |
| `SkillTieredMessages{Beginner, Expert, Master MessageOptions}`; `PoolFor(skillLevel)` unions beginner, plus expert at 34, plus master at 67 | `internal/items/attack_messages.go:63-125` |
| `TogetherMessages.Render` / `SeparateMessages.Render` hand per-role pools to `narration.Render`, which picks one index for all roles | `internal/items/attack_messages.go:147-190`; `internal/narration/render.go:18-23` |
| `Validate` checks every intensity is present and per-tier role pools are equal and non-empty | `internal/items/attack_messages.go:213-276` |
| Loader: `fileloader.LoadAllFlatFiles[ItemSubType, *WeaponAttackMessageGroup](dataPath + "/combat-messages")`; fileloader decodes with `gopkg.in/yaml.v2` | `internal/items/itemspec.go:904`; `internal/fileloader/fileloader.go:18` |
| A yaml.v2 custom decoder already exists in the codebase, signature `UnmarshalYAML(unmarshal func(interface{}) error) error` | `internal/conditions/caster.go:39`, `internal/conditions/effects.go:112` |
| 20 files in `_datafiles/world/dogmud/combat-messages/`, 2,280 attacker slots; every `together` block has `actor`, `actee`, `observer` (161 blocks); every `separate` block has all four roles (17 blocks) | count over the store |
| The default world's 8 combat-message files still use the pre-M4b keys (`toattacker`, `todefender`, `toroom`) and the default world does not boot (no `recipes/`) | `_datafiles/world/default/combat-messages/`; `docs/README.md` row 7 plan entry |
| Go code builds `SkillTieredMessages` / `TogetherMessages` / `SeparateMessages` literals in tests | `internal/hooks/combat_wait_round_sight_test.go:51-54`, `internal/combat/wait_remote_room_hiding_test.go:46-48`, `internal/combat/coup_de_grace_test.go:52,60` |
| Tests that decode store YAML into `items.WeaponAttackMessageGroup` (keep working through the custom decoder) | `internal/combat/crit_banner_wrap_test.go:23`, `internal/items/attack_messages_vertical_test.go:20`, `shipped_narration_data_guard_test.go:119`, `internal/narration/snapshot_test.go:325-334` |
| A test that decodes `generic.yaml` into its own role-first struct (`Together map[string]map[string][]string`) and must change | `internal/combat/coup_de_grace_test.go:82-88` |
| Line-scanning guards over the store text (role-agnostic, unaffected by layout if each role line stays on its own line) | `natural_weapon_agreement_test.go:143-147`, `consistent_attack_messages_guard_test.go:144-148`, `prose_dash_guard_test.go:21`, `combat_idiom_guard_test.go:26` |
| Read-only tools that parse the role-first layout | `tools/combat_message_pool_audit.py`, `tools/combat_message_pad_check.py` |
| Golden: `internal/narration/testdata/stores/combat_messages.golden` | snapshot test above |
| Banner words: 1,047 `cyan-bold` and 692 `red-bold` tags in the store; 575 slots carry a banner in some role | count over the store |

## Design

### 1. One record per blow

The authored shape moves from role, then tier, then list, to tier, then list
of records:

```yaml
critical:
  together:
    master:
    - actor: 'Reading every move, you deliver a <ansi fg="cyan-bold">FLAWLESS BITE</ansi> ...'
      actee: '<ansi fg="{actortype}">{actor}</ansi> reads your every move and ...'
      observer: '<ansi fg="{actortype}">{actor}</ansi> reads every move and ...'
```

- `TogetherMessages` and `SeparateMessages` gain a yaml.v2 `UnmarshalYAML`
  that reads `map[tier][]record` and fills the existing per-role
  `SkillTieredMessages` fields in record order. The in-memory types, `PoolFor`,
  `Render` and every Go literal in tests stay as they are; only the authored
  layout changes.
- A record must carry exactly the roles of its split (together: `actor`,
  `actee`, `observer`; separate: those plus `remote_observer`), each a
  non-empty string. A missing role, an empty line or an unknown key (a typo
  such as `acter`) is a load error naming file, intensity, split, tier and
  record index.
- **Transition:** until the last batch lands the decoder also accepts the
  current role-first layout, told apart by its top-level keys (role names
  versus tier names). The final PR deletes the role-first path, so no file can
  return to parallel lists.
- The default world stays untouched: it already uses keys this decoder never
  read and does not boot. Noted in the final PR.

### 2. Banner-word test

A root guard, `combat_banner_agreement_guard_test.go`: for every record, the
bold banner words (`<ansi fg="*-bold">...</ansi>`) must agree across roles.
Agreement is word by word in order; two words agree when the shorter (at least
three letters) is a prefix of the longer or both share their first five
letters, so SHRED/SHREDS, MAUL/MAULED, EVISCERATE/EVISCERATION agree and
DEVASTATING CRITICAL against FLAWLESS BITE does not. If any role has a banner,
every role must. No allowlist: a slot that fails gets its banners aligned. On
master this rule flags 88 of 575 banner slots in all 20 files (the
DEVASTATING CRITICAL orphans, a room line missing its FUMBLE banner, CRUSHES
against DEVASTATED, DEVASTATING CHOP against DEVASTATING CLEAVE); the audit
fixes them. It ships with the decoder and checks converted files; the
final PR makes it check every file.

### 3. Audit, review, apply, per batch

- **Audit (read only):** one Opus subagent per file reads every (verb, split,
  tier) group and returns structured proposals: slot, kind (reorder, move,
  rewrite, banner-align), exact before and after text. The word-overlap scan
  and the banner guard's failures are hints, not the authority.
- **Review page (row 27):** one Artifact page per batch showing every record
  of every file, changed records flagged with before and after, rewrites
  highlighted. The owner approves or comments per file; skimming the unflagged
  records is row 27's human read.
- **Apply:** a subagent writes each approved file in record form with the
  Write tool (no conversion script, no Python file writes). Moves and reorders
  keep text byte-identical; rewrites follow the copy rules the guards enforce
  (no dashes, under 80 visible characters, known tokens, "their" never "its",
  idioms).
- **Checker:** `tools/combat_message_pad_check.py` learns the record layout
  and a mode that ignores tier and index: per (file, verb, split, role), the
  multiset of lines after must equal the multiset before, except lines listed
  in a per-batch rewrite manifest (JSON: file, verb, split, role, old, new).
  `tools/combat_message_pool_audit.py` learns the record layout too (in
  records, unequal pools cannot occur, so it reports layout only).

### 4. Batches

Five PRs, about four files each, ledger-named files first:

1. decoder, banner guard, checker; `bite`, `claws`, `gore`, `maul`
2. `slashing`, `bludgeoning`, `stabbing`, `slam`
3. `throttle`, `cleaving`, `whipping`, `pounce`
4. `sting`, `drain`, `staff`, `sceptre`, `wand`
5. `generic`, `shooting`, `unarmed`; role-first path deleted; banner guard
   over every file; ledger rows 26 and 27 closed

Each batch: audit, page, owner approval, apply, checker, regenerate the golden,
full gates, merge.

## Corners

- Moving a line between tiers changes which skill level sees it (allowed,
  owner 2026-10-12).
- Tier sizes may change when lines move; the decoder makes per-tier role
  equality automatic, so `Validate`'s equality check becomes unreachable for
  files in record form and stays for the transition.
- `coup_de_grace_test.go` decodes its own struct; it changes to read records.
- The golden is keyed by authored role names and index, so reorders and moves
  show as row changes; each batch's golden diff is read against its manifest.

## Out of scope

- New copy beyond partner lines and banner alignment; tier sizes as a goal;
  the selection code.
- Defence messages and the special-moves store share the parallel-list shape;
  a follow-up issue proposes the same record form for them.

## Testing

- `items`: record layout decodes to the same pools as the role-first layout
  (one file converted both ways in test data); missing role, empty line and
  unknown key each fail with the slot named; separate requires
  `remote_observer`; after the final PR, a role-first file fails to load.
- Banner guard proven able to fail (a mismatched record in a temp copy) and
  able to pass inflections.
- Checker probed in both directions: an unlisted edit fails, a listed rewrite
  and a pure move pass.
- Each batch: full `go test ./...`, `go vet`, golangci-lint, isolated boot.
- Playtest after batch 1: a few swings at expert and master skill against a
  mob, attacker and room seats read the same blow every round.
