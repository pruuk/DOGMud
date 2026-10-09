# internal/lightnotice

Tells a player, in world terms, when the light they can see by crosses a
**band**: dark, shapes, faces or dazzled. Added by lighting plan 3d
(transition notices), the fourth plan of the graded room lighting arc.

Go decides WHETHER a notice fires and names its cause and transition. Go
holds no wording: every line lives in
`_datafiles/world/dogmud/narration/light-notices/<cause>.yaml`, in the
`internal/movenarration` pattern.

## Bands and trigger rules

A band is `messaging.Band` (`internal/messaging/band.go`): `BandDark`,
`BandShapes`, `BandFaces`, `BandDazzled`, ordered darkest to brightest so a
comparison reads "did it get darker" the way this package needs it to.
`messaging.LightBand(observer, room)` computes one; it is optics only,
exactly like `ParticipantSight`, and does not consult sleep.

| Trigger | Fires on | Notes |
|---|---|---|
| `TriggerMove` | a band getting **darker**, or moving **into dazzle** | Walking into ordinary light needs no notice; the room description already says it. Runs after `RoomChange`, in the new room |
| `TriggerCombatRound` | any band change, **both directions**, in the SAME room | Once per combat round the player is fighting in, after aggro validation and retarget, before the round's attacks |
| `TriggerCommand` | any band change, **both directions**, in the SAME room | Before every command's own output, so an intercepted command still gets it |
| `TriggerQuiet` | never speaks | Login, and the placeholder first check. Records the band silently |

**The move rule keys off the room, not the trigger.** `decide` compares
`now.roomId` against the stored record's room on every trigger. If they
differ, the move rule applies (a lighter band is recorded but not
announced) whether or not the trigger is `TriggerMove`. This matters because
`TriggerCombatRound` and `TriggerCommand` can themselves run after a room
change has already happened but before the queued `RoomChange` listener
fires `TriggerMove`; without this rule such a check would announce a
lighter band with the movement cause, which the move rule is supposed to
suppress. A room change that got DARKER still speaks under any trigger,
attributed to `CauseMovement`.

**Sleeping and blinded players get nothing, and their state's end is silent.**
`decide` refuses to speak while `now.asleep` or `now.blinded`, and marks the
stored record `quiet`. The next attentive check (whatever trigger it runs
under) resyncs the record without a line, so waking or regaining sight never
reads as the light itself changing. There is no single seam for the end of
either state; see "The seams" below.

**Mobs never get a notice.** Every seam either only accepts a real user
(`Check` takes `*users.UserRecord`) or is filtered before the call:
`LightNoticeOnMove` skips any `RoomChange` whose `MobInstanceId != 0`.

## Attribution order, and why the eyes counterfactual comes first

`attribute(prev, now)` names the likeliest cause of a band change:

1. **Movement**, if the room id changed.
2. **Eyes (counterfactual)**: would the OLD light, read through the
   observer's CURRENT sight, already give the NEW band? If `now.bandAt`
   reports that band for `prev.terms.Level`, the light never had to move;
   the observer's own sight did (a draught wearing off, or taking hold).
   **2b. Eyes (direction, #448):** each record keeps the sight window it
   was read through (`sightWindow`: night-sight strength and infra reach).
   If the window changed AND the OLD light through the CURRENT sight moves
   the band off the old one in the same direction the band moved
   (`movedSameWay`), the cause is the eyes even when a light term drifted
   too and the eyes alone fall short of the new band. A window that did not
   change never takes this step, so a light moving while the eyes hold
   still is never blamed on them.
3. Otherwise, the first light term that moved: **darkness** (lighting plan
   5d: `Darkened` flipped or `Dark` moved, a carried darkness arriving,
   lapsing or changing strength), then **carried**, then the **room's own
   lamp**, then **weather** filtering the sky, then the **sky** itself.
   The lamp is named only when it moved the same way as the room
   (`lampAgrees`, lighting plan 6): street lamps now light when the clear
   sky, as the dimmest lamplit street sees it, grows too dim to read a face
   and go out once it is bright again
   (`gametime.LampsLit`), so across a long gap (noon to midnight) the lamp
   can come on while the room darkens, and that change belongs to the sky.
   A lamp whose light did not move at all (an authored `lamp: 0` switching)
   is never named either.
   Darkness comes first because without its own cause a darkness moves no
   light term and fell to `CauseEyes`. Its lines live in
   `narration/light-notices/darkness.yaml`, all six transitions, like every
   cause (`LoadFrom` refuses a missing file).
4. **Eyes**, as a fallback, if nothing above explains it.

Step 2 runs BEFORE step 3 on purpose. A plain direction test (did the light
level move the same way the band moved) is not enough, because the sky
drifts a little almost every round; when it drifts the same direction as an
eyes-caused change, a direction test blames the sky for a change the
observer's own sight already explains. The counterfactual does not have that
failure mode: it holds the light fixed at its OLD value and only varies the
sight. This replaced an earlier direction-test version that would blame a
draught wearing off at dusk on the sky (found and fixed during this plan's
execution, `9e03c4e20`).

## The seams

`Check(user, trigger)` is the one entry point every seam calls. It loads the
player's room, computes an `observation`, runs `decide` against the stored
`record`, and sends through `internal/narration` on `messaging.CategoryLight`
when `decide` says to speak.

It also feeds the web client's Game-window border (lighting plan 5d, ruling
D7): when the band it computed differs from the one last handed on (kept in
`sentBands`, apart from `records`, because a record keeps its old band while
its player sleeps), or none was, it queues `events.SightBandChanged`, which
`modules/gmcp` answers with `Char.Sight`. A repeat check in the same band
queues nothing; `Forget` clears it with the record.

