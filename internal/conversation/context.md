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

## Rules

- Observers see a conversation, never a menu: the player's line and the NPC's
  answer go to the room through `actions.Say`; menus, cards and hints go to the
  player only.
- Every choice re-checks eligibility right before it acts.
