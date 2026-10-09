# internal/movenarration

The shipped wording for special-move narration: the player-side verbs (kick,
bash, trip, gore, maul, rake, pounce, drain, throttle, hamstring, grapple,
shoot, charge) and their mob twins.

Added by M4e PR 1a of the messaging unification arc, which moved thirteen
`internal/mobcommands` files off Go string literals. M4e PR 1b then moved the
twelve `internal/usercommands` player twins the same way, so all 25
special-move Go files now read their wording from this store. There are 14
verb YAML files, one more than the 13 on the mob side, because `throw`
carries only a player twin.

## What this package is for

Go decides WHICH event fires and names it by a typed key. Go holds no wording.

That split is the whole point. Before this package, a mob's kick sentence lived
in `internal/mobcommands/kick.go` as a `fmt.Sprintf` format string, which meant
content work required a Go change and a rebuild, and no content author could
find it.

## Data

`_datafiles/world/dogmud/narration/special-moves/<verb>.yaml`, one file per
verb, loaded flat by verb id.

```yaml
moveid: kick
events:
  standard_hit:
    actee:
      - '{actor} kicks you hard! (<ansi fg="damage">{damage}</ansi>)'
    observer:
      - '{actor} kicks {actee}!'
```

Role keys are the arc's canonical four: `actor`, `actee`, `observer`,
`remote_observer`. A mob has no client, so mob events author no `actor` line.

**Player events take `player_` prefixed keys, in the SAME file as their mob
twin (M4e-1b, owner ruling, option B).** The original PR 1b design assumed a
player call site would simply fill in the empty `actor` role on the existing
mob event; it cannot, because the two shipped DIFFERENT prose for the same
event (`drain`'s mob `hit` actee line and the player's four lifesteal actee
lines share nothing). `bash.yaml`'s `player_hit` and `hit` are therefore
separate top-level events, not one event with a filled-in role. Merging the
two prose sets into one shared pool is a bigger content change, filed as M6
ledger row 56, and is explicitly out of scope for this migration. A verb with
no mob twin at all (`throw`, which mobs never do) carries ONLY `player_*`
keys in its file.

**Event keys carry the variant axis where one exists.** `kick` resolves as
stomp, knee or standard and `trip` as tailsweep or trip, so their keys are
`<variant>_<outcome>` (`stomp_hit`, `standard_knockdown`, `tailsweep_miss`).
An outcome key alone would make three different moves collide on one pool.
Every other verb has a single variant and keeps the bare outcome key.

## Public surface

Verified against source 2026-09-21 with
`grep -nE '^(func|type|const|var)\s' internal/movenarration/store.go`.