| Seam | File | Trigger / call |
|---|---|---|
| Before every command | `internal/usercommands/usercommands.go` (`TryCommand`) | `lightnotice.Check(user, lightnotice.TriggerCommand)`, before scripts, behaviour trees and quest intercepts |
| Each combat round | `internal/hooks/NewRound_DoCombat.go` | `lightnotice.Check(user, lightnotice.TriggerCombatRound)`, after aggro validation and retarget, before the round's attacks |
| Arriving in a room | `internal/hooks/LightNotice_Triggers.go` (`LightNoticeOnMove`), on `events.RoomChange` | `TriggerMove`; players only (`MobInstanceId == 0`) |
| Login | `LightNoticeOnSpawn`, on `events.PlayerSpawn` | `TriggerQuiet` |
| Logout | `LightNoticeOnDespawn`, on `events.PlayerDespawn` | `lightnotice.Forget(evt.UserId)` |
| Every round, for attention | `LightNoticeAttention`, on `events.NewRound` | `lightnotice.NoteAttention` for every online user |
| Boot | `main.go` | `lightnotice.LoadLightNoticeFiles()`, matching `movenarration.LoadMoveNarrationFiles()`; panics on failure |

`NoteAttention` is the seam for waking and the end of blindness. There is no
single event for either: sleep ends at eleven hand-rolled sites plus
condition expiry, and nothing fires a "condition removed" event. Rather than
hook all eleven, `NoteAttention` runs every round for every online player,
reads two flags (`HasConditionFlag(conditions.Sleeping)`, `Perception.State()
== perception.Blinded`) and marks the record `quiet` when either is true. It
computes no light and sends no text, which is what makes a per-round call
for every player affordable.

## The data rules

`_datafiles/world/dogmud/narration/light-notices/{movement,carried,lamp,weather,sky,eyes}.yaml`,
one file per `Cause`. Each file authors ALL SIX `Transition`s
(`CauseGroup.Validate` fails the boot on a missing one, because the trigger
rules can reach any of them; a missing pool would be silence in play rather
than a boot failure).

A transition's `Pools` is either one `any` pool, or a matched `outdoor` +
`indoor` split (declaring both `any` and a split, or only one side of a
split, fails validation). Of the six causes, only `sky.yaml` splits by
setting; the other five author `any` only.

Every line obeys `dogmud-player-copy`, enforced by `validateLine`: at most 80
columns, no digits, no em or en dash, and no `{token}` braces (this store
fills none). Every pool holds at least `MinVariants` (2) lines, so a player
who sees the same crossing twice does not read the same sentence.

## The dazzle ruling (owner, 2026-09-25)

A normal observer CAN be dazzled without any ability: a carried light
outdoors near midsummer noon (sky about 73.7 plus the carried term 50
combine to about 75.1) and a `city_thoroughfare` near midsummer noon (sky
73.1 plus lamp 52 combine to 75) both cross the dazzle edge. The owner ruled
this intended: "they should have used the spell or a hooded lantern that
adjusts; it makes the lantern actually valuable." Every dazzle line in
`sky.yaml`, `carried.yaml` and `lamp.yaml` therefore reads true for any
observer, not only one whose vision ability shifted their window down. This
corrects the design spec's fact 3 and the celestial amendment's claim that
natural daylight never dazzles: the sky ALONE never reaches 75, but the
combine with a carried light or a thoroughfare lamp does, near midsummer
noon.

## Public surface

Verified against source 2026-09-25 with
`grep -nE '^(func|type|const|var)\s' internal/lightnotice/*.go`.

