# PR #208 review: rifts, correctness lens

Blind reviewer `rifts:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I reviewed the rift framework (internal/rifts, modules/rifts, rooms/ephemeral_owned.go, routing_hooks.go) for correctness. I found four real issues. The most serious: when a room is torn down its id gets reused, but the parent room's door still points at that id. A party member who stays behind can then walk through that door into an unrelated room deeper in the rift, which gets around locked and puzzle-sealed doors. The other three: a player can avoid the death penalty by dropping their connection just before dying; the lost-items record is saved to disk at a different moment from the player save, so a crash can duplicate or destroy items; and rooms written by the model are saved inside the tracked world-content folder, with no gitignore entry. The assorted fixes in the same commit (crafting output count, salvage share, flee routing, mob go routing, picklock, targeting, blind resync, volley hits) looked correct on reading, with no new bugs found.

## Coverage

What I read in full at PR head: internal/rifts runtime.go, router.go, instance.go, events.go, lost.go, keys.go, hunter.go, portal.go, puzzle.go, roll.go; the trigger, bank and validation parts of gen.go (about lines 95-110, 520-540, 590-610, 640-660, 700-937); data.go Load/validate (about lines 255-500, 677-763); modules/rifts/rifts.go; internal/rooms/ephemeral_owned.go; and the diffs to ephemeral.go and routing_hooks.go. From the commit's assorted fixes, I read the diffs to flee.go, move.go, mobcommands/go.go and cast.go, craft.go, crafting.go, salvage.go, NewRound_UserRoundTick.go, NewRound_MobRoundTick.go, NewRound_DoCombat_helpers.go, mob_area_harm.go, spell_resolution.go, spells.go, targeting/select.go, picklock.go, characters/sight.go and validate.go, behaviortree actions_vanish.go and conditions_player.go, mobs.go, and main.go. I also checked the surrounding death, corpse, respawn and despawn hooks and plugins.WriteBytes. Not reviewed in depth: memory.go (the lens puzzle; I only checked its solve and dark guards), lore.go apart from ClaimRubble, most of the prompt-building and reply-parsing code in gen.go and modules/rifts/gen.go and prompt.go (the consent and security reviewers cover those), the roomlife and forager changes, the tests, and the world YAML content. One inconsistency I noted but did not report because it predates the PR: the shop crafter paths in internal/mobs/crafter.go still loop on the raw recipe.Output.Quantity rather than the new RecipeSpec.OutputCount().

## Findings (4)

<a id="f006"></a>
### F006 [high] Removed rift room's id is reused while a live parent's door still points at it: party member walks into an unrelated room, past locked and sealed doors

`internal/rifts/router.go:78` · status **confirmed** · reported as high

When a rift room is torn down (Run.sweep -> OwnedChunk.RemoveRoom), its chunk slot is freed. OwnedChunk.AddRoom always hands out the lowest free slot, so the next room built gets the same id. Nothing resets the parent's Door.DestRoomId or room.Exits[name].RoomId when a child is removed: DestRoomId is written only once, in expand. The Router's only staleness check is `door.DestRoomId == 0 || rooms.LoadRoom(door.DestRoomId) == nil`, and a reused id passes it because a different room now loads under it. The parent room stays alive whenever another party member is standing in it (removable returns false while the room has players).

**Failure scenario.** P1 and P2 are partied in rift room A. P1 goes through A's `north` door into B, where B is entered and expanded, then walks on into B's child D. On the next NewRound, B is removed: it was entered, it is now empty, and there is no going back. That frees B's slot, and B's unentered siblings are removed too. P1 walks on, and expand(D) builds new rooms. AddRoom gives the first of them B's old id. It may be a boss room, a room behind a puzzle-sealed door, or the exit room behind a locked Facet door. P2, still in A, types `north`. Router finds A's door unlocked or unlocked-before, LoadRoom(DestRoomId) succeeds, and P2 lands in that unrelated room. That room's own parent door is never checked, and OnRoomChange spends no key because toRR.ParentRoomId != fromRoomId. `look north` from A also describes the wrong room.

**Suggested fix.** When sweep removes a room with ParentRoomId != 0, clear the parent door that led to it: set DestRoomId = 0 and point the parent's room.Exits[ViaDoor] back at the parent itself, so the door reads as 'unsettled'. Alternatively, have the Router check that run.Rooms[door.DestRoomId].ParentRoomId == fromRoomId && ViaDoor == exitName before routing. Another option is to never reuse a slot within a run's lifetime.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
ephemeral_owned.go:92-98 `for i, used := range c.slots { if !used { slot = i; break } }` then `r.RoomId = ephemeralRoomIdMinimum + (c.id * ephemeralChunkSize) + slot`. ephemeral_owned.go:137 `c.slots[slot] = false` on RemoveRoom. runtime.go:487 `door.DestRoomId = child.RoomId` is the only write (grep DestRoomId in internal/rifts finds router.go:78, 81 and runtime.go:84, 487 only). runtime.go:553-557 sweep deletes run.Rooms[id] / runByRoom[id] without touching the parent's Door. router.go:78-81 `if door.DestRoomId == 0 || rooms.LoadRoom(door.DestRoomId) == nil { refuse unsettled } return ExitRoute{RoomId: door.DestRoomId}`. events.go:79 `if fromRun == toRun && fromRR != nil && toRR.ParentRoomId == fromRoomId { spendKeyOnDoor }`.
```

