# Sight Gates 5b: Speech and Emotes in the Dark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A speaker's or emoter's name follows each listener's sight for players and mobs alike (clear: the name; shapes: "A figure"; nothing seen: "Someone" for speech, "Something" for sounds, nothing for emotes), through shared bodies in `internal/actions`, with the deafen filter kept on player free text only.

**Architecture:** `messaging` gains `HideSpeakerNames` and a `NameHider` type. `rooms.Room` gains two audio senders (`SendTextHidingNames`, `SendCommunicationHidingNames`) that hide names per listener with no lit-room shortcut, and one visual sender (`SendVisualCommunicationHidingNames`) that is `SendTextVisualHidingNames` with the deafen flag set. `actions.Say` and a new `actions.Shout` own the room line, reveal, adjacent line and waking; `actions.SendHeard` (rally, warcry) and `actions.SendSeen` (emotes) are the shared room-line senders. The command wrappers keep only wording, mute, drunk text and escaping. The two mob darkness helpers and the dead `Actor.SendRoomCommunication` are deleted, and a repo-root guard stops the wrappers re-forking.

**Tech Stack:** Go 1.25, the `messaging` render pipeline, `internal/actions` Actor seam, repo-root `go/ast` guard tests.

**Spec (binding):** `docs/superpowers/specs/2026-09-29-sight-and-gates-parity-design.md`, section "5b: Speech and emotes in the dark", owner rulings 3, 5, 6 and 7.

