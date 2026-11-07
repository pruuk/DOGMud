# Comms tabs and sleeping shopkeepers: design

**Status:** built 2026-11-07 (branch feat/comms-tabs-and-sleeping-shopkeepers). **Owner:** Calabe.
**Source:** Calabe's rulings in session 2026-11-07; Malia's #808 (channel
descriptions). **Plan:** `plans/2026-11-07-comms-tabs-and-sleeping-shopkeepers.md`
(waits for this spec).

## Goal

A new player can tell what each Chat channel is for and how to use it, the
Broadcasts tab means what its name says, shouts are kept where players can
read them back, and a player never has to shout at a sleeping shopkeeper to
buy a loaf of bread.

## Rulings (Calabe, 2026-11-07)

1. Channel hints at the top of an empty channel window, gone once messages
   arrive (#808). Copy approved.
2. A whisper you send shows in your own Whisper tab.
3. `broadcast` is admin only. Only admins and the tips timer post to
   Broadcasts; every online player gets it, deafened players included. Tips
   reach deafened players too. The `` ` `` shortcut moves to `chat`.
4. Shout shows in the Chat sub-tab ("shout is loud, it is not just room
   speech").
5. A sleeping shopkeeper wakes when a player walks in. A hidden player does
   not wake them. This replaces, for shopkeepers only, the 2026-07-30 rule
   that a sleeping NPC refuses rather than wakes.
6. `wake <name>` wakes other sleeping NPCs; the asleep hint points at it.
7. Light mode stays visible. We fix the 512 icon's see-through strip.

## What players see

### Chat channel hints
- An empty channel window shows its hint. The first message in that channel
  replaces it, and it does not come back until the next login (messages last
  the session).
- On a laptop, resting the mouse on a channel button shows the same words.

| Channel | Hint |
|---|---|
| Say | What people say in the room you are in. To talk, type `say hello`. |
| Whisper | Private messages between you and one player, wherever they are. Type `whisper <name> <message>`. `reply <message>` answers the last one. |
| Party | Only your party hears this. Type `party say <message>`. No party yet? `party create`, then `party invite <name>`. |
| Broadcasts | Game tips and announcements. No one can post here. `set tips` turns tips off or on. |
| Chat | Everyone online, for general talk. Type `chat <message>`. Shouts from nearby show here too. `channels` turns a channel off. |
| Newbie | Everyone online. Ask anything about the game, and other players will help. No question is too small. Type `newbie <message>`. |
| Trade | Everyone online, for buying and selling. Type `trade <message>`. |

The Broadcasts and Chat lines differ from the copy approved earlier only to
match rulings 3 and 4.

### Whispers
- The player you whisper sees `Kesh: hi` in their Whisper tab, as now.
- You see `You (to Ana): hi` in yours.
- A mob's whisper reaches only you, as now.

### Broadcast and tips
- An admin types `broadcast The server restarts in five minutes.` Every
  online player sees, in the game text and the Broadcasts tab:
  `(announcement) Calabe: The server restarts in five minutes.`
- A player who types `broadcast` gets the usual answer for a command they
  do not have. `` ` hello `` talks on Chat, as `broadcast` did before.
- Tips arrive every few minutes as now, now also for deafened players.
  `set tips` still turns them off.

### Shout
- The room you shout in, and the rooms next to it, see the shout in the game
  text exactly as now.
- It also goes into the Chat window:
  - in your room: `Kesh shouts: HERE I AM!` (by sight, as say: "A figure"
    or "Someone" when they cannot see you; you always see your own name)
  - next door: `Someone (north) shouts: HERE I AM!`
- A player's shout skips deafened listeners, as say does. An NPC's shout
  (a town crier) does not.

### Sleeping shopkeepers
- Walk into a shop whose keeper is asleep and they wake:
  `Marn wakes as you come in.` Trade works at once.
- They stay up while a player who is not hidden is in the shop. Once no
  such player is there and the wake grace (about three minutes) has run, they
  go back to sleep if their schedule still says sleep. Walking in again
  wakes them again.
- A hidden player walking in does not wake them. If that player then uses
  `list`, `buy`, `sell`, `appraise` or `offer`, the keeper wakes and serves
  them, with the same line, rather than refusing.
- A keeper never dozes off with a customer (not hidden) in the shop. A
  hidden customer's shop command wakes them, as above.
- A keeper woken at night in their own shop can always trade: they carry
  their own light while awake (the lighting rule), and the night-trade guard
  checks this for every hour a keeper can sleep there.

### `wake <name>`
- `wake marn` wakes a sleeping NPC or player in your room:
  `You wake Marn.` / the room sees `Kesh wakes Marn.`
- Talking to or giving to a sleeping NPC now says
  `Marn is fast asleep. Try wake marn.` at clear sight, `wake figure`
  at shapes, and `Try shout wake up.` in the dark.
- `shout` still wakes everyone in the room.
- Bare `wake` does what it does today (nothing; it is an alias of a no-op
  command, kept for a sleeping player typing it).

## Edge cases

- **Waking a hostile sleeper** with `wake` or by walking in: only
  shopkeepers wake on entry, so an aggressive sleeping mob still has to be
  woken on purpose.
- **Several keepers in one shop:** entry wakes every sleeping keeper there.
- **A keeper off shift** (in someone else's room) is not a shopkeeper there
  and is not woken by entry.
- **Admin broadcast and the web client:** one targeted message per player,
  as tips do, so a player sees it once.

## Not changing

- The rule that sleeping NPCs refuse talk and give stays for everyone except
  shopkeepers; `wake` is the way in.
- Shout's game-text lines, its reveal of a hidden shouter, and waking the
  room.
- Who hears say, party and the chat channels.

## Answered (Calabe, 2026-11-07)

1. **Announcement label** shows the admin's name: `(announcement) Calabe: ...`.
2. **A keeper with a customer in the room does not doze off** on schedule.
   While a player who is not hidden is in the shop, the keeper stays awake;
   once the room has no such player and the wake grace has run, the schedule
   puts them back to sleep.