- **confirmed** (high): The claim holds as written. OwnedChunk.AddRoom always takes the lowest free slot, and RemoveRoom clears that slot, so a removed room's id goes to the next room built in the same chunk. A parent's Door.DestRoomId and room.Exits[name] are written only once, in expand, and Run.sweep never resets them when it removes a child. The router's only staleness check is `DestRoomId == 0 || LoadRoom(DestRoomId) == nil`, and a reused id passes it.  The parent room can stay alive because removable() returns false while anyone is in it. B, by contrast, is removable once it has been entered and is empty, and its unentered siblings go with it in the same sweep pass.  There is no other check that would catch the move. EntryGuard only checks run membership, and P2 is already a member, so the move into the wrong room is allowed. That room's own parent door, with its lock or puzzle seal, is never consulted. OnRoomChange spends no key because toRR.ParentRoomId != fromRoomId.  The sequence works as described. P1 goes from A into B into D: expand(D) runs on entry, then a later sweep removes B and frees its slot. P1 walks into E, and expand(E) calls AddRoom, which reuses the freed slot, possibly B's. P2, still in A, then takes `north` into an unrelated room, which may sit behind a lock or a sealed door, or be the boss or exit room. Whether B's id is the one reused depends on which slots are lowest, but some freed id from that sweep will be reused, so the bug is real. `look north` is routed through the same Router, so it would describe the wrong room too.

- **confirmed** (high): Every link in the chain holds when I read the code, and nothing elsewhere prevents it. OwnedChunk.AddRoom always takes the lowest free slot, and RemoveRoom frees that slot. Run.sweep removes a room that was entered and is now empty, even while its parent is still alive. The parent's Door.DestRoomId and room.Exits[name].RoomId are written only in expand and never cleared. The Router's only staleness check is that DestRoomId is non-zero and LoadRoom on it succeeds, and a reused id passes both. EntryGuard does not catch it either: it returns true immediately when run.Members[userId] is set, and a party member standing in A is already a member. So P2 is placed in whatever room now holds B's old id, which may be a sealed or locked or boss room. No key is spent because toRR.ParentRoomId != fromRoomId. The reused id is registered to the same run (runByRoom[roomId] = run in buildRoom), so RoomInfo reports it as an ordinary room of this run. One small inaccuracy in the claim: expand(D) runs when P1 enters D, before B is swept. The reuse therefore comes from a later expand, for example P1 entering E beyond D. The defect stands either way. It needs the party to split, with one member waiting in A while another goes at least two rooms deeper. That is ordinary play, and the effect is to bypass locks and seals, so high severity is reasonable.

- **confirmed** (medium): The mechanism holds. OwnedChunk.AddRoom always takes the lowest free slot. RemoveRoom frees the slot. The parent's Door.DestRoomId and room.Exits[name].RoomId are written only in expand (runtime.go:487-490) and never cleared when sweep removes the child (runtime.go:545-562). Router's only staleness check (router.go:78) is `DestRoomId == 0 || LoadRoom(DestRoomId) == nil`, and a reused id passes it. EntryGuard (router.go:143) lets the move through because P2 is already a run member. OnRoomChange spends no key because toRR.ParentRoomId != fromRoomId (events.go:79), and the destination's own door state is never consulted. So a party member left behind in a live parent room can take a stale door into whatever new room got that id: possibly behind a locked door, past a sealed puzzle, or a boss room. `look` from that exit describes the wrong room.  The scenario overstates a few details: (1) B's unentered siblings are NOT removed while A is alive. removable returns false for an unentered room whose parent is alive. B's own children (D's siblings) are what get removed. (2) expand(D) runs in OnRoomChange when P1 enters D. That is before B is swept on the next NewRound, so the reuse happens on a later expand, at E or beyond. (3) Lower freed slots, such as A's already-swept ancestors, are taken before B's slot. So B's id is not necessarily the first one reused. Within a few expansions it will be, since each expand builds several rooms.  None of this changes the outcome. It needs a split party, and when the party splits it is likely. The impact is a sequence break and bypass of locks, puzzles and keys inside one run's own instance. It is not cross-run or cross-player, and not a data-integrity or security issue, so high is somewhat overstated. Medium fits.