**Branch:** `feature/sight-gates-5b`, cut from master `6b6ff7ddf` (after 5a, #195, and baubles slice C, #197, merged). Both are in the base, so there is no pending conflict to resolve. The worktree already exists:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feature/sight-gates-5b C:/tmp/dogmud-5b 6b6ff7ddf
```

All paths below are relative to that worktree. Run Go commands from its root.

---

## Facts verified against source (first 2026-09-29 at `7d6d4ac38`; re-verified 2026-09-30 at `6b6ff7ddf`)

Every row below was re-read at `6b6ff7ddf` (master after 5a #195 and baubles slice C #197). Of the 74 Go/internal files those merges changed, the only ones this plan reads or edits are `raw_events_message_guard_test.go` (F36), `internal/actions/context.md` (Task 12 anchor), `internal/mobcommands/context.md` and `internal/usercommands/context.md` (no anchor moved), and four new or grown `internal/actions` test files (no Actor fake, no fixture id or helper name collides). Every other source file named here is byte-identical to `7d6d4ac38`, and every "replace" / "delete" code block in Tasks 1 to 10 was matched verbatim against HEAD by script. Rows marked **NEW** are facts the spec does not state and that change the plan. Tests load Go config defaults, not `config.yaml`.

| # | Fact | Where |
|---|---|---|
| F1 | `actions.Say(actor, text) SayResult` reveals (`TransitionToRevealing`, `TriggerNoisyAction`, `"command": "say"`), computes `isSneaking := char.IsHidden()`, calls `actor.GetRoom().SendTextToExits("You hear someone talking.", true)`, queues `events.Communication{SourceUserId, SourceMobInstanceId, CommType: "say", Name, Message}`; no room line. `FormatSayText(name, text, isSneaking, nameColor, textColor)` builds `<ansi fg="%s">%s</ansi> says, "<ansi fg="%s">%s</ansi>"` or the lowercase `someone says, ...`, then `util.SplitStringNL(msg, 80)` | `internal/actions/say.go:21-61` |
| F2 | Player say: drunkify, `util.EscapeAnsiTags`, `actions.Say`, then `room.SendTextCommunication(FormatSayText(Name, ..., "username", "saytext"), user.UserId)`, then self line `You say, "<ansi fg="saytext">%s</ansi>"` on `CategorySpeech` | `internal/usercommands/say.go:16-42` |
| F3 | Player shout: mute; reveal (`"command": "shout"`); uppercase; drunkify; escape; named `<ansi fg="username">%s</ansi> shouts, "<ansi fg="yellow">%s</ansi>"` (or `someone shouts, ...`) via `SendTextCommunication(SplitStringNL(msg, 80), user.UserId)`; `ForEachAdjacentRoom` sends `Someone shouts from the <ansi fg="exit">%s</ansi> direction, "<ansi fg="yellow">%s</ansi>"` via `otherRoom.SendTextCommunication(..., user.UserId)`; self line `You shout, ...` on `CategoryShout`; wakes sleeping players (skipping the shouter) and every sleeping mob | `internal/usercommands/shout.go:18-88` |
| F4 | Mob say: `if room.PlayerCt() < 1 { return }`; `actions.Say`; hidden branch plain `room.SendText(CategorySpeech, someone says ...)`; else `sendAudioRoomText(room, mob, CategorySpeech, anon, named)` with `"mobname"`, `"saytext-mob"` | `internal/mobcommands/say.go:13-33` |
| F5 | Mob shout: no reveal; `sendAudioRoomText(CategoryShout, someone shouts..., <ansi fg="mobname">%s</ansi> shouts, "<ansi fg="saytext-mob">%s</ansi>")`; adjacent `otherRoom.SendText(CategoryShout, "Someone is shouting from the <ansi fg="exit">%s</ansi> direction.")` with no words; wakes every sleeping player and every sleeping mob except itself | `internal/mobcommands/shout.go:14-59` |
| F6 | `sendAudioRoomText(room, mob, cat, anonMsg, fullMsg, excluded...)` shortcuts `room.IsLit()` to `room.SendText(cat, fullMsg)` (so a blinded listener in a lit room reads the name), else per listener `ParticipantSight == SightFull` gets `fullMsg`, everyone else `anonMsg`, via `u.SendText`. `sendAudioRoomTextHidingNames(room, cat, fullMsg, names, excluded...)` is three-tier: `u.SendText(cat, messaging.HideNames(fullMsg, names, ParticipantSight))` | `internal/mobcommands/darkness.go:25-52,66-82` |
| F7 | Production callers: `sendAudioRoomText` 10 (`say.go:29`, `shout.go:24`, `rally.go:25`, `warcry.go:25`, `howl.go:47,68`, `taunt.go:54,75,86,99`); `sendAudioRoomTextHidingNames` 4 (`howl.go:58,79`, `taunt.go:153,189`). Mentions in comments: `go.go:34`, `taunt.go:125`, `taunt_store_test.go:75`. Test caller: `audio_room_text_sight_test.go:56` | grep `sendAudioRoomText` over `*.go` |
| F8 | `HideNames(text, names, d)` returns text at `SightFull`; else replaces each non-`NoName` name, longest first, first as a whole identity tag (`hideTaggedName`) then as a bare whole word (`hideOneName`), with `<ansi fg="combat-anon">` + `UnseenNoun(d)` ("a figure" / "something"), capitalised at a sentence start, looking back through tags | `internal/messaging/hidenames.go:30-72,74-105,140-167` |
| F9 | `hideTaggedName(text, name, word)` matches `identityTagPattern` (`username`, `mobname`, suffixed forms, `petname`), case-insensitive, ignoring a ` #N` duplicate index, and swallows one adjective span after the tag | `internal/messaging/hidenames_tagged.go:10,14,33-35,49-84` |
| F10 | `Room.SendTextCommunication(txt, excl...)` queues ONE RoomId-keyed `events.Message{IsCommunication: true}`, no category, no render. `Room.SendText` renders per user through `RenderForRecipient` on `ChannelAudio` with `LineWidth: u.GetLineWidth()` and queues `events.Message{UserId, Text}` | `internal/rooms/rooms.go:218-236,241-264` |
| F11 | `sendTextVisualJudgedBy(lighting, cat, txt, names, excl...)` judges with `CanSeeClearly` / `CanSeeShapes` (not `ParticipantSight`, so a sleeper reads nothing), and at shapes with names runs `HideNames(Anonymize(txt), names, SightShapes)`; queues `events.Message{UserId, Text}`. Four callers: `SendTextVisual` (`:270`), `SendTextVisualHidingNames` (`:277`), `SendTextVisualAsLit` (`:300`), `SendTextVisualAsLitHidingNames` (`:314`) | `internal/rooms/rooms.go:266-369` |
| F12 | `RenderForRecipient`: normalize, visual sight gate (`SightNone` returns "", shapes anonymizes), then `applyCategoryColor` wraps `<ansi fg="<cat>">...</ansi>` for any category but `CategoryDefault`, then wrap for `shouldWrap` categories. Audio ignores sight. `shouldWrap` includes `Rally`, `Warcry`, `Taunt*`, `MobEmote`; excludes `Speech`, `Shout`, `Emote` | `internal/messaging/pipeline.go:53-84,115-145` |
| F13 | `skipStages` skips every normalize stage for `Speech`, `Shout`, `Emote`, `MobEmote`, `NPCDialogue` (not `Rally`, `Warcry`, `Taunt*`) | `internal/messaging/normalize.go:24-47` |
| F14 | Colour aliases: `rally` 14, `warcry` 9, `combat-anon` 196, `speech` 111, `shout` 215, `emote` 144, `mob-emote` 137. So player say, shout and free-form emote, sent today through `SendTextCommunication`, carry NO category colour; mob speech does | `_datafiles/world/dogmud/ansi-aliases.yaml:222-249` |
| F15 | The deafen filter is `message.IsCommunication && user.Deafened`, on the per-user branch (`:29`) and the RoomId branch (`:83`). It is the only reader of `Message.IsCommunication` (`Broadcast.IsCommunication` at `eventtypes.go:114` is a different struct, read by `Broadcast_SendToAll.go:33` and `discord/listeners.go:106`) | `internal/hooks/Message_SendMessages.go:29,83`; grep `IsCommunication` |
| F16 | **NEW.** No test helper returns a queued `Message`'s flags or a RoomId-keyed message: `DrainQueuedMessagesForTest(userId) []string` returns `Text` of `UserId`-keyed messages only. The deafen verdict is applied in the hook, after the queue, so a test cannot observe it today | `internal/events/events.go:357-377` |
| F17 | `Actor.SendRoomCommunication(msg, excludeSelf)`: interface `actor.go:27-31`, `UserActor` `actor_user.go:46-52`, `MobActor` `actor_mob.go:45-53`. No production call (grep `\.SendRoomCommunication(` finds none). Nine test fakes: `actions/{consider:54,economy:53,forage:73,salvage:74,scan:68,search:72,sleep:39,track:69}_test.go`, `hooks/spell_foldanchor_test.go:47`. 5a and slice C added no Actor fake and no Actor method; `grep -rn SendRoomCommunication --include=*.go .` prints 15 lines | grep |
| F18 | **NEW.** `actions.Say` has three more callers the spec does not list, each sending its OWN named line through plain `room.SendText(CategorySpeech, FormatSayText(mob.Character.Name, ..., false, "mobname", "saytext-mob"))` in any light: the justice guard speech, the merchant refusal in `actions/sell.go`, and the offer/appraise merchant line in `usercommands/offer.go` (its own package-local `merchantSay`, also called by `appraise.go:104,129,131`). Once `Say` sends the room line all three would double-send, so all three drop their own send | `internal/hooks/justice_wiring.go:15-23`; `internal/actions/sell.go:102-110`; `internal/usercommands/offer.go:20-28` |
| F19 | `FormatSayText` callers: `usercommands/say.go:35`, `mobcommands/say.go:27,28`, `sell.go:109`, `justice_wiring.go:22`, `usercommands/offer.go:27`; pinned by `actions_test.go:67-126` | grep `FormatSayText(` |
| F20 | `SendTextCommunication` production callers: `actor_user.go:48,50`, `usercommands/emote.go:52`, `say.go:36`, `shout.go:51,54,59`, and `modules/aicompanion/listeners.go:417` (the player `ask` line, out of scope). So the method stays | grep |
| F21 | Player emote: empty `room.SendTextVisual(CategoryEmote, <ansi fg="username">%s</ansi> emotes., user.UserId)`; alias `room.SendTextVisual(CategoryEmote, aliasMsg, user.UserId)`; free-form and `@` `room.SendTextCommunication(FormatEmoteText(Name, rest, "username"), user.UserId)`; self line `You Emote: %s` except the `@` form | `internal/usercommands/emote.go:14-58` |
| F22 | Mob emote: `PlayerCt() < 1` early return; empty and free/alias through `room.SendTextVisual(CategoryMobEmote, ...)` with no exclusion. `FormatEmoteText(name, text, color)` = `<ansi fg="%s">%s</ansi> <ansi fg="137">%s</ansi>` split at 80. `EmoteAliases["beam"]` = `beams with pride.` | `internal/mobcommands/emote.go:12-32`; `internal/actions/emote.go:27-35`; `emote_aliases.go:9` |
| F23 | Player rally and warcry send their room lines through `room.SendTextVisual` at `rally.go:42-45,73-76` and `warcry.go:44-47,77-80`; mob rally and warcry through `sendAudioRoomText` at `rally.go:25-27`, `warcry.go:25-27` with anon lines `Something lets out a rallying roar!` / `Something lets out a bone-shaking warcry!` | files named |
| F24 | **NEW.** `condition_apply_path_guard_test.go` keys six allowlist entries by LINE NUMBER in the player rally and warcry files: `rally.go|57,86,114`, `warcry.go|57,90,118` (the `AddConditionMagnitude` calls). Any edit that shifts those lines breaks `TestPlayerConditionsTravelTheEventPath`, so Task 7's edits are line-count neutral | `condition_apply_path_guard_test.go:126-131,404` |
| F25 | Howl and taunt anon/full pairs: howl fumble `:47-49`, howl aggro-pull `:68-70` (anon `Something turns, drawn snarling toward a new foe.`); taunt fumble fallback `:54-56`, hit fallback `:75-77` (anon via `messaging.Anonymize`), aggro-pull `:86-88` (anon `Something wheels around, drawn to a new challenger.`), miss fallback `:99-101`; hiding calls howl `:58-60,79-81`, taunt triad `:153`, channel defence `:189-190` | `internal/mobcommands/howl.go`, `taunt.go` |
| F26 | `predator_test.go:583-700` pins the dark routing of `sendChannelDefenceMessages` and the taunt/howl runtime ("something" for SightNone, "a figure" for infrared, `fg="taunt-resist"`); the AST tests at `:473-545` count `executeTauntAction` and `sendChannelDefenceMessages` calls, unaffected | `internal/mobcommands/predator_test.go` |
| F27 | `ForEachAdjacentRoom` loads each exit's room with `LoadRoom` and calls back only where `otherRoom.FindExitTo(r.RoomId)` is non-empty | `internal/rooms/adjacency.go:20-50` |
| F28 | `mobs.OnSleeperWoken(c)` is a no-op for a non-mob; `Character.CancelConditionsWithFlag(flag) bool` | `internal/mobs/sleeper.go:18-21`; `internal/characters/conditions.go:33` |
| F29 | `ParticipantSight` returns `SightNone` for `Perception.State() == perception.Blinded` before reading light; tests blind with `c.Perception = characters.New().Perception` then `TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"})` | `internal/messaging/predicates.go:56-62`; `internal/actions/stolen_bauble_test.go:327-329` |
| F30 | Light fixtures: `Room.SkyLight *float64`, `Room.Lamp *int` override the biome (`rooms.go:101-102`), `rooms.SkyLightPtr`, `rooms.LampPtr`. `LightDazzleAbove` default 75, `LightDimBelow` 50, `LightBlindBelow` 25, so a lit fixture uses Lamp 60 ("faces band, no dazzle", `usercommands_test.go:203-209`). Infrared needs `InfraredVision` plus `EffectInfraReach` (`rooms/participant_sight_test.go:36-45`) | files named; `config.balance.go:1150-1158` |
| F31 | Fixtures: usercommands `seedAllRegistries` puts user 1 Aliceia and user 2 Bobrick in room 1 (`Lamp: 60`, `Biome: "city"`, `north` -> 2; room 2 `south` -> 1); mobcommands `seedAllRegistries` puts users 1, 2 and mob 100 "Skeleton" in room 1 (`Biome: "city"`, no Lamp); rooms `sightTestRoom(t, biome)` seeds 7411 Aliceia, 7412 Bobrick, 7413 Ordel in room 7410 and infrared condition `sightTestInfraredConditionId` (7401); actions `hideRhetoricActor(t, char)` hides a character | `usercommands_test.go:107-260`; `mobcommands_test.go:72-285`; `rooms/participant_sight_test.go:13-57`; `actions/rhetoric_progression_test.go:127-134` |
| F32 | Existing say, shout and emote tests assert only `handled`/`err` (`usercommands_test.go:602-681,4107-4118,7170-7183`; `mobcommands_test.go:379-418,1219`), so they stay unchanged | grep |
| F33 | Narration guard: `narrationRecognizeCall` returns observer only for `SendText`/`SendTextToUser` on a receiver named `room` and for `SendTextVisual`, `SendTextVisualHidingNames`, `SendTextVisualAsLit`, `SendTextVisualWithAudio` on `room`; a candidate event has an actor call plus exactly one of actee/observer, keyed `file|first literal`, keys collapse in a map. Registered keys this slice keeps: `usercommands/emote.go|You Emote: %s`, `|You emote.` (`:1345-1346`), `usercommands/rally.go|...You rally...`, `|...Your layered voice looses...` (`:1389-1390`), `usercommands/warcry.go|...Your layered voice weaves...`, `|...You let out a thunderous warcry...` (`:1414-1415`). Moving an observer call to an unrecognised sender turns those events actor-only and the entries stale | `messaging_surface_guard_test.go:863-927,1106-1124,1143-1199,1454-1515` |
| F34 | `bauble_finder_view_guard_test.go` `beyondReaderCalls` lists `SendTextCommunication`, the visual family, `SendRoomCommunication`, `SendTrio`, `merchantSay` and others by callee name | `bauble_finder_view_guard_test.go:89-98` |
| F35 | `m2_routing_guard_test.go:261-265` still recognises `sendAudioRoomText`; `m2RoutingFiles` is empty (`:70-101`, comment lines only). `send_trio_only_guard_test.go` names `sendAudioRoomText` at `:20,:45` and `actor_mob.go:52` at `:73` in comments | files named |
| F36 | `raw_events_message_guard_test.go` allows `events.Message{` only in `rooms.go`, `userrecord.go`, `print.go`, `admin.bauble.go` (added by slice C), `hooks.go`, `Message_SendMessages.go`. This plan's only new `events.Message{` literals are in `rooms.go` | `raw_events_message_guard_test.go:19-26` |
| F37 | `awareness.TransitionToRevealing` passes through `Revealing` to `Visible` in the same call (spec S2) | `internal/state/awareness/awareness.go:190-214` |
| F38 | Mob shout is live from `species/1-human.yaml:9-13` `angrycommands` (three `shout` lines, `:11-13`) | file named |
| F39 | `drink_wrapper_guard_test.go` is the re-fork guard template (package `main`, `os.ReadFile`, one regexp). 5a added `sight_gates_wrapper_guard_test.go` (package `main`, `TestSightGateWrappersDoNotReFork`), which drops comments the same way Task 11 does (`parser.ParseFile(..., 0)` then `printer.Fprint`); the spec names a separate 5b file, and no name in Task 11 collides with it | repo root |
| F40 | Admin tools for the playtest: `deafen <user>` (`admin.deafen.go:18-35`), `setcondition <target> <id>` (`admin.setcondition.go:48`), condition 3 Blinded lasts 3 rounds (`conditions/3-blinded.yaml`), `command <mob> <cmd>` issues a mob command (`admin.command.go:21-60`); dark cave 3101 and profiles `admin`, `m2-witness`, `slice-a-infrared` exist (`tools/playtest/scenarios/slice-a-dark-cave.yaml`, `tools/playtest/profiles/`) | files named |

## Drift found on re-verify (2026-09-30)

1. **F18, F19: a fifth `actions.Say` caller, `usercommands/offer.go` `merchantSay` (`:20-28`).** Not new drift (the file predates `7d6d4ac38`), but the first pass missed it. It double-sends once `Say` owns the room line, and Task 11's `TestSayRoomLineHasOneFormatter` would fail on it. Task 5 now thins it the same way as `actions/sell.go`; the file map, Tasks 5, 11 and 12 and "Where the spec could not be implemented" item 3 are updated.
2. **F36:** slice C added `internal/usercommands/admin.bauble.go` to `rawEventsMessageAllowed`; the map is now `:19-26`. No effect on this plan.
3. **F35, F38:** line ranges corrected (`m2RoutingFiles` spans `:70-101`; the human `angrycommands` start at `:9`). Files unchanged; the first pass cited them loosely.
4. **F39:** 5a's `sight_gates_wrapper_guard_test.go` is a second template with the same comment-dropping approach; no collision with Task 11.
5. **Task 8 Step 5:** the `SendTextCommunication(` caller check runs before Task 10, so `actions/actor_user.go:48,50` still appear; the expected list now says so.
6. **Task 12:** `internal/actions/context.md` grew 91 lines under 5a; its `Social` files row moved from `:1581` to `:1672` (`:34` and `:82-84` did not move). The patch-notes heading takes the merge date, since 5a already holds a 2026-09-29 entry at the top.
7. **Branch:** 5a merged; the conflict guidance is gone (5a did not touch `economy_test.go`, `bauble_finder_view_guard_test.go` or `messaging_surface_guard_test.go` after all).

Unchanged and re-confirmed: every quoted "replace" / "delete" block (39 of them, plus the prose anchors in Tasks 2, 3, 4 and 8), the F24 line keys (`rally.go|57,86,114`, `warcry.go|57,90,118`), the F33 registered keys, the F34 `beyondReaderCalls` map, the F7 caller list, the F20 caller list, the nine F17 fakes, every fixture in F29 to F31, and every root guard test name the plan runs.

## Where the spec could not be implemented as written

1. **One `SendTextHidingNames` cannot serve both nouns.** The spec routes mob speech ("Someone", ruling 3) and sounds ("Something", kept for rally, warcry, howl, taunt) through the same `SendTextHidingNames(cat, text, names, excl...)`. The plan adds one parameter, `hide messaging.NameHider`, so the caller picks `HideSpeakerNames` or `HideNames`. `SendCommunicationHidingNames` is speech only and keeps the spec's signature, hiding with `HideSpeakerNames`.
2. **`HideSpeakerNames` as "the same matcher" would rewrite the spoken words.** `HideNames` also replaces bare mentions, so `Kesh says, "I am Kesh"` would reach a shapes listener as `A figure says, "I am a figure"`, against ruling 3 ("the words are always heard"). `HideSpeakerNames` hides the name only where it stands as an identity tag (F9's `hideTaggedName`), which is exactly the speaker label every speech line opens with. A player's words cannot forge an identity tag (the wrappers escape them). Emotes still hide a bare own name (they go through the visual `HideNames` path, as the spec asks).
3. **`actions.Say` has five callers, not two (F18).** The plan moves the justice guard speech, the merchant refusal (`actions/sell.go`) and the offer/appraise merchant line (`usercommands/offer.go`) onto the shared room line too, so they gain the three tiers.
4. **Deafen is not observable in a test (F16).** The plan adds `events.Message.HiddenFromDeafened(deafened bool) bool`, makes the hook call it, and adds two queue-drain helpers, so the per-listener table asserts the real rule.
5. **The player's adjacent-room shout line keeps `SendTextCommunication`** so it stays byte-identical and deafen-filtered; the mob's keeps `SendText(CategoryShout, ...)` and gains the words. The spec's "one line for both" is kept as one format string with the speaker's words colour.

## Player-visible lines that change (everything else stays byte-identical)

Room lines only; no self line changes.

| Line | Before | After |
|---|---|---|
| Player say, shout (room) | Named to every listener in any light, raw text, no category colour | Clear sight: same text, now inside the `speech` / `shout` category colour (F14). Shapes: `A figure says, "..."`. No sight or blinded in a lit room: `Someone says, "..."` (`Someone` in `combat-anon` colour) |
| Mob say, shout (room) | Lit room: named to all, blinded included. Dark: SightFull named, else `someone says, "..."` / `someone shouts, "..."` (lowercase, no tag) | Clear: named. Shapes: `A figure says, "..."`. No sight or blinded: `Someone says, "..."` |
| Mob shout, adjacent room | `Someone is shouting from the <exit> direction.` | `Someone shouts from the <exit> direction, "<words>"` (words in `saytext-mob`) |
| Player rally, warcry, and both Resonant Larynx fold lines | Visual: nothing for a listener who cannot see | Heard: `Something rallies everyone with an inspiring shout!`, `Something lets out a thunderous warcry!`, `Something's shout hardens into a warcry in the same breath!`, `Something's shout gathers into a rally in the same breath!` at no sight; shapes unchanged (`A figure ...`) |
| Mob rally, warcry | Lit room named to all; dark shapes read `Something lets out a rallying roar!` / `...bone-shaking warcry!` | Shapes: `A figure lets out a rallying roar!` / `A figure lets out a bone-shaking warcry!`; no sight or blinded: `Something ...` (word now in `combat-anon` colour) |
| Mob howl fumble | Lit: named. Dark: `Something lets out a pitiful howl that trails off weakly.` | Shapes `A figure lets out ...`; no sight `Something lets out ...` |
| Mob howl aggro-pull | Dark: `Something turns, drawn snarling toward a new foe.` | Shapes `A figure turns from its prey and snarls at a figure!`; no sight `Something turns from its prey and snarls at something!` |
| Mob taunt fumble fallback (store empty) | Dark: `Something bellows a challenge that breaks into a strangled gasp.` | Shapes `A figure bellows ...`; no sight `Something bellows ...` |
| Mob taunt hit fallback | Dark: `Something bellows a thunderous challenge at a figure!` | Shapes `A figure bellows a thunderous challenge at a figure!`; no sight `Something bellows a thunderous challenge at something!` |
| Mob taunt aggro-pull | Dark: `Something wheels around, drawn to a new challenger.` | Shapes `A figure wheels around and locks onto a figure!`; no sight `Something wheels around and locks onto something!` |
| Mob taunt miss fallback | Dark: `Something bellows a challenge at a figure, but they shrug it off.` | Shapes `A figure bellows a challenge at a figure, but they shrug it off.`; no sight `Something bellows a challenge at something, but they shrug it off.` |
| Every howl, taunt and mob rally/warcry line | Lit-room shortcut named the mob to a blinded listener | Blinded listener reads `Something` |
| Player free-form and `@` emote | Named to every listener in any light, no category colour | Clear sight named, in the `emote` colour; shapes `A figure ...`; no sight or blinded: nothing; deafened: nothing (unchanged) |
| Every emote form, player and mob | A bare mention of the emoter's own name in the text read through at shapes | Hidden: `A figure waves, and a figure grins.` |

## File map

| File | Change |
|---|---|
| `internal/messaging/hidenames.go` | `NameHider`, `HideSpeakerNames`, `speakerNoun`, `longestFirst` (shared with `HideNames`) |
| `internal/messaging/hidenames_speaker_test.go` | Create |
| `internal/events/eventtypes.go` | `Message.HiddenFromDeafened` |
| `internal/events/events.go` | `DrainQueuedMessageEventsForTest`, `DrainQueuedRoomMessagesForTest` |
| `internal/events/message_deafen_test.go` | Create |
| `internal/hooks/Message_SendMessages.go` | Both deafen checks call `HiddenFromDeafened`; comment |
| `internal/rooms/rooms.go` | `SendTextHidingNames`, `SendCommunicationHidingNames`, `SendVisualCommunicationHidingNames`, `sendAudioHidingNames`; `sendTextVisualJudgedBy` gains `communication bool`; `SendTextCommunication` doc |
| `internal/rooms/hiding_senders_test.go` | Create |
| `internal/actions/room_lines.go` | Create: `SendHeard`, `SendSeen`, `sendSpoken` |
| `internal/actions/say.go` | `Say` sends the room line |
| `internal/actions/shout.go` | Create: `ShoutResult`, `Shout`, `wakeSleepers` |
| `internal/actions/speech_sight_test.go` | Create: the per-listener table and the shout, reveal, wake tests |
| `internal/actions/sell.go` | `merchantSay` drops its own send |
| `internal/usercommands/offer.go` | Its package-local `merchantSay` drops its own send |
| `internal/actions/actor.go`, `actor_user.go`, `actor_mob.go` | Delete `SendRoomCommunication` |
| `internal/actions/{consider,economy,forage,salvage,scan,search,sleep,track}_test.go`, `internal/hooks/spell_foldanchor_test.go` | Delete the fake `SendRoomCommunication` |
| `internal/hooks/justice_wiring.go` | Drops its own send |
| `internal/usercommands/say.go`, `shout.go`, `emote.go`, `rally.go`, `warcry.go` | Thin wrappers |
| `internal/usercommands/speech_sight_wrapper_test.go` | Create |
| `internal/mobcommands/say.go`, `shout.go`, `emote.go`, `rally.go`, `warcry.go`, `howl.go`, `taunt.go` | Thin wrappers; howl and taunt on `SendTextHidingNames` |
| `internal/mobcommands/darkness.go` | Delete |
| `internal/mobcommands/audio_room_text_sight_test.go` | Delete (ported into the next file) |
| `internal/mobcommands/speech_sight_test.go` | Create |
| `internal/mobcommands/go.go`, `taunt_store_test.go` | Comments |
| `messaging_surface_guard_test.go` | Recognise the three new room senders and `actions.SendHeard`/`SendSeen` |
| `bauble_finder_view_guard_test.go` | `beyondReaderCalls` gains five names, loses `SendRoomCommunication` |
| `m2_routing_guard_test.go` | Drop the dead `sendAudioRoomText` recogniser |
| `send_trio_only_guard_test.go` | Comments |
| `speech_wrapper_guard_test.go` | Create (repo root) |
| `context.md` in `internal/actions`, `internal/rooms`, `internal/messaging`, `internal/events`, `internal/hooks`, `internal/mobcommands`, `internal/usercommands` | Update |
| `docs/PATCH_NOTES.md`, `docs/README.md` | Entry; row for the new guard |

---

### Task 1: `messaging.HideSpeakerNames` and `NameHider`

**Model:** haiku (mechanical, code given).

**Files:**
- Modify: `internal/messaging/hidenames.go`
- Create: `internal/messaging/hidenames_speaker_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/messaging/hidenames_speaker_test.go`:

```go
package messaging

import "testing"

// Both hiders fit the one type the room senders take.
var (
	_ NameHider = HideNames
	_ NameHider = HideSpeakerNames
)

const speakerLine = `<ansi fg="username">Kesh</ansi> says, "<ansi fg="saytext">I am Kesh</ansi>"`

func TestHideSpeakerNames_ClearSightKeepsTheName(t *testing.T) {
	if got := HideSpeakerNames(speakerLine, []string{"Kesh"}, SightFull); got != speakerLine {
		t.Fatalf("clear sight changed the line: %q", got)
	}
}

func TestHideSpeakerNames_ShapesHearAFigure(t *testing.T) {
	want := `<ansi fg="combat-anon">A figure</ansi> says, "<ansi fg="saytext">I am Kesh</ansi>"`
	if got := HideSpeakerNames(speakerLine, []string{"Kesh"}, SightShapes); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// Ruling 3: an unseen speaker is "Someone", and the words arrive untouched,
// even when they say the speaker's own name.
func TestHideSpeakerNames_NoSightHearsSomeoneAndEveryWord(t *testing.T) {
	want := `<ansi fg="combat-anon">Someone</ansi> says, "<ansi fg="saytext">I am Kesh</ansi>"`
	if got := HideSpeakerNames(speakerLine, []string{"Kesh"}, SightNone); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestHideSpeakerNames_MobTagWithDuplicateIndex(t *testing.T) {
	line := `<ansi fg="mobname">guard #2</ansi> shouts, "<ansi fg="saytext-mob">HALT</ansi>"`
	want := `<ansi fg="combat-anon">Someone</ansi> shouts, "<ansi fg="saytext-mob">HALT</ansi>"`
	if got := HideSpeakerNames(line, []string{"guard"}, SightNone); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestHideSpeakerNames_NoNameIsIgnored(t *testing.T) {
	if got := HideSpeakerNames(speakerLine, []string{NoName}, SightNone); got != speakerLine {
		t.Fatalf("NoName hid something: %q", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/messaging/ -run 'TestHideSpeakerNames' -count=1`
Expected: FAIL to compile, `undefined: NameHider` and `undefined: HideSpeakerNames`.

- [ ] **Step 3: Implement**

In `internal/messaging/hidenames.go`, replace the body of `HideNames` so its name ordering is shared, and add the new symbols after it. Replace:

```go
	word := UnseenNoun(d)
	ordered := make([]string, 0, len(names))
	for _, n := range names {
		if n != NoName {
			ordered = append(ordered, n)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, name := range ordered {
		text = hideTaggedName(text, name, word)
		text = hideOneName(text, name, word)
	}
	return text
}
```

with:

```go
	word := UnseenNoun(d)
	for _, name := range longestFirst(names) {
		text = hideTaggedName(text, name, word)
		text = hideOneName(text, name, word)
	}
	return text
}

// NameHider is the shape HideNames and HideSpeakerNames share: rewrite text
// so a reader at d cannot tell who names are. The room senders that hide
// names on the audio channel take one, so a sound ("Something lets out a
// roar!") and a speaker ("Someone says, ...") share one delivery path.
type NameHider func(text string, names []string, d SightDecision) string

// speakerNoun is what a listener at d calls a speaker it cannot make out: "a
// figure" at shapes, "someone" otherwise. A voice belongs to a person, so the
// unseen word is "someone" where UnseenNoun says "something" (owner ruling 3,
// sight gates slice 5b).
func speakerNoun(d SightDecision) string {
	if d == SightShapes {
		return "a figure"
	}
	return "someone"
}

// HideSpeakerNames hides a speaker's name in a speech line from a listener at
// d: each of names, where it stands as a whole identity tag, becomes "a
// figure" or "someone", capitalised at a sentence start. Clear sight returns
// text unchanged.
//
// Only the tagged name goes, never a bare mention. Every speech line opens
// with the speaker's name as an identity tag (actions.FormatSayText and
// actions.Shout tag it), and the spoken words must arrive untouched (owner
// ruling 3): "I am Kesh" stays "I am Kesh". A player's words cannot forge an
// identity tag, because the wrappers escape them (util.EscapeAnsiTags).
func HideSpeakerNames(text string, names []string, d SightDecision) string {
	if d == SightFull || text == "" {
		return text
	}
	word := speakerNoun(d)
	for _, name := range longestFirst(names) {
		text = hideTaggedName(text, name, word)
	}
	return text
}

// longestFirst is names without NoName, longest first, so a longer name is
// hidden before a shorter one it contains.
func longestFirst(names []string) []string {
	ordered := make([]string, 0, len(names))
	for _, n := range names {
		if n != NoName {
			ordered = append(ordered, n)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	return ordered
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/messaging/ -count=1`
Expected: `ok` (the new tests pass and every existing `HideNames` test still passes).

- [ ] **Step 5: Commit**

```bash
git add internal/messaging/hidenames.go internal/messaging/hidenames_speaker_test.go
git commit -m "feat(messaging): HideSpeakerNames hides an unseen speaker as a figure or someone"
```

---

### Task 2: The deafen rule as one method, and queue helpers that keep the flag

**Model:** haiku (mechanical, code given).

**Files:**
- Modify: `internal/events/eventtypes.go`, `internal/events/events.go`, `internal/hooks/Message_SendMessages.go`
- Create: `internal/events/message_deafen_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/events/message_deafen_test.go`:

```go
package events

import "testing"

func TestMessageHiddenFromDeafened(t *testing.T) {
	cases := []struct {
		communication, deafened, want bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}
	for _, c := range cases {
		m := Message{IsCommunication: c.communication}
		if got := m.HiddenFromDeafened(c.deafened); got != c.want {
			t.Errorf("IsCommunication=%v deafened=%v: got %v, want %v", c.communication, c.deafened, got, c.want)
		}
	}
}

func TestDrainQueuedMessageHelpersKeepTheWholeMessage(t *testing.T) {
	DrainQueuedMessageEventsForTest(99001)
	DrainQueuedRoomMessagesForTest(99002)
	AddToQueue(Message{UserId: 99001, Text: "to a user", IsCommunication: true})
	AddToQueue(Message{RoomId: 99002, Text: "to a room", IsCommunication: true})

	got := DrainQueuedMessageEventsForTest(99001)
	if len(got) != 1 || got[0].Text != "to a user" || !got[0].IsCommunication {
		t.Fatalf("user drain = %+v, want the one flagged message", got)
	}
	room := DrainQueuedRoomMessagesForTest(99002)
	if len(room) != 1 || room[0].Text != "to a room" || !room[0].IsCommunication {
		t.Fatalf("room drain = %+v, want the one flagged message", room)
	}
	if again := DrainQueuedMessageEventsForTest(99001); len(again) != 0 {
		t.Fatalf("drain did not remove: %+v", again)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/events/ -run 'TestMessageHiddenFromDeafened|TestDrainQueuedMessageHelpers' -count=1`
Expected: FAIL to compile, `m.HiddenFromDeafened undefined`, `undefined: DrainQueuedMessageEventsForTest`.

- [ ] **Step 3: Implement**

In `internal/events/eventtypes.go`, after `func (m Message) Type() string { return `Message` }`, add:

```go

// HiddenFromDeafened reports whether a listener under the Deafened moderation
// flag must not receive m: it is player chatter (IsCommunication) and they
// are deafened. The one statement of the rule; hooks/Message_SendMessages.go
// applies it on both of its branches and tests read it rather than copy it.
func (m Message) HiddenFromDeafened(deafened bool) bool {
	return m.IsCommunication && deafened
}
```

In `internal/events/events.go`, after `DrainQueuedMessagesForTest`, add:

```go

// DrainQueuedMessageEventsForTest is DrainQueuedMessagesForTest returning the
// whole Message, so a test can read IsCommunication as well as the text.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedMessageEventsForTest(userId int) []Message {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Message
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		msg, ok := pe.event.(Message)
		if ok && msg.UserId == userId {
			found = append(found, msg)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedRoomMessagesForTest removes every RoomId-keyed Message queued for
// roomId (Room.SendTextCommunication, Room.SendTextToExits) and returns them.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedRoomMessagesForTest(roomId int) []Message {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Message
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		msg, ok := pe.event.(Message)
		if ok && msg.UserId == 0 && msg.RoomId == roomId {
			found = append(found, msg)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}
```

In `internal/hooks/Message_SendMessages.go`, replace both occurrences (lines 29 and 83):

```go
			if message.IsCommunication && user.Deafened {
```

and

```go
				if message.IsCommunication && user.Deafened {
```

with, respectively:

```go
			if message.HiddenFromDeafened(user.Deafened) {
```

and

```go
				if message.HiddenFromDeafened(user.Deafened) {
```

(The comment block below them is rewritten in Task 8, once the player speech it describes has moved.)

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/events/ ./internal/hooks/ -count=1`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add internal/events/eventtypes.go internal/events/events.go internal/events/message_deafen_test.go internal/hooks/Message_SendMessages.go
git commit -m "refactor(events): the deafen rule is Message.HiddenFromDeafened; drain helpers keep the flag"
```

---

### Task 3: The three name-hiding room senders

**Model:** sonnet (touches the render pipeline and two guards).

**Files:**
- Modify: `internal/rooms/rooms.go`, `messaging_surface_guard_test.go`, `bauble_finder_view_guard_test.go`
- Create: `internal/rooms/hiding_senders_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/rooms/hiding_senders_test.go` (it reuses `sightTestRoom`, `sightTestTag` and `sightTestInfraredConditionId` from `participant_sight_test.go`):

```go
package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func hidingSenderText(m events.Message) string {
	return strings.TrimSpace(sightTestTag.ReplaceAllString(m.Text, ""))
}

// hidingSenderLit lights the fixture room exactly (no sky, a lamp in the faces
// band) and blinds Ordel (7413) with the perception machine.
func hidingSenderLit(t *testing.T) *Room {
	t.Helper()
	r := sightTestRoom(t, "city")
	r.SkyLight, r.Lamp = SkyLightPtr(0), LampPtr(60)
	ordel := users.GetByUserId(7413).Character
	ordel.Perception = characters.New().Perception
	require.NoError(t, ordel.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.Equal(t, messaging.SightFull, r.ParticipantSight(7412))
	require.Equal(t, messaging.SightNone, r.ParticipantSight(7413))
	return r
}

// hidingSenderDark is the unlit fixture with Bobrick (7412) on infrared.
func hidingSenderDark(t *testing.T) *Room {
	t.Helper()
	r := sightTestRoom(t, "cave")
	require.True(t, users.GetByUserId(7412).Character.Conditions.AddCondition(sightTestInfraredConditionId, true))
	require.Equal(t, messaging.SightShapes, r.ParticipantSight(7412))
	require.Equal(t, messaging.SightNone, r.ParticipantSight(7413))
	return r
}

func TestSendTextHidingNames_HeardByAllNamedByNone(t *testing.T) {
	r := hidingSenderDark(t)
	r.SendTextHidingNames(messaging.CategoryRally, `<ansi fg="username">Aliceia</ansi> lets out a roar!`,
		[]string{"Aliceia"}, messaging.HideNames, 7411)

	shapes := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, shapes, 1)
	require.Equal(t, "A figure lets out a roar!", hidingSenderText(shapes[0]))
	require.False(t, shapes[0].IsCommunication, "an authored sound is never deafen-filtered")

	none := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, none, 1, "audio reaches a listener who sees nothing")
	require.Equal(t, "Something lets out a roar!", hidingSenderText(none[0]))

	require.Empty(t, events.DrainQueuedMessageEventsForTest(7411), "the excluded maker reads nothing")
}

// The defect the deleted mobcommands.sendAudioRoomText carried: a lit room
// named the speaker to a blinded listener.
func TestSendTextHidingNames_NoLitRoomShortcut(t *testing.T) {
	r := hidingSenderLit(t)
	r.SendTextHidingNames(messaging.CategorySpeech,
		`<ansi fg="mobname">Grel</ansi> says, "<ansi fg="saytext-mob">hello</ansi>"`,
		[]string{"Grel"}, messaging.HideSpeakerNames)

	lit := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, lit, 1)
	require.Equal(t, `Grel says, "hello"`, hidingSenderText(lit[0]))
	blind := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, blind, 1)
	require.Equal(t, `Someone says, "hello"`, hidingSenderText(blind[0]))
}

func TestSendCommunicationHidingNames_IsPlayerChatter(t *testing.T) {
	r := hidingSenderDark(t)
	r.SendCommunicationHidingNames(messaging.CategorySpeech,
		`<ansi fg="username">Aliceia</ansi> says, "<ansi fg="saytext">I am Aliceia</ansi>"`,
		[]string{"Aliceia"}, 7411)

	shapes := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, shapes, 1)
	require.Equal(t, `A figure says, "I am Aliceia"`, hidingSenderText(shapes[0]), "the words arrive untouched")
	require.True(t, shapes[0].IsCommunication)
	none := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, none, 1)
	require.Equal(t, `Someone says, "I am Aliceia"`, hidingSenderText(none[0]))
	require.True(t, none[0].IsCommunication)
	require.Empty(t, events.DrainQueuedMessageEventsForTest(7411))
}

func TestSendVisualCommunicationHidingNames_SeenAndMarked(t *testing.T) {
	r := hidingSenderDark(t)
	r.SendVisualCommunicationHidingNames(messaging.CategoryEmote,
		`<ansi fg="username">Aliceia</ansi> <ansi fg="137">waves, and Aliceia grins.</ansi>`,
		[]string{"Aliceia"}, 7411)

	shapes := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, shapes, 1)
	require.Equal(t, "A figure waves, and a figure grins.", hidingSenderText(shapes[0]))
	require.True(t, shapes[0].IsCommunication)
	require.Empty(t, events.DrainQueuedMessageEventsForTest(7413), "an emote is seen, not heard")
}

func TestVisualSendersStayOffTheDeafenFilter(t *testing.T) {
	r := sightTestRoom(t, "city")
	r.SkyLight, r.Lamp = SkyLightPtr(0), LampPtr(60)
	require.Equal(t, messaging.SightFull, r.ParticipantSight(7413))
	sends := map[string]func(){
		"SendTextVisual": func() { r.SendTextVisual(messaging.CategoryEmote, "Aliceia waves.", 7411) },
		"SendTextVisualHidingNames": func() {
			r.SendTextVisualHidingNames(messaging.CategoryEmote, "Aliceia waves.", []string{"Aliceia"}, 7411)
		},
		"SendTextVisualAsLit": func() { r.SendTextVisualAsLit(messaging.CategoryEmote, "Aliceia waves.", 7411) },
		"SendTextVisualAsLitHidingNames": func() {
			r.SendTextVisualAsLitHidingNames(messaging.CategoryEmote, "Aliceia waves.", []string{"Aliceia"}, 7411)
		},
	}
	for name, send := range sends {
		send()
		msgs := events.DrainQueuedMessageEventsForTest(7413)
		require.Len(t, msgs, 1, name)
		require.False(t, msgs[0].IsCommunication, "%s must not be deafen-filtered", name)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/rooms/ -run 'HidingNames|VisualSenders' -count=1`
Expected: FAIL to compile, `r.SendTextHidingNames undefined` (and the other two senders).

- [ ] **Step 3: Implement in `internal/rooms/rooms.go`**

(`SendTextCommunication`'s doc comment is rewritten in Task 8, once its callers have moved.)

a) Replace:

```go
func (r *Room) SendTextVisual(cat messaging.Category, txt string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(r, cat, txt, nil, excludeUserIds...)
}
```

with:

```go
func (r *Room) SendTextVisual(cat messaging.Category, txt string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(r, cat, txt, nil, false, excludeUserIds...)
}
```

b) Replace:

```go
func (r *Room) SendTextVisualHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(r, cat, txt, names, excludeUserIds...)
}
```

with:

```go
func (r *Room) SendTextVisualHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(r, cat, txt, names, false, excludeUserIds...)
}

// SendVisualCommunicationHidingNames is SendTextVisualHidingNames for a
// player's free text that is seen rather than heard (a free-form emote): the
// same sight gate and name hiding, with every message marked IsCommunication
// so the Deafened moderation filter spares a deafened player. Only the shared
// bodies in internal/actions call it (speech_wrapper_guard_test.go).
func (r *Room) SendVisualCommunicationHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(r, cat, txt, names, true, excludeUserIds...)
}

// SendTextHidingNames is SendText for an authored line that names who made
// it (a rally, a howl, an NPC's speech): heard by everyone in the room
// whatever they can see, with each of names hidden per listener by hide at
// that listener's messaging.ParticipantSight. hide is messaging.HideNames
// for a sound ("Something lets out a roar!") and messaging.HideSpeakerNames
// for speech ("Someone says, ..."). There is no lit-room shortcut, which is
// how the deleted mobcommands.sendAudioRoomText named a speaker to a blinded
// listener. Never deafen-filtered: NPC lines are authored content (owner
// ruling 6, sight gates slice 5b).
func (r *Room) SendTextHidingNames(cat messaging.Category, txt string, names []string, hide messaging.NameHider, excludeUserIds ...int) {
	r.sendAudioHidingNames(cat, txt, names, hide, false, excludeUserIds...)
}

// SendCommunicationHidingNames is a player's speech to the room: every
// listener hears the words, the speaker's name hidden by
// messaging.HideSpeakerNames at that listener's sight, and every message is
// marked IsCommunication so the Deafened moderation filter still applies.
// Only the shared bodies in internal/actions call it
// (speech_wrapper_guard_test.go).
func (r *Room) SendCommunicationHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
	r.sendAudioHidingNames(cat, txt, names, messaging.HideSpeakerNames, true, excludeUserIds...)
}

// sendAudioHidingNames is the one delivery path of the two audio senders
// above: per listener, hide the names at that listener's sight, render on the
// audio channel (category colour, normalize and wrap, no sight gate), and
// queue a per-user Message carrying the communication flag.
func (r *Room) sendAudioHidingNames(cat messaging.Category, txt string, names []string, hide messaging.NameHider, communication bool, excludeUserIds ...int) {
	for _, uid := range r.GetPlayers() {
		if excluded(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		rendered := messaging.RenderForRecipient(messaging.RenderInput{
			Category:  cat,
			Text:      hide(txt, names, messaging.ParticipantSight(u.Character, r)),
			Channel:   messaging.ChannelAudio,
			LineWidth: u.GetLineWidth(),
		})
		if rendered == "" {
			continue
		}
		events.AddToQueue(events.Message{
			UserId:          u.UserId,
			Text:            rendered + "\n",
			IsCommunication: communication,
		})
	}
}
```

c) In `SendTextVisualAsLit` and `SendTextVisualAsLitHidingNames`, change `r.sendTextVisualJudgedBy(litRoom{}, cat, txt, nil, excludeUserIds...)` to `r.sendTextVisualJudgedBy(litRoom{}, cat, txt, nil, false, excludeUserIds...)` and `r.sendTextVisualJudgedBy(litRoom{}, cat, txt, names, excludeUserIds...)` to `r.sendTextVisualJudgedBy(litRoom{}, cat, txt, names, false, excludeUserIds...)`.

d) Replace the `sendTextVisualJudgedBy` doc and signature:

```go
// sendTextVisualJudgedBy is SendTextVisual with the lighting it judges sight
// against passed in, so SendTextVisualAsLit shares one delivery path. names,
// when given, are hidden from an observer who makes out shapes only, including
// bare names Anonymize cannot see.
func (r *Room) sendTextVisualJudgedBy(lighting messaging.RoomVisibility, cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
```

with:

```go
// sendTextVisualJudgedBy is SendTextVisual with the lighting it judges sight
// against passed in, so SendTextVisualAsLit shares one delivery path. names,
// when given, are hidden from an observer who makes out shapes only, including
// bare names Anonymize cannot see. communication marks every message as player
// chatter for the Deafened filter; only SendVisualCommunicationHidingNames
// sets it.
func (r *Room) sendTextVisualJudgedBy(lighting messaging.RoomVisibility, cat messaging.Category, txt string, names []string, communication bool, excludeUserIds ...int) {
```

and in the same function replace:

```go
		events.AddToQueue(events.Message{
			UserId: u.UserId,
			Text:   rendered + "\n",
		})
	}
}

// SendTextVisualWithAudio delivers visualTxt to everyone who can see and
```

with:

```go
		events.AddToQueue(events.Message{
			UserId:          u.UserId,
			Text:            rendered + "\n",
			IsCommunication: communication,
		})
	}
}

// SendTextVisualWithAudio delivers visualTxt to everyone who can see and
```

- [ ] **Step 4: Teach the two root guards the new senders**

In `messaging_surface_guard_test.go` replace:

```go
		case "SendTextVisual", "SendTextVisualHidingNames",
			"SendTextVisualAsLit", "SendTextVisualWithAudio":
```

with:

```go
		case "SendTextVisual", "SendTextVisualHidingNames",
			"SendTextVisualAsLit", "SendTextVisualWithAudio",
			// Sight gates slice 5b: the name-hiding room senders, audio and
			// visual, are room broadcasts too.
			"SendTextHidingNames", "SendCommunicationHidingNames",
			"SendVisualCommunicationHidingNames":
```

In `bauble_finder_view_guard_test.go` replace:

```go
	"SendTextVisualAsLit": true, "SendTextVisualAsLitHidingNames": true, "SendTextVisualWithAudio": true,
```

with:

```go
	"SendTextVisualAsLit": true, "SendTextVisualAsLitHidingNames": true, "SendTextVisualWithAudio": true,
	"SendTextHidingNames": true, "SendCommunicationHidingNames": true, "SendVisualCommunicationHidingNames": true,
```

- [ ] **Step 5: Run the tests**

Run:
```bash
gofmt -l internal/rooms/ .
go test ./internal/rooms/ -count=1
go test . -run 'TestNarrationSitesMatchViewpointAudit|TestFinderView|TestNoRawEventsMessageOutsidePipeline' -count=1
```
Expected: gofmt prints nothing; both `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/rooms/rooms.go internal/rooms/hiding_senders_test.go messaging_surface_guard_test.go bauble_finder_view_guard_test.go
git commit -m "feat(rooms): name-hiding audio senders and a deafen-marked visual sender"
```

---

### Task 4: `actions.SendHeard`, `actions.SendSeen`, `sendSpoken` and the per-listener table

**Model:** sonnet (the table is the slice's main proof).

**Files:**
- Create: `internal/actions/room_lines.go`, `internal/actions/speech_sight_test.go`
- Modify: `messaging_surface_guard_test.go`, `bauble_finder_view_guard_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/actions/speech_sight_test.go`:

```go
package actions

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The sight gates 5b per-listener table. Five listeners, each verdict forced
// exactly (a lamp and the perception machine, never a roll):
//
//	clear   lit room, plain eyes            SightFull
//	blind   lit room, Perception Blinded    SightNone
//	deaf    lit room, plain eyes, Deafened  SightFull
//	shapes  dark room, infrared             SightShapes
//	dark    dark room, plain eyes           SightNone
//
// The speaker is Kesh (a player) or Grel (a mob). Every line is sent twice,
// once from the lit room and once from the dark room.
const (
	speechSpeakerId = 8851
	speechClearId   = 8852
	speechBlindId   = 8853
	speechDeafId    = 8854
	speechShapesId  = 8855
	speechDarkId    = 8856
	speechNextId    = 8857
	speechMobInst   = 98851
	speechInfraCond = 8861
	speechSleepCond = 8862
	speechLitRoom   = 8870
	speechDarkRoom  = 8871
	speechNextRoom  = 8872
)

var speechTag = regexp.MustCompile(`<[^>]*>`)

type speechScene struct {
	lit, dark, next *rooms.Room
	player          *users.UserRecord
	mob             *mobs.Mob
}

func newSpeechScene(t *testing.T) *speechScene {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		speechInfraCond: {ConditionId: speechInfraCond, Name: "Test Heat Eyes",
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		speechSleepCond: {ConditionId: speechSleepCond, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	player := users.NewTestUser(speechSpeakerId, "kesh", "Kesh", 98851)
	people := map[int]*users.UserRecord{
		speechSpeakerId: player,
		speechClearId:   users.NewTestUser(speechClearId, "clara", "Clara", 98852),
		speechBlindId:   users.NewTestUser(speechBlindId, "bram", "Bram", 98853),
		speechDeafId:    users.NewTestUser(speechDeafId, "dena", "Dena", 98854),
		speechShapesId:  users.NewTestUser(speechShapesId, "sorn", "Sorn", 98855),
		speechDarkId:    users.NewTestUser(speechDarkId, "dusk", "Dusk", 98856),
		speechNextId:    users.NewTestUser(speechNextId, "nell", "Nell", 98857),
	}
	t.Cleanup(users.SeedUsersForTest(people))

	people[speechDeafId].Deafened = true
	blind := people[speechBlindId].Character
	blind.Perception = characters.New().Perception
	require.NoError(t, blind.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.True(t, people[speechShapesId].Character.Conditions.AddCondition(speechInfraCond, true))

	lit := &rooms.Room{RoomId: speechLitRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60),
		Exits: map[string]exit.RoomExit{"north": {RoomId: speechNextRoom}}}
	dark := &rooms.Room{RoomId: speechDarkRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0),
		Exits: map[string]exit.RoomExit{"up": {RoomId: speechNextRoom}}}
	next := &rooms.Room{RoomId: speechNextRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60),
		Exits: map[string]exit.RoomExit{"south": {RoomId: speechLitRoom}, "down": {RoomId: speechDarkRoom}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{speechLitRoom: lit, speechDarkRoom: dark, speechNextRoom: next},
		map[string]*rooms.ZoneConfig{}))
	place := func(r *rooms.Room, ids ...int) {
		for _, id := range ids {
			people[id].Character.RoomId = r.RoomId
			r.AddPlayer(id)
		}
	}
	place(lit, speechClearId, speechBlindId, speechDeafId)
	place(dark, speechShapesId, speechDarkId)
	place(next, speechNextId)

	// Forced, not rolled: pin every verdict before any line is sent.
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(people[speechClearId].Character, lit))
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(people[speechDeafId].Character, lit))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(blind, lit))
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(people[speechShapesId].Character, dark))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(people[speechDarkId].Character, dark))

	mobChar := characters.New()
	mobChar.Name = "Grel"
	sc := &speechScene{lit: lit, dark: dark, next: next, player: player,
		mob: &mobs.Mob{InstanceId: speechMobInst, Character: *mobChar}}
	sc.drain()
	return sc
}

// speaker stands the player or the mob in r and returns it as an Actor.
func (sc *speechScene) speaker(player bool, r *rooms.Room) Actor {
	if player {
		for _, old := range []*rooms.Room{sc.lit, sc.dark, sc.next} {
			old.RemovePlayer(speechSpeakerId)
		}
		sc.player.Character.RoomId = r.RoomId
		r.AddPlayer(speechSpeakerId)
		return &UserActor{User: sc.player, Room: r}
	}
	sc.mob.Character.RoomId = r.RoomId
	return &MobActor{Mob: sc.mob, Room: r}
}

func (sc *speechScene) drain() {
	for _, id := range []int{speechSpeakerId, speechClearId, speechBlindId, speechDeafId, speechShapesId, speechDarkId, speechNextId} {
		events.DrainQueuedMessageEventsForTest(id)
	}
	for _, id := range []int{speechLitRoom, speechDarkRoom, speechNextRoom} {
		events.DrainQueuedRoomMessagesForTest(id)
	}
}

// speechHeard is what uid's client prints: its queued lines minus those the
// deafen filter drops (events.Message.HiddenFromDeafened, the rule
// hooks/Message_SendMessages.go applies), tags stripped.
func speechHeard(t *testing.T, uid int) []string {
	t.Helper()
	u := users.GetByUserId(uid)
	require.NotNil(t, u)
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		if m.HiddenFromDeafened(u.Deafened) {
			continue
		}
		out = append(out, strings.TrimSpace(speechTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

func speechExpectOne(t *testing.T, who string, got []string, want string) {
	t.Helper()
	if want == "" {
		require.Empty(t, got, "%s should read nothing", who)
		return
	}
	require.Equal(t, []string{want}, got, "%s", who)
}

// speechWant is what each listener reads; "" means nothing at all.
type speechWant struct {
	clear, blind, deaf, shapes, dark string
}

// checkSpeech sends from the lit room (clear, blind and deaf listen), then
// from the dark room (shapes and dark listen), and checks every listener.
// A player speaker never reads its own room line.
func checkSpeech(t *testing.T, sc *speechScene, player bool, send func(Actor), want speechWant) {
	t.Helper()
	sc.drain()
	send(sc.speaker(player, sc.lit))
	speechExpectOne(t, "clear", speechHeard(t, speechClearId), want.clear)
	speechExpectOne(t, "blind", speechHeard(t, speechBlindId), want.blind)
	speechExpectOne(t, "deaf", speechHeard(t, speechDeafId), want.deaf)
	require.Empty(t, speechHeard(t, speechSpeakerId), "the speaker reads its own room line")
	sc.drain()
	send(sc.speaker(player, sc.dark))
	speechExpectOne(t, "shapes", speechHeard(t, speechShapesId), want.shapes)
	speechExpectOne(t, "dark", speechHeard(t, speechDarkId), want.dark)
	require.Empty(t, speechHeard(t, speechSpeakerId), "the speaker reads its own room line")
	sc.drain()
}

// Rally and warcry are heard: the exact room lines the four wrappers pass
// to SendHeard (speech_wrapper_guard_test.go pins that they do). Authored
// text, so the deafened listener hears them from either speaker.
func TestSendHeard_RallyAndWarcry(t *testing.T) {
	cases := []struct {
		name   string
		player bool
		cat    messaging.Category
		line   string
		want   speechWant
	}{
		{"player rally", true, messaging.CategoryRally,
			fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi> rallies everyone with an inspiring shout!</ansi>`, "Kesh"),
			speechWant{"Kesh rallies everyone with an inspiring shout!", "Something rallies everyone with an inspiring shout!",
				"Kesh rallies everyone with an inspiring shout!", "A figure rallies everyone with an inspiring shout!",
				"Something rallies everyone with an inspiring shout!"}},
		{"mob rally", false, messaging.CategoryRally,
			fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="mobname">%s</ansi> lets out a rallying roar!</ansi>`, "Grel"),
			speechWant{"Grel lets out a rallying roar!", "Something lets out a rallying roar!",
				"Grel lets out a rallying roar!", "A figure lets out a rallying roar!", "Something lets out a rallying roar!"}},
		{"player warcry", true, messaging.CategoryWarcry,
			fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi> lets out a thunderous warcry!</ansi>`, "Kesh"),
			speechWant{"Kesh lets out a thunderous warcry!", "Something lets out a thunderous warcry!",
				"Kesh lets out a thunderous warcry!", "A figure lets out a thunderous warcry!", "Something lets out a thunderous warcry!"}},
		{"mob warcry", false, messaging.CategoryWarcry,
			fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="mobname">%s</ansi> lets out a bone-shaking warcry!</ansi>`, "Grel"),
			speechWant{"Grel lets out a bone-shaking warcry!", "Something lets out a bone-shaking warcry!",
				"Grel lets out a bone-shaking warcry!", "A figure lets out a bone-shaking warcry!", "Something lets out a bone-shaking warcry!"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := newSpeechScene(t)
			checkSpeech(t, sc, c.player, func(a Actor) { SendHeard(a, c.cat, c.line) }, c.want)
		})
	}
}

// Emotes are seen: nothing for a listener who cannot see, a bare own name
// hidden at shapes, and only a player's free text spared the deafened.
func TestSendSeen_Emotes(t *testing.T) {
	cases := []struct {
		name    string
		player  bool
		cat     messaging.Category
		line    string
		chatter bool
		want    speechWant
	}{
		{"player free-form", true, messaging.CategoryEmote,
			FormatEmoteText("Kesh", "waves, and Kesh grins.", "username"), true,
			speechWant{"Kesh waves, and Kesh grins.", "", "", "A figure waves, and a figure grins.", ""}},
		{"player empty", true, messaging.CategoryEmote,
			`<ansi fg="username">Kesh</ansi> emotes.`, false,
			speechWant{"Kesh emotes.", "", "Kesh emotes.", "A figure emotes.", ""}},
		{"player alias", true, messaging.CategoryEmote,
			FormatEmoteText("Kesh", EmoteAliases["beam"], "username"), false,
			speechWant{"Kesh beams with pride.", "", "Kesh beams with pride.", "A figure beams with pride.", ""}},
		// chatter true on purpose: a mob is never deafen-filtered (ruling 6).
		{"mob free-form", false, messaging.CategoryMobEmote,
			FormatEmoteText("Grel", "sniffs the air, and Grel growls.", "mobname"), true,
			speechWant{"Grel sniffs the air, and Grel growls.", "", "Grel sniffs the air, and Grel growls.",
				"A figure sniffs the air, and a figure growls.", ""}},
		{"mob empty", false, messaging.CategoryMobEmote,
			`<ansi fg="mobname">Grel</ansi> emotes.`, false,
			speechWant{"Grel emotes.", "", "Grel emotes.", "A figure emotes.", ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := newSpeechScene(t)
			checkSpeech(t, sc, c.player, func(a Actor) { SendSeen(a, c.cat, c.line, c.chatter) }, c.want)
		})
	}
}

// A speaker somehow still hidden after the reveal (spec S2) is unseen by
// every listener, whatever their sight.
func TestSendSpoken_AStillHiddenSpeakerIsUnseenByAll(t *testing.T) {
	sc := newSpeechScene(t)
	actor := sc.speaker(true, sc.lit)
	sendSpoken(actor, sc.lit, messaging.CategorySpeech,
		FormatSayText("Kesh", "psst", false, "username", "saytext"), true)
	speechExpectOne(t, "clear", speechHeard(t, speechClearId), `Someone says, "psst"`)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/actions/ -run 'TestSendHeard|TestSendSeen|TestSendSpoken' -count=1`
Expected: FAIL to compile, `undefined: SendHeard`, `undefined: SendSeen`, `undefined: sendSpoken`.

- [ ] **Step 3: Implement**

Create `internal/actions/room_lines.go`:

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// The shared room-line senders of sight gates slice 5b. Every speech and
// emote wrapper, player and mob, sends its room line through one of these, so
// the name each listener reads follows that listener's sight, and the deafen
// filter applies to a player's free text only (owner rulings 3, 6 and 7).
// speech_wrapper_guard_test.go stops a wrapper sending its own.

// SendHeard sends a line the actor made heard (a rally, a warcry) to everyone
// else in its room, whatever they can see: the actor's name at clear sight,
// "a figure" at shapes, "something" when the listener sees nothing. It is
// authored text, not chatter, so the deafen filter does not apply. A player
// actor does not read its own room line.
func SendHeard(actor Actor, cat messaging.Category, text string) {
	room := actor.GetRoom()
	if room == nil {
		return
	}
	room.SendTextHidingNames(cat, text, []string{actor.GetName()}, messaging.HideNames, actor.GetUserId())
}

// SendSeen is SendHeard's visual twin, for an emote: the name at clear sight,
// "a figure" at shapes (a bare mention in the text included), and nothing for
// a listener who cannot see. chatter marks a player's free text, which keeps
// the deafen filter; a mob's line never meets it, whatever chatter says
// (owner ruling 6).
func SendSeen(actor Actor, cat messaging.Category, text string, chatter bool) {
	room := actor.GetRoom()
	if room == nil {
		return
	}
	names := []string{actor.GetName()}
	if chatter && actor.IsPlayer() {
		room.SendVisualCommunicationHidingNames(cat, text, names, actor.GetUserId())
		return
	}
	room.SendTextVisualHidingNames(cat, text, names, actor.GetUserId())
}

// sendSpoken sends a speech line (say, shout) the actor spoke to everyone else
// in room. Every listener hears the words; the speaker's name reads "A
// figure" at shapes and "Someone" when the listener sees nothing (ruling 3).
// A player's words are chatter and keep the deafen filter; an NPC's are
// authored and do not (ruling 6). A speaker still hidden after the reveal is
// unseen by everyone.
func sendSpoken(actor Actor, room *rooms.Room, cat messaging.Category, line string, stillHidden bool) {
	if room == nil {
		return
	}
	names := []string{actor.GetName()}
	if stillHidden {
		line = messaging.HideSpeakerNames(line, names, messaging.SightNone)
	}
	if actor.IsPlayer() {
		room.SendCommunicationHidingNames(cat, line, names, actor.GetUserId())
		return
	}
	room.SendTextHidingNames(cat, line, names, messaging.HideSpeakerNames)
}
```

- [ ] **Step 4: Teach the two root guards the shared senders**

In `messaging_surface_guard_test.go`, inside `narrationRecognizeCall`'s `switch fun.Sel.Name`, add a case directly above `case "SendTextToUser":`:

```go
		// Sight gates slice 5b: actions.SendHeard and actions.SendSeen are the
		// shared room-line senders every speech and emote wrapper calls. The
		// receiver is the package, so it is matched by name.
		case "SendHeard", "SendSeen":
			if recv.Name == "actions" {
				return viewpointObserver, true
			}
```

In `bauble_finder_view_guard_test.go`, replace:

```go
	"SendMessage": true, "Command": true, "merchantSay": true,
```

with:

```go
	"SendMessage": true, "Command": true, "merchantSay": true,
	"SendHeard": true, "SendSeen": true,
```

- [ ] **Step 5: Run the tests**

Run:
```bash
gofmt -l internal/actions/ .
go test ./internal/actions/ -run 'TestSendHeard|TestSendSeen|TestSendSpoken' -count=1 -v
go test . -run 'TestNarrationSitesMatchViewpointAudit|TestFinderView' -count=1
```
Expected: gofmt prints nothing; every subtest PASS; root `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/room_lines.go internal/actions/speech_sight_test.go messaging_surface_guard_test.go bauble_finder_view_guard_test.go
git commit -m "feat(actions): SendHeard and SendSeen, the shared speech and emote room lines"
```

---

### Task 5: `actions.Say` sends the room line

**Model:** sonnet (five callers, three of them outside the spec).

**Files:**
- Modify: `internal/actions/say.go`, `internal/actions/sell.go`, `internal/hooks/justice_wiring.go`, `internal/usercommands/offer.go`, `internal/usercommands/say.go`, `internal/mobcommands/say.go`, `internal/actions/speech_sight_test.go`
- Create: `internal/usercommands/speech_sight_wrapper_test.go`, `internal/mobcommands/speech_sight_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/actions/speech_sight_test.go`:

```go
func TestSay_PerListener(t *testing.T) {
	t.Run("player", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, true, func(a Actor) { Say(a, "hello there") }, speechWant{
			clear: `Kesh says, "hello there"`, blind: `Someone says, "hello there"`, deaf: "",
			shapes: `A figure says, "hello there"`, dark: `Someone says, "hello there"`})
	})
	t.Run("mob", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, false, func(a Actor) { Say(a, "hello there") }, speechWant{
			clear: `Grel says, "hello there"`, blind: `Someone says, "hello there"`, deaf: `Grel says, "hello there"`,
			shapes: `A figure says, "hello there"`, dark: `Someone says, "hello there"`})
	})
}
```

Create `internal/usercommands/speech_sight_wrapper_test.go`:

```go
package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var speechWrapperTag = regexp.MustCompile(`<[^>]*>`)

// speechWrapperHeard drains uid's queued lines the way its client prints
// them: minus those the deafen filter drops, tags stripped.
func speechWrapperHeard(uid int) []string {
	u := users.GetByUserId(uid)
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		if m.HiddenFromDeafened(u.Deafened) {
			continue
		}
		out = append(out, strings.TrimSpace(speechWrapperTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

// speechWrapperScene is the fixture's room 1 lit exactly (no sky, Lamp 60):
// Aliceia (user 1) speaks, Bobrick (user 2) listens.
func speechWrapperScene(t *testing.T) (*users.UserRecord, *users.UserRecord, *rooms.Room) {
	t.Helper()
	alice, room := getTestUserAndRoom(t)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	bob := users.GetByUserId(2)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(alice.Character, room))
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(bob.Character, room))
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return alice, bob, room
}

func blindForSpeechTest(t *testing.T, u *users.UserRecord) {
	t.Helper()
	u.Character.Perception = characters.New().Perception
	require.NoError(t, u.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
}

func TestSay_BlindedListenerInALitRoomHearsNoName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, room := speechWrapperScene(t)
	blindForSpeechTest(t, bob)

	_, err := Say("hello there", alice, room, 0)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone says, "hello there"`}, speechWrapperHeard(2))
	require.Equal(t, []string{`You say, "hello there"`}, speechWrapperHeard(1))
}

