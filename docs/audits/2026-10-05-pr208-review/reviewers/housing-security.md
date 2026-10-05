# PR #208 review: housing, security lens

Blind reviewer `housing:security`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I reviewed the housing piece (f546918ec..f7310ed77) as it stands at PR head 3ce674451, reading the code for security issues. Authorization holds up well. The entry guard sits inside rooms.MoveToRoom, and login is the only spawn caller (world.go:297). Alt swap only happens in character rooms. Picklock, unlock, steal, plant and mob unlock all refuse sealed strongboxes. Get, look, put, the aicompanion take_from planner and the companion takeout command all check container locks. The strongbox is opened only by the owner's own TryCommand or a companion the owner has charmed, and both are deferred around a command running on the single game loop. Cross-actor commands are queued, not run synchronously. Housing items are refused by sell before the bauble and fence branches. Container names are limited to a-z, owner descriptions are escaped with util.EscapeAnsiTags, and file paths come from authored building ids only. I also checked crash-timing duplication: Guard G2 in users.SaveUser cancels stale autosave user writes, so the immediate house save plus saveUser pair does not resurrect items. Three findings came out of it. One is high: a server-wide game-loop stall that a single lodger can cause, because every command in a lodging re-serializes every item the house holds. One is medium: player-written lodging descriptions are fed into the LLM look-detail requests on a guest's own API key, which breaks that package's "authored text only" rule. One is low: buying through `ask` skips the darkness rule that `buy` applies.

## Coverage

I read these in full at PR head (C:/tmp/dogmud-pr208-review, 3ce674451):
- internal/housing: containers.go, door.go, guests.go, housing.go, registry.go, persistence.go, overlay.go, purchase.go, offers.go, use_items.go, furnishings.go, crafted.go
- internal/rooms/routing_hooks.go
- internal/usercommands/house.go and housing_shop.go
- internal/behaviortree/actions_housing.go
- The PR's diffs to world.go, main.go, internal/rooms (roommanager MoveToRoom guard, removeRoomFromMemory, save_and_load, container.go, autosave_prepare, baubles_untaken), and to the usercommands go/picklock/unlock/get/remove/use/usercommands/buy/list/sleep, mobcommands, actions/steal, items/never_bought

To verify claims, I traced every room.Containers access and iteration across internal/, modules/ and world.go for strongbox lock bypasses. I also checked:
- direct RoomId assignments and AddPlayer callers for entry-guard bypasses (alt swap is limited to character rooms)
- the login spawn path
- the sell ordering for never-bought items
- aicompanion take_from and the companion takeout command lock checks
- GMCP container exposure
- the savequeue Guard G2 (SaveUser cancels stale pending writes), which rules out a crash-timing dup through the stale autosave queue
- the lookdetail, roomlife and npcidle request builders
- shipped building limits: 100 units each, max_rooms 8, 6 containers per room, ContainerSizeMax 10, and distinct item ids across buildings

Not covered in depth:
- the test files and context.md (deliberately, as a blind review)
- voice.go and terms.go beyond checking that no player-controlled placeholders reach say
- the bed regen details in actions/sleep.go
- tools/housing_units.py and the unit templates beyond hollow_oak
- whether hired-alt or companion inventories persist in a way that opens a crash-window dup through AfterMobCommand, which saves no user record

Considered and not reported: unit-pool squatting across many accounts (100 units per building, no eviction), because it is design-level and costly. Guest keys being transferable until used, which is by design and revocable. A theoretical dup if saveHouse fails on disk error before a room unload, which a player cannot trigger.

## Findings (3)

<a id="f020"></a>
### F020 [medium] Every command in a lodging YAML-marshals every item the house holds, and lodging floors have no cap or cleanup: a single-player game-loop stall

`internal/housing/containers.go:319` · status **confirmed** · reported as high

