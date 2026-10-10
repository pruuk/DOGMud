# Loose issues sweep (2026-10-10)

Fourteen small open issues that sit outside the big arcs, fixed in one branch
(`worktree-loose-issues-sweep`). Each is a few lines of code, copy or YAML,
plus a test where the change is behavioural.

Coordinated with the Messaging M6 slice 1 session (branch
`feat/messaging-m6-slice1`). It holds #249 and owns `internal/conditions`,
most spell hooks, `characters/combat.go`, `characters/conditions.go` and
`keywords.yaml`. Nothing below touches those files.

## Facts verified against source (master 47c0d6902)

| Issue | Fact | Where |
|---|---|---|
| #461 | Template reads `$room.SkillTraining`; that field was removed in 0f83dfc96 (2026-07-25) and no longer exists on `Room` | `templates/admincommands/ingame/roominfo.template:13`, `internal/rooms/rooms.go` |
| #461 | `mapslug` is registered and takes a string, so the #455 change is not the cause | `internal/templates/templatesfunctions.go:86`, `internal/mapper/maptag.go:14` |
| #459 | `parties.New` runs before `AimBySight` and every refusal after it | `internal/usercommands/party.go:173-175`, `:185` |
| #289 | `all+` injects every quest and skips the `Secret` filter; `GetQuestProgress` returns a copy, so no save is mutated | `internal/usercommands/quests.go:34,45-51,60`, `internal/characters/quests.go:74-85` |
| #269 | Floor branch treats any first word that is a prefix of `gold` as the gold pile, whatever follows | `internal/usercommands/get.go:575-576` |
| #271 | `LoadDataFiles` loads into temporaries and panics on error before swapping, so old data survives; `reload items` prints success unconditionally | `internal/items/itemspec.go:861-895`, `internal/usercommands/admin.reload.go:41-43` |
| #267 | `portal loot` with no qualifying room returns `fmt.Errorf`, which `world.go` logs as a warning | `internal/mobcommands/portal.go:47-52`, `world.go:1106-1108` |
| #302 | Unhandled mob command emits `emote looks a little confused (%s %s).` to the room | `world.go:1112-1116` |
| #284 | Default `{AutoTap, 15}` is set only in `characters.New`; a save without the key loads `{AutoTap, 0}`; parser only accepts thresholds 1-100, so 0 is never a chosen value | `internal/characters/character.go:432`, `submission_policy.go:76-95` |
| #296 | `help <name>` only tries `help/<name>`; 50 admin help templates live under `admincommands/help/` | `internal/usercommands/help.go:193` |
| #307 | `help attack` says switching "Immediately retargets" | `templates/help/attack.template:83-84` |
| #361 | Torch, Hooded Lantern, Umbral Lantern have no `rarity_tier` | `items/armor-20000/light/20096..20098` |
| #357 | `TriggerCombatInterrupt` has no Go reference outside its declaration | `internal/state/activity/transitions.go:36`, `context.md:151` |
| #306 | Comment still says there is no hot-reload | `internal/behaviortree/engine.go:82-94` |
| #305 | Comment says "effectively unkillable" | `mobs/newcomer_antechamber/9614-straw_effigy.yaml:27-36` |

## Fixes

**#461 Admin room info.** Delete the `Training:` line from both
`roominfo.template` copies (`dogmud` and `default`). Add a test that renders
the template against a loaded room and fails on `[TEMPLATE ERROR]`, so a
future field removal is caught.

**#459 Refused invite creates a party.** Move `parties.New` below the target
checks (sight, resolve, is-a-player, not-already-in-a-party), so a party is
created only when an invite is actually sent. The leader check stays first for
a player already in a party. Test: refused invite leaves `parties.Get` nil.

**#289 `quests all+`.** Gate `all+` behind the admin role (owner call).
Players typing it get the normal `all` view.

**#269 Gold-prefixed items.** Take the gold-pile branch only when the whole
argument is the gold word or its prefix (`get gold`, `get gol`). `get gold
wire` and `get all` then reach item matching. Test with a Gold Wire on the
floor plus a gold pile.

**#271 `reload items` on bad YAML.** Split `items.LoadDataFiles` into
`LoadDataFilesE() error` and a boot wrapper that keeps today's panic. The
reload command calls the `E` form and tells the admin which file failed and
that the previous items are still live.

**#267 Loot goblin portal log.** Return `true, nil` when no loot room
qualifies; it already portals home and drops. Log at debug level.

**#302 Confused emote.** Stop emoting to the room; log the unhandled command
once at warn level with the mob id (owner call).

**#284 Surrender policy zero value.** In `Character.Validate`, map
`{AutoTap, 0}` to the same `{AutoTap, 15}` default `characters.New` uses,
sharing one constant. Test: a save without the key reads "auto-tap-below 15".

**#296 Admin help.** When `help/<name>` misses and the user has permission
for that admin command, render `admincommands/help/command.<name>`.
Non-admins still get "no help found". Test both roles.

**#270 Arm and wrist labels (owner call: "arm 3-6", wrists to match).**
Today the same slot is named three ways. The strings are also lookup keys
(`IsBlockedBy2H "extra arm 2"` in the inventory templates, the
`"extra wrist 1"` cases in `worn.go:225-236`, the sources in
`inventory.go:521-536`), so the keys stay as they are and only what a player
reads changes. One display label per slot, taken from `Worn.AllSlots`
(`worn.go:57-63`): extra arms read "arm 3" to "arm 6"; wrists read "wrist 1"
to "wrist 6", which renames the two natural wrists from "Wrist" to "Wrist 1"
and "Wrist 2" in `inventory.template` and `inventory-look.template`. Equip,
remove and inventory messages look the label up instead of printing the key.
Test: equip into each extra arm and wrist slot; the message and the eq list
name the same slot.

**#307, #361, #357, #306, #305.** Copy and YAML: attack help describes the
roll and the lost round; add tiers 50/40/30; delete the dead constant and its
`context.md` row; point the behaviour-tree comment at the `Evict*` functions;
make the effigy comment say it is hard, not impossible, to kill.

## Left out

- #249 (DoT "poisoned" tag): held by the M6 slice 1 session.
- #236 (help index): needs `keywords.yaml`, which M6 slice 1 edits; do after
  it merges.
- #217 (corpse selection) and #440 (shuffle-order tests): real but larger;
  better as their own branches.

## Testing

`go test ./internal/usercommands ./internal/items ./internal/characters
./internal/mobcommands ./internal/templates`, the full pre-push gate, then a
local boot to run `room info`, `quests all+` as player and admin, `help
build`, and `reload items` with a broken file.