func TestSay_DeafenedListenerIsSpared(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, room := speechWrapperScene(t)
	bob.Deafened = true

	_, err := Say("hello there", alice, room, 0)
	require.NoError(t, err)
	require.Empty(t, speechWrapperHeard(2))
}
```

Create `internal/mobcommands/speech_sight_test.go`:

```go
package mobcommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var mobSpeechTag = regexp.MustCompile(`<[^>]*>`)

// mobSpeechHeard drains uid's queued lines the way its client prints them:
// minus those the deafen filter drops, tags stripped.
func mobSpeechHeard(uid int) []string {
	u := users.GetByUserId(uid)
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		if m.HiddenFromDeafened(u.Deafened) {
			continue
		}
		out = append(out, strings.TrimSpace(mobSpeechTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

// mobSpeechRoom lights room 1 exactly (no sky, Lamp 60) and blinds Bobrick
// (user 2) with the perception machine; Aliceia (user 1) sees clearly. The
// speaker is mob 100, "Skeleton".
func mobSpeechRoom(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	mob, room := getTestMobAndRoom(t)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	bob := users.GetByUserId(2)
	bob.Character.Perception = characters.New().Perception
	require.NoError(t, bob.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(bob.Character, room))
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return mob, room
}

// The defect sendAudioRoomText's lit-room shortcut carried: a blinded
// listener in a lit room was told the speaker's name.
func TestMobSay_BlindedListenerInALitRoomHearsNoName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := Say("hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton says, "hello there"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone says, "hello there"`}, mobSpeechHeard(2))
}

// Ruling 6: NPC speech is authored content, so a deafened player hears it.
func TestMobSay_ReachesADeafenedPlayer(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	users.GetByUserId(1).Deafened = true

	_, err := Say("hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton says, "hello there"`}, mobSpeechHeard(1))
}
```

- [ ] **Step 2: Run them to verify they fail**

Run:
```bash
go test ./internal/actions/ -run TestSay_PerListener -count=1
go test ./internal/usercommands/ -run 'TestSay_BlindedListener|TestSay_DeafenedListener' -count=1
go test ./internal/mobcommands/ -run 'TestMobSay_' -count=1
```
Expected: the actions test FAILS (clear reads nothing: `Say` sends no room line yet); the usercommands blinded test FAILS (Bobrick gets nothing per user, the line is RoomId-keyed); the mobcommands blinded test FAILS (Bobrick reads `Skeleton says, ...`: the lit shortcut). `TestSay_DeafenedListenerIsSpared` and `TestMobSay_ReachesADeafenedPlayer` may pass already; they pin behaviour that must survive.

- [ ] **Step 3: Rewrite `internal/actions/say.go`**

Replace the whole file with:

```go
package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// SayResult contains the results of a Say action for the wrapper to use.
type SayResult struct {
	IsSneaking bool
	Text       string
}

// Say is the one say body for a player and a mob (sight gates slice 5b). It
// reveals a hidden speaker, echoes "You hear someone talking." through the
// exits, fires the Communication event, and sends the room line through
// sendSpoken: every listener hears the words, and the speaker's name follows
// that listener's sight. A player's line keeps the deafen filter; an NPC's
// does not (owner ruling 6).
//
// The wrappers keep only their own concerns: mute, drunk text, escaping and
// the speaker's own line (player), and the no-players shortcut (mob).
func Say(actor Actor, text string) SayResult {
	char := actor.GetCharacter()

	// Speaking aloud is a noisy action: reveal if hidden.
	if char.IsHidden() {
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger:  awareness.TriggerNoisyAction,
			Metadata: map[string]any{"command": "say"},
		})
	}

	isSneaking := char.IsHidden()

	room := actor.GetRoom()
	room.SendTextToExits(`You hear someone talking.`, true)

	events.AddToQueue(events.Communication{
		SourceUserId:        actor.GetUserId(),
		SourceMobInstanceId: actor.GetMobInstanceId(),
		CommType:            `say`,
		Name:                actor.GetName(),
		Message:             text,
	})

	nameColor, textColor := "mobname", "saytext-mob"
	if actor.IsPlayer() {
		nameColor, textColor = "username", "saytext"
	}
	sendSpoken(actor, room, messaging.CategorySpeech,
		FormatSayText(actor.GetName(), text, false, nameColor, textColor), isSneaking)

	return SayResult{
		IsSneaking: isSneaking,
		Text:       text,
	}
}

// FormatSayText formats the say message for room display.
// nameColor is "username" for players, "mobname" for mobs.
// textColor is "saytext" for players, "saytext-mob" for mobs.
func FormatSayText(name string, text string, isSneaking bool, nameColor string, textColor string) string {
	var msg string
	if isSneaking {
		msg = fmt.Sprintf(`someone says, "<ansi fg="%s">%s</ansi>"`, textColor, text)
	} else {
		msg = fmt.Sprintf(`<ansi fg="%s">%s</ansi> says, "<ansi fg="%s">%s</ansi>"`, nameColor, name, textColor, text)
	}
	return util.SplitStringNL(msg, 80)
}
```

- [ ] **Step 4: Thin the two command wrappers**

In `internal/usercommands/say.go` replace:

```go
	actor := &actions.UserActor{User: user, Room: room}
	result := actions.Say(actor, rest)

	roomMsg := actions.FormatSayText(user.Character.Name, result.Text, result.IsSneaking, "username", "saytext")
	room.SendTextCommunication(roomMsg, user.UserId)
```

with:

```go
	// actions.Say sends the room line: the speaker's name follows each
	// listener's sight, and the deafen filter still applies (sight gates 5b).
	result := actions.Say(&actions.UserActor{User: user, Room: room}, rest)
```

Replace the whole of `internal/mobcommands/say.go` with:

```go
package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Say speaks for a mob through actions.Say, which sends the room line with
// the mob's name hidden by each listener's sight and no deafen filter (NPC
// lines are authored content, owner ruling 6).
func Say(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	actions.Say(&actions.MobActor{Mob: mob, Room: room}, rest)

	return true, nil
}
```

- [ ] **Step 5: The three callers the spec missed (F18)**

In `internal/actions/sell.go`, replace the body of `merchantSay`:

```go
	actor := &MobActor{Mob: mob, Room: room}
	result := Say(actor, line)
	room.SendText(messaging.CategorySpeech,
		FormatSayText(mob.Character.Name, result.Text, false, "mobname", "saytext-mob"))
}
```

with:

```go
	// Say sends the room line itself (sight gates slice 5b).
	Say(&MobActor{Mob: mob, Room: room}, line)
}
```

In `internal/hooks/justice_wiring.go`, replace:

```go
		actor := &actions.MobActor{Mob: mob, Room: room}
		result := actions.Say(actor, line)
		room.SendText(messaging.CategorySpeech,
			actions.FormatSayText(mob.Character.Name, result.Text, false, "mobname", "saytext-mob"))
	})
