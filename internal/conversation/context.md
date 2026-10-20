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
| `playerstate.go` | `PlayerState(user)`: the `dialogue.PlayerState` callbacks over a character |
| `marker.go` | `Marker(viewerUserId, mob)`: the viewer's talk marker (`[?]`, `[!]`, `[*]` or empty); its `init` installs it as `rooms.NpcMarker`, which the room roster and the GMCP `Room.Info` NPC list call |
| `talk.go` | `Talk(user, room, mob)`: the approach line, opener and breadcrumbs, then the private talk menu; info and hand-in choices (player line, handover line, NPC reply, then the menu re-shown), the offer pitch and its Accept / Leave card, and `player_topic` for `behavior` topics |

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
- An NPC with no eligible topic left opens no menu: once the last topic is
  used up the conversation simply ends.
- `Talk` checks `users.(*UserRecord).MenuBlockedReason` before anyone speaks:
  a gate rule or an open prompt gets one private refusal line and the room
  hears nothing.
- During a choice the PlayerState callbacks' private lines (delivery, gold
  received or paid, refusals) are buffered (`talkState`, `lineBuffer`) and
  flushed after the handover line and before the NPC reply, also when Apply
  refuses. `PlayerState(user)` itself still sends at once for other callers.
  A price the player cannot pay says so in the talk flow only.
- `RemoveItem` queues `events.ItemOwnership{Gained: false}` for the taken
  item, as `give.go` does for a consumed hand-in.