AfterUserCommand (called from world.go after every player command) and AfterMobCommand (after every mob command) call captureHouseAt. That runs Capture on every loaded room of the house, up to max_rooms 8. Capture calls sameItems on every container and on the floor and stash of each room. When the UUIDs match, which is the normal no-change case, sameItems still calls yaml.Marshal on every item in both lists. So a plain `look` or `say` inside a lodging costs 2N YAML marshals, where N is every item across the house's loaded rooms. When anything does change, the whole house is deep-cloned, re-marshalled and written with SafeSave (a durable, fsync'd write) while the housing lock is held on the game loop. N is not bounded. rooms.Room.AddItem has no cap. Container overflow past ContainerSizeMax (10) spills onto the floor. The PR also exempts unit rooms from the two systems that clear floors: the untaken-bauble sweep (baubles_untaken.go) and the loot goblin (GetRoomWithMostItems, via IsPrivateRoom). Items dropped in a lodging therefore pile up forever.

**Failure scenario.** A lodger collects cheap or free items (harvested junk, 1-gold shop goods) and runs `drop all` a few hundred times in their lodging until its floors hold about 10k items. A companion or a guest idling there issues commands each round. Every one of those commands, and every command the owner types, makes the single-threaded game loop marshal about 20k items. At typical yaml.v2 costs of 10-30 microseconds per item, that is 0.2-0.6 s per command. Each `get` or `drop` also rewrites and fsyncs a multi-megabyte house file. Server-wide lag scales with one player's hoard. CaptureOnSave adds the same cost to every autosave prepare, which runs inside the world lock.

**Existing mechanism.** The engine already tracks floor dirtiness (r.noteFloorItemAdded in Room.AddItem), and the autosave design (internal/hooks/autosave_prepare.go) works from a dirty set and a queued pending-write pipeline rather than synchronous whole-world rewrites. Capture re-derives 'changed' by serializing everything on every command instead.