```

with:

```go
		// actions.Say sends the room line itself (sight gates slice 5b).
		actions.Say(&actions.MobActor{Mob: mob, Room: room}, line)
	})
```

and delete the now-unused import line `"github.com/GoMudEngine/GoMud/internal/messaging"` from `justice_wiring.go`.

In `internal/usercommands/offer.go`, replace the body of its package-local `merchantSay` (used by `Offer` and by `appraise.go`):

```go
	actor := &actions.MobActor{Mob: mob, Room: room}
	result := actions.Say(actor, line)
	room.SendText(messaging.CategorySpeech,
		actions.FormatSayText(mob.Character.Name, result.Text, false, "mobname", "saytext-mob"))
}
```

with:

```go
	// actions.Say sends the room line itself (sight gates slice 5b).
	actions.Say(&actions.MobActor{Mob: mob, Room: room}, line)
}
```

Keep the `mob == nil || room == nil` guard above it, and keep the `messaging` import (`Offer` still uses `messaging.CategorySystem`).

- [ ] **Step 6: Run the tests**

Run:
```bash
gofmt -l internal/ .
go build ./...
go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/hooks/ -count=1
go test . -run 'TestNarrationSitesMatchViewpointAudit|TestFinderView|TestNoRawEventsMessageOutsidePipeline' -count=1
```
Expected: gofmt prints nothing; build clean (if `go build` reports `messaging` unused in `sell.go`, remove that import; it is expected to stay used elsewhere in the file); every package `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/actions/say.go internal/actions/sell.go internal/actions/speech_sight_test.go internal/hooks/justice_wiring.go internal/usercommands/offer.go internal/usercommands/say.go internal/usercommands/speech_sight_wrapper_test.go internal/mobcommands/say.go internal/mobcommands/speech_sight_test.go
git commit -m "feat(say): actions.Say sends the room line with the speaker's name by sight"
```

---

### Task 6: `actions.Shout`

**Model:** sonnet.

**Files:**
- Create: `internal/actions/shout.go`
- Modify: `internal/usercommands/shout.go`, `internal/mobcommands/shout.go`, `internal/actions/speech_sight_test.go`, `internal/usercommands/speech_sight_wrapper_test.go`, `internal/mobcommands/speech_sight_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/actions/speech_sight_test.go`:

```go
func TestShout_PerListener(t *testing.T) {
	t.Run("player", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, true, func(a Actor) { Shout(a, "HELP") }, speechWant{
			clear: `Kesh shouts, "HELP"`, blind: `Someone shouts, "HELP"`, deaf: "",
			shapes: `A figure shouts, "HELP"`, dark: `Someone shouts, "HELP"`})
	})
	t.Run("mob", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, false, func(a Actor) { Shout(a, "HELP") }, speechWant{
			clear: `Grel shouts, "HELP"`, blind: `Someone shouts, "HELP"`, deaf: `Grel shouts, "HELP"`,
			shapes: `A figure shouts, "HELP"`, dark: `Someone shouts, "HELP"`})
	})
}