| Symbol | Kind | Notes |
|---|---|---|
| `LoadMoveNarrationFiles()` | func | Boot-time loader, called from `main.go` beside `combat.LoadTauntMessageFiles()`. **Panics** on any failure |
| `LoadFrom(dir string) error` | func | What `LoadMoveNarrationFiles` calls internally, exported for tests. A test binary never reads `config.yaml`, so `configs.GetFilePathsConfig` would resolve to `_datafiles/world/default`, which does not carry this store; a test that needs real prose loads the shipped dogmud dir explicitly (M4e-1, `internal/combat/grapple_narration_pin_test.go`; M4e-1b, `internal/usercommands/usercommands_test.go:96`'s `TestMain`) |
| `GetMove(moveId string) *MoveNarrationGroup` | func | nil if the store is unloaded or the verb is absent |
| `MoveNarrationGroup` | type | One verb's file. Fields `MoveId`, `Events` |
| `(*MoveNarrationGroup) Variants(EventKey) (narration.Variants, bool)` | method | The lookup call sites use. `ok=false` for an absent event |
| `(*MoveNarrationGroup) Validate() error` | method | Run by `fileloader` on load |
| `(*MoveNarrationGroup) Id()` / `Filepath()` | methods | The `fileloader` generic contract |
| `EventMessages` | type | One event's four role pools |
| `EventKey` | type | Names one outcome branch of one verb |
| `TokenDamage`, `TokenLabel`, `TokenWith`, `TokenVerb`, `TokenWeapon`, `TokenExitName`, `TokenPosition`, `TokenItem` | consts | This store's event tokens, beyond the four canonical name tokens. `TokenItem` was added in M4e-1b for `throw`'s thrown item name |

## Using it correctly

Most call sites do not touch this package directly. They go through
`internal/mobcommands/move_narration.go` (mob verbs) or its player-side twin
`internal/usercommands/move_narration.go` (M4e-1b), each of which owns
`sendMoveEvent` (the ordinary case) and `renderMoveEvent` (render without
sending, for the channel-defended partial branch whose observer line comes
from the defence triad instead of the store). The two helpers are not shared:
`internal/usercommands` cannot import `internal/mobcommands`'s unexported
pair, and the player side's shape genuinely differs -- it has a real `Actor`
recipient (the acting player has a client), and its pre-migration call sites
split categories by viewpoint (`CategorySystem` for the actor/actee lines,
the verb's own category for the room line) rather than sharing one, so its
`sendMoveEvent` takes a `moveCategories` (one category per role) instead of
a single `messaging.Category`. Since #449 every role of a special move rides
the move's own category (the actor and actee lines no longer use
`CategorySystem`, which never wraps); only refusals and cost notices stay
on `CategorySystem`.

The one exception is `internal/combat/grapple_narration.go`
(`renderGrappleEvent`), added by M4e-1's grapple slice. Grapple's crit-failure
and disarm results are consumed by BOTH `internal/mobcommands/grapple.go` and
`internal/usercommands/grapple.go`, so the rendering has to live in
`internal/combat` (where `HandleGrappleCritFailure` / `AttemptCritDisarm`
already are) rather than in either twin's command file, and it cannot call
`internal/mobcommands`'s helper without an import cycle. It duplicates
`renderMoveEvent`'s shape rather than sharing it.

🪤 **Check what the YAML already wraps before filling a token.** The shipped
files bake `<ansi fg="damage">` around `{damage}`, so the call site passes the
bare description. Wrapping it again double-tags the line.

🪤 **The identity tags belong to the CALL SITE, not the helper.** `kick` tags
its target `<ansi fg="username">`; `shoot` tags its target `<ansi fg="mobname">`
and builds its actor name pre-tagged (`shoot.go:49`). A helper that hardcoded
either would recolour the other.

🔑 **Do not add a darkness branch.** `messaging.SendTrio` already hides each
party's name from a reader who cannot make them out, judged by that reader's
`SightDecision`, at three tiers. Every file this package replaced carried a
hand-rolled two-tier version, and deleting those was the point of the slice.

## Two rules this package enforces, and why

**Role pools within one event must be equal length.** `narration.Variants`
pairs roles by index: variant N of each role describes the same moment.
`narration.ValidateVariants` rejects ragged pools, and `Variants.Len()` returns
0 for them, which every caller treats as render nothing.

**Unknown tokens are rejected.** `textutil.ValidateTokens` knows only the four
canonical NAME tokens, so event tokens ship unvalidated in every other store: a
value reading `{exit_name}` where the caller fills `{exitname}` would render its
own braces to a player. `validateEventTokens` closes that for this store by
declaring the vocabulary in `allowedTokens` and refusing anything outside it.
Widening the set is a deliberate edit to that map.

🪤 **`ValidateVariants` SKIPS EMPTY POOLS.** An event that omits a whole role
validates clean and then tells that audience nothing. An absent role cannot
simply be banned, because mob events legitimately have no `actor` line and
`shoot`'s arrival events are remote-observer only. The real check is role-set
agreement with the call site, in the root guard.

## Tests that hold this together

- `internal/movenarration/store_test.go` - round trip, ragged pools rejected,
  unknown token rejected, and every declared token accepted
- `shipped_narration_data_guard_test.go` - `TestShippedNarrationDataValidates`
  loads and validates the shipped files, and
  `TestNoLegacyRoleKeysInShippedData` walks this store for retired role-key
  spellings
- `internal/narration/testdata/stores/special_moves.golden` via
  `TestSnapshotStores` - renders every event at every variant index
- `internal/mobcommands/special_move_net_test.go` - the byte-identity net:
  118 rows asserting each event renders exactly what the original Go literal
  produced. This is what proves the migration changed no wording
- `internal/usercommands/special_move_net_test.go` - the player-side twin,
  `TestMigratedWordingIsByteIdentical`: 305 rows across all twelve player
  files, all comparing, 0 skipped and 0 failed now that the migration is
  complete
- `internal/combat/grapple_narration_pin_test.go` - the same proof for
  grapple's crit-failure and disarm events, which live in `internal/combat`
  rather than a mob command file and so sit outside the 118-row net
- `move_narration_migration_guard_test.go` (root package) -
  `TestMigratedFilesHoldNoNarrationLiterals` walks all 25 migrated files (13
  mob, 12 player) and fails on any backtick prose literal reached from a
  function body. It replaced the M2 freeze machinery (`TestM2LiteralsAreFrozen`,
  `TestM2FrozenFilesAllCarryText`), deleted in M4e PR 1b once the last frozen
  file migrated; a fingerprint saying wording had not changed was strictly
  weaker than this guard's claim that no narration literal remains at all.
  It scopes by DELIVERY PATH, not an allowlist: a literal inside a `SendText`
  call or a `refuse*` helper is a refusal (Group A, owned by M7, and
  legitimately present in player files though never in mob ones), a
  package-level const is a declaration, and everything else reached from a
  function body is narration. The same file also carries
  `TestMoveEventKeysAgree` and `TestMoveEventRoleSetsAgree`, which scan
  `internal/mobcommands`, `internal/combat` AND `internal/usercommands`
  (M4e-1b) for `sendMoveEvent`/`renderMoveEvent`/`renderGrappleEvent` call
  sites, and fail if a Go call site and the shipped YAML ever name a
  different set of (verb, event) pairs or role sets

## Related

- `internal/narration` - `Variants`, `Render`, `ValidateVariants`, the tokens
- `internal/messaging` - `SendTrio`, `Audience`, `ParticipantSight`, `HideNames`
- `internal/combat/taunt_messages.go` - the store this one is modelled on