</details>

<a id="f040"></a>
### F040 [medium] Dropping the connection before a rift death avoids the gold loss and the worn-gear loss

`internal/rifts/events.go:160` · status **confirmed** · reported as medium

OnPlayerDeath charges a player who is linkdead (zombie connection) only the logout cost: loose carried items. A real death also takes all carried gold (gold_on_death: true in obelisk.yaml) and each worn item at worn_chance (25). The player controls whether they are linkdead, so a player who is losing a fight in a rift can close the client and let the zombie die. They keep all their gold and all their worn gear. Leaving deliberately is therefore always cheaper than dying.

**Failure scenario.** A player carrying 5000 gold and wearing valuable gear is losing to a rift boss (doors sealed, no flee route because the hunter holds them). They kill their telnet client. The character stays in the world as a zombie, keeps getting hit and dies. linkdead(u) is true, so forfeitOnLogout runs: loose pack items go, but Gold is untouched and no worn item is rolled. Dying while connected would have cost all 5000 gold and about 25% of their worn gear.

**Suggested fix.** Charge a linkdead death in a rift at the full death rate. If leniency for a genuine disconnect is wanted, decide it by how long the link has been dead (for example, dead longer than N rounds before the killing blow), not just by whether it is dead.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
events.go:160-164 `if linkdead(u) { forfeitOnLogout(u, run.Profile); return }; forfeitOnDeath(u, run.Profile)`. lost.go:165 worn removal only `if how == "death"`; lost.go:179 gold only `if how == "death" && l.GoldOnDeath`. lost.go:277-279 linkdead = users.IsZombieConnection(...). _datafiles/world/dogmud/rifts/profiles/obelisk.yaml:225-227 `worn_chance: 25`, `gold_on_death: true`.
```

- **confirmed** (medium): The code reproduces the claim as written. OnPlayerDeath (internal/rifts/events.go:150-165) sends a linkdead player to forfeitOnLogout and returns before forfeitOnDeath. linkdead is users.IsZombieConnection on the player's connection (lost.go:277-279). In forfeit (lost.go:150-200), the worn-item roll only runs when how == "death" (line 165) and so does the gold take (line 179), so the logout path takes only loose carried items.  Disconnecting is entirely up to the player. A dropped connection leaves the character in the world as a zombie for ZombieSeconds (60 in _datafiles/config.yaml), and the PlayerDeath event still reaches the rifts module with no zombie filter (modules/rifts/rifts.go:207-211). While a zombie, the character does nothing but run zombieact (NewRound_UserRoundTick.go:240), which only emotes, so a hostile keeps hitting it and it can die.  If it survives the 60 seconds, CleanupZombies despawns it and OnPlayerDespawn also charges forfeitOnLogout. So once a player disconnects, the cost tops out at the logout cost either way: no gold loss and no worn-item rolls. The comment at events.go:157-159 shows the logout-cost rule was meant for genuine connection drops, but nothing tells a genuine drop from a deliberate one, so the claimed exploit (kill the client while losing a rift fight, keep the gold and worn gear) holds.  Medium is the right severity. It is an economy and penalty bypass, not a crash or a data leak.

</details>

<a id="f094"></a>
### F094 [low] Rooms written by the model are saved into the tracked world-content folder, with no gitignore entry

`internal/rifts/gen.go:725` · status **confirmed** · reported as low

addGenerated saves each room the model writes as <DataFiles>/rifts/rooms/<profile>/gen-*.yaml. DataFiles is _datafiles/world/dogmud, the git-tracked authored content tree. These files are living state, written at runtime on prod, yet unlike users/, plugin-data/, shops/ and baubles/ they have no .gitignore entry. They sit next to authored templates in the same folder.

**Failure scenario.** On the prod droplet every generation leaves an untracked file in the deploy checkout, so the tree is dirty at deploy time. A clean or reset step during deploy, or a rebuilt image, silently deletes the generated room bank. In development, a careless commit sweeps them into the repo.

**Existing mechanism.** _datafiles/**/plugin-data (module living state, gitignored), which this module already uses for portal-sites and lost-items

**Suggested fix.** Write generated rooms under the module's plugin-data folder, or another gitignored living-state folder, and load them from there. At minimum, add `_datafiles/**/rifts/rooms/**/gen-*` to .gitignore and document the folder as living state.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
gen.go:725 `var genDir = func(p *Profile) string { return filepath.Join(DataDir(), "rooms", p.Id) }`. data.go:365-367 DataDir = configs DataFiles + "rifts". _datafiles/config.yaml:242 `DataFiles: _datafiles/world/dogmud`. .gitignore covers users, plugin-data, shops, warehouses and baubles, but nothing under rifts.
```