func TestShout_NextDoorHearsTheWordsAndNoName(t *testing.T) {
	sc := newSpeechScene(t)

	Shout(sc.speaker(true, sc.lit), "HELP")
	far := events.DrainQueuedRoomMessagesForTest(speechNextRoom)
	require.Len(t, far, 1, "a player's shout reaches next door once")
	require.Equal(t, `Someone shouts from the south direction, "HELP"`,
		strings.TrimSpace(speechTag.ReplaceAllString(far[0].Text, "")))
	require.True(t, far[0].IsCommunication, "a player's shout next door is still player chatter")

	sc.drain()
	Shout(sc.speaker(false, sc.lit), "HELP")
	msgs := events.DrainQueuedMessageEventsForTest(speechNextId)
	require.Len(t, msgs, 1, "a mob's shout next door now carries its words")
	require.Equal(t, `Someone shouts from the south direction, "HELP"`,
		strings.TrimSpace(speechTag.ReplaceAllString(msgs[0].Text, "")))
	require.False(t, msgs[0].IsCommunication, "an NPC's shout is never deafen-filtered")
}

// Rider 5: a hidden mob that shouts is revealed, as a player is.
func TestShout_RevealsAHiddenMob(t *testing.T) {
	sc := newSpeechScene(t)
	hideRhetoricActor(t, &sc.mob.Character)

	res := Shout(sc.speaker(false, sc.lit), "HELP")
	require.False(t, sc.mob.Character.IsHidden(), "shouting reveals a hidden mob")
	require.False(t, res.IsSneaking)
	speechExpectOne(t, "clear", speechHeard(t, speechClearId), `Grel shouts, "HELP"`)
}

func TestShout_WakesTheRoom(t *testing.T) {
	for _, player := range []bool{true, false} {
		sc := newSpeechScene(t)
		sleeper := users.GetByUserId(speechClearId).Character
		require.True(t, sleeper.Conditions.AddCondition(speechSleepCond, false))
		require.True(t, sleeper.HasConditionFlag(conditions.Sleeping))

		Shout(sc.speaker(player, sc.lit), "WAKE UP")
		require.False(t, sleeper.HasConditionFlag(conditions.Sleeping), "player speaker %v", player)
	}
}
```

Append to `internal/usercommands/speech_sight_wrapper_test.go`:

```go
func TestShout_BlindedListenerHearsTheWordsNotTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, room := speechWrapperScene(t)
	blindForSpeechTest(t, bob)

	_, err := Shout("help", alice, room, 0)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone shouts, "HELP"`}, speechWrapperHeard(2))
	require.Equal(t, []string{`You shout, "HELP"`}, speechWrapperHeard(1))
}
```

Append to `internal/mobcommands/speech_sight_test.go`:

```go
func TestMobShout_BlindedListenerHearsTheWordsNotTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := Shout("intruders!", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton shouts, "intruders!"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone shouts, "intruders!"`}, mobSpeechHeard(2))
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/actions/ -run 'TestShout_' -count=1`
Expected: FAIL to compile, `undefined: Shout`.

- [ ] **Step 3: Create `internal/actions/shout.go`**

```go
package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// ShoutResult reports a shout to its wrapper.
type ShoutResult struct {
	// IsSneaking is true when the shouter is still hidden after the reveal,
	// which TransitionToRevealing makes unreachable today (it lands on
	// Visible in the same call); every listener then reads "Someone".
	IsSneaking bool
	Text       string
}

// Shout is the one shout body for a player and a mob (sight gates slice 5b):
// it reveals a hidden shouter (rider 5), sends the room line through
// sendSpoken (the name by each listener's sight, the words always), sends
// the anonymous line with the words to every adjacent room, and wakes every
// sleeper in the room but the shouter.
//
// The player wrapper keeps mute, uppercase, drunk text, escaping and the
// self line; the mob wrapper keeps nothing but this call.
func Shout(actor Actor, text string) ShoutResult {
	char := actor.GetCharacter()

	// Shouting is a noisy action: reveal if hidden.
	if char.IsHidden() {
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger:  awareness.TriggerNoisyAction,
			Metadata: map[string]any{"command": "shout"},
		})
	}
	isSneaking := char.IsHidden()

	room := actor.GetRoom()
	if room == nil {
		return ShoutResult{IsSneaking: isSneaking, Text: text}
	}

	nameColor, textColor := "mobname", "saytext-mob"
	if actor.IsPlayer() {
		nameColor, textColor = "username", "yellow"
	}
	line := fmt.Sprintf(`<ansi fg="%s">%s</ansi> shouts, "<ansi fg="%s">%s</ansi>"`,
		nameColor, actor.GetName(), textColor, text)
	sendSpoken(actor, room, messaging.CategoryShout, util.SplitStringNL(line, 80), isSneaking)

	// Next door hears the words and no name, from either speaker. A player's
	// line stays player chatter (deafen-filtered, byte-identical to before);
	// a mob's is authored and unfiltered. Standard, temporary and
	// mutator-added exits, each neighbour once.
	room.ForEachAdjacentRoom(func(otherRoom *rooms.Room, sourceExit string) {
		far := fmt.Sprintf(`Someone shouts from the <ansi fg="exit">%s</ansi> direction, "<ansi fg="%s">%s</ansi>"`,
			sourceExit, textColor, text)
		if actor.IsPlayer() {
			otherRoom.SendTextCommunication(far, actor.GetUserId())
			return
		}
		otherRoom.SendText(messaging.CategoryShout, far)
	})

	wakeSleepers(actor, room)

	return ShoutResult{IsSneaking: isSneaking, Text: text}
}