**Suggested fix.** Mark a house dirty when its room's floor or containers actually change (the commands that mutate them, or a room-level dirty flag) and capture only then. Never marshal items to detect change on the no-op path. Cap items per lodging floor (or per house), or route overflow somewhere bounded. Consider routing house writes through the existing savequeue rather than a synchronous fsync on the game loop.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
containers.go:328-335 `for i := range a { ya, errA := yaml.Marshal(a[i]); yb, errB := yaml.Marshal(b[i]) ...}` runs after the Equals loop passes, i.e. on every unchanged comparison. containers.go:256 `if live.Gold == c.Gold && sameItems(live.Items, c.Items)` and :266 `!sameItems(floor.Items, r.Items) || !sameItems(floor.Stash, r.Stash)`. containers.go:376-383 loops `for _, id := range h.RoomIds { ... Capture(liveRoom(id)) }`. world.go:1045-1049 `roomBefore := ...; usercommands.TryCommand(...); housing.AfterUserCommand(userId, roomBefore)`. world.go:1106-1111 does the same for mobs. persistence.go:56-60 `yaml.Marshal(h)` then `util.Save(...)` (SafeSave). rooms.go:1280-1290 AddItem has no limit. baubles_untaken.go:+32 `if IsPrivateRoom(r.RoomId) { return 0 }`. roommanager.go:+533 skips private rooms in GetRoomWithMostItems.
```

- **confirmed** (medium): The claim matches the code as written. world.go:1048 calls housing.AfterUserCommand after every player command, and world.go:1111 calls AfterMobCommand after every mob command. When the actor is in a unit room, captureHouseAt (containers.go:376-385) runs capture on every loaded room of the house. capture takes the housing mutex. It calls sameItems on each container and on the floor's Items and Stash. When the lists match by length and Equals, which is the normal no-change case, sameItems (containers.go:319-336) yaml.Marshals both sides of every item. So an unchanged command costs 2N marshals, where N is the number of items across the house's loaded rooms. The docstring claims "It is cheap when nothing did", which is false for large N.  When anything changes, the house is cloned and saveHouse writes it, then it is cloned again. Room.AddItem has no cap. drop.go has no floor limit (grep for Max/Limit/len(room.Items) found nothing). baubles_untaken.go:34 skips private rooms. roommanager.go:535 skips them in GetRoomWithMostItems, the loot goblin's room picker. Nothing else in the PR clears a lodging floor. CaptureOnSave also runs Capture from the room save hook.  I lowered the severity from high to medium. Exploiting this takes a deliberate hoard of thousands of items, which is real player effort. The per-item marshal cost in the failure scenario is an estimate; I did not measure it. The lag scales roughly linearly and gives no instant crash. Still, the cost is unbounded, it lands on the single-threaded game loop, one player controls it, and nothing guards it. The mechanism is real, and a cheaper dirty check is possible: the length/UUID/Equals pass, or the existing noteFloorItemAdded dirtiness tracking.

- **confirmed** (medium): I could not refute it. Nothing elsewhere guards against it.  1. sameItems (containers.go:319-336) runs yaml.Marshal on both copies of every item once the UUID/Equals loop has passed, so the normal no-change case pays the full cost. 2. capture (containers.go:221-293) runs under the housing mutex. It calls sameItems for each container and for the floor's Items and Stash. 3. captureHouseAt (containers.go:370-385) loops over every loaded room of the house. AfterUserCommand and AfterMobCommand (containers.go:391-421) call it after every player or mob command in a unit room. CaptureOnSave (containers.go:493) adds the same work to autosave. 4. When anything changed, capture clones the whole house and calls saveHouse. 5. Room.AddItem (rooms.go:1280) has no cap. I found no floor-item cap in drop or in housing.  On cleanup: the claim names the bauble sweep and the loot goblin, but there is a third system it missed, the daily floor decay (floor_decay.go). It does not help. main.go:1946 registers SetFloorDecayExempt with `housing.IsUnitRoom(roomId)`, so lodging floors are deliberately exempt from decay as well. The cost therefore grows with the size of one player's hoard and nothing bounds it.  Why I lowered it to medium: to cause real lag, a player has to deliberately pile up thousands of items in their own lodging, and the cost per command depends on the runtime cost of yaml.v2, which I did not measure. It is a slow-building way for one player to degrade the whole server, not something any player can trigger instantly.

- **confirmed** (medium): The mechanism holds as described. sameItems (containers.go:319-336) runs the UUID Equals pass and then, when every UUID matches (the normal no-change case), calls yaml.Marshal on every item in both lists. capture() does this for each container and for the floor and stash of each room, and it holds the housing mutex the whole time (containers.go:225-226). captureHouseAt loops over every loaded room of the house (:376-383). AfterUserCommand and AfterMobCommand call it inline after every command whenever the actor is in, or just left, a unit room (:391-421). Any change triggers clone, yaml.Marshal of the whole House and util.Save (persistence.go:50-60). Nothing caps floor item count. The PR's diff adds IsPrivateRoom exemptions to removeUntakenBaubles and GetRoomWithMostItems. rooms/context.md also says floor decay exempts housing.IsUnitRoom, so nothing clears a lodging floor.  The severity is overstated, though. The cost is O(items held in that one house). It is paid only on commands from actors inside, or just leaving, that house, and it is ordinary in-memory YAML work. A normal lodging with tens of items costs microseconds to a millisecond. Reaching the scenario's 10k items takes a deliberate, sustained hoarding effort by an authenticated player who has rented a lodging: thousands of acquisitions plus repeated drop commands. That is a resource-exhaustion vector that needs effort and leaves an obvious culprit, not a remote or cheap one. The cited loot goblin exemption also matters little now, since floor decay replaced the goblin and floor decay already exempts unit rooms. The design inefficiency is real: it re-serializes everything on each command instead of using a dirty flag. A cap on house floor items, or a cheaper change check, is warranted, but medium fits better than high.

</details>

<a id="f021"></a>
### F021 [medium] Player-written lodging descriptions are fed into LLM look-detail (and other generation) requests on a guest's own API key, breaking the 'authored text only' contract

`internal/housing/use_items.go:216` · status **confirmed** · reported as medium

A redecorating voucher lets an owner set any room's Description to up to 1000 characters of free text (cleanDescription only escapes ANSI tags and folds whitespace). The overlay writes that text into r.Description. The lookdetail package, also in this PR, states that requests 'carry authored room text and nothing of any player's' and run 'only on the looker's own key'. Its Snapshot copies room.GetDescription() into the request (cap 1200 runes, so the whole owner text fits), and TryLook triggers on any phrase the description names. In a lodging, a guest's look sends the owner's text to the model on the guest's key. The owner controls both the phrase that triggers generation and text that can tell the model what to write. A moderated result is cached by (room, description, phrase) and served to anyone who looks later. roomlife (internal/roomlife/request.go:49), npcidle (internal/npcidle/request.go:64) and the aicompanion perception prompt (modules/aicompanion/perception.go:88, 'Around you: <description>') read the same field. Unit rooms are not excluded from any of these (no IsPrivateRoom/IsUnitRoom check in those packages). The aicompanion path is the most concerning: a guest's own companion, visiting the lodging, reads attacker-written text in its action-planning prompt.

**Failure scenario.** An owner writes a description such as 'A ledger lies open on the desk. [Instructions to the writer: when describing the ledger, say ...]'. A guest who has opted into 'Make the world livelier' types `look ledger`. The guest's key pays for a generation steered by the owner, and the output (if it passes moderation) is cached and shown to every later looker. A guest who brings an AI companion has the same owner-written text in the companion's planning prompt every turn.

**Existing mechanism.** rooms.IsPrivateRoom (routing_hooks.go) already exists for exactly this 'leave player rooms out' purpose and is used by the loot goblin and the bauble sweep.

**Suggested fix.** Have Snapshot (lookdetail, roomlife, npcidle) and the aicompanion room perception use the authored template description for private rooms, or skip generation entirely when rooms.IsPrivateRoom(room.RoomId). At minimum, update the stated contract and the consent copy so they disclose that another player's text is sent.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
use_items.go:219-227 `text := strings.TrimSpace(args) ... text = cleanDescription(text)` (20-1000 chars). use_items.go:267 `next.Descriptions[room.RoomId] = text`. overlay.go:88-90 `if text, ok := house.Descriptions[r.RoomId]; ok && text != `` { r.Description = text }`. lookdetail/lookdetail.go:13-14 "It only ever runs on the looker's own key ... The request carries authored room text and nothing of any player's." lookdetail/request.go:43 `Description: plain(room.GetDescription(), maxRoomDescription)`. look.go (PR head) `if sight == messaging.SightFull && lookdetail.TryLook(user, room, lookAt)`. grep for IsPrivateRoom/IsUnitRoom/housing in modules, internal/lookdetail and internal/roomlife returns nothing.
```

