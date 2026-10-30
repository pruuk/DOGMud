# #317 Ambient retune: per-source knobs, a listener budget, zone and mob multipliers

**Status:** design approved in brainstorm 2026-10-30; awaiting owner review of this file.
**Issue:** #317 (last open item under the messaging epic #381; M6 #370 and M7 #371 are closed).
**Sequencing (owner 2026-10-30):** ships before the #375 phase 7 Pothole Coulee plans execute.
**Owner requirement (recorded on #317):** idle movement and each idle chatter source are configured
independently, so the feel can be tuned per source. Weather speaks far less often than a
shopkeeper talking about their goods.

## Facts verified against source (master `1b3eaca3a`, 2026-10-30)

| Fact | Source |
|---|---|
| A round is 4 seconds (`Timing.RoundSeconds: 4`), 15 rounds a real minute | `_datafiles/config.yaml:193`; `internal/configs/config.timing.go:37` |
| Room idle: chance hardcoded `chanceIn100 := 5` (20 for room -1); zone `idlemessages` fallback; a room with one line repeats it (no-repeat rule skips `idleMsgCt == 1`); sight-gated send | `internal/hooks/NewRound_UserRoundTick.go:162-221` |
| 30 room files have exactly one idle line; no zone config ships `idlemessages` | inventory 2026-10-30 (tally script) |
| Mob idle: every idle mob each round rolls `util.Rand(100) < mob.ActivityLevel`, then `GetIdleCommand()` picks uniformly from one pool mixing emote, say, `wander`, sneak, portal, cast, consume and blank entries; blank or none falls to `lookfortrouble` | `internal/hooks/MobIdle_HandleIdleMobs.go:313-320`; `internal/mobs/mobs.go:1200-1216` |
| MobIdle fires once a round for every mob not in combat, not targeted and not walking a path, whether or not a player is present | `internal/hooks/NewRound_IdleMobs.go:173-177` |
| `ActivityLevel` below 1 becomes 10 and is capped at 100 (an author cannot write 0); it also drives combat AI | `internal/mobs/mobs.go:1268-1272`; `internal/hooks/NewRound_DoCombat_helpers.go:1274,1303` |
| 138 scheduled mobs swap their pool per segment; schedule pools carry no blank entries (e.g. Marek's has 0 blanks, 9 emote/say) | `internal/hooks/NewRound_IdleMobs_schedule.go:93,169`; `schedules/thornwall_city/thornwall_tavern_keeper.yaml` |
| Weather ambient: one pass per room every `Modules.weather.EmoteEveryRounds` (24) +/-25%; chances `EmoteMildChancePct` 30, `EmoteStrongChancePct` 100; calm zones get a seasonal line on 1 pass in 3, hardcoded `seasonalEmoteOneIn = 3`; plugin config read once at load | `_datafiles/config.yaml:2839-2841`; `modules/weather/engine/emotes.go:15-101`; `modules/weather/weather.go:63,88-91` |
| Conversation knobs `ConversationBaseChancePct`, `ConversationCooldownRounds`, `ConversationPlayerArrivalBoostPct` are absent from `config.yaml` (live values are Go defaults 1.0, 50, 25) and `<= 0` resets to the default | `internal/configs/config.balance.go:758-768`; `config.balance.mobs.go:43-51`; grep of `config.yaml` returns 0 |
| Tips every 75 rounds, hardcoded `tipIntervalRounds = 75`; per-player opt-out `set tips` | `internal/hooks/NewRound_BroadcastTips.go:15,38` |
| Gossip: `GossipIntervalRounds` 75 (values below 20 reset to 75); runs before the idle roll | `config.yaml:2079`; `config.balance.mobs.go:149`; `MobIdle_HandleIdleMobs.go:237-256` |
| Item chatter has the only per-listener cap: in-memory `listenerCapNext`, `ItemChatterListenerCapRounds` 10 | `internal/behaviortree/actions_item_voice.go:49,126-133,253-260`; `config.yaml:2217-2223` |
| Room sends fan out one player at a time, and the user send path knows player and category, so a per-listener filter is possible | `internal/rooms/rooms.go:258-281`; `internal/users/userrecord.go:506-520` |
| Messaging categories cannot tell ambient from prompted (`CategorySpeech` carries player say, mob idle say, gossip, conversations, greetings) | `internal/messaging/messaging.go` |
| Zone config fields live on `ZoneConfig` (`internal/rooms/zoneconfig.go:9-24`); the Elemental Oasis zone config is `rooms/instance_planar_oasis/zone-config.yaml` | files |
| Oasis movers: Sand Elemental 318 and Storm Elemental 319 (`activitylevel` 25, one `wander` in an 8-entry pool); the four royals (320-322, 377, archetype `leader`) move through `move_toward_tracked` and the goal planner | `mobs/instance_planar_oasis/*.yaml`; `behaviors/archetypes/leader.yaml:59-75` |
| Measured 2026-10-17: tavern 472 about 6.6 lines a minute, Thornwall street 462 about 4.6, Market Square 465 about 3.9, steppe 3116 at night about 0.3 | #317 comment |
| Balance knob pattern: field in `config.balance*.go`, default and floor in the subsystem `validate*()`, shipped value in `config.yaml`, read through `configs.GetBalanceConfig()` at the call site | `.claude/skills/dogmud-balance-config/SKILL.md`; e.g. `GossipIntervalRounds` |

## Design

### 1. Per-source knobs (0 means off for every one)

| Source | Knob | Today | Ships at |
|---|---|---|---|
| Mob idle chatter (emote, say, consume, cast) | `Balance.MobIdleChatterScale` | none | 0.5 |
| Mob idle movement (wander, sneak, portal) | `Balance.MobIdleMovementScale` | none | 1.0 |
| Room idle lines | `Balance.RoomIdleChancePct` (room -1 keeps 4x) | hardcoded 5 | 3 |
| Weather cadence | `Modules.weather.EmoteEveryRounds` (existing) | 24 | 48 |
| Seasonal weather line | `Modules.weather.SeasonalEmoteOneIn` (new; 0 off) | hardcoded 3 | 4 |
| NPC conversations | the three conversation knobs, written into `config.yaml`; 0 now means off | absent | 1.0, 50, 25 (today's live values) |
| Tips | `Balance.TipIntervalRounds` | hardcoded 75 | 75 |
| Gossip | `GossipIntervalRounds` (existing); 0 now means off | 75 | 75 |

- **Split roll, weighted so 1.0 means no change** (review finding 2026-10-30). Each idle round a
  mob makes two independent rolls, each weighted by that class's share of the whole pool, blank
  entries included in the pool size:
  - chatter chance = `AL x MobIdleChatterScale x zone ambient multiplier x (chatter entries / pool size)`,
    then a uniform pick among the chatter entries;
  - movement chance = `AL x MobIdleMovementScale x mob wander multiplier x zone wander multiplier x (movement entries / pool size)`,
    then a uniform pick among the movement entries.

  At every multiplier 1.0 each mob's expected chatter and movement rates equal today's (today:
  one `AL` roll, then a uniform pick over the pool, blanks falling to `lookfortrouble`). Blank
  entries keep working as the authors' rate dial. The one behaviour change at 1.0: a mob can
  now speak and move in the same round, which today's single pick never allowed; at these rates
  it is rare and accepted. A mob whose pool has no entry of a class never passes that roll.
  `lookfortrouble` still runs on a round where neither roll picks a command. `ActivityLevel`
  itself is untouched (combat AI keeps reading it). The 1% forced-blank in `GetIdleCommand` is
  dropped; its effect on rates is under 1%.
- **Verb classes** are a fixed table in code (chatter: emote, say, consume, cast; movement:
  wander, sneak, portal; anything else, e.g. wear, alchemy: chatter). A guard test fails on an
  idle verb in shipped content that the table does not classify.
- **Single-line rooms** fire their line at most once per player visit.

### 2. The listener budget

`Balance.AmbientListenerBudgetRounds`, ships at 5 (about 20 seconds): a player hears at most one
ambient line per window. A line over budget is skipped for that player only; the rest of the room
still hears it. The budget is in-memory per player, on the item-chatter `listenerCapNext` shape.
0 turns the budget off.

- **Counts toward the budget:** room idle lines, mob idle chatter, weather ambient emotes, gossip,
  item chatter.
- **Exempt:** NPC conversations (the budget only gates whether one starts, and a conversation
  starts only if every player present has budget, so none is cut off mid-exchange); anything a
  player's own action triggers (arrival greetings, `player_enter` emotes); tips (`set tips`);
  event lines (sunrise and moon splashes, severe-weather onset, ferry, caravans, shop restock).
- Each counted send site marks its send as ambient through one helper, so the budget check lives
  in one place.

### 3. Zone and mob multipliers (owner 2026-10-30)

| Field | Where | Unset means | Scales |
|---|---|---|---|
| `ambientmultiplier` | `ZoneConfig` | 1.0 | that zone's chatter roll and room idle chance; the budget window is divided by it (1.5 = livelier, 0.5 = hushed) |
| `wandermultiplier` | `ZoneConfig` | 1.0 | idle movement in that zone |
| `wandermultiplier` | mob YAML | 1.0 | that mob's idle movement |

- Unset and 0 must be told apart (0 is a legal "never"), so the fields are pointers in Go.
- No existing zone or mob is set except the Elemental Oasis. Its value is chosen after the
  baseline playtest's telemetry (section 4). If it shows the royals' tracking or goal-planner moves are
  what read as too fast, the zone wander multiplier also scales `move_toward_tracked` and goal
  `pathto` while no player is the tracked target; otherwise it scales idle movement only.

### 4. Measurement: ambient telemetry and two playtests (owner 2026-10-30)

**Telemetry, kept behind a switch.** `Balance.AmbientTelemetry` (bool, ships `false`). When on,
the ambient helper and the idle-movement path count, per room and per source: lines offered,
lines delivered, lines suppressed by the budget, and mob moves; plus listener-minutes (players
present x minutes) so rooms visited briefly compare fairly with rooms sat in. Every
`AmbientTelemetryReportMinutes` (ships 5) it writes one summary log line per room with activity,
then resets. Off costs nothing beyond a bool check. It stays in the code after tuning as the tool
for zone tuning (owner: keep it, defaulted off).

**Report script.** `tools/ambient_report.py` reads a server log and prints, per room and source,
lines per listener-minute, delivered and suppressed, sorted worst first, plus mob moves per minute
per zone.

**Two playtests** with the same feel-tester seats walking fixed routes (tavern 472, Market Square
465, Thornwall street 462, the Pothole Coulee hub, steppe 3116 at night, the Elemental Oasis
instance), telemetry on:
1. **Baseline:** every multiplier and knob at its "today" value (scales 1.0, room idle 5,
   weather 24, budget 0). This equals today's behaviour.
2. **Shipped values.**

Each run yields the seats' feel reports and the script's table; both go on #317. The Oasis
wander multiplier is chosen from run 1's movement numbers. Tuning after run 2 is a config edit.

## Testing

- Each new knob: the shipped value is read (config test), and 0 silences its source (unit test that
  would fail if the source still fired).
- Split roll: at every multiplier 1.0, the expected chatter rate and movement rate (computed from
  the formula, and checked by a seeded many-round run) equal today's single-roll rates for a
  padded pool (e.g. the Sand Elemental's: one `wander` in eight entries), a schedule pool with no
  blanks, and a movement-only pool; a mob with only chatter entries never moves; with only
  movement entries never speaks; the movement roll honours both wander multipliers;
  `ActivityLevel` is unchanged after a round.
- Budget: two ambient lines within the window reach a player once; a second player who has budget
  still hears the line; exempt categories bypass it; a conversation does not start while a present
  player is over budget.
- Zone multiplier: unset behaves as 1.0; 0 silences the zone's chatter; 2.0 doubles the chance and
  halves the window.
- Single-line room fires once per visit.
- The verb-class guard can fail (planted unknown verb).
- Telemetry: off writes nothing and keeps no counters; on counts offered, delivered and suppressed
  per room and source and writes one line per active room per report window; the report script
  parses a fixture log into the expected table.
- Boot check, full test suite, lint, as `dogmud-shipping`.

## Docs

`context.md` for every touched package; `dogmud-balance-config` lists the new knobs if it keeps a
list; `docs/schemas` or the zone-config doc gains the two zone fields and the mob field;
`PATCH_NOTES.md` entry ("the world is a little quieter; busy places stay busy").