// wakeSleepers wakes every sleeper in room but the actor. Chunk 3.3: a shout
// wakes the room it is shouted in; next door is out of scope.
func wakeSleepers(actor Actor, room *rooms.Room) {
	for _, uid := range room.GetPlayers() {
		if uid == actor.GetUserId() {
			continue
		}
		if other := users.GetByUserId(uid); other != nil && other.Character.HasConditionFlag(conditions.Sleeping) {
			other.Character.CancelConditionsWithFlag(conditions.Sleeping)
			mobs.OnSleeperWoken(other.Character)
		}
	}
	for _, instId := range room.GetMobs() {
		if instId == actor.GetMobInstanceId() {
			continue
		}
		if m := mobs.GetInstance(instId); m != nil && m.Character.HasConditionFlag(conditions.Sleeping) {
			m.Character.CancelConditionsWithFlag(conditions.Sleeping)
			mobs.OnSleeperWoken(&m.Character)
		}
	}
}
```

- [ ] **Step 4: Thin the wrappers**

Replace the whole of `internal/usercommands/shout.go` with:

```go
package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Shout keeps the player's own concerns (mute, uppercase, drunk text,
// escaping, the self line) and hands the rest to actions.Shout: the reveal,
// the room line with the name by each listener's sight, the line next door,
// and waking the room (sight gates slice 5b).
func Shout(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if user.Muted {
		user.SendText(messaging.CategoryWarning, `You are <ansi fg="alert-5">MUTED</ansi>. You can only send <ansi fg="command">whisper</ansi>'s to Admins and Moderators.`)
		return true, nil
	}

	rest = strings.ToUpper(rest)

	if user.Character.HasConditionFlag(conditions.Drunk) {
		// modify the text to look like it's the speech of a drunk person
		rest = drunkify(rest)
	}

	// Neutralise <ansi> markup before interpolation. ToUpper above happens to
	// break the parser's byte-exact lowercase match, but that is an accident of
	// the current parser, not a defence, so escape explicitly. Shout crosses
	// room boundaries, so a forged tag here reaches players who never opted in.
	rest = util.EscapeAnsiTags(rest)

	actions.Shout(&actions.UserActor{User: user, Room: room}, rest)

	selfMsg := fmt.Sprintf(`You shout, "<ansi fg="yellow">%s</ansi>"`, rest)
	user.SendText(messaging.CategoryShout, util.SplitStringNL(selfMsg, 80))

	return true, nil
}
```

(Behaviour note: drunkenness was read after the reveal before and is read before `actions.Shout` now; the reveal never changes it.)

Replace the whole of `internal/mobcommands/shout.go` with:

```go
package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Shout shouts for a mob through actions.Shout: a hidden mob is revealed, the
// room hears its name by each listener's sight, next door hears the words,
// and the room's sleepers wake (sight gates slice 5b).
func Shout(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	actions.Shout(&actions.MobActor{Mob: mob, Room: room}, rest)
	return true, nil
}
```

- [ ] **Step 5: Run the tests**

Run:
```bash
gofmt -l internal/ .
go build ./...
go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ -count=1
go test . -run 'TestNarrationSitesMatchViewpointAudit|TestFinderView' -count=1
```
Expected: gofmt prints nothing; everything `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/shout.go internal/actions/speech_sight_test.go internal/usercommands/shout.go internal/usercommands/speech_sight_wrapper_test.go internal/mobcommands/shout.go internal/mobcommands/speech_sight_test.go
git commit -m "feat(shout): actions.Shout reveals, names by sight and carries the words next door"
```

---

### Task 7: Rally and warcry are heard

**Model:** haiku (four line-count-neutral edits, code given).

**Files:**
- Modify: `internal/usercommands/rally.go`, `internal/usercommands/warcry.go`, `internal/mobcommands/rally.go`, `internal/mobcommands/warcry.go`

The player files are keyed by LINE NUMBER in `condition_apply_path_guard_test.go` (F24). Each replacement below is four lines for four lines, so lines 57, 86, 114 (rally) and 57, 90, 118 (warcry) do not move.

- [ ] **Step 1: Player rally**

In `internal/usercommands/rally.go` replace:

```go
	room.SendTextVisual(messaging.CategoryRally,
		fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi> rallies everyone with an inspiring shout!</ansi>`, user.Character.Name),
		user.UserId,
	)
```

with:

```go
	// Heard, not seen: the name follows each listener's sight (sight gates 5b).
	actions.SendHeard(&actions.UserActor{User: user, Room: room}, messaging.CategoryRally,
		fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi> rallies everyone with an inspiring shout!</ansi>`, user.Character.Name),
	)
```

and replace:

```go
		room.SendTextVisual(messaging.CategoryWarcry,
			fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi>'s shout hardens into a warcry in the same breath!</ansi>`, user.Character.Name),
			user.UserId,
		)
```

with:

```go
		// Heard, not seen, like the rally line above.
		actions.SendHeard(&actions.UserActor{User: user, Room: room}, messaging.CategoryWarcry,
			fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi>'s shout hardens into a warcry in the same breath!</ansi>`, user.Character.Name),
		)
```

- [ ] **Step 2: Player warcry**

In `internal/usercommands/warcry.go` replace:

```go
	room.SendTextVisual(messaging.CategoryWarcry,
		fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi> lets out a thunderous warcry!</ansi>`, user.Character.Name),
		user.UserId,
	)
```

with:

```go
	// Heard, not seen: the name follows each listener's sight (sight gates 5b).
	actions.SendHeard(&actions.UserActor{User: user, Room: room}, messaging.CategoryWarcry,
		fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi> lets out a thunderous warcry!</ansi>`, user.Character.Name),
	)
```

and replace:

```go
		room.SendTextVisual(messaging.CategoryRally,
			fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi>'s shout gathers into a rally in the same breath!</ansi>`, user.Character.Name),
			user.UserId,
		)
```

with:

```go
		// Heard, not seen, like the warcry line above.
		actions.SendHeard(&actions.UserActor{User: user, Room: room}, messaging.CategoryRally,
			fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi>'s shout gathers into a rally in the same breath!</ansi>`, user.Character.Name),
		)
```

- [ ] **Step 3: Mob rally and warcry**

In `internal/mobcommands/rally.go` replace:

```go
	result := actions.ExecuteRally(&actions.MobActor{Mob: mob, Room: room})
```

with:

```go
	actor := &actions.MobActor{Mob: mob, Room: room}
	result := actions.ExecuteRally(actor)
```

and replace:

```go
	sendAudioRoomText(room, mob, messaging.CategoryRally,
		`<ansi fg="cyan-bold">Something lets out a rallying roar!</ansi>`,
		fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="mobname">%s</ansi> lets out a rallying roar!</ansi>`, mob.Character.Name))
```

with:

```go
	// Heard, not seen: the name follows each listener's sight (sight gates 5b).
	actions.SendHeard(actor, messaging.CategoryRally,
		fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="mobname">%s</ansi> lets out a rallying roar!</ansi>`, mob.Character.Name))
```

In `internal/mobcommands/warcry.go` replace:

```go
	result := actions.ExecuteWarcry(&actions.MobActor{Mob: mob, Room: room})
```

with:

```go
	actor := &actions.MobActor{Mob: mob, Room: room}
	result := actions.ExecuteWarcry(actor)
```

and replace:

```go
	sendAudioRoomText(room, mob, messaging.CategoryWarcry,
		`<ansi fg="red-bold">Something lets out a bone-shaking warcry!</ansi>`,
		fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="mobname">%s</ansi> lets out a bone-shaking warcry!</ansi>`, mob.Character.Name))
```

with:

```go
	// Heard, not seen: the name follows each listener's sight (sight gates 5b).
	actions.SendHeard(actor, messaging.CategoryWarcry,
		fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="mobname">%s</ansi> lets out a bone-shaking warcry!</ansi>`, mob.Character.Name))
```

- [ ] **Step 4: Verify the line keys did not move and every guard holds**

Run:
```bash
grep -n "AddConditionMagnitude" internal/usercommands/rally.go internal/usercommands/warcry.go
gofmt -l internal/ .
go build ./...
go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/actions/ -count=1
go test . -run 'TestPlayerConditionsTravelTheEventPath|TestNarrationSitesMatchViewpointAudit|TestFinderView' -count=1
```
Expected: the grep prints `rally.go:57`, `rally.go:86`, `rally.go:114`, `warcry.go:57`, `warcry.go:90`, `warcry.go:118`; gofmt prints nothing; everything `ok`. If a line moved, the edit was not four-for-four: fix the edit, do not re-key the guard.

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/rally.go internal/usercommands/warcry.go internal/mobcommands/rally.go internal/mobcommands/warcry.go
git commit -m "feat(rally,warcry): heard in the dark on both sides, name by sight"
```

---

### Task 8: Emotes are seen through `actions.SendSeen`

**Model:** sonnet (the deafen split is wrapper-owned and easy to get backwards).

**Files:**
- Modify: `internal/usercommands/emote.go`, `internal/mobcommands/emote.go`, `internal/usercommands/speech_sight_wrapper_test.go`, `internal/mobcommands/speech_sight_test.go`, `internal/rooms/rooms.go` (doc comment), `internal/hooks/Message_SendMessages.go` (comment)

- [ ] **Step 1: Write the failing tests**

Append to `internal/usercommands/speech_sight_wrapper_test.go`:

```go
func TestEmote_FreeFormFollowsSightAndDeafen(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, room := speechWrapperScene(t)

	_, err := Emote("waves.", alice, room, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"Aliceia waves."}, speechWrapperHeard(2))
	require.Equal(t, []string{"You Emote: Aliceia waves."}, speechWrapperHeard(1))

	bob.Deafened = true
	_, err = Emote("waves.", alice, room, 0)
	require.NoError(t, err)
	require.Empty(t, speechWrapperHeard(2), "free text is chatter: the deafened are spared it")
	bob.Deafened = false

	blindForSpeechTest(t, bob)
	_, err = Emote("waves.", alice, room, 0)
	require.NoError(t, err)
	require.Empty(t, speechWrapperHeard(2), "an emote is seen, not heard")
}

func TestEmote_AtFormSkipsOnlyTheSelfLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, room := speechWrapperScene(t)

	_, err := Emote("@waves silently.", alice, room, 0)
	require.NoError(t, err)
	require.Empty(t, speechWrapperHeard(1))
	require.Equal(t, []string{"Aliceia waves silently."}, speechWrapperHeard(2))
}

func TestEmote_AliasAndEmptyReachTheDeafened(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, room := speechWrapperScene(t)
	bob.Deafened = true

	_, err := Emote("beam", alice, room, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"Aliceia beams with pride."}, speechWrapperHeard(2))

	_, err = Emote("", alice, room, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"Aliceia emotes."}, speechWrapperHeard(2))
}
```

Append to `internal/mobcommands/speech_sight_test.go`:

```go
func TestMobEmote_SeenNotHeard(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	users.GetByUserId(1).Deafened = true

	_, err := Emote("growls.", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{"Skeleton growls."}, mobSpeechHeard(1), "a mob emote reaches the deafened (ruling 6)")
	require.Empty(t, mobSpeechHeard(2), "the blinded see nothing")
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/usercommands/ -run 'TestEmote_' -count=1`
Expected: `TestEmote_FreeFormFollowsSightAndDeafen` FAILS (Bobrick reads nothing per user: the free-form line is still RoomId-keyed) and `TestEmote_AtFormSkipsOnlyTheSelfLine` FAILS for the same reason. The alias test and the mob test may pass already; they pin what must not change.

- [ ] **Step 3: Rewrite the player wrapper**

Replace the whole of `internal/usercommands/emote.go` with:

