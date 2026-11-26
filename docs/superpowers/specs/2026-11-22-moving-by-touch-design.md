# Touch arc spec 2: moving by touch (design)

Issue #781, epic #783. Roadmap row 2 of `docs/roadmaps/TOUCH_ARC_ROADMAP.md`:
2a server, 2b deck movement, 2c the Map tab's movement buttons. Brainstormed
with the owner on 2026-11-22. Mockup, approved by the owner for this spec:
`docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html` (Malia was away; her
look comes before plan 2b is built, see "Before building").

**What a player gets:** walking without typing, with one thumb, on phones and
tablets. Only exits the player knows light up; locked doors show a padlock;
in the dark the player can still feel their way; the Map tab carries the same
buttons.

## Owner rulings (2026-11-22)

1. **Secret exits are known only once found with `search`.** A player cannot
   walk through a secret exit they have not uncovered.
2. **Followers learn it.** A player who moves through a secret exit with the
   leader they follow learns it from then on.
3. **Locked doors:** shown lit with a padlock taken from the door's real
   locked state; a tap does what typing the direction does (unlocks and
   walks with a key or a known pick sequence, otherwise the locked line).
4. **Dark rooms:** the six T buttons stay tappable in a "can't see" style
   that reveals nothing; other exits cannot be used. **Dark marks:** a
   direction already tried in the dark shows a ✕ after "You're bumping into
   walls." and a padlock after the locked line; leaving the room or the room
   becoming lit clears them.
5. **The Map tab uses the same inverted T as the deck** (roadmap open
   question 2, settled).
6. **Approach A:** `Room.Info` becomes the one exit list; one server function
   decides what a player knows about a room's exits.
7. **Movement buttons are the first slice of the button protocol** (roadmap
   sync rule: the client never builds commands). Ids are opaque, issued per
   player, accepted only if issued; a press goes through the normal input
   queue; an unknown or old id is refused as stale.
8. **The six T directions get fixed ids** ("north from wherever you are"), so
   a fast double tap works like typing; only other exits get per-room ids.
9. **Sideways phone (#938):** the session bar and tab bar fold away and the
   deck's side column carries a **Menu** button that brings both back over
   the game until the player picks something or taps away. The existing
   once-per-device toast "This game is easier to play with your phone held
   upright." stays.
10. **Symbols:** compass and diagonal exits show an arrow with its letter
    (↑ N, ↗ NE); up and down show an ISO 7001 style stairs pictogram.
11. **Dark look:** option O, olive grey from the palette spec, with a
    closed-eye "Dark" tile in the exits slot.
12. **Mockup:** the owner approves it for this spec; Malia's look moves to
    before plan 2b.
13. **NPCs and secret exits** (on review of this spec): an NPC moves through
    a secret exit only if it found it (searched it, like a player) or the
    move is scripted, because a scripted route is that NPC's regular way and
    it would know it. Scripted: patrol routes, schedules, a behaviour tree or
    quest action that names the move, an admin `command`. Unscripted, and so
    limited to found secret exits: wandering, chasing or pursuing, a
    planner's errand route, following a player. Fleeing already skips every
    secret exit and stays as it is.
14. **Onlookers learn it** (during the build, from the batch 2 review):
    someone who sees another character leave through a secret exit
    ("Mira leaves down.") or arrive through one learns that exit. Only an
    onlooker who sees the mover learns: in the dark, or when the mover is
    unseen (sneaking, hidden), no line prints and nothing is learned. The
    arrival side teaches the exit of the arrival room that leads back the way
    the mover came, when that exit is secret.
15. **Seeing it used teaches it** (2026-11-25, from the Task 5b review): any
    line an onlooker sees that names a secret exit because someone used it
    teaches that exit, the same way ruling 14 does: picking or unlocking its
    lock, peering through it, shooting through it, a mob trying it and
    finding it locked. It teaches the exit, not how to pick its lock: the
    lockpick sequence stays with whoever picked it. Shooting through a locked
    exit is already refused. Mobs cannot pick locks yet; that behaviour is
    its own issue.
16. **Hearing through it does not** (2026-11-25): a sound heard from the next
    room through a secret exit the listener has not found says "(From
    nearby)" instead of naming the exit. A listener who knows the exit still
    reads its name. The AI companion is out of scope: it stays off for good.
