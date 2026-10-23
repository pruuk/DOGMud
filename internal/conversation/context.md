# conversation: Package Documentation

## Purpose

`internal/conversation` is the game side of talking to NPCs (quest arc #375
phase 1). It turns a player's `talk`, a topic choice, a `give` and a bonded
companion's question into `internal/dialogue` calls, speaks the lines, and
opens the menus through `internal/users`. `internal/dialogue` stays pure; this
package owns everything that needs a user, a room or a mob.

## Files

| File | What it holds |
|---|---|
| `playerstate.go` | `PlayerState(user)`: the `dialogue.PlayerState` callbacks over a character; `removeBackpackItem(user, itemId, prefer)` takes one item, the `prefer` instance when the player holds it (the one a `give` named) or else the first match |
| `marker.go` | `Marker(viewerUserId, mob)`: the viewer's talk marker (`[?]`, `[!]`, `[*]` or empty), empty for a charmed NPC; installed as `rooms.NpcMarker` explicitly at boot by `wireConversationHooks` (`conversation_wiring.go` in `main`) and also by this package's `init`; the room roster and the GMCP `Room.Info` NPC list call it. `RefreshMarkers(userId)` queues `events.NpcMarkersChanged` (unique per player) so a GMCP client re-reads its NPC list |
| `give.go` | `Give(user, room, mob, itm) bool`: `give <item> <npc>` resolved against the NPC's topics before any transfer (R8). A topic wants the item through `requiresItem` (taken) or `carriesItem` (shown, never taken, so no handover line and the item stays with the player); the first eligible one in authored order runs through `resolve`, the same path as a menu choice (an offer shows its card and takes the item only on Accept), and takes the instance the player named (`give 2.purse`); a wanted item with no eligible topic is handed back by `handBack`, which first offers the give to the mob's tree as a `player_give` hand-back (`behaviortree.GiveHandBack`) so an authored refusal written for that item plays ("See Drunn first, then come back."), crediting nothing, and prints the generic "looks at the X and hands it back." line when no branch ran `return_item`; the item never moves; false (nothing changed) when no topic wants it or the NPC is charmed |
| `companion.go` | `CompanionTopic(user, mob, what)`: resolves the owner's `companion-ask` words to one eligible info topic key (key, or the one label holding every word; never an offer, a hand-in or a `behavior` topic) plus the info labels for the refusal line; `AskForOwner(ownerId, mobInstanceId, key, authorized)`: plays that info topic as the owner choosing it, with no owner line and no menu; this package's `init` installs it as the companion NPC asker (`companionai.SetNpcAsker`), so the module's `sayto` reaches it through `companionai.AskNpc`; `modules/aicompanion` imports this package (as `talkflow`) only for `CompanionTopic` |
| `talk.go` | `Talk(user, room, mob)`: the approach line, opener and breadcrumbs, then the private talk menu (refused first with `TalkSightLine` when the player cannot see the NPC by name, `seesByName`); info and hand-in choices (player line, handover line, NPC reply, then the menu re-shown), the offer pitch and its Accept / Leave card, and `player_topic` for `behavior` topics (a behavior topic whose effects landed closes the menu instead of re-showing it, R18) |

## Rules

- Observers see a conversation, never a menu: the player's line and the NPC's
  answer go to the room through `actions.Say`; menus, cards and hints go to the
  player only.
- Every choice re-checks eligibility right before it acts. `StillValid` (the
  NPC in the same room, the room at full sight, the NPC perceived) runs
  before every choice; a failed check closes the menu with the context-lost
  line.
- Quest grants are queued (`events.Quest`), so the menu re-shown after a
  choice counts the token that choice granted as held (`heldAlso`); without
  it the re-shown menu still lists an offer the player just accepted.
- A behavior topic (a timed scene) closes the talk menu once its effects
  landed, whether chosen directly or accepted from an offer card; a refused
  behavior topic still re-shows the menu. The player talks again afterwards.
- An NPC with no eligible topic left opens no menu: once the last topic is
  used up the conversation simply ends.
- `Talk` checks `users.(*UserRecord).MenuBlockedReason` before anyone speaks:
  a gate rule or an open prompt gets one private refusal line and the room
  hears nothing. So does an NPC the player cannot see by name (spec 4 item
  1). At shapes sight every talk aim gets `TalkShapesLine` ("You need more
  light ..."), never the shared aim hint "Try talk figure", which would only
  refuse again: `usercommands.Talk` and `ask`'s redirect call
  `RefusedAtShapes` before resolving any name, and `Talk` itself sends
  `SightRefusal` (`TalkShapesLine` at shapes, otherwise `TalkSightLine`,
  which names no one) before the approach line or the opener. At full sight
  a sleeping NPC's refusal (`usercommands.Talk`, before `Talk`) comes first.
- The talk menu and the Accept / Leave card open with `Session.ShowLast`
  (shown after the choice's lines, quest banner and rewards included) and
  `Session.MobInstanceId` (the menu closes, with a line, the moment the NPC
  leaves the room or dies).
- During a choice the PlayerState callbacks' private lines (delivery, gold
  received or paid, refusals) are buffered (`talkState`, `lineBuffer`) and
  flushed after the handover line and before the NPC reply, also when Apply
  refuses. The handover line names the item the effects actually took
  (`lineBuffer.taken`), and a give's named instance rides along as
  `lineBuffer.prefer`, through an offer's Accept card too. `PlayerState(user)` itself still sends at once for other callers.
  A price the player cannot pay says so in the talk flow only.
- Every choice whose effects landed (info, hand-in, accepted offer, give
  resolution, behavior topic) calls `RefreshMarkers` once, from `after`; a
  pitch, a refused choice or an item handed back does not.
- Charmed NPCs offer no topics: `Talk` gives the approach line and a nod,
  `Give` returns false, and `Marker` is empty.
- `Give` does not check `MenuBlockedReason`: give is a plain command with its
  own refusals in `give.go`. The menu it re-shows afterwards is withheld by
  `OpenMenu` itself while a gate rule holds or a prompt is open.
- `RemoveItem` queues `events.ItemOwnership{Gained: false}` for the taken
  item, as `give.go` does for a consumed hand-in.