```go
package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Emote sends the player's emote to the room through actions.SendSeen. An
// emote is seen, not heard: the name follows each onlooker's sight and a
// listener who cannot see gets nothing (owner ruling 7, sight gates slice
// 5b). Only the free-form line (and its @ form) is chatter, spared a
// deafened player; the empty and alias lines are pre-written and reach
// everyone who can see.
func Emote(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	actor := &actions.UserActor{User: user, Room: room}

	if len(rest) == 0 {
		user.SendText(messaging.CategoryEmote, "You emote.")
		actions.SendSeen(actor, messaging.CategoryEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> emotes.`, user.Character.Name),
			false,
		)
		return true, nil
	}

	// Emote aliases bypass mute/deafen checks (pre-written, not free-form communication).
	result := actions.Emote(rest)
	if result.IsAlias {
		aliasMsg := actions.FormatEmoteText(user.Character.Name, result.AliasText, "username")
		user.SendText(messaging.CategoryEmote, fmt.Sprintf(`You Emote: %s`, aliasMsg))
		actions.SendSeen(actor, messaging.CategoryEmote, aliasMsg, false)
		events.AddToQueue(events.Emote{UserId: user.UserId, RoomId: room.RoomId, Text: result.AliasText})
		return true, nil
	}

	if user.Muted {
		user.SendText(messaging.CategoryWarning, `You are <ansi fg="alert-5">MUTED</ansi>. You can only send <ansi fg="command">whisper</ansi>'s to Admins and Moderators.`)
		return true, nil
	}

	// Neutralise <ansi> markup before interpolation. Only the free-form path
	// is escaped; result.AliasText above comes from the server-side
	// EmoteAliases table and its markup is legitimate.
	rest = util.EscapeAnsiTags(rest)

	if rest[0] == '@' && len(rest) > 1 {
		rest = rest[1:]
	} else {
		emoteMsg := actions.FormatEmoteText(user.Character.Name, rest, "username")
		user.SendText(messaging.CategoryEmote, fmt.Sprintf(`You Emote: %s`, emoteMsg))
	}

	// Free text is chatter: true keeps the deafen filter.
	actions.SendSeen(actor, messaging.CategoryEmote,
		actions.FormatEmoteText(user.Character.Name, rest, "username"),
		true,
	)
	events.AddToQueue(events.Emote{UserId: user.UserId, RoomId: room.RoomId, Text: rest})

	return true, nil
}
```

- [ ] **Step 4: Rewrite the mob wrapper**

Replace the whole of `internal/mobcommands/emote.go` with:

```go
package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Emote sends a mob's emote through actions.SendSeen: seen by sight, a bare
// mention of its own name hidden at shapes too, and never deafen-filtered
// (NPC lines are authored content, owner ruling 6).
func Emote(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	actor := &actions.MobActor{Mob: mob, Room: room}

	if len(rest) == 0 {
		actions.SendSeen(actor, messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> emotes.`, mob.Character.Name), false)
		return true, nil
	}

	result := actions.Emote(rest)
	emoteText := rest
	if result.IsAlias {
		emoteText = result.AliasText
	}

	actions.SendSeen(actor, messaging.CategoryMobEmote,
		actions.FormatEmoteText(mob.Character.Name, emoteText, "mobname"), false)

	return true, nil
}
```

- [ ] **Step 5: The two comments that describe where player chatter goes**

With say, shout and emote moved, two comments are now true. In `internal/rooms/rooms.go` replace the `SendTextCommunication` doc comment (the eight lines from `// SendTextCommunication delivers PLAYER-origin chat` to `// content. Audited 2026-07-10.`) with:

```go
// SendTextCommunication delivers PLAYER-origin chat as ONE RoomId-keyed
// event, unrendered, so the legacy listener (hooks/Message_SendMessages.go,
// RoomId branch) applies the Deafened moderation filter. Two callers remain:
// a player's adjacent-room shout (actions.Shout) and the AI companion's `ask`
// line. Player speech in the speaker's own room goes through
// SendCommunicationHidingNames and a free-form emote through
// SendVisualCommunicationHidingNames, both per user and both still
// deafen-filtered. NPC speech must NOT use any of the three: it goes through
// SendTextHidingNames unfiltered, so moderated players still hear quest
// content (owner ruling 6, sight gates slice 5b).
```

In `internal/hooks/Message_SendMessages.go` replace the comment block above `if message.RoomId > 0 {` (the eight lines from `// RoomId branch: post-T9` to `// cherry-pick parity. Audited 2026-07-10.`) with:

```go
	// RoomId branch: post-T9, Room.SendText/SendTextVisual fan out
	// per-recipient (UserId events above), so this branch serves only the
	// remaining RoomId-keyed emitters: Room.SendTextCommunication (a player's
	// adjacent-room shout and the AI companion's `ask` line) and
	// Room.SendTextToExits. Player speech and free-form emotes arrive per
	// user with IsCommunication set, so the per-user check above carries the
	// Deafened filter for them (sight gates slice 5b). The IsQuiet /
	// SuperHearing filter below is reached by SendTextToExits(txt, true), but
	// no dogmud condition grants superhearing, so those lines reach nobody.
```

Confirm the first claim: `grep -rn "SendTextCommunication(" --include=*.go internal modules | grep -v _test.go` must list only `rooms.go` (the definition), `actions/shout.go`, `modules/aicompanion/listeners.go`, and `actions/actor_user.go:48,50` (the dead `UserActor.SendRoomCommunication`, which Task 10 deletes; after Task 10 the same grep lists only the first three).

- [ ] **Step 6: Run the tests**

Run:
```bash
gofmt -l internal/ .
go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/actions/ ./internal/rooms/ ./internal/hooks/ -count=1
go test . -run 'TestNarrationSitesMatchViewpointAudit|TestFinderView|TestNarrationTrioOnlyCategoriesLeaveOnlyThroughSendTrio' -count=1
```
Expected: gofmt prints nothing; everything `ok`. The narration registry keys `usercommands/emote.go|You Emote: %s` and `|You emote.` still match (each event is still actor plus observer, the observer now `actions.SendSeen`); if the guard reports them stale, Task 4 Step 4 is missing.

- [ ] **Step 7: Commit**

```bash
git add internal/usercommands/emote.go internal/mobcommands/emote.go internal/usercommands/speech_sight_wrapper_test.go internal/mobcommands/speech_sight_test.go internal/rooms/rooms.go internal/hooks/Message_SendMessages.go
git commit -m "feat(emote): emotes are seen by sight; only player free text meets deafen"
```

---

### Task 9: Howl and taunt on `SendTextHidingNames`; the mob darkness helpers go

**Model:** sonnet (many mechanical edits plus comment rewording).

**Files:**
- Modify: `internal/mobcommands/howl.go`, `internal/mobcommands/taunt.go`, `internal/mobcommands/go.go`, `internal/mobcommands/taunt_store_test.go`, `internal/mobcommands/speech_sight_test.go`, `m2_routing_guard_test.go`, `send_trio_only_guard_test.go`
- Delete: `internal/mobcommands/darkness.go`, `internal/mobcommands/audio_room_text_sight_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/mobcommands/speech_sight_test.go` (add `"github.com/GoMudEngine/GoMud/internal/actions"` to its imports):

```go
// The lit-room shortcut named a howling mob to a blinded listener.
func TestMobHowl_FumbleHeardWithoutAName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	mob.Character.SetAggro(1, 0, characters.DefaultAttack)
	original := executeTauntAction
	executeTauntAction = func(actions.Actor) actions.TauntResult {
		return actions.TauntResult{Executed: true, Fumble: true}
	}
	t.Cleanup(func() { executeTauntAction = original })

	_, err := Howl("", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{"Skeleton lets out a pitiful howl that trails off weakly."}, mobSpeechHeard(1))
	require.Equal(t, []string{"Something lets out a pitiful howl that trails off weakly."}, mobSpeechHeard(2))
}

// Ported from audio_room_text_sight_test.go (lighting plan 5c finding 1): a
// night-vision holder in a dark room cannot tell who is speaking.
func TestMobSay_NightVisionInTheDarkHearsNoName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", SkyLight: rooms.SkyLightPtr(0.0), MovementCost: 1},
	})
	defer restoreBiomes()
	const nightId = 9631
	restoreConditions := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		nightId: {ConditionId: nightId, Name: "Test Night Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.NightVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectNightVisionStrength: {Literal: 24}}},
	})
	defer restoreConditions()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	for _, lamp := range []int{0, 10, 24} {
		room := rooms.LoadRoom(2)
		require.NotNil(t, room)
		room.Biome = "cave"
		room.Lamp = rooms.LampPtr(lamp)
		u := users.GetByUserId(1)
		rooms.LoadRoom(1).RemovePlayer(1)
		u.Character.RoomId = 2
		room.AddPlayer(1)
		if !u.Character.HasCondition(nightId) {
			require.True(t, u.Character.Conditions.AddCondition(nightId, true))
		}
		require.NotEqual(t, messaging.SightFull, messaging.ParticipantSight(u.Character, room), "light %d", lamp)

		events.DrainQueuedMessagesForTest(1)
		_, err := Say("growls", mob, room)
		require.NoError(t, err)
		got := strings.Join(mobSpeechHeard(1), "\n")
		require.Contains(t, got, `says, "growls"`, "light %d: the words arrive", lamp)
		require.NotContains(t, got, "Skeleton", "light %d: a nightvision holder in a dark room cannot tell who", lamp)
	}
}
```

Also add `"github.com/GoMudEngine/GoMud/internal/conditions"` to the imports.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/mobcommands/ -run 'TestMobHowl_FumbleHeardWithoutAName' -count=1`
Expected: FAIL: Bobrick reads `Skeleton lets out a pitiful howl ...` (the lit shortcut). The ported say test passes already (Task 5); it keeps the lighting 5c pin alive once its old file is deleted.

- [ ] **Step 3: Move howl onto `SendTextHidingNames`**

In `internal/mobcommands/howl.go` make four replacements.

Replace:

```go
		sendAudioRoomText(room, mob, messaging.CategoryTauntFailure,
			`Something lets out a pitiful howl that trails off weakly.`,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lets out a pitiful howl that trails off weakly.`, mob.Character.Name))
```

with:

```go
		room.SendTextHidingNames(messaging.CategoryTauntFailure,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lets out a pitiful howl that trails off weakly.`, mob.Character.Name),
			[]string{mob.Character.Name}, messaging.HideNames)
```

Replace:

```go
			sendAudioRoomTextHidingNames(room, messaging.CategoryTauntSuccess,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> throws back its head and lets out a bone-chilling howl at <ansi fg="username">%s</ansi>!`, mob.Character.Name, targetName),
				[]string{mob.Character.Name, targetName})
```

with:

```go
			room.SendTextHidingNames(messaging.CategoryTauntSuccess,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> throws back its head and lets out a bone-chilling howl at <ansi fg="username">%s</ansi>!`, mob.Character.Name, targetName),
				[]string{mob.Character.Name, targetName}, messaging.HideNames)
```

Replace:

```go
			sendAudioRoomText(room, mob, messaging.CategoryTauntSuccess,
				`Something turns, drawn snarling toward a new foe.`,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> turns from its prey and snarls at <ansi fg="mobname">%s</ansi>!`, targetName, mob.Character.Name))
```

with:

```go
			room.SendTextHidingNames(messaging.CategoryTauntSuccess,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> turns from its prey and snarls at <ansi fg="mobname">%s</ansi>!`, targetName, mob.Character.Name),
				[]string{mob.Character.Name, targetName}, messaging.HideNames)
```

Replace:

```go
		sendAudioRoomTextHidingNames(room, messaging.CategoryTauntResist,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> howls menacingly at <ansi fg="username">%s</ansi>, but it has no effect.`, mob.Character.Name, targetName),
			[]string{mob.Character.Name, targetName})
```

with:

```go
		room.SendTextHidingNames(messaging.CategoryTauntResist,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> howls menacingly at <ansi fg="username">%s</ansi>, but it has no effect.`, mob.Character.Name, targetName),
			[]string{mob.Character.Name, targetName}, messaging.HideNames)
```

- [ ] **Step 4: Move taunt onto `SendTextHidingNames`**

In `internal/mobcommands/taunt.go` make six replacements.

Replace:

```go
			sendAudioRoomText(room, mob, messaging.CategoryTauntFailure,
				`Something bellows a challenge that breaks into a strangled gasp.`,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge that breaks into a strangled gasp.`, mob.Character.Name))
```

with:

```go
			room.SendTextHidingNames(messaging.CategoryTauntFailure,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge that breaks into a strangled gasp.`, mob.Character.Name),
				[]string{mob.Character.Name}, messaging.HideNames)
```

Replace:

```go
				sendAudioRoomText(room, mob, messaging.CategoryTauntSuccess,
					messaging.Anonymize(fmt.Sprintf(`Something bellows a thunderous challenge at <ansi fg="username">%s</ansi>!`, targetName)),
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a thunderous challenge at <ansi fg="username">%s</ansi>!`, mob.Character.Name, targetName))
```

with:

```go
				room.SendTextHidingNames(messaging.CategoryTauntSuccess,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a thunderous challenge at <ansi fg="username">%s</ansi>!`, mob.Character.Name, targetName),
					[]string{mob.Character.Name, targetName}, messaging.HideNames)
```

Replace:

```go
			sendAudioRoomText(room, mob, messaging.CategoryTauntSuccess,
				`Something wheels around, drawn to a new challenger.`,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wheels around and locks onto <ansi fg="mobname">%s</ansi>!`, targetName, mob.Character.Name))
```

with:

```go
			room.SendTextHidingNames(messaging.CategoryTauntSuccess,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wheels around and locks onto <ansi fg="mobname">%s</ansi>!`, targetName, mob.Character.Name),
				[]string{mob.Character.Name, targetName}, messaging.HideNames)
```

Replace:

```go
			sendAudioRoomText(room, mob, messaging.CategoryTauntResist,
				messaging.Anonymize(fmt.Sprintf(`Something bellows a challenge at <ansi fg="username">%s</ansi>, but they shrug it off.`, targetName)),
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge at <ansi fg="username">%s</ansi>, but they shrug it off.`, mob.Character.Name, targetName))
```

with:

```go
			room.SendTextHidingNames(messaging.CategoryTauntResist,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> bellows a challenge at <ansi fg="username">%s</ansi>, but they shrug it off.`, mob.Character.Name, targetName),
				[]string{mob.Character.Name, targetName}, messaging.HideNames)
```

Replace:

```go
	sendAudioRoomTextHidingNames(room, cat, triad.ToRoom, []string{mob.Character.Name, targetName}, excluded...)
```

with:

```go
	room.SendTextHidingNames(cat, triad.ToRoom, []string{mob.Character.Name, targetName}, messaging.HideNames, excluded...)
```

Replace:

```go
	sendAudioRoomTextHidingNames(room, messaging.CategoryTauntResist, visible,
		[]string{mob.Character.Name, defenderPlainName}, excluded...)
```

with:

```go
	room.SendTextHidingNames(messaging.CategoryTauntResist, visible,
		[]string{mob.Character.Name, defenderPlainName}, messaging.HideNames, excluded...)
```

And in the `sendMobTauntTriad` doc comment replace:

```go
// messaging.ParticipantSight + messaging.HideNames) and the room line (via
// sendAudioRoomTextHidingNames) hide names explicitly rather than inheriting
```

with:

```go
// messaging.ParticipantSight + messaging.HideNames) and the room line (via
// rooms.Room.SendTextHidingNames) hide names explicitly rather than inheriting
```

- [ ] **Step 5: Delete the helpers and fix the comments that name them**

```bash
git rm internal/mobcommands/darkness.go internal/mobcommands/audio_room_text_sight_test.go
```

In `internal/mobcommands/go.go` replace:

```go
// check). What remains hand-rolled is darkness.go's sendAudioRoomText, still
// two-tier by construction for the four speech commands (say.go, shout.go,
// rally.go, warcry.go) that call it directly.
```

with:

```go
// check). The last one, darkness.go's two-tier sendAudioRoomText, was deleted
// by sight gates slice 5b: speech, rally, warcry, howl and taunt now hide
// names through rooms.Room.SendTextHidingNames at every tier.
```

In `internal/mobcommands/taunt_store_test.go` replace:

```go
// inherit and must build by hand: sendAudioRoomText delivers on the AUDIO
```

with:

```go
// inherit and must build by hand: rooms.Room.SendTextHidingNames delivers on the AUDIO
```

In `m2_routing_guard_test.go` delete the dead recogniser:

```go
			if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "sendAudioRoomText" && len(v.Args) >= 5 {
				// (room, mob, cat, anonMsg, fullMsg, excluded...)
				add("observer", v.Args[2], v.Args[4])
				return true
			}
```

In `send_trio_only_guard_test.go` replace:

```go
// such as sendAudioRoomText/sendMovementMessage -- ever carries one of these
```

with:

```go
// such as sendMovementMessage (or the since-deleted sendAudioRoomText) -- ever carries one of these
```

and, after the paragraph ending `...this note is what supersedes it.` (just above `var sendTrioOnlyCategories`), add:

```go
//
// Sight gates slice 5b (2026-09-29): mobcommands/darkness.go's
// sendAudioRoomText and sendAudioRoomTextHidingNames are deleted, and
// Actor.SendRoomCommunication (actor_mob.go:52 above) with them. The Taunt*,
// Rally, Warcry, Shout and Speech categories now leave through
// rooms.Room.SendTextHidingNames and SendCommunicationHidingNames, still not
// SendTrio, so sendTrioOnlyCategories is unchanged. The survey bullets above
// stay as the record of their dates.
```

- [ ] **Step 6: Run the tests**

Run:
```bash
grep -rn "sendAudioRoomText" --include=*.go internal modules .
gofmt -l internal/ .
go build ./...
go test ./internal/mobcommands/ -count=1
go test . -run 'TestM2|TestNarrationTrioOnlyCategoriesLeaveOnlyThroughSendTrio|TestNarrationSitesMatchViewpointAudit' -count=1
```
Expected: the grep finds only comments in `internal/mobcommands/go.go` and `send_trio_only_guard_test.go` (the historical survey bullets and the note just added), and no code (the same grep matched the ten callers, four hiding callers and both definitions before this task, so it can match); gofmt prints nothing; everything `ok`, including `predator_test.go`'s dark-routing tests and the new howl test.

- [ ] **Step 7: Commit**

```bash
git add internal/mobcommands/howl.go internal/mobcommands/taunt.go internal/mobcommands/go.go internal/mobcommands/taunt_store_test.go internal/mobcommands/speech_sight_test.go m2_routing_guard_test.go send_trio_only_guard_test.go
git commit -m "refactor(mobcommands): howl and taunt hide names at every tier; delete the two-tier darkness helpers"
```

(The `git rm` in Step 5 already staged the two deletions.)

---

### Task 10: Delete `Actor.SendRoomCommunication`

**Model:** haiku (compiler-driven removal).

**Files:**
- Modify: `internal/actions/actor.go`, `actor_user.go`, `actor_mob.go`; the nine fakes (F17); `bauble_finder_view_guard_test.go`

- [ ] **Step 1: Delete the interface method first and let the compiler enumerate**

In `internal/actions/actor.go` delete:

```go
	// SendRoomCommunication broadcasts a communication (say/shout/etc.) to
	// the room. Some clients suppress these messages based on deafen settings;
	// this variant goes through the communication pipeline rather than the raw
	// text pipeline. excludeSelf works the same as the deleted SendRoomText.
	SendRoomCommunication(msg string, excludeSelf bool)

```

In `internal/actions/actor_user.go` delete:

```go
func (a *UserActor) SendRoomCommunication(msg string, excludeSelf bool) {
	if excludeSelf {
		a.Room.SendTextCommunication(msg, a.User.UserId)
	} else {
		a.Room.SendTextCommunication(msg)
	}
}

```

In `internal/actions/actor_mob.go` delete:

```go
// SendRoomCommunication broadcasts NPC speech to the room. Mobs do not
// respect client-side mute/deafen settings; the broadcast is sight-gated
// via the messaging pipeline.
func (a *MobActor) SendRoomCommunication(msg string, excludeSelf bool) {
	if a.Room == nil {
		return
	}
	a.Room.SendTextVisual(messaging.CategoryNPCDialogue, msg)
}

```

- [ ] **Step 2: Delete the nine fakes' method**

Delete the one `SendRoomCommunication` line from each of: `internal/actions/consider_test.go:54`, `economy_test.go:53`, `forage_test.go:73`, `salvage_test.go:74`, `scan_test.go:68`, `search_test.go:72`, `sleep_test.go:39`, `track_test.go:69`, `internal/hooks/spell_foldanchor_test.go:47`. Then run `gofmt -w` on those nine files (a deleted line can realign its neighbours).

In `bauble_finder_view_guard_test.go` replace:

```go
	"SendTextToExits": true, "SendRoomCommunication": true, "SendTrio": true, "SendCounterTrio": true,
```

with:

```go
	"SendTextToExits": true, "SendTrio": true, "SendCounterTrio": true,
```

- [ ] **Step 3: Verify**

Run:
```bash
grep -rn "SendRoomCommunication" --include=*.go internal modules .
go build ./... && go vet ./internal/actions/ ./internal/hooks/
go test ./internal/actions/ ./internal/hooks/ -count=1
go test . -run 'TestFinderView' -count=1
```
Expected: the grep prints nothing (it printed 15 lines before this task, so it can match); build and vet clean; everything `ok`.

- [ ] **Step 4: Commit**

```bash
git add internal/actions/actor.go internal/actions/actor_user.go internal/actions/actor_mob.go internal/actions/consider_test.go internal/actions/economy_test.go internal/actions/forage_test.go internal/actions/salvage_test.go internal/actions/scan_test.go internal/actions/search_test.go internal/actions/sleep_test.go internal/actions/track_test.go internal/hooks/spell_foldanchor_test.go bauble_finder_view_guard_test.go
git commit -m "refactor(actions): delete the dead Actor.SendRoomCommunication"
```

---

### Task 11: The re-fork guard

**Model:** sonnet.

**Files:**
- Create: `speech_wrapper_guard_test.go` (repo root)

- [ ] **Step 1: Write the guard**

```go
package main

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Sight gates slice 5b moved every speech and emote rule into shared bodies
// in internal/actions (Say, Shout, SendHeard, SendSeen). Before that, the
// player and mob wrappers each sent their own room line, and they had forked:
// player speech named the speaker in any light, mob speech was two-tier with a
// lit-room shortcut that named the speaker to a blinded listener, and mob
// shouts neither revealed the shouter nor carried their words next door.
// These tests fail if a wrapper sends a room line, hides a name, judges sight,
// reveals, walks the neighbours or wakes sleepers itself again.

