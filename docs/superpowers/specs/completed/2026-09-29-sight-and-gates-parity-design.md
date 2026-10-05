# Sight and gates parity: mobs get, look, remove, equip, craft, speak and emote by the player's rules

Date: 2026-09-29. Player/mob parity slice 5, split into **5a object and
action gates** (get, look, remove, equip over cursed gear, craft) and **5b
speech and emotes in the dark** (say, shout, rally, warcry, emote). Source:
the owner-ordered parity audit (2026-09-28) and the owner's decisions of
2026-09-29. Each sub-slice ships as its own PR, 5a first.

## Facts verified against source (2026-09-29, master `7d6d4ac38`)

Every row below was read at `7d6d4ac38` in a fresh worktree. The Equip (E)
and Emote (M) rows and the disarm row (D1) were added later the same day at the
spec branch's `c9c2e0ec7`, whose only changes since `7d6d4ac38` are this
spec and its README row, so the code read is the same. Rows E11 to E13 were
read at the spec branch's `2855e0222`, and rows E14 to E17 at `f11a3ab07`,
both likewise docs-only since `7d6d4ac38`. Negative rows ("no
callers", "zero") name the search and a positive hit that proves the same
search could match.

### Get

| # | Fact | Where |
|---|---|---|
| G1 | `usercommands.Get` refuses at `messaging.ParticipantSight(user.Character, room) == messaging.SightNone` ("You can't see anything to pick up!") as its first statement, so the refusal covers every branch: floor, gold, stash, containers, corpses, component bag and bandolier | `internal/usercommands/get.go:94-97` |
| G2 | Single get peeks `room.FindOnFloor(rest, getFromStash)` and refuses an `exploding` item ("You can't pick that up, it's about to explode!") BEFORE calling `actions.GetItemFromFloor`, so the exploding check runs before the household-bauble check | `get.go:635-640,642` |
| G3 | The filtered sweep `getAllMatchingFromFloor` (for `get all <name>` and `get all.<name>`) does NOT call `GetItemFromFloor`: it loops `takeableOnFloor`, stops silently on an `exploding` item, and moves each item with `StoreItem` plus its own `ItemOwnership` event | `get.go:30-87` (exploding at `:52`); callers `:222,:251` |
| G4 | Unfiltered `get all` recurses `Get(item.Name(), ...)` per item, so each item meets G2 | `get.go:226-242` |
| G5 | `actions.GetItemFromFloor(actor Actor, itemName string, stash bool) GetItemResult` gates only the household bauble (`ErrHouseholdBauble`), then `TransferItemToBackpack`, which fires the same `ItemOwnership` event G3 fires by hand. No sight gate, no exploding gate | `internal/actions/get.go:27-53`; `internal/actions/transfer.go:44-64` |
| G6 | `actions.GetGoldFromFloor(actor, amount)` has no sight gate; the player reaches it only past G1 | `actions/get.go:57-59`; `usercommands/get.go:610` |
| G7 | `mobcommands.Get` supports `all`, `gold`, floor and stash, with no sight and no exploding gate | `internal/mobcommands/get.go:16-95` |
| G8 | The only Go issuer of a mob `get` is the AI companion (`get gold`, `get <item ref>`). Search `(Command\|issue)\(...get` over `internal/` and `modules/` also hit `actions.go:373`; mob YAML `idlecommands` carry no `- get` (the same pattern finds 415 `- say` lines) | `modules/aicompanion/actions.go:359-375` |
| G9 | The companion lists floor items and gold only when `!sc.Dark`, and `sc.Dark = cannotSee(mob, room)`, which is `sightOf != SightFull`. So it never issues a `get` for a floor item it does not see clearly | `modules/aicompanion/scene.go:134,141-168`; `perception.go:304-316` |

### Look

| # | Fact | Where |
|---|---|---|
| L1 | `usercommands.Look` refuses at `SightNone` ("You can't see anything!") before anything else and keeps the verdict in `sight` | `internal/usercommands/look.go:33-37` |
| L2 | It resolves a creature with `ResolveTargetOptions{Viewer: user.Character}` and only acts on it at `sight == SightFull`; a pet likewise needs `SightFull` (`:446`); at `SightShapes` an unmatched name ends in "You can only make out shapes here." (`:576-579`) | `look.go:88-89,446,576` |
| L3 | Looking through an exit refuses when `!messaging.SeesThroughExit(user.Character, room)` ("It's too dark to see anything in that direction."), then refuses a locked exit | `look.go:285-294` |
| L4 | Player resolution order: sight gate, `events.Looking`, no target, creature, sealed crate, container, exit, direction alias, carried-item noun, carried item, room noun, pet, shapes hint | `look.go:33-579` |
| L5 | `mobcommands.Look` has no sight gate at all. Order: no target, exit (locked refuses silently; no through-sight check), backpack, creature, body, room noun, pet | `internal/mobcommands/look.go:14-170` |
| L6 | It resolves the creature with a bare `actions.ResolveTargetActor(room, lookAt)` (no viewer), so it resolves a hidden player, tells them "X is looking at you." and tells the room "X is looking at <hidden player>." through plain `SendTextVisual` | `mobcommands/look.go:92,95-107` |
| L7 | `ResolveTargetOptions.Viewer`: "a creature it does not perceive ... cannot be named". Mob callers that already pass their own character: `FindAttackTarget(..., viewer)` and `castViewer(actor)`, which returns the mob's character since slice F | `internal/actions/target_resolution.go:23-27,79-90`; `combat_attack.go:32,119`; `cast_sight.go:11-13` |
| L8 | The repo-root lookup guard registers `internal/mobcommands/look.go\|Look` as `{plain: 1, why: whyMob}` and `internal/usercommands/look.go\|Look` as `{viewer: 1}`; it fails on a stale entry | `lookup_viewer_guard_test.go:49,71,233-236` |
| L9 | No production caller issues a mob `look`. Searched `Command\(...look` in `internal/`, `modules/` and `_datafiles/**/*.js` (the same search finds `mob.Command(\`lookforaid\`)` at `hooks/MobIdle_HandleIdleMobs.go:276`), `- look` in mob and schedule YAML (0; the pattern finds 415 `- say`), and behaviour `cmds:` (0). The companion's `look_at` emits its own emote and never issues `look` | grep; `modules/aicompanion/actions.go:353-354,625-655` |

### Remove

| # | Fact | Where |
|---|---|---|
| R1 | `usercommands.Remove` first calls `refuseWhileBusy(user, "change equipment")`, which refuses on `Character.IsActing()` | `internal/usercommands/remove.go:17-19`; `busy_refuse.go:18-25` |
| R2 | Single remove refuses a cursed item when `matchItem.IsCursed() && user.Character.Health > 0` and `GetSkillLevel(skills.Spellcasting) < 4`; at 4 or above it prints "It's CURSED but luckily your enchant skill level allows you to remove it." and proceeds | `remove.go:63-77` |
| R3 | `remove all` loops `Equipment.GetAllItems()` into `actions.RemoveEquipment` with no cursed check, so it strips cursed gear. It prints no per-item line | `remove.go:30-54` |
| R4 | `actions.RemoveEquipment(actor Actor, itemName string) RemoveEquipResult`; its doc says "Cursed-item checks, messaging, and all-remove loops remain in the callers". It already fires one `EquipmentChange` per item | `internal/actions/remove_equip.go:94-131` |
| R5 | `mobcommands.Remove` refuses under `PermaGear` (an emote), then runs the same `all` loop or a single remove. No busy gate, no cursed gate | `internal/mobcommands/remove.go:15-50` |
| R6 | Both `remove all` wrappers queue an aggregate `EquipmentChange` on top of R4's per-item events. The only readers of `ItemsRemoved` are the GMCP refresh and the light recompute, so the duplicate is harmless | `usercommands/remove.go:39-42`; `mobcommands/remove.go:33-36`; `modules/gmcp/gmcp.Char.go:249`; `hooks/Awareness_LightChange.go:115` |
| R7 | `RemoveEquipment` callers: the two wrappers only. The companion issues `remove <worn ref>` | grep `RemoveEquipment(`; `modules/aicompanion/actions.go:408-412` |
| R8 | The companion judges a remove by whether its worn count changed; a refused remove records "You tried to change your gear (X), but nothing changed." | `modules/aicompanion/actions.go:791-799` |

### Equip over cursed gear

| # | Fact | Where |
|---|---|---|
| E1 | `Character.Wear(i items.Item) (returnItems []items.Item, newItemWorn bool, failureReason string)`: type gate, MinStrength, hands, then a reservation snapshot (`savedEquipment := c.Equipment`, `:628`), placement by `wearWeaponOrShield` or `wearArmorSlot` (`:632-636`), and a post-placement reservation check that restores `savedEquipment` and refuses (`:642-650`) | `internal/characters/worn.go:586-672` |
| E2 | `wearWeaponOrShield` refuses a cursed displacement with "Your X is cursed and prevents you from removing it." in three places: the 2H pair (`:385-390`), a shield displacing arm 2 (`:415-417`), a 1H weapon displacing `Weapon` (`:454-456`). The 1H path also displaces arm 2 when `Weapon` is 2H (`:457-460`) without testing it for a curse. No Health and no Spellcasting condition | `worn.go:373-464` |
| E3 | `wearArmorSlot` displaces the slot's occupant for every armour type (both rings full: `Ring`; all wrists full: `Wrist1`) with no cursed check | `worn.go:472-584` |
| E4 | Player `equip` has two paths. The ordinary one calls `actions.EquipItem` (`:276-277`) with no cursed pre-check, so a player today puts a helmet on over a cursed helmet. The arm-slot branch (`equip X armN`, `:122-273`) places directly and never calls `Wear`; its four checks (`:172-196`, "Your X is cursed and can't be removed!") are the only cursed checks in the file, and it also skips `Wear`'s MinStrength and reservation gates | `internal/usercommands/equip.go` |
| E5 | No equip path honours the Spellcasting-4 exception: `Spellcasting` has zero hits in `usercommands/equip.go`, `characters/worn.go` and `actions/remove_equip.go` (the same search hits `usercommands/remove.go:66`). `IsCursed` is `spec.Cursed && !Uncursed` | grep; `internal/items/items.go:398-400` |
| E6 | `actions.EquipItem(actor, itemName) EquipItemResult` calls `Wear` at `:44` and returns its `FailureReason`. Callers: `usercommands/equip.go:277`, `mobcommands/equip.go:65`, `hooks/mob_equip_best_floor_item.go:60` | `internal/actions/remove_equip.go:25-83`; grep `EquipItem(` |
| E7 | Direct `Wear` callers: `actions/sell.go:404` (merchant gear upgrade; a refusal speaks "considers the X, then shelves it instead."), `bountyhunter/bountyhunter.go:91` and `rooms/rooms.go:1001` (affixed loot-pool gear, refusal logged by `mudlog.Warn`), `modules/aicompanion/cooking.go:159` (`equipStartingKit`) | grep `Wear(` |
| E8 | Mob `equip` speaks "X turns the Y over, then sets it aside." on any `FailureReason`. Mob `gearup` issues `wear !<id>` only when `itemvalue.IsUpgrade`; a player's `give` to a mob issues `gearup !<id>` once. `IsUpgrade` is `ItemValueDelta(...).Score > 0`, which already models the displaced items per slot (`displacedItemsForSlot`) and ignores curses (`cursed` in `internal/itemvalue` hits a test only). The floor-equip hook keeps a refused item in the pack, off the floor | `mobcommands/equip.go:67-76`; `mobcommands/gearup.go:34,41,79,89`; `usercommands/give.go:281`; `internal/itemvalue/score.go:47-49`; `delta.go:231-259,287-322`; `hooks/mob_equip_best_floor_item.go:55-65` |
| E9 | The companion's `equip` verb issues `equip <ref>` and is judged, like remove, by its worn count (R8). Autonomy offers `gearup` at most once an hour (`lastGearUp`), and `noveltyBonus` scores a thing -1 after two failures in a day | `modules/aicompanion/actions.go:402-406,791-799`; `autonomy.go:314-316,383-389`; `scene.go:341-351` |
| E10 | Spawn: `rooms.go:1001` wears loot-pool items on a mob fresh from `NewMobById` (`:954`), which already wears its template's gear (62 dogmud mob files carry `equipment:`), so the body is not empty and the loot can displace. But the live world (`DataFiles: _datafiles/world/dogmud`) has zero cursed item specs (`cursed: *true` finds 0 files there and 6 in `world/default`), affix generation adds no curse (`Cursed\|Uncurse` in `internal/items` hits only the spec field, `Uncursed`, `Uncurse`, `IsCursed` and the list flag at `items.go:482`), so no spawn-time `Wear` can meet a cursed slot with shipped content. The builder item editor can set `Cursed` | grep; `_datafiles/config.yaml:240`; `modules/gmcp/gmcp.Item.go:241` |
| E11 | Ring and wrist choice today. Ring: refuse when both `Ring` and `Ring2` are disabled; else fill `Ring` if enabled and empty, else `Ring2` if enabled and empty, else displace `Ring` unconditionally (even a disabled `Ring` when only `Ring2` is enabled). Wrist: the same over `Wrist1`, `Wrist2`, then `ExtraWrist1` to `ExtraWrist4` each gated by `c.ExtraArms >= n`, else displace `Wrist1` unconditionally. `ExtraArms` is the extra-arms mutation level capped at 4, the level `itemvalue`'s `extraArmsLevel` also reads. Disabled is `ItemId < 0` | `worn.go:504-535`; `characters/validate.go:516-521`; `itemvalue/delta.go:40-46`; `items/items.go:205-207` |
| E12 | `itemvalue` does not share that choice. `compatibleSlotsFor` offers every ring slot (`Ring`, `Ring2`) and every wrist slot, `displacedItemsForSlot` reports each slot's own occupant, and `ItemValueDelta` keeps the best-scoring slot (rank breaks ties). So with both rings full it weighs the swap against the WEAKER ring, while `Wear` displaces `Ring`: the scorer can call an upgrade a swap that removes the better ring. Callers of the score: `IsUpgrade` (mob `gearup` `:34,:79`, `mobs/crafter.go:445`), the floor-equip hook `:45`, `planners/shop_upgrade.go:58` | `itemvalue/delta.go:57-112,231-259,287-340`; `score.go:47-49`; grep `ItemValueDelta(\|IsUpgrade(` |
| E13 | The arm-slot branch, `equip X armN` (`equip.go:122-273`). Parsing (`:65-86`) takes a last word `arm#N`, `N.arm` or legacy `armN`, N in 1 to 6, and strips it. Shape refusals, in order: not a weapon or offhand ("You can only wield weapons or shields in arm slots."), shield in arm 1 ("You can't put a shield in your primary weapon hand (arm 1)."), arm missing from `GetHandPairs` or the absent second slot of a half pair ("You don't have arm %d."), a 2H weapon in an even arm ("A two-handed weapon needs a pair of arms. Try arm 1, 3, or 5.") or in a half pair ("That arm doesn't have a partner for a two-handed weapon."). Then the four cursed checks (`:172-196`), displacement of a 2H partner in arm 1 when targeting arm 2 (`:191-199`), of the target slot (`:201-205`) and of the second slot for a 2H (`:206-209`); `CancelConditionsWithFlag(Hidden)`, `RemoveItem`, placement (`:212-214`). Per displaced item it prints "You remove your X and return it to your backpack." plus the room line and calls `StoreItem(old)` ignoring its result, so a full pack loses the item (`:217-230`); then "You equip your X in your <label>." (shield) or "You wield your X in your <label>." with room line "X equips their Y." (`:232-246`), `Validate(true)`, the reservation disclosure, `EquipmentChange`, onStart triggers and the quest notify (`:247-271`). It never calls `Wear` or `EquipItem`: no type-level MinStrength, no hands-over-2, no reservation snapshot | `internal/usercommands/equip.go:65-273` |
| E14 | Arm count. `validateMutationSlots` sets `ExtraArms` to `Mutations["extra-arms"]` capped at 4, so a character has 2 + `ExtraArms` arms, at most 6 (the field comment "(0-2)" is stale). `anatomy.go` holds only `HasBodyPart` (species body-part tags), no count. The mutation declares `max_rank: 1`, which natural deepening honours through `effectiveMax`, but Bloom's `BloomAdvanceMutation` deepens against the global `MutationMaxLevel`, shipped at 4, so a Bloom drinker reaches 4 to 6 arms. Species intrinsics clamp at `MutationMaxRank` 4, and no dogmud species grants extra arms (`extra-arms` under `species/`: 0; `intrinsic_mutations` there hits 10+ files) | `characters/validate.go:514-521`; `character.go:310`; `anatomy.go:9-20`; `mutations/extra-arms.yaml:5`; `mutations/mutations.go:364-392`; `bloom_mutation.go:35-60`; `actions/drink.go:420`; `_datafiles/config.yaml:1807`; `intrinsic.go:11,28-33` |
| E15 | Hand slots. `GetHandPairs`: pair 0 `Weapon`/`Offhand`; pair 1 `ExtraArm1`, plus `ExtraArm2` at `ExtraArms >= 2`; pair 2 `ExtraArm3`, plus `ExtraArm4` at 4. An odd count leaves a half pair (`Second.ItemPtr` nil), which takes no 2H. Arm N is pair `(N-1)/2`, slot `(N-1)%2`, so arms 1 to 6 are `Weapon`, `Offhand`, `ExtraArm1` to `ExtraArm4`. A 2H sits in a pair's First and clears Second; `Is2H` reads `HandsRequired`, which is 1 for a Large species and `Hands+1` for a Small one. `IsEmpty` counts a nil or disabled slot as empty. `ExtraArm` n and `ExtraWrist` n are enabled and disabled together, so extra wrist n exists exactly when arm n+2 does; no wrist is otherwise tied to what an arm holds | `hand_slots.go:21-71`; `usercommands/equip.go:134-136`; `worn.go:53-57,278-302`; `validate.go:524-565` |
| E16 | Weapon and offhand choice today (`wearWeaponOrShield`). 2H (`:377-403`): `FindFirstFreePair`, else `FindCheapestPairToDisplace` (fewest occupants over full pairs, the earlier on a tie); refuses if THAT pair holds a cursed item (First checked first) and tries no other pair. Shield (`:405-422`): `FindFirstEmptySlot(pairs, true)` (`Offhand` if empty and `Weapon` is not 2H, then arms 3 to 6, skipping a pair whose First is 2H); else "Your two-handed weapon leaves no room for a shield." when `Weapon` is 2H; else displaces `Offhand`, refusing if cursed. 1H (`:424-464`): the first empty slot in arm order, skipping 2H pairs, where `Offhand` counts only with `CanDualWield()` (WeaponCombat > 0) or claws over claws in `Weapon`; else displaces `Weapon` (and a stray occupant behind a 2H there), refusing if cursed. No path ever displaces an extra arm, and the 1H path never displaces `Offhand` | `worn.go:373-465`; `hand_slots.go:94-149`; `validate.go:305-308` |
| E17 | The scorer's weapon slots. `compatibleSlotsFor` offers a 1H `Weapon` and `Offhand`, a 2H `Weapon` only, an offhand item `Offhand` only; never an `ExtraArm` slot, though `SlotExtraArm1` to `4` exist and `slotOf` maps them. It reads `spec.Hands`, not `HandsRequired`, and ignores `CanDualWield`. `displacedItemsForSlot` models pair 0 only. So with any extra arm empty `Wear` fills it while the scorer weighs a `Weapon` or `Offhand` swap, and a non-dual-wielder's `Offhand` can win the score though `Wear` never puts a weapon there | `itemvalue/delta.go:24,57-65,179-193,231-259` |

### Craft

| # | Fact | Where |
|---|---|---|
| C1 | `usercommands.Craft` refuses an attempt when `!messaging.CanSeeClearly(user.Character, room)` ("You can't see well enough to work on anything here."), after `craft`/`craft list` and BEFORE the storage pull and the enchanting dispatch | `internal/usercommands/craft.go:83-95,113-120` |
| C2 | Enchanting never reaches `actions.InitiateCraft`; storage pulls and `storageAwareMissingTag` stay in the command layer because storage hangs off the user record | `craft.go:113-124,713-715`; `internal/actions/craft.go:124-130` |
| C3 | `actions.InitiateCraft(actor Actor, recipeName string) CraftResult` has no sight gate; its first gate is `IsCrafting` | `actions/craft.go:131-138` |
| C4 | Callers: the two wrappers only. Mob `craft` is issued by the goal planners (`craft_item.go:70`, `mastery_skill.go:49`) and the companion (`actions.go:314-319`, `autonomy.go:369-375`). The shop crafter restocks shelves directly and never issues `craft` | grep `InitiateCraft(`, `"craft "`; `internal/mobs/crafter.go:250-300` |
| C5 | The companion does NOT skip crafting in the dark: `craftableHere` checks recipe, station and ingredients only; autonomy offers `craft` when `craftableHere` is non-empty (`:301`); the prompt lists the same (`runtime.go:521`) | `modules/aicompanion/cooking.go:58-88`; `autonomy.go:301-303,369-375` |
| C6 | A companion craft that did nothing is recorded as "You set about making X." (the not-ok branch assumes a multi-round craft) | `modules/aicompanion/actions.go:871-877` |
| C7 | `CanSeeClearly` = awake and `ParticipantSight == SightFull`; `ParticipantSight` returns `SightNone` for a blinded observer before reading light | `internal/messaging/predicates.go:56-62,136-138` |

### Speech

| # | Fact | Where |
|---|---|---|
| S1 | `actions.Say(actor, text)` reveals a hidden speaker, echoes "You hear someone talking." to exits (quiet), queues `events.Communication`, and returns `IsSneaking`. The room line is left to callers | `internal/actions/say.go:18-48` |
| S2 | `TransitionToRevealing` passes through `Revealing` to `Visible` in the same call, so after S1 `IsHidden()` is false and every "someone says/shouts" hidden branch is unreachable unless the transition errors | `internal/state/awareness/awareness.go:190-214` |
| S3 | Player say sends `FormatSayText(name, ...)` through `room.SendTextCommunication`, with no darkness handling: a player's name reaches every listener in any light | `internal/usercommands/say.go:32-39` |
| S4 | Player shout reveals, uppercases, drunkifies, escapes, then sends the named line through `SendTextCommunication`; adjacent rooms get `Someone shouts from the <exit> direction, "<WORDS>"` as a communication; it wakes sleepers except the shouter | `internal/usercommands/shout.go:25-85` |
| S5 | Mob say returns early when `room.PlayerCt() < 1` (so no `Communication` event and no exit echo), then calls `actions.Say` and `sendAudioRoomText` | `internal/mobcommands/say.go:15-30` |
| S6 | Mob shout does NOT reveal a hidden mob; it sends through `sendAudioRoomText`; adjacent rooms get `Someone is shouting from the <exit> direction.` with NO words, through plain `SendText`; it wakes sleepers except itself | `internal/mobcommands/shout.go:14-58` |
| S7 | `sendAudioRoomText` is TWO-tier, not three: in a lit room (`room.IsLit()`) it sends the named line to everyone, a blinded listener included; in the dark `SightFull` hears the name and everyone else the anonymous string | `internal/mobcommands/darkness.go:25-52` (lit shortcut `:30-32`) |
| S8 | `sendAudioRoomTextHidingNames` is the three-tier one: per listener, `messaging.HideNames(text, names, ParticipantSight(...))`, delivered with `u.SendText` (audio, no deafen filter) | `darkness.go:66-82` |
| S9 | `HideNames` writes "a figure" at `SightShapes` and "something" (capitalised at sentence start) at `SightNone`, not "someone" | `internal/messaging/hidenames.go:30-35,55-72` |
| S10 | `sendAudioRoomText` callers: `say.go:29`, `shout.go:24`, `rally.go:25`, `warcry.go:25`, `howl.go:47,68`, `taunt.go:54,75,86,99` (10). `sendAudioRoomTextHidingNames` callers: `howl.go:58,79`, `taunt.go:153,189` (4). All in `internal/mobcommands` | grep |
| S11 | Player rally and warcry send their room line through `SendTextVisual` (sight-gated: a listener who cannot see gets nothing); mob rally and warcry use `sendAudioRoomText`. Player taunt is visual too | `internal/usercommands/rally.go:42,73`; `warcry.go:44,77`; `taunt.go:165,192,225`; `mobcommands/rally.go:25`; `warcry.go:25` |
| S12 | `Room.SendTextCommunication` queues one RoomId-keyed `events.Message{IsCommunication: true}` with no category. Its doc: "deafen mutes player chatter only. NPC/merchant speech must NOT use this; it goes through SendText / SendTextVisual unfiltered so moderated players still hear quest content. Audited 2026-07-10." | `internal/rooms/rooms.go:218-236` |
| S13 | The deafen filter is on BOTH branches of the listener: per-user (`:29`) and room (`:83`). `Deafened` is the admin moderation flag ("Cannot HEAR custom communications from anyone but admin/mods"). `IsQuiet` lines pass only to a `SuperHearing` listener. The hook's comment says `IsQuiet` has zero emitters; that is stale, since `SendTextToExits(txt, true)` sets it for say's exit echo. But no dogmud condition grants `superhearing` (the same search finds `world/default/conditions/28-superior_hearing.yaml`), so those lines reach nobody, and speech lines never set `IsQuiet` | `internal/hooks/Message_SendMessages.go:29,44-52,83-92`; `internal/rooms/rooms.go:516-543`; `internal/users/userrecord.go:50`; `usercommands/admin.deafen.go:31` |
| S14 | Mob `say` carries quest dialogue (`npc_say` builds `say <text>`), shopkeeper replies, dialogue trees and the companion's speech | `internal/questengine/bridge.go:415-420,437,439`; `internal/actions/buy.go:169`; `internal/behaviortree/actions_dialogue.go:69,128`; `modules/aicompanion/runtime.go:1115-1123` |
| S15 | `Actor.SendRoomCommunication(msg, excludeSelf)` already encodes a split: `UserActor` sends a communication, `MobActor` sends `SendTextVisual` ("Mobs do not respect client-side mute/deafen settings"). It has no production caller (grep finds only the interface, the two methods and nine test fakes) | `internal/actions/actor.go:27-31`; `actor_user.go:46-52`; `actor_mob.go:45-51` |
| S16 | Mob shout is live: the human species' `angrycommands` hold three `shout` lines. No Go code and no mob YAML `idlecommands` issue `shout` | `_datafiles/world/dogmud/species/1-human.yaml:10-13`; grep |
| S17 | Mob rally and warcry are issued by behaviour archetypes (`leader`, `guard_captain`, `tank_taunter`, `boss_soren`) and the companion's combat moves | `_datafiles/world/dogmud/behaviors/archetypes/*.yaml`; `modules/aicompanion/decision.go:53` |
| S18 | Whisper is a remote, name-addressed tell (`users.GetByCharacterName`) with its own deafen refusal; it is not room-bound, so darkness does not apply. Mob `sayto`/`replyto` send the mob's and the target's names to the room with plain `SendText` and no sight gate, so they leak names in the dark | `internal/usercommands/whisper.go:31-60`; `internal/mobcommands/sayto.go:36-51,65-68,142-158` |

### Emote

| # | Fact | Where |
|---|---|---|
| M1 | Player free-form `emote <text>` sends `FormatEmoteText(name, rest, "username")` through `room.SendTextCommunication`: one RoomId-keyed communication (S12), no sight gate, so every listener reads the name in any light, a blinded one included. A deafened listener is filtered (S13) | `internal/usercommands/emote.go:52-55` |
| M2 | The `@` form (`emote @<text>`) strips the `@` and skips only the actor's own "You Emote:" line; its room line is M1's, same call | `emote.go:45-50` |
| M3 | Empty `emote` ("X emotes.") and alias emotes (`actions.EmoteAliases`) send through plain `SendTextVisual` with `CategoryEmote`, excluding the actor; aliases bypass mute and deafen by design ("pre-written, not free-form communication") | `emote.go:16-33`; `internal/actions/emote.go:18-23` |
| M4 | Mob `emote` returns early when `room.PlayerCt() < 1`, then sends the empty form or `FormatEmoteText(name, text, "mobname")` (alias or free text) through plain `SendTextVisual` with `CategoryMobEmote`: sight-gated, unfiltered by deafen | `internal/mobcommands/emote.go:15-31` |
| M5 | `FormatEmoteText` wraps the name in a `username`/`mobname` tag. Plain `SendTextVisual` therefore already hides that name at shapes: the pipeline's `Anonymize` replaces the tag with "a figure", capitalised at a sentence start since `5c0c91dd0` ("A figure waves."), and a `SightNone` listener gets nothing. So M3 and M4 are three-tier today. What `SendTextVisualHidingNames` adds is `HideNames` over bare, untagged occurrences of the given names in the text | `internal/actions/emote.go:27-35`; `internal/messaging/anonymize.go:21-62`; `pipeline.go:59-68`; `rooms.go:330-369` |
| M6 | Every mob emote reaches `mobcommands.Emote`: 906 `- emote` lines across 173 files under `_datafiles/world/dogmud`, the behaviour-tree `emote` action and dialogue (`actions_dialogue.go:87,148`), quest `npcCommand` (`bridge.go:418`), planners (`helpers.go:245-249`), charm expiry, and the companion's idle, look, thinking and arrow-gathering emotes (`autonomy.go:346`, `actions.go:689`, `runtime.go:1201`, `combat.go:1003`) | grep `emote` over `internal/`, `modules/`, `_datafiles/world` |
| M7 | `sendTextVisualJudgedBy` queues one per-user `events.Message` with no `IsCommunication`; the per-user deafen check (S13, `:29`) would filter such a message if the flag were set. The only other reader of `Message.IsCommunication` is that hook; discord relays only `Broadcast` | `rooms.go:330-369`; `hooks/Message_SendMessages.go:29`; `internal/integrations/discord/listeners.go:100-108`; grep `IsCommunication` |
| M8 | The companion takes in a player's emote from `events.Emote` only when it sees clearly (`cannotSee`) and perceives the emoter, so no change reaches its memory | `modules/aicompanion/listeners.go:191-207` |

### Disarm and forced unequips

| # | Fact | Where |
|---|---|---|
| D1 | Disarm and the offhand break name nobody in the dark. Disarm (`combat/criteffects.go:23-65`, `RemoveFromBody` at `:61`) is delivered by `messaging.SendTrio` from player and mob grapple; the observer line goes through `SendTextVisualHidingNames` with both names, and the actor and actee lines hide the other party by the reader's `ParticipantSight`. The offhand break (`tryWeaponBreak`, `RemoveFromBody` at `hooks/combat_shared_helpers.go:243`) sends its room line through `SendTextVisualHidingNames` | `usercommands/grapple.go:143-150`; `mobcommands/grapple.go:73-82`; `messaging/trio.go:108-132`; `hooks/NewRound_DoCombat_helpers.go:1012-1023,1039-1048` |

### Guards and tests that key on this code

| # | Item | Where |
|---|---|---|
| T1 | Re-fork guard pattern to follow: a regex over the wrapper files | `drink_wrapper_guard_test.go`; `flee_wrapper_guard_test.go` |
| T2 | Lookup guard entries L8 go stale when look resolution moves | `lookup_viewer_guard_test.go:49,71` |
| T3 | `messaging_surface_guard_test.go` matches EXACT room-send method names for the observer viewpoint; a new Room sender must be added | `messaging_surface_guard_test.go:896-897` |
| T4 | `bauble_finder_view_guard_test.go` lists beyond-reader senders by name, `SendRoomCommunication` among them | `bauble_finder_view_guard_test.go:95-98` |
| T5 | `m2_routing_guard_test.go` still recognises `sendAudioRoomText` calls although `m2RoutingFiles` is empty; `send_trio_only_guard_test.go` names it in comments | `m2_routing_guard_test.go:70,261`; `send_trio_only_guard_test.go:20,45` |
| T6 | Say and shout tests assert only `handled`/`err`: `usercommands_test.go:610,617,663-680,1271,7179`; `mobcommands_test.go:385,391,408-414,1219`. `FormatSayText` text is pinned by `internal/actions/actions_test.go:67-126`. `internal/mobcommands/audio_room_text_sight_test.go` pins `sendAudioRoomText` | grep |
| T7 | 5a tests that touch the moved gates: `darkness_gates_sight_test.go` (calls `Get("", ...)` and `Look` in the dark), `look_exit_visibility_test.go`, `shapes_roster_test.go`, `household_bauble_test.go:96` (the sweep), `actions/get_household_test.go`, `actions/economy_test.go:279-340`, `usercommands_test.go:1097` (`TestRemove`), `mobcommands_test.go:430,604` (`TestLook`, `TestRemoveMob`), `remove_reservation_disclosure_test.go` | grep |

## Owner decisions (binding, 2026-09-29)

1. **One spec, two PRs.** 5a object and action gates first, then 5b speech and emotes.
2. **Approach A.** Every rule moves into the shared body both actors already
   call; the command wrappers keep only their wording. A repo-root re-fork
   guard pins it. No per-wrapper copies, no rule-table mechanism.
3. **Dark speech is three-tier for player and mob speakers.** Clear sight
   hears the name, shapes hears "A figure", no sight hears "Someone". The
   words are always heard.
4. **Mob remove gets the busy gate** (the player's `refuseWhileBusy`).
5. **Riders:** shout reveals a hidden mob speaker as it does a player; the
   Spellcasting-4 cursed-removal exception applies to mobs; the mob-only
   `PermaGear` refusal stays.
6. **NPC speech stays unfiltered by deafen** (owner, 2026-09-29, reversing the
   first draft's rider). Deafen is the upstream child-safety tool: it shields a
   player from other players' free text, and NPC lines are authored content,
   so a deafened player keeps hearing quest givers, merchants, dialogue and
   their companion (S12, S14). Mob speech takes the three-tier names through
   `SendTextHidingNames`; player speech keeps the deafen filter through
   `SendCommunicationHidingNames`. That one call is the only difference.
7. **Emotes join 5b where they leak names** (owner, 2026-09-29). An emote is
   seen, not heard, so the player's free-form line (M1, M2) follows the visual
   rule (clear sight: the name; shapes: "a figure"/"A figure"; no sight:
   nothing) and keeps the deafen filter, because it is player free text. Mob
   emotes follow the same visual rule unfiltered, as ruling 6 does for speech.
8. **Equipping over cursed gear joins 5a for mobs** (owner, 2026-09-29). The
   refusal moves into `Character.Wear` for every slot, armour included, with
   the existing `worn.go` wording, so every mob path (E6, E7) inherits it and
   the player wrappers keep only wording. The Spellcasting-4 exception
   follows player equip exactly as it stands (E5: not honoured).
9. **Disarm and forced unequips stay out** (owner, 2026-09-29), because they
   leak no names (D1).
10. **Ring and wrist slot choice skips a cursed slot** (owner, 2026-09-29).
    An empty slot is still filled first, in today's order (E11). When every
    slot is full, `Wear` swaps the first uncursed one, and refuses with the
    shared cursed wording only when all are cursed. `Wear` and the upgrade
    scorer make that choice through one shared helper, not two copies.
11. **`equip X armN` goes through the shared equip body** (owner,
    2026-09-29). The arm-slot branch gains `Wear`'s MinStrength and
    reservation gates and meets the cursed check once, in `Wear`; its own
    cursed checks are deleted. The player's arm wording stays where it still
    applies.
12. **Weapons and offhands skip a cursed slot too, across every arm**
    (owner, 2026-09-29). One-handed weapons and shields follow ruling 10's
    rule, and a two-handed weapon skips any pair holding a cursed item. The
    rule is written over the arms and wrists the character actually has (up
    to 6 arms, E14), never a hardcoded two. `Wear`, `WearInArm` and the
    scorer make the choice through the one helper, so they agree for any arm
    count. An explicitly named arm whose slot is cursed refuses; it never
    silently picks another arm.
13. **A shield next to a two-hander takes the last available hand** (owner,
    2026-09-29). With a 2H in the main hands and every other hand full, a
    shield swaps out the item in the highest uncursed hand that is not part
    of a 2H, instead of today's "no room for a shield" refusal. The refusal
    stays only when no such hand exists (for example at 2 arms).
14. **Bloom pushing extra arms past `max_rank` stays with the mutation
    deepening balance pass** (owner, 2026-09-29); the slot rule handles any
    count up to 6 regardless.

## 5a: Object and action gates

**Shape.** Where a rule guards a branch only the player has (containers,
corpses, storage pulls, enchanting), the wrapper still needs the refusal
before that branch runs. So each gate is ONE exported predicate in
`internal/actions`, called by the shared body for both actors and, where a
player-only branch runs first, by the player wrapper too. The wrapper never
evaluates sight, curses or busyness itself; it asks the shared predicate and
words the answer. The one exception to the package is the equip curse rule,
which lives in `internal/characters` beside `Wear`, and which no wrapper
calls: the arm-slot branch reaches it through `Wear` like every other equip
(ruling 11).

### Get

- `actions.TooDarkToGet(actor Actor) bool`: `ParticipantSight == SightNone`
  (G1, unchanged predicate; shapes are enough to grope).
- `actions.TakeFloorItem(actor Actor, item items.Item, stash bool) error`: the
  gated transfer of an item already found. Order, matching the player today
  (G1, G2, G5): `ErrTooDark`, `ErrExploding`, `ErrHouseholdBauble`, then
  `TransferItemToBackpack`.
- `GetItemFromFloor` becomes find plus `TakeFloorItem`. `ErrTooDark` returns
  `Found: false` (the actor learns nothing about the floor). `GetGoldFromFloor`
  refuses with `ErrTooDark` too (G6).
- The sweep (G3) finds with `takeableOnFloor` as now and moves each item
  through `TakeFloorItem`; `ErrExploding` stops the sweep silently, as today.
  Its hand-built `StoreItem` and `ItemOwnership` go (G5 fires the same event).
- `usercommands.Get` keeps its first-statement refusal by calling
  `TooDarkToGet`, so containers, corpses and the bags stay refused in the
  dark. It maps `ErrExploding` to today's line. Single get drops its peek.
- `mobcommands.Get`: silent on every refusal.

**Companion.** No change in behaviour (G9): it never issues a floor `get` it
cannot see clearly. A companion that issued a `get` just before the room went
dark now records "did not manage it" (G8, the existing not-ok line).

### Look

- New `actions.ResolveLook(actor Actor, lookAt string) LookResolution` holds
  every sight rule of both looks: the `SightNone` refusal (L1), the creature
  resolved with `Viewer: actor.GetCharacter()` and acted on only at
  `SightFull` (L2, L7), the exit with `SeesThroughExit` and the lock (L3), and
  the pet at `SightFull`. `LookResolution` carries `Sight`, a kind
  (`LookDark`, `LookRoom`, `LookCreature`, `LookExit`, `LookExitTooDark`,
  `LookExitLocked`, `LookPet`, `LookOther`), the target `Actor`, the exit
  name and room id, and `NamesCreatures bool` for the wrappers' shapes hint.
- Resolution order is the player's (L4): creature before exit. The mob's
  backpack-before-creature order (L5) goes; with no callers (L9) nothing
  observable depends on it.
- `usercommands.Look`: calls `ResolveLook` and renders each kind with
  today's lines; its sealed-crate, container, noun and item branches run on
  `LookOther` exactly as now. `events.Looking` stays player-side.
- `mobcommands.Look`: calls `ResolveLook`; silent on `LookDark`,
  `LookExitTooDark` and `LookExitLocked`. Its room lines keep their wording
  and move to `SendTextVisualHidingNames` with the same names the player's
  lines hide (looker, and the looked-at player).
- Closes L6: a mob can no longer name a hidden player, tell them it is
  looking, or tell the room.
- The lookup guard (T2): both `Look` entries are removed and
  `internal/actions/look.go|ResolveLook` is registered `{viewer: 1}`.

### Remove

- `actions.CursedHolds(char *characters.Character, item items.Item) (holds,
  overridden bool)`: `IsCursed() && Health > 0`, overridden when
  `GetSkillLevel(skills.Spellcasting) >= 4` (R2). The one statement of the
  rule; the companion calls it too.
- `RemoveEquipment` gains, in order: `Busy` when `IsActing()` (R1), then
  `Cursed` when the curse holds, else `CursedOverridden` set and the item
  comes off. Its doc comment (R4) is corrected.
- New `actions.RemoveAllEquipment(actor Actor) RemoveAllResult`: the busy
  gate once, then each worn item through the same per-item gates. A cursed
  item is skipped and listed in `Cursed`; the rest come off. The two copied
  loops (R3, R5) go, and so do the wrappers' aggregate `EquipmentChange`
  events (R6), since the shared body already fires one per item.
- `usercommands.Remove`: renders `Busy` with `refuseWhileBusy`'s text for
  "change equipment" (the text moves to a helper both use; the wrapper no
  longer calls `IsActing`), `Cursed` with today's cursed line, and
  `CursedOverridden` with today's "luckily" line. In `remove all` each skipped
  item gets the cursed line; the overridden ones come off as silently as every
  other item there.
- `mobcommands.Remove`: `PermaGear` stays first (ruling 5); silent on `Busy`
  and `Cursed`.
- Companion: the `remove` verb refuses up front when `CursedHolds` says the
  item holds ("it will not come off"), the way `get` refuses a household
  bauble at `actions.go:366-368`, so it does not record a futile attempt (R8).

### Equip over cursed gear

The owner's brief read the player's `equip.go:172-194` checks as pre-checks
in front of `Wear`. Source differs (E4, E13): they guard only the arm-slot
branch, which never calls `Wear`, and the ordinary player path has no curse
check on armour either. So the rule is missing for players too. Ruling 11
settles the arm-slot branch: it goes through `Wear`, and its checks go.

- `(*characters.Character).CursedRefusal(it items.Item) string`: empty unless
  `it.ItemId > 0 && it.IsCursed()`, else `worn.go`'s reason, "Your X is
  cursed and prevents you from removing it." The one statement of the equip
  rule. No Health and no Spellcasting condition: player equip honours neither
  today (E2, E5, ruling 8), so equip and remove keep different rules, and a
  Spellcasting-4 wearer still frees the slot with `remove` first. It lives in
  `characters` because `Wear` does, and `actions` imports `characters`.
- `Wear` checks each item in `returnItems` with `CursedRefusal` right after
  placement and BEFORE the reservation check, so a cursed refusal reads as the
  curse. On a hit it restores `savedEquipment`, reruns
  `reapplyPermanentConditions` and refuses, the revert the reservation check
  already uses (E1). Checking what placement actually displaced means no slot
  choice is copied, and it covers every armour slot (E3) and the unchecked
  arm-2 displacement (E2, `:457-460`). The three inline weapon checks (E2)
  go; their wording is the shared reason.
- Every caller inherits it with no change of its own (E6, E7): `EquipItem`
  returns the reason; the player's ordinary path prints it (`equip.go:336-342`);
  mob `equip` speaks its set-aside line; the merchant upgrade shelves the item;
  `equipStartingKit` leaves the item in the pack. The spawn and bounty-hunter
  loot `Wear` cannot meet a cursed slot with shipped content (E10); if a
  builder curses a template's worn item, `Wear` refuses and the existing
  `mudlog.Warn` names it.

**Slot choice (rulings 10 and 12).** One exported helper in `worn.go`,
`(*Character).ChooseWornSlot(i items.Item, arm int) (choice SlotChoice,
refusal string)`, for `Ring`, `Wrist`, `Weapon` and `Offhand` items
(earlier drafts called it `PairedArmorSlot`; it is renamed because it now
covers hands). `SlotChoice{Slots []WornSlot; Displaced []items.Item}` names
the `AllSlots` entries `Wear` writes (one; for a 2H the pair's two, the
second cleared) and every item that comes off. `arm` is 0 for `Wear` and the
scorer, N for `WearInArm`. The helper builds an ordered candidate list from
the slots the character actually has, hands from `GetHandPairs` (2 to 6
arms, half pairs included, E14, E15) and wrists from the enabled ones, and
applies one rule to it:

1. **Fill** the first empty candidate, in today's order.
2. Else **swap** the first candidate none of whose displaced items
   `CursedRefusal` refuses.
3. Else **refuse** with `CursedRefusal` of the first candidate's first
   cursed item: today's line, naming the item today's code names.

The first candidate in every list is today's fallback, so with nothing
cursed the choice is today's, exactly (E11, E16). The candidates:

- **Ring**: `Ring`, `Ring2`. **Wrist**: `Wrist1`, `Wrist2`, then
  `ExtraWrist1` to `ExtraWrist4` as `ExtraArms` enables them (on a 6-armed
  character all six). Disabled slots are skipped, so the swap never lands on
  a disabled slot (E11's disabled-`Ring` case); none enabled refuses with
  today's line ("You can't wear rings." / "You can't wear things on your
  wrists.").
- **1H weapon**: the arm slots in arm order, 1 to 6 as the character has them
  (E15): `Weapon`, `Offhand` only with `CanDualWield()` or claws over claws,
  then `ExtraArm1` to `ExtraArm4`, skipping the Second of any pair whose
  First holds a 2H (that slot is consumed). Fill is today's
  `FindFirstEmptySlot` with its dual-wield detour (E16, `:424-450`). A
  candidate's displaced items are its occupant, plus anything left behind a
  2H it holds (today's `:457-460`). So a cursed `Weapon` sends the one-hander
  to the first uncursed hand after it; a cursed 2H in `Weapon` rules out
  `Offhand` too and the swap looks at arms 3 to 6.
- **Shield**: `Offhand` unless `Weapon` holds a 2H, then `ExtraArm1` to
  `ExtraArm4` on the same skip. Fill is today's `FindFirstEmptySlot(pairs,
  true)`. Today's only swap is `Offhand`, so a cursed offhand item now sends
  the shield to the first uncursed extra arm. When `Weapon` holds a 2H and no
  slot is empty, the shield swaps out the item in the LAST available hand
  (owner, 2026-09-29): the highest arm, counting down, whose slot is not part
  of a 2H and holds nothing cursed. With no such hand (2 arms, or every other
  hand cursed or holding a 2H) today's "Your two-handed weapon leaves no room
  for a shield." stands, or the shared cursed line when the only candidates
  are cursed.
- **2H weapon**: whole pairs only, half pairs never (E15), ordered as today
  chooses: a free pair first, then by fewest occupants, the earlier pair on a
  tie (`FindFirstFreePair`, then `FindCheapestPairToDisplace`; a stable sort
  by occupant count). A pair holding any cursed item is skipped, and the
  next pair is tried; with 2 arms there is one pair, so a cursed item there
  refuses as today.
- **Arm N** (`WearInArm`, `arm > 0`): E13's five shape refusals run in the
  helper with the player's wording, then the one slot the arm names is the
  whole list, displacing what E13 displaces (the occupant, a 2H partner in the
  pair's First for an even arm, a 2H's second slot). A cursed item among them
  refuses with the shared line; the helper never falls through to another
  arm (ruling 12).

`wearWeaponOrShield` and `wearArmorSlot`'s `Ring` and `Wrist` cases each
become one call: refuse on `refusal`, else return `Displaced`, clear
`Slots`, write the item into `Slots[0]`, and (hands) reapply permanent
conditions as today. `FindFirstEmptySlot`, `FindFirstFreePair` and
`FindCheapestPairToDisplace` have no other callers (grep; the same search
finds `GetHandPairs` at `equip.go:135`) and become the helper's internals.
`Wear`'s post-placement `CursedRefusal` pass then never fires for a hand,
ring or wrist, and stays as the guarantee for every single-slot armour type.

**The scorer uses the same helper.** For those four types
`compatibleSlotsFor` returns only the `SlotName` of `choice.Slots[0]`
(`extraarm3` is `SlotExtraArm3`, `ring2` is `SlotRing2` and so on) and
nothing on a refusal, so `ItemValueDelta` scores `SwapDelta{}` and
`IsUpgrade` is false; `displacedItemsForSlot` returns `choice.Displaced`.
The scorer then weighs exactly the swap `Wear` makes, for any arm count:
extra arms appear, `HandsRequired` and dual wield count, and it no longer
picks a weaker ring or an `Offhand` a non-dual-wielder cannot use (E12,
E17). Those are score changes with nothing cursed, the point of ruling 10.
`placementBonus` is untouched, so a weapon in an extra arm earns no
`DualWieldBonus` (tuning, not parity).

**Other slots in the scorer.** For the single-slot types `ItemValueDelta`
skips a slot whose `displacedItemsForSlot` holds an item `CursedRefusal`
refuses, so `IsUpgrade` stops calling that swap an upgrade (E8). `gearup`,
the crafter and the floor-equip hook then never try it.

**The arm-slot branch (ruling 11).** `Wear`'s body becomes a private
`wear(i, place)` taking the placement step as a function: type gate,
MinStrength, hands over 2, reservation snapshot, `place`, the cursed pass,
the reservation check and revert, then the success tail, all unchanged.
`Wear(i)` is `wear(i, <today's weapon-or-armour choice>)`, so its callers
(E6, E7) see no change. A new `(*Character).WearInArm(i items.Item, arm int)`
is `wear(i, c.wearInArm(arm))`, where `wearInArm` places through
`ChooseWornSlot(i, arm)`: E13's placement moved out of `equip.go`, its five
shape refusals with the player's wording and its displacement of the target
slot, a 2H partner and a 2H's second slot, now inside the helper's arm-N
case. Its cursed check is the helper's, on that one arm only: `equip X arm5`
over a cursed arm-5 item refuses and never tries another arm (ruling 12). It
calls `reapplyPermanentConditions` as `wearWeaponOrShield` does. In `actions`, `EquipItem`'s body becomes `equipItem(actor, name,
wear)` and a sibling `EquipItemInArm(actor, itemName string, arm int)`
passes `WearInArm`; `EquipItemResult` gains `ArmLabel string` (the pair
slot's label, set only by the arm path).

`usercommands.Equip` keeps the arm-suffix parsing (E13, `:65-86`), the
"fashionable" check and the reservation snapshot, and calls
`EquipItemInArm` when an arm was named. It renders the result as it renders
`EquipItem`'s: displaced lines (today's two paths already share the text),
then "You equip your X in your <label>." for a shield or "You wield your X
in your <label>." with the room line "X equips their Y.", the disclosure,
onStart triggers and the quest notify. A failure prints `FailureReason`.
The four cursed checks, the pair arithmetic, `GetHandPairs`,
`HandsRequired`, `CancelConditionsWithFlag`, `RemoveItem` and
`StoreItem(old)` leave the file. What the player gets from the shared body:

- MinStrength: a too-weak player is refused on `equip X arm2` with "You
  aren't strong enough to handle X." It runs before placement, so it also
  precedes the arm's shape refusals (a too-weak player naming a missing arm
  hears the strength line first).
- Hands over 2 refuses ("That requires too many hands."), as on every other
  path.
- Reservation: an arm equip that worsens a reservation overage is reverted
  and refused with `ReservationRefusal`, as `equip X` without an arm is.
- Curses: the arm line changes from "Your X is cursed and can't be
  removed!" to the shared wording, checked once in `Wear`.
- A displaced item that does not fit the pack drops to the floor
  (`EquipItem`'s overflow rule) instead of vanishing (E13, `:228`).
- Reveal: `EquipItem`'s `TransitionToRevealing` replaces the branch's
  `CancelConditionsWithFlag(Hidden)`; both end hiding, as the ordinary path
  already does. `Validate()` after `Wear`'s own reapply replaces
  `Validate(true)`, which only added that reapply.

No mob issues an arm suffix (mob `equip` has no arm parsing; its issuers send
`equip <name>` or `wear !<id>`), so `EquipItemInArm` has one caller.

**Companion.** Handed better gear whose slot holds a cursed piece, the
companion considers it (the give line), `gearup` finds no upgrade, and the
item stays in its pack: silent, one attempt, no retry (unless a behaviour
tree takes `player_give` first, as today). With a cursed first ring and a
plain second ring it now wears the new ring in place of the second; with a
cursed main-hand weapon it wields a better one-hander in the first uncursed
hand it may use. The scorer/`Wear` mismatches of E12 and E17 (the weaker
ring, the `Offhand` over a cursed `Weapon`, the extra arms the scorer never
saw) are gone: both use `ChooseWornSlot`, so a scored upgrade is a swap
`Wear` makes, for any arm count, and no companion attempt fails on a curse
the scorer missed.

### Craft

- `actions.TooDarkToCraft(actor Actor) bool`: `!CanSeeClearly` (C1, C7).
- `InitiateCraft` checks it first and returns `CraftResult{CannotSee: true}`.
- `usercommands.Craft` keeps its refusal where it is, before the storage pull
  and enchanting (C1, C2), by calling `TooDarkToCraft`; storage, quest notify
  and enchanting stay player-side as documented (C2).
- `mobcommands.Craft`: `CannotSee` is a silent no-op like its other refusals.
- Companion (C5, C6): `craftableHere` returns nothing when `cannotSee(mob,
  room)`, so autonomy never picks `craft` in the dark and the prompt offers no
  recipe there. Without this the companion would record "You set about making
  X." for a craft that never started.
- Planners (C4): a planner crafter at a station in the dark issues `craft` and
  is refused silently, then retries at its own cadence (slice 4, R3). No hot
  loop; the mob waits for light.

### Guard (5a)

Repo-root `sight_gates_wrapper_guard_test.go`, the `drink_wrapper_guard`
shape, fails if any of these files matches its forbidden set:

| Files (both actors) | Forbidden |
|---|---|
| `usercommands/get.go`, `mobcommands/get.go` | `ParticipantSight`, `CanSeeShapes`, `CanSeeClearly`, `` `exploding` `` |
| `usercommands/look.go`, `mobcommands/look.go` | `ParticipantSight`, `SeesThroughExit`, `CanSeeClearly`, `ResolveTargetActor` |
| `usercommands/remove.go`, `mobcommands/remove.go` | `IsCursed`, `IsActing`, `refuseWhileBusy`, `Spellcasting` |
| `usercommands/equip.go`, `mobcommands/equip.go`, `usercommands/gearup.go`, `mobcommands/gearup.go` | `IsCursed`, `Spellcasting`, `CursedRefusal`, `ChooseWornSlot`, `\.Wear\(` |
| `usercommands/equip.go` (arm-slot placement, ruling 11) | `GetHandPairs`, `HandsRequired`, `ItemPtr` |
| `usercommands/craft.go`, `mobcommands/craft.go` | `CanSeeClearly`, `ParticipantSight` |

The guard matches code, not comments: it drops `//` comment text before
matching, because `get.go:28`, `get.go:635`, `look.go:284` and `craft.go:88`
name these words in comments today. Outside the moved gates the files use
none of them in code (grep: `ParticipantSight` at `look.go:33` only,
`SeesThroughExit` at `look.go:285` only). In the equip row, `IsCursed`
appears today only in code, at `usercommands/equip.go:173,180,185,193` (the
arm-slot checks, deleted); the two `gearup.go` files and
`mobcommands/equip.go` have none, and none of the four has `Spellcasting`,
`\.Wear\(`, `CursedRefusal` or `ChooseWornSlot` (the same search finds
`IsCursed` at those four lines). `GetHandPairs` (`:135`), `HandsRequired`
(`:151`) and `ItemPtr` appear in `usercommands/equip.go` only inside the
arm-slot branch that moves to `wearInArm`.
Proven able to fail by a temporary violation in each row.

### Parity table (5a)

| Rule | Player today | Mob today | Both after |
|---|---|---|---|
| Get refused at no sight | yes | no | yes |
| Gold get refused at no sight | yes | no | yes |
| Exploding item refused (single) | yes | no | yes |
| Exploding item stops a sweep | yes | no sweep | yes (player sweep through shared body) |
| Exploding item refused when a plain `get X` auto-detects X in the player's own stash | no (retrieved) | n/a | yes (matches explicit `get X from stash`) |
| Household bauble refused | yes | yes | yes |
| Look refused at no sight | yes | no | yes (mob silent) |
| Creature named only when perceived | yes | no (hidden player named) | yes |
| Creature named only at clear sight | yes | no | yes |
| Pet named only at clear sight | yes | no | yes |
| Look through an exit needs `SeesThroughExit` | yes | no | yes |
| Remove refused while busy | yes | no | yes |
| Cursed item holds (alive, Spellcasting < 4) | single only | no | single and `all` |
| Spellcasting 4 removes a cursed item | yes | n/a | yes |
| `remove all` skips cursed, removes the rest | no (strips all) | no | yes |
| `PermaGear` refuses | n/a | yes | unchanged (mob only) |
| Equip refused over a cursed weapon or shield | yes (`Wear`, arm-slot branch's own copy) | yes (`Wear`) | yes when no eligible hand is free of curses, one `CursedRefusal` through `ChooseWornSlot` |
| Equip refused over cursed armour, light | no | no | yes |
| Rings or wrists full: swap the first uncursed one | no (always `Ring` / `Wrist1`) | no | yes (`ChooseWornSlot`, ruling 10), all six wrists on 6 arms |
| Rings or wrists full and all cursed: refused | no | no | yes |
| Hands full, main hand cursed: 1H goes to the first uncursed hand it may use (2 to 6 arms) | no (refused) | no (refused) | yes (ruling 12) |
| Hands full, offhand item cursed: shield goes to the first uncursed extra arm | no (refused) | no (refused) | yes (ruling 12) |
| 2H in the main hands, no empty hand: shield swaps the last available hand | no ("no room") | no ("no room") | yes, from 3 arms (ruling 13) |
| 2H skips a pair holding a cursed item, tries the next pair | no (refused on today's pair) | no | yes (ruling 12) |
| `equip X armN` over a cursed item in arm N refuses, never another arm | yes (own copy) | n/a | yes (helper's arm-N case) |
| Nothing cursed: `Wear`'s slot choice | today's | today's | today's, for every arm count |
| Upgrade scoring picks the slot `Wear` will use, extra arms included | no (best of `Weapon`/`Offhand` or of the rings; no extra arms) | no | yes (same helper) |
| `equip X armN` meets MinStrength | no | n/a (no arm suffix) | yes (`WearInArm`, ruling 11) |
| `equip X armN` meets the reservation ceiling | no | n/a | yes |
| `equip X armN` displaced item kept on a full pack (floor) | no (lost) | n/a | yes (`EquipItem` overflow) |
| Spellcasting 4 lets equip displace a cursed item | no | no | no (matches player equip, ruling 8) |
| Upgrade scoring skips a swap a curse would refuse | n/a | no | yes (`ItemValueDelta`; rings and wrists through the helper) |
| Craft refused below clear sight | yes | no | yes |

## 5b: Speech and emotes in the dark

**Shape.** Two audio Room senders beside `SendTextVisualHidingNames`, both
three-tier through a hiding function, per listener, never shortcutting on a
lit room (which is what leaks S7 to a blinded listener), and one visual
communication sender for emotes (below):

- `Room.SendCommunicationHidingNames(cat messaging.Category, text string,
  names []string, excludeUserIds ...int)`: per listener, hides `names` by
  that listener's `ParticipantSight`, renders through `RenderForRecipient` on
  the audio channel (so the category's colour and wrap apply, as mob speech
  gets today), and queues a per-user `events.Message{IsCommunication: true}`,
  which the per-user deafen check (S13, `:29`) filters. The owner's sketch had
  no category; one is added because mob lines carry `CategorySpeech`,
  `CategoryShout`, `CategoryRally` and `CategoryWarcry` today.
- `Room.SendTextHidingNames(cat, text, names, excludeUserIds...)`: the same,
  unfiltered, for authored sounds that are not chatter (S12's audited line).
  It replaces `sendAudioRoomTextHidingNames`.

**The unseen word.** `HideNames` says "something" at no sight (S9); ruling 3
wants "Someone" for a speaker. `messaging` gains `HideSpeakerNames(text,
names, d)`, the same matcher with "a figure" and "someone". Speech lines use
it; sounds (rally, warcry, howl, taunt) keep `HideNames`, whose "Something
lets out a rallying roar!" is today's anonymous mob line word for word.

**Shared bodies.**

- `actions.Say(actor, text)` now also sends the room line:
  `FormatSayText` with the actor's colours (from `IsPlayer()`), with the
  speaker's name hidden by each listener's sight, excluding a speaking player.
  A player speaker sends through `SendCommunicationHidingNames` (deafen
  applies); a mob speaker through `SendTextHidingNames` (authored NPC speech,
  deafen does not apply, ruling 6). The wrappers keep: mute, drunk, escaping, the self line,
  and the mob's `PlayerCt() < 1` early return (S5, a cost shortcut, left).
- New `actions.Shout(actor, text) ShoutResult`: reveal (rider 5, S6), the room
  line through the same per-speaker sender as `Say` (ruling 6), the
  adjacent-room line, and
  waking sleepers except the shouter. The player wrapper keeps mute,
  uppercase, drunk, escaping and the self line; the mob wrapper keeps nothing
  but the call. If a speaker is somehow still hidden after the reveal (S2),
  every listener reads the no-sight form, so a hidden name never leaks.
- **Adjacent rooms** keep hearing an anonymous shout, and now one line for
  both: the player's `Someone shouts from the <exit> direction, "<words>"`
  (ruling 3, words always heard). Mob shouts next door gain their words (S6).
- **Rally and warcry.** `ExecuteRally` and `ExecuteWarcry` are already
  shared. The room line (wording per side, unchanged) goes through one
  shared sender, `actions.SendHeard(actor, cat, text)`, which calls
  `SendTextHidingNames` with the actor's name and excludes a player actor.
  The player's lines, including the Resonant Larynx fold lines
  (`rally.go:73`, `warcry.go:77`), stop being visual (S11): a roar is heard in
  the dark, name hidden by sight. They are authored text, not chatter, so
  deafen does not apply (S12).
- **Howl and taunt** (mob only, S10) move mechanically from the two mob
  helpers to `SendTextHidingNames`: each `(anon, full)` pair becomes the full
  line with `[mob name, target name]`. Player taunt stays visual (out of
  scope).
- `sendAudioRoomText` and `sendAudioRoomTextHidingNames` are deleted with
  `darkness.go`, and `audio_room_text_sight_test.go` is ported to the Room
  senders. `Actor.SendRoomCommunication` (S15), dead and now contradicting the
  shipped rule, is deleted with its nine test fakes.

**Emotes (ruling 7).** An emote is seen, so it stays on the visual channel;
the one thing the free-form player line needs that no visual sender gives is
the deafen flag (M7).

- `Room.SendVisualCommunicationHidingNames(cat, text, names,
  excludeUserIds...)`: `SendTextVisualHidingNames` with each per-user
  `events.Message` marked `IsCommunication`, so the per-user deafen check
  (S13, `:29`) filters it. The smallest shared change: the private
  `sendTextVisualJudgedBy` gains a `communication bool` that sets that one
  field; its four current callers pass `false`. Sight, `Anonymize` (capitalised
  "A figure", M5), `HideNames` at shapes and the `SightNone` drop are the
  existing path, unchanged.
- New `actions.SendSeen(actor Actor, cat messaging.Category, text string,
  chatter bool)`, the visual twin of `SendHeard`: names `[actor name]`,
  excludes a player actor, and sends through
  `SendVisualCommunicationHidingNames` when the actor is a player and
  `chatter` is set, else through `SendTextVisualHidingNames`. A mob is never
  deafen-filtered whatever `chatter` says (ruling 6).
- `usercommands.Emote`: the free-form line, `@` form included (M1, M2), goes
  through `SendSeen(..., CategoryEmote, line, true)`; it gains
  `CategoryEmote`, which it lacks today (the alias line already has it), and
  normalize skips every stage for it (`normalize.go:26-44`) and it is not in
  the wrap allowlist (`pipeline.go:137`), so only its colour tag is new. The empty and alias lines (M3) go through `SendSeen(..., false)`: still
  unfiltered by design, now also hiding a bare own name. Mute, escaping, the
  self line and `events.Emote` stay in the wrapper; the companion already
  ignores an emote it cannot see (M8).
- `mobcommands.Emote`: both lines through `SendSeen(..., CategoryMobEmote,
  line, false)`. Mob emotes were already three-tier through the tag (M4, M5),
  so the only change is the bare-name hiding; the `PlayerCt() < 1` early
  return stays, as for say (S5). Every mob emote issuer rides this (M6).

**Guard (5b).** Repo-root `speech_wrapper_guard_test.go` fails if
`usercommands/{say,shout,rally,warcry,emote}.go` or
`mobcommands/{say,shout,rally,warcry,emote}.go` matches `SendTextCommunication`,
`sendAudioRoomText`, `HideNames`, `HideSpeakerNames`, `ParticipantSight`,
`TransitionToRevealing`, `ForEachAdjacentRoom`, `OnSleeperWoken`,
`SendTextVisualHidingNames`, `room\.SendText\(` or `room\.SendTextVisual\(`,
and if any file outside `internal/rooms` and `internal/actions` calls
`SendCommunicationHidingNames` or `SendVisualCommunicationHidingNames`.
Comments are dropped before matching, as in 5a. Party member lines
(`memberUser.SendText`) do not match. Proven able to fail. T3, T4 add the
three new senders; T5 drops the dead recogniser and updates the comments.

### Parity table (5b)

| Rule | Player today | Mob today | Both after |
|---|---|---|---|
| Say: name by listener sight, three tiers | no (always named) | two tiers | yes |
| Say: blinded listener in a lit room hears no name | no | no (lit shortcut) | yes |
| Say: words always heard | yes | yes | yes |
| Say: reveals a hidden speaker | yes | yes | yes |
| Say: deafened listener filtered | yes | no | unchanged: players yes, NPCs no (ruling 6) |
| Shout: name by listener sight, three tiers | no | two tiers | yes |
| Shout: reveals a hidden speaker | yes | no | yes |
| Shout: adjacent rooms hear an anonymous line with the words | yes | no words | yes |
| Shout: wakes sleepers in the room | yes | yes | yes |
| Shout: deafened listener filtered | yes | no | unchanged: players yes, NPCs no (ruling 6) |
| Rally/warcry heard, name by sight | no (visual) | two tiers | yes |
| Rally/warcry deafen-filtered | no | no | no |
| Unseen speaker reads "Someone" | n/a | "someone" (2 tiers) | "A figure" / "Someone" |
| Free-form emote: name by sight (name / "A figure" / nothing) | no (named to all, blinded included) | yes (tag, M5) | yes |
| Empty and alias emote: name by sight | yes (tag) | yes (tag) | yes |
| Emote hides a bare own name at shapes | no | no | yes (`SendSeen`) |
| Free-form emote: deafened listener filtered | yes | no | unchanged: players yes, NPCs no (rulings 6, 7) |
| Empty and alias emote: deafen-filtered | no | no | no |

## What changes in play

**Players see, 5a.** Equipping armour or a light over a cursed piece now
fails with "Your X is cursed and prevents you from removing it." (it
silently swapped the cursed piece out before, E4). With every ring or wrist
slot full, a new ring or wrist piece replaces the first uncursed one instead
of always the first; only when all are cursed does it fail with that line.
Hands work the same way on every arm a character has: with a cursed weapon
stuck in the main hand, a new one-hander goes into the first uncursed hand
it may use (the offhand only for a dual wielder, then extra arms 3 to 6);
a shield skips a cursed offhand item for the first uncursed extra arm; a
two-hander skips a pair holding anything cursed for the next pair. A
character with three or more arms holding a two-hander can now take up a
shield even with every hand full: it swaps out the item in the last free
hand (ruling 13); at two arms "no room for a shield" still refuses. Only when
every eligible hand or pair is cursed does it fail with that line, and with
nothing cursed every equip lands exactly where it does today. `equip X armN` now behaves like `equip X` apart from where the item goes: a
player too weak for the item is refused ("You aren't strong enough to handle
X."), even before the arm's own refusals; an arm equip that would worsen a
reservation overage is refused and undone; the cursed line takes the shared
wording, and a cursed item in the named arm still refuses rather than moving
the item elsewhere; and an item it knocks off a full pack lands on the floor instead
of vanishing. `remove all` no longer strips cursed gear: each cursed item stays on with
the cursed line, unless the player has Spellcasting 4. `get all <name>`
behaves exactly as before. A plain `get X` that finds X in the player's own
stash now refuses an exploding item with the same line an explicit `get X
from stash` already gave; before, this auto-detect path retrieved it. With
no cursed items in the live world (E10), the curse changes are latent until
a builder makes one.

**Players see, 5b.** In a dark room a speaker's name now follows the
listener's sight: "A figure says, ..." at shapes, "Someone says, ..." in
blackness, and a blinded listener in a lit room hears "Someone" too. This
applies to other players (named to everyone today) and to NPCs (two tiers
today). Mob shouts next door now carry their words. A player's rally or
warcry is heard in the dark instead of vanishing. A player's free-form
`emote` is seen like the other emotes: named at clear sight, "A figure ..."
at shapes, and nothing at all in blackness or to a blinded listener, where
today it names the emoter to everyone. A deafened player still does not see
another player's free-form emote.

**Mobs, 5a.** A mob cannot pick up anything, or gold, when it sees nothing,
nor an exploding item. A mob cannot look at, or be seen to look at, a hidden
player. A busy mob cannot take gear off, and a mob's cursed gear stays on
against `remove` (Spellcasting 4 aside) and against any equip (no
exception): no mob swaps out a cursed piece, and upgrade scoring stops
offering that swap, so a companion handed better gear for a cursed slot keeps
it in its pack without trying. With a cursed first ring and a plain second,
a mob now wears a better ring over the second, and likewise a better
one-hander past a cursed main-hand weapon. The scorer judges every ring,
wrist, weapon and shield swap against the piece that actually comes off, in
the slot `Wear` will actually use, extra arms included, so a multi-armed mob
also stops scoring a main-hand swap when `Wear` would fill an empty extra
arm. Crafters stop in the dark: a planner crafter
at a station after the lamps fail waits for light, and the AI companion
neither offers nor starts a recipe it cannot see to make.

**Mobs, 5b.** A hidden human mob that shouts "it's time to die!" on entering
combat now reveals itself. The AI companion's speech, quest NPC lines and
shopkeepers follow the three-tier rule, and a deafened player still hears
them (ruling 6). Mob emotes read as they do today, except that a bare
mention of the mob's own name in its emote text is hidden at shapes too.

## Testing and gates

- 5a: a table drives each shared body through a `UserActor` and a `MobActor`
  and asserts the same outcome for every row of the 5a parity table
  (`TakeFloorItem`, `GetGoldFromFloor`, `ResolveLook`, `RemoveEquipment`,
  `RemoveAllEquipment`, `InitiateCraft`), with light set by `lamp` and
  blindness by the perception machine so the verdicts are exact, not rolled.
  Wrapper tests pin today's wording for each player refusal and the silent
  mob path. The companion's `craftableHere` returns nothing in the dark and
  its `remove` refuses a cursed item.
- 5a equip: `Wear` tests in `internal/characters` put a cursed item in each
  slot family on a 2-armed character with no uncursed alternative (2H pair,
  shield over arm 2, 1H over `Weapon` with no dual wield, arm 2 behind a
  2H, every armour type, both rings full, all wrists full, light) and assert
  the refusal text, that `Equipment` is unchanged and that the candidate is
  not worn; an uncursed (`Uncursed`) item swaps as today; a cursed refusal
  wins over a reservation refusal. `EquipItem` through a `UserActor` and a
  `MobActor` returns the same reason and leaves the item in the pack.
  `ItemValueDelta` scores no upgrade for a slot a curse holds, so mob
  `gearup !<id>` after a `give` wears nothing and speaks nothing.
- 5a ring and wrist choice (`ChooseWornSlot`): an empty slot is filled
  first in E11's order; a cursed `Ring` with a plain `Ring2` swaps `Ring2`
  and leaves `Ring` on; both cursed refuses with the shared line naming the
  `Ring` item and leaves `Equipment` unchanged; the same three for
  `Wrist1`/`Wrist2` and for extra wrists at `ExtraArms` 2; a disabled `Ring`
  with a full `Ring2` swaps `Ring2`, never writing the disabled slot.
  `ItemValueDelta` for a ring with both slots full reports the helper's slot
  (`Ring2` when `Ring` is cursed; `Ring` when neither is, even when `Ring2`
  holds the weaker ring) and `SwapDelta{}` when both are cursed; a mob
  `gearup` with a cursed `Ring` and a plain `Ring2` wears the new ring in
  `Ring2`.
- 5a hands across arm counts (`ChooseWornSlot`, ruling 12): one table,
  every row run at 2, 3, 4 and 6 arms (`ExtraArms` 0, 1, 2, 4; 3 is the
  natural mutation's reach and the half-pair case, E14), and every row
  asserts that `Wear` fills or swaps exactly the slot `ItemValueDelta`
  reports and displaces exactly its `Displaced`, or that both refuse:
  - nothing cursed: every fill and swap lands where today's code puts it
    (a golden run of today's `wearWeaponOrShield` over the same loadouts,
    captured before the change);
  - cursed main hand, plain or empty other hand, dual wielder: the
    one-hander goes to `Offhand`; without dual wield, to `ExtraArm1` from 3
    arms up and refused at 2;
  - every hand the one-hander may use cursed: refused with the shared line
    naming the `Weapon` item, `Equipment` unchanged;
  - cursed 2H in `Weapon`: a one-hander skips `Offhand` and swaps the first
    uncursed extra arm, refused at 2 arms;
  - shield with a cursed offhand item, hands full: refused at 2 arms; from
    3 arms up the first uncursed extra arm, refused when every extra arm is
    cursed too;
  - shield with a 2H in `Weapon` and no empty slot: "no room for a shield"
    at 2 arms; at 3, 4 and 6 arms it swaps the last available hand (arm 3,
    arm 4 and arm 6 respectively, a one-hander's slot, never a 2H pair); with
    that hand cursed it takes the next one down; with every candidate cursed,
    the shared cursed line;
  - two-hander with a cursed item in the cheapest pair: the next whole pair
    at 4 and 6 arms (never the half pair at 3), refused at 2 and 3;
  - wrists: `Wrist1` cursed, every wrist full, extra wrists present: the
    first uncursed of `Wrist2`, `ExtraWrist1` onward; all six cursed at 6
    arms refuses;
  - `equip X arm5` at 6 arms with a cursed arm-5 (`ExtraArm3`) item:
    refused with the shared line though arms 1 to 4 and 6 are plain, and
    nothing moves; at 4 arms, today's "You don't have arm 5.".
- 5a arm slot (`WearInArm`, `EquipItemInArm`): `equip X arm2` is refused with
  the strength line when too weak, and with `ReservationRefusal` (equipment
  restored) when it worsens a reservation overage; a cursed item in the
  target arm, in a 2H's second slot, or a cursed 2H partner in arm 1 when
  arm 2 is named, each refuses with the shared line through `Wear`, the
  candidate stays in the pack and `Equipment` is unchanged. The five shape
  refusals keep today's wording; a successful arm equip prints "You wield
  your X in your <label>." (or "You equip ..." for a shield) and the room
  line; a displaced item with a full pack lands on the floor. These are new:
  no test covers the arm-slot branch today (grep `arm2`, `arm1`, `arm#` and
  `targetArmSlot` over `*_test.go` finds only a comment's `extraarm1` in
  `combat/defence_sets_per_slot_test.go:33`).
- 5b: a per-listener table for say, shout, rally and warcry, each with a
  player speaker and a mob speaker, over five listeners in the speaker's room:
  clear sight (named), shapes ("A figure"), dark ("Someone"), blinded in a lit
  room ("Someone"), deafened (a player speaker's say and shout: nothing; a
  mob speaker's say and shout, and rally and warcry from either: heard, per
  ruling 6). Every row asserts the words arrive.
  Adjacent rooms get the one anonymous line with the words. A hidden mob that
  shouts is revealed.
- 5b emote: the same five listeners for a player's free-form, `@`, empty
  and alias emote and a mob's free-form and empty emote. Clear sight: named;
  shapes: "A figure ..." (capitalised) and a bare own name hidden; dark and
  blinded in a lit room: nothing; deafened: nothing for a player's free-form
  and `@` lines, seen for a player's empty and alias lines and every mob
  line. The actor gets its own "You Emote:" line only outside the `@` form.
  `sendTextVisualJudgedBy`'s four existing callers still queue messages
  without `IsCommunication`.
- The T6 and T7 tests move with the code, unchanged in what they assert;
  `FormatSayText`'s tests stay.
- Both re-fork guards, each proven able to fail; the lookup guard re-key;
  T3 to T5.
- `context.md` updated for `internal/actions`, `internal/rooms`,
  `internal/messaging`, `internal/characters`, `internal/itemvalue`,
  `internal/mobcommands`, `internal/usercommands` and `modules/aicompanion`.
- Gate per PR: gofmt, vet, build, `go test ./...`, golangci-lint
  new-from-merge-base, boot check, playtest (5a: companion in a dark room
  with a station and cursed gear, then handed better gear for the cursed
  slot; the live world ships no cursed item (E10), so the playtest curses one
  through the item editor on its ephemeral checkout; 5b: say, shout and
  emote between two players and an NPC across light, shapes, dark and
  blinded). **5a ships first; 5b follows as a second PR.**

## Out of scope

- Whisper and reply: remote tells, not room speech (S18).
- Mob `sayto` and `replyto`, which name both parties to the room with no
  sight gate (S18). A finding, filed, not fixed here.
- Player `ask` room lines, and player taunt's visual channel. (Emotes moved
  into 5b, ruling 7.)
- (The arm-slot branch, the ring and wrist choice and weapon and offhand
  placement over a cursed hand, out in earlier drafts, are now in 5a by
  rulings 10 to 12.)
- `placementBonus` for extra arms: a weapon or shield there earns no
  `DualWieldBonus` or `ShieldBonus` in the score. Tuning, not parity.
- Bloom deepens `extra-arms` past its `max_rank: 1` because
  `BloomAdvanceMutation` reads the global `MutationMaxLevel` instead of
  `effectiveMax` (E14). Owner ruling 14: stays with the mutation deepening
  balance pass (already filed there); the slot choice
  handles every count up to the cap of 4 extra arms either way.
- Disarm and forced unequips (`combat/criteffects.go:61`,
  `hooks/combat_shared_helpers.go:243`), which bypass `RemoveEquipment` and
  so strip a cursed weapon or break a cursed shield. Out because they leak no
  names (ruling 9, D1): disarm goes through `messaging.SendTrio`, whose
  observer line is `SendTextVisualHidingNames` with both names and whose
  actor and actee lines hide the other party by sight; the offhand break's
  room line is `SendTextVisualHidingNames`.
- The mob `say` early return with no players present (S5).
- Retuning any sight threshold.