- **confirmed** (low): The claim checks out against the code. addGenerated (gen.go:845-862) writes `gen-<pool>-<slug>.yaml` into genDir(p). genDir is filepath.Join(DataDir(), "rooms", p.Id) (gen.go:727), and DataDir is <DataFiles>/rifts (data.go:366). The shipped config.yaml (HEAD blob, line 242) sets DataFiles to _datafiles/world/dogmud. So runtime output lands in _datafiles/world/dogmud/rifts/rooms/obelisk/, which is a git-tracked folder of authored templates. `git check-ignore` on a sample gen-*.yaml path returns rc=1, so the file is not ignored. The PR adds a .gitignore entry for housing/ living state but none for rift generated rooms. The persistence and shipping skills it touched also do not list them as living state.  The module keeps its other runtime state (portal-sites, lost-items) in plugin storage, the gitignored plugin-data folder, so the reviewer's comparison holds. One mitigating point is that docs/README.md calls the gen-* rooms "banked" beside the authored ones, and the gen.go header says they load at every boot "like the rest". So keeping them in that folder may be deliberate, perhaps for later curation. Even so, nothing ignores them and nothing documents them as living state. Untracked files would build up in the prod checkout, and a careless commit or a clean or rebuild during deploy could sweep them in or delete them.  The harm depends on how deploys are run and on generation being switched on, so the severity stays low.

</details>

<a id="f095"></a>
### F095 [low] Lost-items record is written to disk at once while the player save lags: a crash duplicates or destroys items

`internal/rifts/lost.go:194` · status **confirmed** · reported as low

forfeit (on death) and findLost (on rubble search) move an item between a player's inventory and the lost-items record, then call lostChanged(). That writes the record straight to plugin storage through plugins.WriteBytes, which is immediate unless an autosave prepare happens to be collecting at that moment. The player's side of the move is only saved at the next autosave or logout. If the process dies in between, the two files disagree.

**Failure scenario.** Duplication: a player dies in a rift. The record (now holding their items) is written at once. The server crashes before the next user autosave. On restart the player file still holds the items, and the record holds them too, so a later rubble search hands copies to someone else. Loss: findLost gives an item to the searcher and writes the shrunken record. A crash before the player is saved means the item exists nowhere.

**Existing mechanism.** internal/plugins savequeue / autosave prepare (the `collecting` path in plugins.WriteBytes), which rooms and users already save through so their writes land together

**Suggested fix.** Mark the record dirty and write it in the module's OnSave (the same autosave cycle as users), instead of writing on every change. Alternatively, queue the affected user's save in the same step.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
lost.go:193-195 `if len(names) > 0 { lostChanged() }`. lost.go:253-260 findLost removes from lostItems, then lostChanged(). modules/rifts/rifts.go:115 `rifts.SetLostChanged(m.saveLost)` and :151-156 saveLost -> m.plug.WriteStruct. plugins.go:352-377 WriteBytes writes directly unless `collecting != nil`.
```

- **confirmed** (low): The claim checks out against the code for both the death path and the rubble-search path.  - **Death:** OnPlayerDeath (events.go:150-165) calls forfeitOnDeath, which calls forfeit. forfeit removes items from the character in memory and appends them to lostItems, then calls lostChanged() (lost.go:193-195). That callback is m.saveLost, wired in modules/rifts/rifts.go onLoad. saveLost calls plug.WriteStruct, which goes through WriteBytes. WriteBytes writes to disk at once unless `collecting != nil` (plugins.go). Nothing on the death path saves the user, so the player file keeps the items until the next autosave. - **Rubble search:** findLost (lost.go:236-266) is called from lore.go:192. It calls StoreItem on the searcher, removes the item from lostItems, and calls lostChanged(), which writes at once. Again no SaveUser follows.  A crash between that write and the next user autosave leaves the two files disagreeing: - After a death, the item is in both the player file and the lost record, so it can be duplicated. - After a search, the item is in neither file, so it is lost.  **Logout path:** on logout, OnPlayerDespawn runs forfeitOnLogout. The PlayerDespawn_HandleLeave hook then calls users.SaveUser (line 173), so that window is small. The claim does not cite logout, so this does not weaken it.  **Existing pattern:** the auctions module shows how the codebase already handles cross-file transfers: it calls users.SaveUser right after a transfer (auctions.go:554, 597). The rifts code follows neither that nor the savequeue batching.  **Severity:** it stays low. It needs a process crash inside the autosave interval, and the result is a duplicated or lost item, not a broader corruption.

</details>