var speechWrapperFiles = []string{
	"internal/usercommands/say.go", "internal/mobcommands/say.go",
	"internal/usercommands/shout.go", "internal/mobcommands/shout.go",
	"internal/usercommands/rally.go", "internal/mobcommands/rally.go",
	"internal/usercommands/warcry.go", "internal/mobcommands/warcry.go",
	"internal/usercommands/emote.go", "internal/mobcommands/emote.go",
}

var speechWrapperForbidden = regexp.MustCompile(`SendTextCommunication|sendAudioRoomText|HideNames|HideSpeakerNames|ParticipantSight|TransitionToRevealing|ForEachAdjacentRoom|OnSleeperWoken|SendTextVisualHidingNames|room\.SendText\(|room\.SendTextVisual\(`)

// speechWrapperRequired is the shared body each wrapper must call, so a
// wrapper that simply stops sending a room line fails too.
var speechWrapperRequired = map[string]*regexp.Regexp{
	"internal/usercommands/say.go":    regexp.MustCompile(`actions\.Say\(`),
	"internal/mobcommands/say.go":     regexp.MustCompile(`actions\.Say\(`),
	"internal/usercommands/shout.go":  regexp.MustCompile(`actions\.Shout\(`),
	"internal/mobcommands/shout.go":   regexp.MustCompile(`actions\.Shout\(`),
	"internal/usercommands/rally.go":  regexp.MustCompile(`actions\.SendHeard\(`),
	"internal/mobcommands/rally.go":   regexp.MustCompile(`actions\.SendHeard\(`),
	"internal/usercommands/warcry.go": regexp.MustCompile(`actions\.SendHeard\(`),
	"internal/mobcommands/warcry.go":  regexp.MustCompile(`actions\.SendHeard\(`),
	"internal/usercommands/emote.go":  regexp.MustCompile(`actions\.SendSeen\(`),
	"internal/mobcommands/emote.go":   regexp.MustCompile(`actions\.SendSeen\(`),
}

// speechGuardCode is path's Go source with every comment removed: parsed
// without ParseComments and printed back, so a comment that names a
// forbidden word cannot fail the guard and one that names a required call
// cannot pass it.
func speechGuardCode(path string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestSpeechWrappersDoNotReFork(t *testing.T) {
	for _, path := range speechWrapperFiles {
		code, err := speechGuardCode(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if loc := speechWrapperForbidden.FindStringIndex(code); loc != nil {
			t.Errorf("%s handles speech sight or delivery itself (%q); call the shared body in internal/actions instead",
				path, code[loc[0]:loc[1]])
		}
		if !speechWrapperRequired[path].MatchString(code) {
			t.Errorf("%s no longer calls %s", path, speechWrapperRequired[path])
		}
	}
}

// speechGuardWalk calls fn with the comment-free code of every production Go
// file under internal/ and modules/.
func speechGuardWalk(t *testing.T, fn func(rel, code string)) {
	t.Helper()
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			code, perr := speechGuardCode(path)
			if perr != nil {
				// A syntax error is the compiler's to report, and another
				// root test may create and remove a scratch file mid-walk.
				return nil
			}
			fn(filepath.ToSlash(path), code)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}

// The two deafen-marked senders carry player chatter only. A caller outside
// the rooms package that defines them and the actions package that owns the
// shared bodies could mark NPC speech as chatter, which ruling 6 forbids.
func TestOnlyTheSharedBodiesSendPlayerChatter(t *testing.T) {
	pattern := regexp.MustCompile(`SendCommunicationHidingNames|SendVisualCommunicationHidingNames`)
	seenInActions := false
	speechGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if strings.HasPrefix(rel, "internal/actions/") {
			seenInActions = true
			return
		}
		if !strings.HasPrefix(rel, "internal/rooms/") {
			t.Errorf("%s calls a deafen-marked sender; only internal/actions may", rel)
		}
	})
	if !seenInActions {
		t.Fatal("no call found in internal/actions: the pattern cannot match, so this guard proves nothing")
	}
}

// actions.Say owns the say room line. A second formatter call is a second
// room line: hooks/justice_wiring.go, actions/sell.go and
// usercommands/offer.go each sent one until this slice.
func TestSayRoomLineHasOneFormatter(t *testing.T) {
	pattern := regexp.MustCompile(`FormatSayText\(`)
	seen := false
	speechGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if rel == "internal/actions/say.go" {
			seen = true
			return
		}
		t.Errorf("%s formats a say line itself; call actions.Say", rel)
	})
	if !seen {
		t.Fatal("FormatSayText not found in internal/actions/say.go: the pattern cannot match")
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test . -run 'TestSpeechWrappersDoNotReFork|TestOnlyTheSharedBodiesSendPlayerChatter|TestSayRoomLineHasOneFormatter' -count=1 -v`
Expected: all three PASS.

- [ ] **Step 3: Prove each test can fail**

One at a time, make the change, run the same command, see the named failure, then undo with `git checkout -- <file>`:
1. In `internal/mobcommands/rally.go` add a line `room.SendText(messaging.CategoryRally, "x")` before `return true, nil`: expect `TestSpeechWrappersDoNotReFork` to name `room.SendText(`.
2. In `internal/usercommands/emote.go` change the final `actions.SendSeen(` call to `_ = actor` plus a `room.SendTextVisual(messaging.CategoryEmote, "x", user.UserId)`: expect both the forbidden and the required failure.
3. In `internal/hooks/justice_wiring.go` add `_ = actions.FormatSayText("", "", false, "", "")`: expect `TestSayRoomLineHasOneFormatter` to name the file.
4. In `internal/hooks/justice_wiring.go` add `room.SendCommunicationHidingNames(0, "", nil)` inside the closure: expect `TestOnlyTheSharedBodiesSendPlayerChatter` to name the file.
5. Add `// actions.Say(` as a comment to `internal/mobcommands/say.go` after replacing its `actions.Say(` call with `_ = mob`: expect the required failure (the comment must not satisfy it).

Record the five failure messages in the commit body, and confirm `git status` is clean of the probes before committing.

- [ ] **Step 4: Commit**

```bash
git add speech_wrapper_guard_test.go
git commit -m "test(guard): speech and emote wrappers must not re-fork the shared bodies"
```

---

### Task 12: Docs, full gate, boot check

**Model:** sonnet.

**Files:**
- Modify: `internal/actions/context.md`, `internal/rooms/context.md`, `internal/messaging/context.md`, `internal/events/context.md`, `internal/hooks/context.md`, `internal/mobcommands/context.md`, `internal/usercommands/context.md`, `docs/PATCH_NOTES.md`, `docs/README.md`

- [ ] **Step 1: context.md, every symbol verified first**

Before naming a symbol, confirm it with `Select-String -Path internal\<pkg>\*.go -Pattern '^(func|type|const|var)\s'` (or `codegraph_search`).
- `actions`: remove `SendRoomCommunication` from the `Actor` block (`:34`) and the `MobActor` paragraph (`:82-84`); add a "Speech and emotes (sight gates 5b)" section: `Say` sends the room line, `Shout`/`ShoutResult`, `SendHeard`, `SendSeen`, the private `sendSpoken`, the deafen split (player chatter yes, NPC no), `merchantSay` now only calls `Say` (so does `usercommands/offer.go`'s); the `Social` files row at `:1672` gains `shout.go`, `room_lines.go`; name `speech_wrapper_guard_test.go`.
- `rooms`: after the `SendTextVisualHidingNames` paragraph (`:23`), add `SendTextHidingNames` (takes a `messaging.NameHider`, no lit shortcut, unfiltered), `SendCommunicationHidingNames` (player speech, deafen-marked), `SendVisualCommunicationHidingNames` (player free-form emote, deafen-marked); `SendTextCommunication`'s two remaining callers.
- `messaging`: `NameHider`, `HideSpeakerNames` (tagged name only, "a figure" / "someone", why the words are never touched) beside `HideNames` (`:338-360`) and in the files table (`:461`).
- `events`: `Message.HiddenFromDeafened`, `DrainQueuedMessageEventsForTest`, `DrainQueuedRoomMessagesForTest`.
- `hooks`: the deafen checks call `HiddenFromDeafened`; `justice_wiring.go` only calls `actions.Say`.
- `mobcommands`: `darkness.go` is gone; say, shout, rally, warcry, emote are thin wrappers over the shared bodies; howl and taunt hide names through `rooms.Room.SendTextHidingNames`.
- `usercommands`: say, shout, rally, warcry, emote are thin wrappers; the free-form emote is seen by sight and deafen-filtered, the empty and alias forms are not filtered; `offer.go`'s `merchantSay` (offer and appraise) only calls `actions.Say`.

Run `python tools/context_md_audit.py` and expect no phantom symbol in these seven packages.

- [ ] **Step 2: Patch notes**

Add at the top of `docs/PATCH_NOTES.md`, above 5a's `## 2026-09-29: Cursed gear and the dark` (player-facing, no numbers, no dashes; the heading takes the date the PR merges, shown here as 2026-09-30):

```markdown
## 2026-09-30: Voices in the dark

- In a dark room you now hear who is speaking only as well as you can see
  them. If you can make out shapes, a speaker is "a figure". If you see
  nothing, or you have been blinded, a speaker is "someone". You always hear
  the words.
- This is the same for other players and for creatures, shopkeepers and
  quest givers.
- A hidden creature that shouts gives itself away, just as you would.
- When a creature shouts in the next room, you now hear what it shouts.
- A rally or a warcry can be heard in the dark, even when you cannot see who
  let it out.
- Emotes are seen, not heard. If you cannot see, you do not see someone's
  emote, and in the gloom you only see "a figure" do it.
```

- [ ] **Step 3: `docs/README.md`**

Add a row for this plan if the parent commit did not, and a row for the new repo-root file `speech_wrapper_guard_test.go` ("Sight gates 5b re-fork guard: the ten speech and emote wrappers call the shared bodies and send no room line themselves; only internal/actions sends player chatter; one say formatter").

- [ ] **Step 4: Full gate**

```bash
gofmt -l internal/ modules/ .
go vet ./...
go build ./...
go test ./... -count=1
golangci-lint run --new-from-merge-base=origin/master
```

Expected: gofmt and vet print nothing; every package `ok`; lint `0 issues`. A root guard keyed by `file|literal` or by line (`TestNarrationSitesMatchViewpointAudit`, `TestPlayerConditionsTravelTheEventPath`, `TestFinderViewReachesOnlyItsReader`, `TestNoRawEventsMessageOutsidePipeline`, `TestM2RoutingIsFrozen`) that reports a moved site is re-keyed to the new key in this commit, never deleted. A failure unrelated to this slice is compared against a detached master worktree (`git worktree add --detach C:/tmp/dogmud-5b-base origin/master`), reported, and not fixed here.

- [ ] **Step 5: Boot check (per `dogmud-shipping`)**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp "C:/Users/Calabe Davis/workspace/DOGMud/_datafiles/config.yaml" C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log
grep -c "Server Ready" boot.log
```

Expected: exit 124, then `0`, then `1`. Remove the worktree afterwards (PowerShell `Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check` if Windows holds a lock, then `git worktree prune`). Never stop a server this session did not start.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/context.md internal/rooms/context.md internal/messaging/context.md internal/events/context.md internal/hooks/context.md internal/mobcommands/context.md internal/usercommands/context.md docs/PATCH_NOTES.md docs/README.md
git commit -m "docs(sight-gates-5b): context.md, patch notes and README for speech and emotes in the dark"
```

(Add by name any guard file re-keyed in Step 4.)

---

### Task 13: Playtest and PR

**Model:** opus (judgment on findings).

- [ ] **Step 1: Playtest**

Load `dogmud-playtesting` and follow it: an ephemeral scenario, `--checkout` of this worktree, never kill the owner's server, reports are gitignored so findings go to memory. Write the scenario under the scratchpad (not the repo) modelled on `tools/playtest/scenarios/slice-a-dark-cave.yaml`: room 3101 (unlit cave), roster an `admin` profile (the operator), an `m2-witness` (plain eyes), a `slice-a-infrared` tester, and a second `m2-witness` to be deafened. Every actor quotes lines verbatim. Group goals, each checking the exact line:
1. Confirm dark: everyone types `look`; nobody reads a room description.
2. Player speech: the operator types `say hello there` and `shout help`. Witnesses read `Someone says, "hello there"` and `Someone shouts, "HELP"`; the infrared tester reads `A figure ...`. A name anywhere is the defect.
3. NPC speech: the operator spawns or finds a mob and types `command <mob> say hello there`, then `command <mob> shout intruders`, then `command <mob> emote growls.`. Witnesses hear `Someone says ...` and `Someone shouts ...` and see no emote; the infrared tester reads `A figure ...` for all three.
4. Emotes: the operator types `emote waves.` and `emote beam`. Witnesses read nothing; the infrared tester reads `A figure waves.` and `A figure beams with pride.`.
5. Deafen: the operator types `deafen <second witness>`, then `say testing` and `command <mob> say testing`. The deafened witness reads the NPC line (`Someone says, "testing"`) and NOT the player's.
6. Blinded in a lit room: everyone moves to a lit room next door (any lit room adjacent to 3101); the operator types `setcondition <first witness> 3` (Blinded, three rounds) and at once `say can you hear me` and `command <mob> say can you hear me`. The blinded witness reads `Someone says, ...` twice; the infrared tester, now in light, reads the names.
7. Next door: the operator stays in the lit room, the mob in 3101; `command <mob> shout intruders`. The operator reads `Someone shouts from the <exit> direction, "intruders"`.

Extract every finding to memory.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feature/sight-gates-5b
gh pr create --repo pruuk/DOGMud --base master --head feature/sight-gates-5b --title "feat(sight): speech and emotes follow the listener's sight (parity slice 5b)" --body-file <scratchpad>/pr-body.md
```

Body: the 5b parity table from the spec, the five departures in "Where the spec could not be implemented as written", the changed-lines table, gate results with counts, the boot check, the playtest outcome, the spec and plan paths, and a line that CI minutes are exhausted for September so the local gate and boot check are the merge gate. End with the attribution line. Read back the URL `gh` prints and confirm it says `pruuk/DOGMud`. The owner runs any deploy; do not deploy.

---

## Self-review

- **Spec coverage.** Three-tier speaker rule and `HideSpeakerNames` (T1); `SendCommunicationHidingNames`, `SendTextHidingNames`, `SendVisualCommunicationHidingNames` with the `communication` flag on `sendTextVisualJudgedBy` and its four callers passing false (T3); `actions.Say` owning the room line, player deafen-filtered and NPC not (T5); `actions.Shout` with reveal (rider 5), adjacent line with words, wake (T6); rally and warcry heard through `SendHeard`, fold lines included (T7); emotes through `SendSeen`, player free-form and `@` deafen-filtered, empty and alias not, mob never (T8); howl and taunt moved and the two helpers deleted, `audio_room_text_sight_test.go` ported (T9); `Actor.SendRoomCommunication` and its nine fakes deleted (T10); re-fork guard with the spec's forbidden set and the outside-rooms-and-actions clause, proven able to fail (T11); T3, T4, T5 guard items (T3, T4, T9, T10); context.md for every touched package, patch notes, gate, boot, playtest, PR (T12, T13). The per-listener table covers say, shout, rally and warcry for a player and a mob speaker and the emote forms (T4, T5, T6), with the `@` form, deafen split and alias at the wrapper (T8), five listeners each forced by lamp or perception machine and asserted before sending.
- **Beyond the spec, stated above.** The `NameHider` parameter; `HideSpeakerNames` hiding the tagged name only; the justice and both merchant callers (`actions/sell.go`, `usercommands/offer.go`); `Message.HiddenFromDeafened` and the drain helpers; the player's adjacent shout staying on `SendTextCommunication`; the line-neutral rally/warcry edits for the line-keyed guard; the extra `FormatSayText` guard.
- **Names across tasks.** `NameHider`, `HideSpeakerNames`, `speakerNoun`, `longestFirst` (T1); `Message.HiddenFromDeafened`, `DrainQueuedMessageEventsForTest`, `DrainQueuedRoomMessagesForTest` (T2); `SendTextHidingNames(cat, txt, names, hide, excl...)`, `SendCommunicationHidingNames(cat, txt, names, excl...)`, `SendVisualCommunicationHidingNames(cat, txt, names, excl...)`, `sendAudioHidingNames`, `sendTextVisualJudgedBy(lighting, cat, txt, names, communication, excl...)` (T3); `SendHeard(actor, cat, text)`, `SendSeen(actor, cat, text, chatter)`, `sendSpoken(actor, room, cat, line, stillHidden)` (T4); `Say(actor, text) SayResult` (T5); `Shout(actor, text) ShoutResult`, `wakeSleepers(actor, room)` (T6). Test helpers `newSpeechScene`, `speechHeard`, `speechExpectOne`, `checkSpeech`, `speechWant` (actions), `speechWrapperScene`, `speechWrapperHeard`, `blindForSpeechTest` (usercommands), `mobSpeechRoom`, `mobSpeechHeard` (mobcommands) are defined once and used consistently.