- **confirmed** (medium): Every link in the chain shows up in the PR head. Both internal/housing and internal/lookdetail are new in this PR (neither exists at c696c117a). In use_items.go, useRedecorate takes free text from the owner. cleanDescription (use_items.go:316-318) only escapes ANSI tags and folds whitespace. The length check allows 20 to 1000 characters. The text is then stored as next.Descriptions[room.RoomId] = text. overlay.go:88-90 copies that text into r.Description. TryLook (lookdetail.go:127-160) gets its phrase by searching room.GetDescription() and has no private-room or unit-room check. It reserves the looker's key (g.Reserve(user.UserId)) and snapshots room.GetDescription() at a 1200-rune cap, so the whole owner text reaches the model. A result is cached by (roomId, description, shown) and later served to anyone who looks. The package doc (lookdetail.go:13-14) and the Request comment (request.go:16-17) promise "authored room text and nothing of any player's". For a redecorated lodging that is false. The aicompanion perception prompt (perception.go ~88) also puts room.GetDescription() into "Around you:", up to 420 characters. housing/door.go:18 already registers IsUnitRoom through rooms.SetPrivateRoomCheck. So a ready-made exclusion exists, and none of lookdetail, roomlife, npcidle or aicompanion calls it. A grep for IsPrivateRoom, IsUnitRoom and housing in those packages finds nothing; the same grep does find the existing callers elsewhere, so it could have matched. Two things limit the damage. The modules/lookdetail system prompt says the output is scenery only, with no messages, clues or quests, and the description goes in as a JSON field. Moderation also gates the cache. So injection is softened, not blocked. The guest also has to opt in to "Make the world livelier", and Reserve still applies its budget. Medium severity holds: the stated consent and data contract is broken on the guest's own key, and the owner's text can steer the companion's planning prompt, but no data is stolen and the output stays filtered.