| Symbol | Kind | Notes |
|---|---|---|
| `Cause` | type | `string`. `CauseMovement`, `CauseCarried`, `CauseLamp`, `CauseWeather`, `CauseSky`, `CauseEyes`, `CauseDarkness` (lighting plan 5d) |
| `Transition` | type | `string`. `DarkerFaces`, `DarkerShapes`, `DarkerDark`, `LighterShapes`, `LighterFaces`, `IntoDazzle` |
| `Causes() []Cause` | func | Copy of the full cause vocabulary, fixed order, for tests and the narration snapshot |
| `Transitions() []Transition` | func | Copy of the full transition vocabulary, same purpose |
| `MinVariants` | const | `2`. The fewest lines any one pool may hold |
| `Pools` | type | One transition's lines: `Any`, `Outdoor`, `Indoor []string` |
| `CauseGroup` | type | One cause's file: `Cause Cause`, `Transitions map[Transition]*Pools` |
| `(*CauseGroup) Id() string` / `Filepath() string` | methods | The `fileloader` generic contract |
| `(*CauseGroup) Validate() error` | method | Run by `fileloader` on load; every cause must author every transition |
| `LoadFrom(dir string) error` | func | Loads the store from an explicit directory; what tests call, since a test binary never reads `config.yaml` |
| `LoadLightNoticeFiles()` | func | Boot-time loader, called from `main.go`. **Panics** on any failure |
| `Pool(c Cause, tr Transition, indoor bool) []string` | func | The lines one notice draws from; `nil` when the store is unloaded or the entry is absent |
| `Trigger` | type | `uint8`. `TriggerMove`, `TriggerCombatRound`, `TriggerCommand`, `TriggerQuiet` |
| `Check(user *users.UserRecord, trigger Trigger)` | func | The one entry point every seam calls |
| `NoteAttention(user *users.UserRecord)` | func | Marks a sleeping or blinded player so their next attentive check resyncs silently |
| `Forget(userId int)` | func | Drops a player's record, at logout |
| `ResetForTest()` | func | Unloads the store and clears every record |

`decide`, `attribute`, `transitionOf`, `observe`, `observation`, `record` and
`notice` are unexported; `decide` is a pure function (`(prev record, known
bool, now observation, trigger Trigger) (notice, bool, record)`) so every
trigger rule is table-testable without a room, a user or the store loaded.

## In-memory only

Per-player records (`records map[int]record`, guarded by a `sync.Mutex`) live
in memory only. `Forget` clears one at logout; nothing here is saved, and a
restart forgets every player's last-announced band, which is correct: the
next check after reconnect is a fresh `TriggerQuiet` login record.

## Testing notes

- A test that wants real prose calls `LoadFrom(shippedDir)` (or the literal
  `_datafiles/world/dogmud/narration/light-notices` path from another
  package) and registers `t.Cleanup(ResetForTest)`, exactly like
  `movenarration`'s pattern. `internal/lightnotice/main_test.go` declares
  `shippedDir` for the package's own tests.
- **An unloaded store is silent, not an error.** `line()` returns `ok=false`
  when `Pool` returns nothing, which is always true before `LoadFrom` runs.
  This is what keeps every other test package that never loads this store
  free of new messages: `Check` simply has nothing to send.
- The snapshot golden is `internal/narration/testdata/stores/light_notices.golden`,
  built by `buildLightNoticesGolden` in `internal/narration/snapshot_test.go`
  and run as the `light_notices` subtest of `TestSnapshotStores`. It freezes
  every line of every cause, transition and setting, at every index, keyed
  `<cause>|<transition>|<setting>|<index> => <line>`.
- `internal/lightnotice/tracker_test.go` table-tests `decide`, `transitionOf`
  and `attribute` directly, with no room or user built. `check_test.go` and
  `integration_test.go` exercise `Check` against seeded rooms and the shipped
  data, including `TestObserveBandAtMatchesBand` (the counterfactual) and the
  dusk/thoroughfare dazzle scenarios from the owner ruling above.

## Related

- `internal/messaging` - `Band`, `BandThroughWindow`, `LightBand`,
  `CategoryLight`
- `internal/rooms` - `LightTerms`, the terms `attribute` compares
- `internal/narration` - `Render`, the rendering core this store's `line()`
  calls through
- `internal/movenarration` - the store shape this package's data and loader
  are modelled on
- `internal/hooks/LightNotice_Triggers.go` - the four event seams

## Fixtures and scheduled carried light (lighting 5e, spec X5)

`attribute` names `carried` when a carried light arrives or leaves
(`Carried`) OR changes strength while lit (`LightTerms.CarriedLight`: a
lantern its schedule dims, a sunstone fading, a second light joining one
already here; all of these used to fall to `eyes`), and `lamp` when the
room's lamp OR its light fixtures move (`LightTerms.Fixture`: the North
Gate's arch lantern as it is lit and snuffed with the street lamps). A darkness fixture moves `Dark` and
reads `darkness`, checked first. Cadence is unchanged: an idle player learns
of dusk on their next command, move or combat round. `lamp.yaml`'s header
names fixtures; `carried.yaml`'s two darker "is gone" lines read "fades"
(owner ruling R8).
