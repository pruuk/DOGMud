# PR #208 Review: Synthesized Report

Blind adversarial review of [PR #208](https://github.com/pruuk/DOGMud/pull/208)
("Merge from recent code changes by finaltwist"), run 2026-10-05 against PR
head `3ce674451` (base `c696c117a`).

PR #208 bundles seven systems in 1,743 files (+86k lines): merchant chests,
player housing, floor decay and city scavengers, the AI companion overhaul,
the API-driven "livelier world" features, rifts, and the wilderness trades
(gathering, crafting, mining, wear and repair). It is the last bundled PR;
later work follows [CONTRIBUTING](../../../.github/CONTRIBUTING.md).

## How the review ran

- The PR was split into nine pieces. Each piece had three independent
  reviewers: correctness, security, and reuse of existing mechanisms.
- Three more reviewers covered the whole PR: API on/off for the operator,
  player data sent to the API, and exploits that cross systems.
- Reviewers were blind to each other and told not to trust commit messages,
  the PR text, or the existence of tests.
- Independent verifiers then tried to refute every finding: three votes for
  critical and high, one for the rest. Severity below is the verifiers'
  median.
- Result: 139 findings, 130 confirmed, 9 refuted. Several systems were found
  by more than one reviewer; this report merges those.

Files in this folder:

- This report: what blocks the merge, and what we follow up on.
- [`findings-index.md`](findings-index.md): all 139 findings, ranked, with
  links.
- [`reviewers/`](reviewers/): one file per reviewer, with its summary, what it
  covered, and each finding's failure scenario, suggested fix, evidence and
  verifier votes.

## What the PR does well

Much of the work reuses the engine properly. Housing persists through
`util.Save`, `util.ReadLivingState` and `util.QuarantineCorrupt`, writes before
it changes memory, and builds strongboxes on the existing lock system. Chop
and survey go through the shared contest functions and are registered in the
contest guard tests. Scavengers pick up through `actions.TakeFloorItem` and
narrate through the sight-aware messaging path. Rifts extend the existing
ephemeral room system instead of building a new one. The housing work went
through two review rounds of its own and it shows.

## Blocking: fix before merge

### B1. Model output runs as NPC commands (critical)

`internal/npcidle/npcidle.go:290` runs the model's reply as
`mob.Command(res.Kind + " " + res.Text)`. `CleanResult` (`reply.go:80`) allows
`;`, and the command parser splits on it. The request goes through the
player's own API key, so the player controls the reply: a line of speech
followed by `;give 5000 gold <name>`, an attack, a despawn or a broadcast.

Fix: never pass model text to `mob.Command`; send the say or emote through the
messaging path directly. Run all player-key text through
`baubles.CheckPlayerKeyText`, the existing allowlist, instead of a looser
filter. Apply the same to `lookdetail` and `roomlife`.

Findings: [F001](reviewers/lively-api-correctness.md#f001),
[F002](reviewers/lively-api-security.md#f002),
[F003](reviewers/lively-api-unification.md#f003),
[F008](reviewers/api-consent-player-data.md#f008),
[F004](reviewers/lively-api-unification.md#f004),
[F093](reviewers/lively-api-security.md#f093).

### B2. API features are on without consent (high)

`npcidle`, `roomlife` and `lookdetail` ship `Enabled: true` in both the Go
defaults and `_datafiles/config.yaml`. The relay's "Make the world livelier"
box starts ticked, keys saved before it existed are treated as ticked, and
rift room generation sits under the same box. The existing "finds"
permission beside it is opt-in. An admin `aicompanion grant` also records
consent for a player who never saw the question, and Hollow treats any
unaddressed `say` as consent.

Fix: every API feature defaults off in Go and in `config.yaml`. The livelier
box starts unticked and existing keys are not opted in. Each new feature asks
for its own consent. `Enabled` takes effect without a restart. Admin grant
does not record consent.

Findings: [F007](reviewers/api-consent-operator-switch.md#f007),
[F005](reviewers/lively-api-unification.md#f005),
[F060](reviewers/api-consent-operator-switch.md#f060),
[F063](reviewers/api-consent-player-data.md#f063),
[F097](reviewers/rifts-security.md#f097),
[F129](reviewers/api-consent-operator-switch.md#f129),
[F029](reviewers/ai-companions-security.md#f029),
[F059](reviewers/api-consent-operator-switch.md#f059),
[F128](reviewers/api-consent-operator-switch.md#f128).

### B3. Player-controlled text becomes world text (medium, repeated)

- `lookdetail` caches a reply that came through a player's key and shows it
  to every later player as the room's description.
- Rift rooms built from such replies are saved permanently, into the tracked
  world-content folder with no gitignore entry, with no record of who wrote
  them.
- Player-written house descriptions are sent through other players' keys,
  marked as carrying no player data.

Fix: text a player's key produced is shown only to that player, or passes the
same allowlist and is recorded with its author. Generated rifts are living
state outside the tracked world folder. Player-written text is never sent on
another player's key.

Findings: [F035](reviewers/lively-api-correctness.md#f035),
[F036](reviewers/lively-api-security.md#f036),
[F041](reviewers/rifts-security.md#f041),
[F064](reviewers/api-consent-player-data.md#f064),
[F094](reviewers/rifts-correctness.md#f094),
[F021](reviewers/housing-security.md#f021),
[F058](reviewers/api-consent-operator-switch.md#f058),
[F061](reviewers/api-consent-player-data.md#f061).

### B4. Rift doors lead into unrelated rooms (high)

The new owned chunk (`internal/rooms/ephemeral_owned.go`) gives a new room the
lowest free slot, so a removed rift room's id goes to the next room built. A
parent room's door keeps pointing at the old id, and the router only checks
that a room with that id loads. A party member left in the parent walks into
an unrelated room, past locked and sealed doors, and spends no key. The
existing ephemeral system never reuses an id inside a live chunk, which is why
it never had this bug.

Fix: do not reuse slots in an owned chunk until the chunk is released, or
clear the parent's door and exit when the rift removes a child.

Finding: [F006](reviewers/rifts-correctness.md#f006).

### B5. The PR fails its own tests

gofmt, build and vet are clean. `go test ./...` fails 12 tests in 7 packages:

| Package | Test | What it says |
|---|---|---|
| root | `TestEveryItemHolderIsASweepRootOrTransient` | `combat.SkillMoveParams` and `WeaponHitInfo` hold items the bauble sweep does not know about |
| root | `TestNarrationSitesMatchViewpointAudit` | `actions/repair.go:243` narration has no viewpoint verdict |
| root | `TestToolLadderContent` | Corwin the Tanner (9337) carries no light, so cannot trade at night |
| actions | `TestMineNumbers`, `TestRepairDisciplineAndCost` | gem chance and repair cost are 0 |
| characters | `TestCritWear`, `TestCritWearStriker_RightItem` | 200 critical hits cause no wear |
| gather | `TestWearTool` | a tool never breaks |
| rifts | `TestValidate_RejectsBrokenData` | the fixture drifted from the rift room files |
| shops | `TestWalkInBuyPrice_NeverRisesAndSlidesGently` | later units do not get cheaper as expected |
| templates | `TestU8ActionAdmissionHelpStatesExactPolicyWithoutTuning` | `craft` and `woodwork` help show raw percentages |

The wear, repair and mining failures all read 0. Test binaries load the Go
defaults, not `config.yaml`, so those knobs have Go defaults of 0: any server
without the keys has no wear and free repair. Give each knob a working Go
default.

The full test suite must pass in CI. CI's lint check cannot judge this PR: at
over 300 files the "new issues only" mode breaks and reports the whole
existing backlog. For #208 only, a clean local
`golangci-lint run --new-from-merge-base=origin/master` stands in for it.

### B6. Exploits from guards left off a sibling path (medium)

Each of these is a rule the PR added to one path and not to its twin.

| Exploit | Fix | Findings |
|---|---|---|
| A companion's `put` overfills a picked merchant chest, so goods spill to the floor unmarked and sell clean | Refuse merchant chests in `mobcommands.Put` and the companion `put` errand, as the player `put` already does | [F009](reviewers/merchant-chests-correctness.md#f009), [F011](reviewers/merchant-chests-security.md#f011), [F017](reviewers/merchant-chests-unification.md#f017), [F066](reviewers/cross-cutting-security.md#f066) |
| The chest take contest skips the steal cooldown and rank gate, so it can be retried every round to farm skill | Route it through the steal path's cooldown and rank gate | [F010](reviewers/merchant-chests-security.md#f010), [F013](reviewers/merchant-chests-unification.md#f013) |
| Skin a carcass, then have a companion salvage it through the old flat table: it pays twice, and the second batch never spoils | Mob and companion salvage use the harvest tables and respect `Skinned`, `Butchered` and `HarvestedParts` | [F043](reviewers/craft-gather-hunt-correctness.md#f043), [F047](reviewers/craft-gather-hunt-security.md#f047), [F049](reviewers/craft-gather-hunt-unification.md#f049), [F065](reviewers/cross-cutting-security.md#f065), [F033](reviewers/ai-companions-unification.md#f033) |
| Broken affixed gear sells at full price and is reshelved at full price, still broken | Apply the wear penalty and scrap rule on the affixed sale path | [F053](reviewers/craft-mining-wear-fixes-correctness.md#f053), [F055](reviewers/craft-mining-wear-fixes-security.md#f055) |
| Merchants repair while asleep or in the dark | Repair goes through the shared shop gates (`actions.ShopSightRefusal`, sleeping merchant) | [F054](reviewers/craft-mining-wear-fixes-security.md#f054), [F056](reviewers/craft-mining-wear-fixes-unification.md#f056) |
| Dropping the connection before a rift death avoids the gold and gear loss | Resolve the forfeiture whether or not the player is connected | [F040](reviewers/rifts-correctness.md#f040) |

## Follow-up: we handle these after merge

These are confirmed but not blocking. We will fix them behind the merge, as
with earlier patches. Ids link to the full finding.

**Exploits and economy.** Scavengers turn a dropped honest bauble into
fence-premium stolen goods (F025). Companion starting kit can be farmed by
re-bonding (F028). Spoilage is bypassed by bank storage and crafting accepts
spoiled goods (F044, F048). Hollow courtship can be forged through the
player's relay (F031, F062). A crash after a rift death can duplicate items
(F095, F096). Survey re-rolls rare-species identification for free (F113).
`mine <ore>` and `chop <word>` reveal what the Perception gate hides (F111,
F121, F123). `plant` puts items into a locked merchant chest (F069). An
honest owner buys back its own cooled stolen goods (F130).

**Reliability and load.** Every command inside a house compares the whole
house as YAML and floors have no cap (F018, F020). Housing units never return
to the pool (F019). A recipe with an output id below 1 hangs the game loop
(F122). `Item.GetSpec` copies the whole balance config several times per call
(F109). A full scavenger does quadratic work every idle tick (F082).

**Reuse of existing mechanisms.** `internal/mining` is a near line-for-line
copy of `internal/timber` (F057). Spoilage adds a second clock beside item
aging (F051). Lively duplicates the baubles player-key call and moderation
(F037, F038). Hollow skips output moderation on the server key (F030, F034).
Merchant chests add a second stolen-goods model, guard registry and relock
clock (F070, F015, F016), and copy the fence sale (F014). Other copies: F022,
F023, F032, F087, F099, F100, F103, F115, F118, F119, F120, F126, F127.

**Sight gates.** Rift narration and hunter lines name players past the sight
gate (F042, F102). Lively requests list hidden NPCs (F039).

**Balance numbers in code.** Hardcoded tuning in harvest, chop, mining and
tool durability (F052, F104, F105, F107, F114, F117, F125). Chest and housing
numbers outside the balance config (F067, F071, F078). Floor decay cannot be
switched off because 0 is replaced with a default (F080, F083, F085). The bed
bonus is a flat value (F072, F076).

**Smaller correctness items.** A sleeping merchant's Perception multiplier
leaks into every sneak, search and detection check (F012). F024, F026, F027, F045, F046, F050, F068, F073,
F074, F075, F077, F079, F081, F084, F086, F088, F089, F090, F091, F092, F098,
F101, F106, F108, F110, F112, F116, F124.

See [`findings-index.md`](findings-index.md) for each one.

## Refuted

Nine findings did not survive verification and need no action: F131 to F139
in the index.