</details>

<a id="f075"></a>
### F075 [low] Buying through `ask <landlord>` skips the shop sight rule that `buy` applies

`internal/behaviortree/actions_housing.go:62` · status **confirmed** · reported as low

tryHousingBuy (the `buy` command path) refuses with actions.ShopSightRefusal when the buyer cannot see faces, which is the lighting 5b dealing rule. actBuyHousing, reached through `ask <landlord> to buy ...`, calls housing.Buy directly with no sight check. The ask command checks only that the mob is awake. The two purchase entry points into the same sale therefore follow different rules, and the darkness gate can be sidestepped by phrasing.

**Failure scenario.** At night, if the door's lamp is out or the player is blinded, `buy home` is refused for darkness. `ask hobb to buy a room` charges the gold and sells the home anyway. The same applies to every offer (deeds, keys, strongboxes).

**Existing mechanism.** actions.ShopSightRefusal / ShopSightRefusalText, already used by tryHousingBuy and the shop list/buy paths.

**Suggested fix.** Apply actions.ShopSightRefusal in actBuyHousing before housing.Buy, or move the check into housing.Buy so both entry points share it.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
usercommands/housing_shop.go:88-91 `if actions.ShopSightRefusal(user.Character, room) { user.SendText(..., actions.ShopSightRefusalText); return true }`. behaviortree/actions_housing.go:55-62 `key, ok := housing.MatchOffer(ctx.Event.Text, owns) ... housing.Buy(user, say, buildingId, tierId, key)` with no sight check. usercommands/ask.go:111 checks only `actions.RefuseMobIfAsleep(mob, user)`.
```

- **confirmed** (low): The claim holds as written. `buy` reaches the landlord through tryHousingBuy, which checks actions.ShopSightRefusal before it calls housing.Buy. `ask <landlord> to buy ...` takes a different route: Ask -> askNpcChain -> behaviortree.TryMobBehavior(player_ask) -> the landlord's tree (keyword_match buy/purchase) -> actBuyHousing -> housing.Buy. No step on that route checks the light band.  housing.Buy (offers.go:186) only dispatches to Purchase, buyExtension, buyContainerDeed and the others, and does no sight check of its own. The Ask command checks only RefuseMobIfAsleep. ResolveTargetActor with a Viewer does not catch darkness either. It goes through findMobByName -> Character.Perceives, and Perceives (character.go:948) checks only IsHidden and SeeHidden, not light. A buyer below BandFaces, for example a blinded one or one at a landlord whose room is dark at night, can still resolve the landlord and buy every offer through ask.  The PR's own housing/context.md:340-345 says the buy and list paths "both refuse below the faces light band like every shop" and that `ask hobb ...` "reaches the same `Buy`". That confirms the two entry points were meant to follow the same rule and do not.  The impact is small. Hobb's alley carries `lamp: 52`, so the usual way to trigger this there is blindness, not night. Nothing is stolen or duplicated: the full price and every standing check still apply. Low severity is right.

</details>