17. **Trackers follow through what they found** (2026-11-25): a trail
    through a secret exit the tracker has found reads like any other ("They
    went north"); through one the tracker has not found, the trail is lost
    there as before. Same one rule: you use the exits you know.
18. **Exit names fit a button** (2026-11-26, owner and Malia, mockup review):
    every exit name is at most 6 letters, so it fits a 48px deck button with
    its text at 12px. Five were renamed: `watchtower` to `tower` (Clearstone
    Rise 5346 and Eagle's Reach 5365, both to Tower Top 5243), `overlook` to
    `ledge` (Tower Top 5243 to Eagle's Reach 5365), the ferry's `gangplank`
    to `plank`, a mob's `dark portal` to `rift`, and an instance's `return
    portal` (its way back out) to `return`. `shrine` and `portal` stay. The
    ten compass words are exempt: they have fixed buttons. The 6 is one
    constant, `exit.MaxNameLetters` (`internal/exit/name.go`), and it is
    enforced: the room loader (`Room.Validate`) and the mutator loader
    refuse a longer named exit at boot, naming the room or mutator and the
    exit; `SaveRoomTemplate`, `build room`, `room exit`, the `room exits`
    editor and the web builder refuse one before writing; a behaviour tree's
    `add_temp_exit` name is refused at tree load; tests pin the ferry, mob
    portal and instance return constants; and
    `internal/guards/exit_name_length_guard_test.go` walks the shipped rooms
    and mutators. **Instance portals** keyed
    `<zone>-<userId>` (`actOpenInstancePortal`) are exempt and keep their
    key. Requirement for plan 2b: that player's deck button is wired to
    their own instance exit, and its label leaves the number off (`arena`,
    not `arena-42`).

## Facts verified against source (2026-11-22)

| Fact | Where |
|---|---|
| `Room.Info` exits come from `GMCPRoomModule.GetRoomNode`: `exits` and `exitsv2` (`num`, `dx`, `dy`, `dz`, `details`) | `modules/gmcp/gmcp.Room.go:480-540`, `:610` |
| A secret exit is listed only if the player is on the far room's visitor trail (`HasVisited`), which lapses after 180 seconds | `gmcp.Room.go:491`; `internal/rooms/rooms.go:31` (`visitorTrackingTimeout = 180`) |
| The text exits line uses the same trail rule | `internal/rooms/roomdetails.go:59` (`GetDetails`), ~367-379 |
| `Zone.Map` hides a secret exit until the player visited the far room (permanent visited set) | `internal/mapper/mapper.snapshot.go:245-288` |
| `search` records a discovery (`characters.SecretExitDiscoveryKey`); quest destinations already read it, nothing else does | `internal/actions/search.go:33`; `internal/characters/character.go:673`; `internal/questengine/destination.go:385` |
| Any actor can search, NPCs included, and a find records the discovery on that character | `internal/actions/search.go:449` (corrected by plan 2a); callers `internal/mobcommands/search.go:11`, `internal/behaviortree/actions_scout.go:133` |
| Plain NPC wandering already skips secret and locked exits (`GetRandomExit`); only `wander loot` and `wander players` pick from every exit, secret ones included (corrected by plan 2a) | `internal/rooms/rooms.go:1547-1550`; `internal/mobcommands/wander.go:82-105` |
| Fleeing (`fleeExit`) already skips secret, locked and quest-gated exits; the behaviour tree's out-of-combat walk-away (`pickRetreatExit`) does not (corrected by plan 2a) | `internal/actions/flee.go:215-221` |
| `go` and a bare exit name reach exits through `FindExitByName`, which ignores `Secret`; so do `look`, `lock`, `unlock`, `picklock`, `shoot`, `fire`, `defuse` and the key ring (11 files, corrected by plan 2a) | `internal/rooms/rooms.go:2677`; `internal/usercommands/usercommands.go:415-420` |
| `exitsv2` gets the string `locked` in `details` from `HasLock()` (the door has a lock), not `IsLocked()` | `gmcp.Room.go:524-526`; `internal/exit/exit.go:24` |
| `Room.Info` leaves out temporary exits (`ExitsTemp`: ferry gangplank, return portal, tree and mob portals) and mutator exits, which `look` shows and `go` accepts | `internal/rooms/rooms.go:119`, ~689; `roomdetails.go:363`, ~435 |
| `Room.Info` is sent on `RoomChange` and `SightBandChanged`; nothing resends it after a lock change, a relock, a temporary exit or a found secret exit | `gmcp.Room.go:38-42` |
| A relock is lazy (time compare), and lock changes come from about a dozen call sites with no event | `internal/gamelock/gamelock.go`; unlock, lock, picklock, go, behaviour tree, quest engine, aicompanion |
| A per-round "send only if changed" watcher exists for vitals | `modules/gmcp/gmcp.CharVitalsWatch.go:30` (`lastSentVitals`) |
| At no sight, `Room.Info` sends empty exit maps (present, not omitted) | `gmcp.Room.go:480`; `gmcp.RoomSight_test.go` |
| Movement refusals in order: combat (`refusal.WhileFighting`), quest sequence lock, `no-go` condition, locked door, quest `room_gate`, exhaustion, instance not opened, no such exit ("You're bumping into walls.") | `internal/usercommands/go.go:77, 82, 96, 185, 206, 227, 262-268, 615` (corrected by plan 2a) |
| `actions.IsCompassDirection` knows full words only; a bare `n` with no north exit answers "not recognized" | `internal/actions/go.go:18` |
| Each player's `Commands.State` (mode: exploration, combat, dead) is pushed each round | `modules/gmcp/gmcp.Commands.go:102` |
| The client sends a typed line with `playerSend` | `_datafiles/html/public/webclient-pure.html:2956` |
| The deck draws empty slots today, `AUTOMATIC_SHOWS = false` | `_datafiles/html/public/static/js/touchdeck.js:35` |
| The sideways toast exists, once per device | `_datafiles/html/public/static/js/viewportfit.js:81` |
| Secret exits shipped: 5, none on a quest route (checked against every quest and dialogue file) | `rooms/`: 6323 west, 5441 east, 6418 north, 4161 down, 487 down (locked) |
| Exits that are not the six T directions: 22 diagonal, 10 named (`out`, `shrine`, `overlook`, `watchtower`, `drift`, `in`, `portal`, `crack`); no room has more than two (corrected by plan 2a) | room YAML count |
| Brand: darkest colour umber `#19140c`; olive grey ramp `#5d5c4e` `#7e8073` `#969889` `#b4b4a3` `#cec6aa`; muted text `#8b897a` (4.92 on the dark panel); warning red `#e35349` | `docs/superpowers/specs/completed/2026-11-01-brand-palette-design.md:26-32`; `docs/guides/BRAND_STANDARDS.md` section 2 |

## 2a: the server

**One rule for what a player knows about a room's exits.** A new function
(working name `rooms.KnownExits(room, user)`) returns, per exit the player
knows here: its name, its kind (compass, vertical, diagonal, named,
temporary), whether it is locked **now** (`Lock.IsLocked()`), and the
destination for the server's own use. At no sight it returns "dark" and no
exits. It covers `room.Exits`, `ExitsTemp` and active mutator exits.

Everything that lists exits reads it: the exits line (`GetDetails`), `Room.Info`,
the map snapshot (`Zone.Map`) and the deck. They can no longer disagree.

**Secret exits (rulings 1 and 2).** Known means the character holds the
discovery `SecretExitDiscoveryKey` for that exit, which `search` records and
which a follower gains when moving through the exit behind their leader. This
replaces the 180-second visitor trail (look, `Room.Info`) and the visited-room
rule (`Zone.Map`). `FindExitByName` callers on the player's movement path
refuse an unknown secret exit exactly as if no exit were there: "You're
bumping into walls." for a compass name, "not recognized" otherwise, so a
guess reveals nothing.

**NPCs (ruling 13).** The same discovery decides for an NPC's own choices:
wandering, chasing, a planner's errand route and following a player use a
secret exit only if that NPC holds the discovery (it searched and found it).
Scripted moves (patrol routes, schedules, a behaviour tree or quest action
naming the move, an admin `command`) keep using them, since that route is the
NPC's regular way. A companion or charmed NPC moving with its leader through
a secret exit learns it, like a player follower. Fleeing is unchanged (it
already skips secret exits).

**`Room.Info`.** `exitsv2` lists only known exits; `locked` means locked now;
a new `kind` field per exit; a new top-level `dark` flag. At no sight the exit
maps stay present and empty (no leak). Each exit's button id (2b) rides here
too.

**Kept current.** `Room.Info` is still sent on a move and on a light change.
A new once-per-round watcher (the `lastSentVitals` pattern) keeps a signature
per player of their known exits, locked states, dark flag and dark marks, and
resends `Room.Info` when it changes. This covers doors unlocked or relocking
on their timer, gangplanks and portals coming and going, a secret exit found,
and a light going out, without touching the lock call sites.

## The button protocol slice (2a with 2b)

The roadmap's sync rule holds: the client never builds commands.

- **Ids.** Each movement button carries an opaque id. The six T directions
  have **fixed ids** (one per direction, the same in every room), which the
  server runs as the bare direction word through the same parser as typing:
  pressing north is typing `north`. Other exits (diagonal, named, temporary)
  get **per-room ids** issued to that player for that room. No id encodes a
  room, a destination or whether an exit exists.
- **Press.** The client sends the id (a GMCP message). The server accepts an
  id only if it issued it to that player (fixed direction ids are always
  issued while the deck is on); an unknown or old per-room id is refused as
  stale with a plain line. A press goes through the normal input queue, one
  command per turn like typing.
- **Dark.** In the dark only the six fixed ids are offered. They leak
  nothing because they are the same everywhere.
- **Dark marks (ruling 4).** The server records, per player and room, the
  directions tried in the dark that answered "You're bumping into walls." (✕)
  or the locked line (padlock), from typed and tapped tries alike, and clears
  them on leaving the room or on the room becoming lit. They ride in
  `Room.Info`.
- Plan 3a widens this protocol to every button; nothing here is thrown away.

## 2b: deck movement

**Layout** (roadmap, settled): the inverted T on the left, up, north and down
on the top row, west, south and east below, the other-exits button beside it,
its slot kept when empty. Symbols per ruling 10. The command slots on the
right stay empty until the command bar (spec 3).

**States** (mockup key): lit (a known exit); dim (none that way); lit with a
padlock (locked now); dark (olive grey, all six tappable, ✕ or padlock marks,
the exits slot becomes the closed-eye "Dark" tile, whose tap prints "It is too
dark to see the way. Light a torch or cast glow."); in combat, movement dims
(refused anyway; Flee arrives with plan 3d).

**Other exits.** Tapping it opens a level that replaces the whole deck: Back
top-left, one button per other exit in a fixed order (diagonals clockwise
from northeast, then named and temporary exits A to Z), More bottom-right only
when they do not fit (no room needs it today). After a move from the list,
the deck returns to the T.

**Turning it on.** `AUTOMATIC_SHOWS` becomes true: phones and tablets
(`pointer: coarse`) show the deck by default; the Touch controls setting
(automatic, always, never) still overrides.

## 2c: the Map tab and the sideways phone

**Map tab.** On a phone, an open tab covers the deck, so the Map tab carries
the same T and other-exits button beside the map (about 160px; the map is
pushed left and keeps its size as far as the buttons allow). Same ids, states
and marks as the deck. Other exits opens in place of the buttons; the map
stays. Shown only when the deck would show.

**Sideways phone (#938).** The session bar and tab bar fold away; the deck's
side column carries a Menu button that brings both bars back over the game
until the player picks something or taps away. The Map tab is reached through
Menu, then Map, and carries the same T. The sideways toast stays.

**Not in this spec:** tapping a room on the map to walk there (a pathfinding
feature; `onRoomClick` stays unused).

## Tests and checks

**Server (2a)**
- `KnownExits`: a secret exit is unknown until searched; known after a search
  or after following a leader through it; locked now versus merely having a
  lock; temporary and mutator exits listed; dark returns no exits.
- `go` and a bare exit name refuse an unfound secret exit with exactly the
  "no exit" answer for that word; a found one walks.
- NPCs: a wandering, chasing, errand-running or following NPC never takes a
  secret exit it has not found, and takes one it has; a patrol or schedule
  route through a secret exit still walks; a companion moving with its
  leader learns the exit.
- `Room.Info` (extending `gmcp.RoomSight_test.go`): nothing leaks in the
  dark; unfound secret exits never sent; `locked` follows lock, unlock and a
  timed relock; the watcher resends only on a real change.
- One shared fixture checks the exits line, `Zone.Map` and `Room.Info` agree.

**Protocol slice, under the owner-approved guards**
- Guard 1 (round trip): every movement id the server issues in fixed rooms
  (compass, vertical, diagonal, named, temporary, locked, dark) becomes a
  command the real parser accepts.
- Guard 5 (sight): a dark room or an unfound secret exit yields no exit-specific id.
- Abuse rules, each with a can-fail case: an id never issued is refused; an
  old per-room id after moving is refused as stale; a press uses the input
  queue (one command per turn); ids reveal nothing (the dark ids equal the
  fixed ids).
- Dark marks: a typed or tapped bump marks ✕, a locked line marks the
  padlock, leaving or light clears both.

**Client (2b, 2c)**
- jstests: every button state (lit, dim, padlock, dark, ✕, combat dim), the
  other-exits level and Back, the Map tab copy, the sideways Menu button,
  `AUTOMATIC_SHOWS` on.
- Guard 6 (touch targets): 48px with 8px gaps across the size matrix, 44px
  with 5px at 320 wide.
- Colour contrast measured and written beside each new colour in the CSS
  (brand rule).

**Measurement.** On a local server, the time between a move's room text and
the new `Room.Info` reaching the client. If it is noticeable, it is fixed or
filed before 2b ships. The owner's report that the Conditions panel trails
the game text by about half a second (Rally) is measured at the same time and
filed as its own issue.

**Every plan ends with** the headless size matrix screenshots and Malia's
device check on her S21 (#834). Plan 2a also gets one adversarial playtest
seat that tries to find, guess or walk through secret exits, and to learn
anything from dark-room bumps.

## Before building

- **Malia's look at the mockup** before plan 2b is built (ruling 12).

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| A schedule's route counts as scripted, and so does the pathfinding it uses to reach its target room | The owner named an NPC's regular route as known; a schedule is that route | Treat schedule pathfinding as unscripted and route schedules around secret exits |
| A companion or charmed NPC moving with its leader learns the exit (ruling 2 applied to NPCs) | It walked through behind its leader, as a player follower does | It follows without learning, and cannot use the exit alone later |
| An NPC's discovery lives on that NPC instance's character and is lost when the instance is gone; a fresh spawn has found nothing | NPC discoveries are recorded the same way as players' today (`search.go:45`) | Keep them per template, so every copy of an NPC knows what one found |
| An unfound secret exit answers "not recognized" for a named exit and "bumping into walls" for a compass word, the same as no exit at all | Anything else would confirm the exit exists | Use one line for both |
| The dark marks ride in `Room.Info`, kept on the server, not in the client | The client never builds state the server should own (sync rule), and a typed bump must mark the button too | Track them in the client from the bump lines |
| In combat the movement buttons dim rather than disappear | Fixed slots: nothing moves; Flee replaces them in 3d | Hide them |
| Other exits are listed diagonals clockwise from northeast, then named and temporary A to Z | A fixed order keeps slots stable between visits | Order by the room file's order |
| After a move from the other-exits list the deck returns to the T | The next room's other exits are different; Back would be stale | Stay in the list when the new room has other exits |
| The "Dark" tile's tap line: "It is too dark to see the way. Light a torch or cast glow." | Tells the player why and what to do | Reword |
| Diagonals follow the compass symbol style (↗ NE) | The owner chose arrows with letters for "the directions" | Arrows only for diagonals |
| `Zone.Map` moves to the discovery rule too. A character who walked through a secret exit before this change without ever searching it loses it from the map until they search it | Ruling 1 is one rule everywhere; the map was the only place using the visited set; there are no live players to migrate | Migrate: record a discovery for every secret exit a saved character has visited |
| Ruling 18 counts letters as characters of the name (spaces and hyphens count), and refuses a mapper distance name such as `north-x2` like any other long name | One plain count; no shipped dogmud exit uses a distance name | Exempt distance names as compass words |
| Ruling 18 is enforced in the builders too (`build room`, `room exit`, the `room exits` editor, the web builder, `SaveRoomTemplate`), not only at boot | A name the boot refuses would otherwise reach disk and stop the next boot | Enforce at boot and in the guard only |