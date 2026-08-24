# U10c Slice D — Guards, Player Copy, Playtest, Ship

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the last U10c guard, replace every player-facing sentence charm
tells about itself (all of which the redesign made wrong), and put the whole arc
through an adversarial in-game playtest before it ships.

**Architecture:** No new mechanics. One regression test over an existing guard,
one bounded investigation with an explicit drop condition, two rewritten pieces
of player copy, patch notes for the arc, and the playtest gate.

**Tech Stack:** Go, `internal/mobs` (instance saves), `internal/actions`
(harm-target policy), DOGMud content YAML, the `mudagent` playtest harness.

---

## What I verified before writing this

Two of Slice D's presumed tasks dissolved when checked. Both are recorded here so
nobody re-derives the wrong version of them.

**1. Non-combatants are ALREADY charm-proof. No new guard is needed.**
Charm is `type: harmsingle` (`_datafiles/world/dogmud/spells/charm.yaml`), and
`internal/actions/cast.go` calls `rejectHarmTarget` on the `HarmSingle`
named-target path (`cast.go:95`) and on the implicit-aggro path (`cast.go:145`).
That routes to `mobs.CheckPlayerHarm`, which returns `HarmBlockedNonCombatant`.

I counted 369 mobs with `non_combatant: true`, of which 38 lack
`charm_immune: true` — including one shopkeeper, `9326-flower_seller.yaml`.
**That count is a red herring.** Those 38 do not need the flag; the code covers
them. Do **not** open 38 YAML edits. What is missing is a *test*, and it matters
because the protection is one `switch` arm away from being lost silently.

**2. The "`EverCharmed` instance-save guard" was aimed at a field that cannot
carry it.** `Character.EverCharmed` is declared `yaml:"-"`
(`internal/characters/character.go:138`) — it is **never persisted** — and its
only production reader is `Death_MobLoot.go:87`, which sets `Corpse.WasCharmed`
to stop players *raising* an ex-companion. It has nothing to do with items.

The duplication loop it was meant to close is already shut from both ends:

- `mobs.SaveMobInstance` returns `nil` immediately for a charmed mob
  (`instance_save.go:83-85`), so a live charmed companion never writes a file.
- `scheduleMobDespawnFromLife` calls `mobs.DeleteMobInstance` on death
  (`Death_MobInstanceCleanup.go:72`) before destroying the instance, so a killed
  mob's file does not outlive the loot drop.

What is genuinely true, and is the only remaining window: `MobInstanceData`
**does** persist `Equipment` (`instance_save.go:50`), and `equipmentDiffers` is
itself one of the triggers that makes a mob worth saving
(`instance_save.go:388`). So an **ex**-charmed mob still wearing gear you handed
it *will* write an instance file containing that gear on the next
`MobSaveIntervalRounds` boundary (100 rounds).

**I could not construct an actual duplication from that window** — the gear is on
the mob, not on the player, and killing the mob deletes the file. Task 2 is
therefore a bounded investigation with a stated drop condition, not a pre-decided
fix. Slice C set the precedent: its gate 3 was kept only after confirming
link-dead could reach it, and that plan said in advance to "say so in a comment
rather than adding a guard against nothing."

**3. Defy is Willpower + rhetoric × SkillWeight**
(`internal/characters/combat.go:326-329`). The helpfile's "opposed by the
target's willpower" survives the redesign. Its "Defense: Mental" line does not.

---

## File structure

| File | Responsibility | Task |
|---|---|---|
| `internal/actions/cast_harm_authorization_test.go` | **Modify (append).** Pins that charm is routed by the harm-target policy and that a non-combatant shopkeeper is refused. | 1 |
| `internal/mobs/instance_save_charm_test.go` | **Create.** Pins the charmed-mob save skip. | 2 |
| `_datafiles/world/dogmud/spells/charm.yaml` | `description:` — the sentence the spell list shows. | 3 |
| `_datafiles/world/dogmud/templates/help/charm.template` | The full helpfile. **Owner: REQUIRED for completion.** | 4 |
| `docs/PATCH_NOTES.md` | One dated entry for the whole U10c arc. | 5 |
| `tools/playtest/profiles/charmer.yaml` | **Create.** A reusable playtest character that knows charm. Owner ruling: the permanent fix, not a one-off grant. | 6 |
| `tools/playtest/goals/2026-08-24-u10c-charm-arc.yaml` | **Create.** Goals file for the adversarial gate. | 6 |

---

### Task 1: Pin the non-combatant charm refusal

**Files:**
- Modify: `internal/actions/cast_harm_authorization_test.go` (append)

**Do not create a new file.** `cast_harm_authorization_test.go` already exists and
already owns this policy — it was written for chunk 5.2 finding 3 and has
`TestInitiateCast_HarmSingle_RefusesNonCombatant` at line 94. It carries the
fixtures you need: `newPlayerActor()` (line 40) and `seedRoomMob(t, room,
instanceId, name, mutate)` (line 48).

**The gap those existing tests leave** is precisely the one that matters here.
They all use a *synthetic* spell built by `seedTestSpell(id, spells.HarmSingle,
4)`. They prove the policy protects `HarmSingle`. They say nothing about whether
**charm** is `HarmSingle` — so if someone changed charm's `type:` in the YAML,
every one of them would stay green while shopkeepers became charmable.

Two tests close that: one pins the data contract, one pins the effect arm.

- [ ] **Step 1: Write the data-contract test**

Anchor the path on `runtime.Caller`. Test binaries in this repo do **not**
reliably run with the package directory as CWD — `internal/actions/economy_test.go`
chdirs to the repo root, and all tests in a package share one binary, so a
relative path passes or fails depending on test order.

```go
// Charm's protection from being cast at shopkeepers, tutorial NPCs and quest
// givers is not written anywhere in charm's own code. It is inherited: charm is
// type: harmsingle, so InitiateCast runs it through rejectHarmTarget ->
// mobs.CheckPlayerHarm exactly like every other harmful spell.
//
// That inheritance is invisible at the call site and would break silently. 369
// mobs carry non_combatant: true and only 331 of them ALSO carry
// charm_immune: true. The other 38 -- including the New Plymouth flower seller,
// a shopkeeper -- rely on this line of YAML and nothing else.
func TestCharmSpellYAML_IsHarmSingle(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	path := filepath.Join(repoRoot, "_datafiles", "world", "dogmud", "spells", "charm.yaml")

	raw, err := os.ReadFile(path)
	require.NoError(t, err, "charm.yaml must exist at %s", path)

	var sd struct {
		Type       string `yaml:"type"`
		EffectType string `yaml:"effect_type"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &sd))

	assert.Equal(t, "harmsingle", sd.Type,
		"charm must stay HarmSingle: it is what routes charm through "+
			"rejectHarmTarget, and it is the ONLY thing protecting the 38 "+
			"non-combatant mobs that lack charm_immune")
	assert.Equal(t, "charm", sd.EffectType)
}
```

- [ ] **Step 2: Write the effect-arm test**

`seedTestSpell` returns the `*spells.SpellData` it registered, so set
`EffectType` on it directly rather than adding a parameter.

```go
// The mirror of the contract test: given a HarmSingle spell whose effect arm is
// charm, a non-combatant is refused before the charm arm is ever reached. This
// is the shopkeeper case the owner named.
//
// Deliberately NOT using CharmImmune here. Most shop mobs carry both flags, so a
// CharmImmune fixture would be caught by applyMobEffect_charm's own early return
// and would keep passing even if the harm-target policy stopped covering charm.
// The exposure is a non-combatant WITHOUT charm_immune, so that is what this
// seeds.
func TestInitiateCast_HarmSingle_RefusesNonCombatantShopkeeperForCharm(t *testing.T) {
	sd, cleanupSpell := seedTestSpell("charm-policy-probe", spells.HarmSingle, 4)
	defer cleanupSpell()
	sd.EffectType = "charm"

	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9051, "Flower Seller", func(m *mobs.Mob) {
		m.NonCombatant = true
		// No CharmImmune on purpose -- see the comment above.
	})()

	result := InitiateCast(actor, "charm-policy-probe", "flower")

	assert.True(t, result.NoTarget)
	assert.False(t, result.Initiated)
	assert.Empty(t, result.TargetMobInstanceIds,
		"a non-combatant must never reach the charm effect arm")
}
```

Add any missing imports (`os`, `path/filepath`, `runtime`, `gopkg.in/yaml.v3`) —
check which yaml package the repo uses before importing:

```bash
grep -rn "yaml.v3\|yaml.v2\|ghodss" go.mod
```

- [ ] **Step 3: Run them and expect PASS immediately**

```bash
go test ./internal/actions/ -run "Charm" -v
```

Expected: both PASS. **This is the one place in this plan where green-on-first-run
is correct**, because the guard already exists. These are regression pins, not
fixes.

- [ ] **Step 4: Prove they can fail (mutation check)**

A test that has never failed proves nothing. Break the routing and confirm the
contract test goes red.

```bash
sed -i 's/^type: harmsingle/type: neutralsingle/' _datafiles/world/dogmud/spells/charm.yaml
go test ./internal/actions/ -run "TestCharmSpellYAML_IsHarmSingle" -v   # expect FAIL
sed -i 's/^type: neutralsingle/type: harmsingle/' _datafiles/world/dogmud/spells/charm.yaml
go test ./internal/actions/ -run "TestCharmSpellYAML_IsHarmSingle" -v   # expect PASS
git diff --exit-code _datafiles/world/dogmud/spells/charm.yaml         # must be clean
```

For the effect-arm test, mutate the code instead: temporarily comment out the
`rejectHarmTarget` call at `cast.go:95` and confirm
`TestInitiateCast_HarmSingle_RefusesNonCombatantShopkeeperForCharm` fails. Then
restore it and re-run the whole package, because several existing tests in this
file depend on that same line.

```bash
go test ./internal/actions/ -count=1
git diff --exit-code internal/actions/cast.go
```

- [ ] **Step 5: Commit**

```bash
git add internal/actions/cast_harm_authorization_test.go
git commit -m "test(charm): pin that non-combatants cannot be charmed"
```

---

### Task 2: The gear-persistence window — investigate, then decide

**Files:**
- Create: `internal/mobs/instance_save_charm_test.go`
- Possibly modify: `internal/mobs/instance_save.go` (only if Step 2 finds a real defect)

**Do not write a guard before Step 2 finishes.** The owner's concern was real in
intent; the mechanism it was attached to was not. Establish the fact first.

- [ ] **Step 1: Pin the existing skip, which is the load-bearing guard**

The path helper is `instancePath(mobId, zone, mobName, homeRoomId)` at
`internal/mobs/instance_save.go:64`. It is unexported, which is fine — put the
test in `package mobs` alongside `death_resets_progression_test.go`, which is the
established pattern for save tests in this package.

Note two preconditions inside `SaveMobInstance` that a naive fixture will trip:
it returns early unless `Balance.MobProgressionEnabled` is true, and again unless
`hasPersistableState(mob)` is true. `hasProgression` short-circuits before the
template lookup, so setting a `Training` value satisfies the second without
needing a registered template.

```go
// SaveMobInstance returns early for a charmed mob, and that early return is what
// keeps a companion's state single-sourced. A charmed companion's gear and
// progression live on CompanionInfo (saveCompanionState in
// PlayerDespawn_HandleLeave.go) and NOWHERE else. If this skip were removed the
// same sword would exist in two places at once -- the companion record AND the
// instance file -- reconciled by whichever loader happened to run last.
//
// Nothing tested this. It is one deleted `if` away from being lost.
func TestSaveMobInstance_SkipsCharmedMob(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.MobProgressionEnabled = true
	configs.SetConfigForTest(t, cfg)

	const (
		mobId      = MobId(9911)
		homeRoomId = 99110
		zone       = "TestZone"
	)

	mob := &Mob{MobId: mobId, InstanceId: 99111, HomeRoomId: homeRoomId, Zone: zone}
	mob.Character.Name = "Testcharmed"
	// Progression, so hasPersistableState would otherwise say yes.
	mob.Character.Stats.Strength.Training = 7

	path := instancePath(mobId, zone, mob.Character.Name, homeRoomId)
	_ = os.Remove(path)
	t.Cleanup(func() { _ = os.Remove(path) })

	mob.Character.Charm(1, 100, "")
	require.NoError(t, SaveMobInstance(mob))

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err),
		"a charmed mob must not write an instance file: its state belongs to "+
			"the owner's CompanionInfo, and a second copy on disk is how the "+
			"same gear ends up existing twice")
}

// The mirror. Without it the test above would pass even if SaveMobInstance were
// broken outright and never wrote anything at all.
func TestSaveMobInstance_UncharmedMobDoesSave(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.MobProgressionEnabled = true
	configs.SetConfigForTest(t, cfg)

	const (
		mobId      = MobId(9912)
		homeRoomId = 99120
		zone       = "TestZone"
	)

	mob := &Mob{MobId: mobId, InstanceId: 99121, HomeRoomId: homeRoomId, Zone: zone}
	mob.Character.Name = "Testfree"
	mob.Character.Stats.Strength.Training = 7

	path := instancePath(mobId, zone, mob.Character.Name, homeRoomId)
	_ = os.Remove(path)
	t.Cleanup(func() { _ = os.Remove(path) })

	require.NoError(t, SaveMobInstance(mob))

	_, err := os.Stat(path)
	assert.NoError(t, err, "an uncharmed mob with progression must still save")
}
```

Confirm the field name `MobProgressionEnabled` and the `Mob` struct's `Zone`
field before running — check with
`Select-String -Path internal\mobs\mobs.go -Pattern 'Zone'` and
`grep -n "MobProgressionEnabled" internal/configs/config.balance.go`. If either
differs, fix the fixture rather than the assertion.

- [ ] **Step 2: Try to actually reproduce a duplication. Time-box this.**

The one open window: an **ex**-charmed mob still wearing player gear writes an
instance file on the next 100-round boundary, because `equipmentDiffers` is a
save trigger.

Walk these four and write down what each produces:

1. Charm, equip, bond expires, **kill the mob** — corpse drops gear, file deleted
   by `Death_MobInstanceCleanup.go:72`. How many copies of the sword exist?
2. Charm, equip, bond expires, **do not kill**, force a save, restart the server.
   Does the mob reload wearing the gear, and does the player still hold one?
3. Charm, equip, **log out**. `saveCompanionState` copies gear into the record and
   destroys the instance; `PlayerSpawn_HandleJoin.go:56-63` strips charmed
   companion records on login. Confirm the gear is destroyed, not restored.
4. Charm, equip, **`dismiss`**. Mob keeps gear, is uncharmed and hostile, then
   saves. Same question as 2.

**Exit condition — this is binding.** If none of the four produces two copies of
one item, **write no guard.** Record the finding in the commit message and in
spec section 16, and move on. A guard against nothing is worse than no guard: it
reads as protection and will be maintained forever by people who cannot tell what
it is for. If one of them *does* duplicate, stop and report the reproduction
before writing anything — the fix likely belongs to the instance-save system
generally, which the owner has already flagged as wanting a proper look.

- [ ] **Step 3: Run and commit**

```bash
go test ./internal/mobs/ -run "TestSaveMobInstance" -v
gofmt -l internal/
git add internal/mobs/instance_save_charm_test.go
git commit -m "test(charm): pin the charmed-mob instance-save skip"
```

---

### Task 3: Rewrite the spell description

**Files:**
- Modify: `_datafiles/world/dogmud/spells/charm.yaml`

Current text, verbatim:

```
  You reach into the mind of a hostile creature and bend its
  will to yours, turning it into a loyal companion. The charm
  requires intense focus and is much harder against creatures
  already in combat. Stronger creatures resist more fiercely.
```

Two problems. It never mentions the clock or the betrayal, which are now the
whole point. And its last sentence, **"Stronger creatures resist more fiercely,"
is false** — spec 3.6 removed the power term. What resists is a strong *will*,
not a strong *body*, and spec 10.1 requires the new text to convey exactly that
distinction.

- [ ] **Step 1: Replace `description:`**

```yaml
description: |
  You reach into the mind of a hostile creature and bend its
  will to yours. A stubborn mind fights you hardest; a strong
  back is no defence at all. What you bind will follow you and
  fight beside you, but not forever, and it will not warn you
  when its own mind begins to return. The more completely you
  overpower it, the longer your hold lasts. When that hold
  breaks, whatever you bound is standing next to you, and it
  remembers.
```

Sentence two is the spec 10.1 requirement, not decoration: it tells the player
that a huge brute may be *easy* to charm and a frail scholar hard, which is the
single most counter-intuitive consequence of dropping the power term.

House rules this obeys, several of which the old text and the helpfile break: no
raw numbers, no en dashes or em dashes, wrapped under 80 columns, plain words an
ESL reader can follow.

- [ ] **Step 2: Verify it parses**

A malformed spell YAML panics at startup, not at build, so `go build` proves
nothing here.

```bash
go build ./... && echo BUILD_OK
python -c "import yaml,io; yaml.safe_load(io.open('_datafiles/world/dogmud/spells/charm.yaml',encoding='utf-8')); print('YAML_OK')"
```

- [ ] **Step 3: Commit**

```bash
git add _datafiles/world/dogmud/spells/charm.yaml
git commit -m "content(charm): the spell description tells you about the clock"
```

---

### Task 4: Rewrite `help charm` — OWNER-REQUIRED FOR COMPLETION

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/charm.template`

**The owner named this a required completion step for U10c.** The slice is not
done without it.

The file is wrong in five places and breaks two house rules.

| Current line | Why it is now wrong |
|---|---|
| "Your hold gradually loosens over time." | The re-roll ladder is deleted. The hold does not loosen; it ends. |
| "The stronger your charisma and the higher your manifestation skill, the longer the charm endures." | **False.** Duration is bought with the margin of the winning contest, not with your stats. |
| "Defense: Mental (opposed by target's willpower)" | It is answered by **defy** now. The willpower half is still right; "Mental" is not. |
| "Duration: Scales with charisma and manifestation skill" | The same falsehood as above. |
| "against the creature's willpower and **mental fortitude**" | The defending skill is **rhetoric** now, not a vague fortitude. |
| *(absent)* | **Logging out destroys the creature.** A major rule the file never states. |

**Read the current file before writing the new one.** Spec 10.2 makes the point
that it is the closest thing to a design document the original intent ever had:
it already describes a duration, a hold that breaks, and a creature that turns
hostile *only when it is near you*. None of that has ever run. This slice makes
it true, so the rewrite is mostly a matter of removing the two stat-scaling
claims and adding the rules that are genuinely new.

**Lines spec 10.2 requires you to PRESERVE, because they are still true:** charm
cannot target players; charm-immune creatures are beyond reach; creatures already
in combat resist more strongly; the companion limit and `dismiss`.

House-rule breaks: six em dashes (U+2014) and CRLF line endings. Check before and
after:

```bash
grep -c "$(printf '\u2014')" _datafiles/world/dogmud/templates/help/charm.template   # currently 6, must end 0
file _datafiles/world/dogmud/templates/help/charm.template
```

- [ ] **Step 1: Read the current file, then a clean exemplar**

```bash
cat _datafiles/world/dogmud/templates/help/charm.template
ls _datafiles/world/dogmud/templates/help/ | head -30
```

Use a recently-written helpfile as the ANSI-and-layout model; `help progression`
was written 2026-08-24 and is known clean. **Preserve CRLF if every other
helpfile in that directory uses CRLF** — check first, because a lone LF file in a
CRLF directory is a new inconsistency, not a repair. The em dashes go regardless.

- [ ] **Step 2: Rewrite, keeping the existing section skeleton**

Keep the header, `Usage:`, the labelled block, `Notes:` and `See also:`. Change
the prose. Content that must appear:

- It becomes a companion and fights for you.
- **The hold ends.** You are never told when, and there is no warning sign.
- **How completely you won the contest decides how long you keep it** — not your
  charisma, not your skill rank. Say it in plain words, without numbers.
- **A creature caught asleep is held longest.** A real tactic; the file should
  reward the player who works it out.
- When the hold ends and you are standing there, **it turns on you.** If you are
  elsewhere, it simply goes back to what it was.
- **Logging out destroys the creature**, along with anything you gave it.
- It keeps the gear you hand it and grows stronger while it serves you. Both cut
  both ways.
- It cannot be used on another player, on a merchant, or on anyone who is not a
  fighter.
- **A powerful creature is not harder to charm — it is far more dangerous when
  the bond breaks.** Spec 10.2 asks for this line, and it is the honest summary
  of dropping the power term.
- Corrected label lines: defence is **defy**, answered by the creature's
  willpower and its skill at arguing back; duration is decided by **how
  completely you won**.

**Do not state the duration formula or the rounds remaining.** Per spec 3.3 the
uncertainty *is* the mechanic. The copy must convey that the hold is finite and
that a decisive win buys a longer one, without ever becoming quantitative.

- [ ] **Step 3: Verify it renders on a booted server, over telnet**

Writing the file is not evidence that it displays. ANSI tags render wrong when
mis-nested, and the playtest harness strips colour, so use the raw telnet port.

```bash
# against a running server on the AI port
printf 'help charm\n' | <telnet to 33333>
```

Confirm: no stray `<ansi>` text, nothing past column 80, no em dashes.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/charm.template
git commit -m "docs(charm): help charm describes the spell that now exists"
```

---

### Task 5: Patch notes for the whole arc

**Files:**
- Modify: `docs/PATCH_NOTES.md`

One entry for U10c entire, not one per slice. Newest entries go at the top,
directly under `# DOGMud Patch Notes`.

- [ ] **Step 1: Read the two most recent entries for voice**

```bash
head -40 docs/PATCH_NOTES.md
```

The register is second person, plain, no numbers, no em dashes, and it explains
what changed for the player rather than what changed in the code.

- [ ] **Step 2: Add the entry**

Use **the real date on the day you write it**, not the date below. The heading
here says 2026-08-24 because that is when the plan was written.

```markdown
## 2026-08-24: Charm is a gamble now, not a purchase

Bending a creature to your will used to be permanent, which made it a
straightforward trade: pay the conviction, keep the creature. It is not that
any more.

The hold now ends. How long you keep it depends on how completely you won the
contest of wills, and nothing tells you how long that is. A creature you barely
overpowered may turn on you within the hour. One you dominated outright may
serve you all evening. You will not know which you have until it stops.

Catch something asleep and you will hold it longest.

When the hold breaks and you are standing beside it, it comes for you. If you
are somewhere else, it simply goes back to being what it was.

This cuts both ways with everything you invest in it. A charmed creature keeps
the gear you hand it and grows stronger fighting at your side, so the more you
put into one, the worse the moment it remembers itself.

Size is no longer any protection against you, and no longer any comfort either.
A stubborn mind is what resists; a big one is simply worse news later. Binding
something far above your weight is a real bet now rather than a shopping trip.

Logging out destroys anything you have charmed, along with whatever it was
carrying. Bring it home before you go.

One smaller thing. Charm now refuses to target other players, which the help
file always claimed it did and which nothing actually enforced.
```

**Do not write "no longer hunts you across zones" or anything of that shape.**
Charm was permanent before this arc, so no bond ever lapsed and no creature ever
came looking. Presenting the absent-caster rule as a fix would be claiming credit
for repairing something that never happened.

- [ ] **Step 3: Commit**

```bash
git add docs/PATCH_NOTES.md
git commit -m "docs: patch notes for the U10c charm arc"
```

---

### Task 6: The adversarial playtest gate — DO NOT SKIP

**Files:**
- Create: `tools/playtest/goals/2026-08-24-u10c-charm-arc.yaml`

**This gate carries more weight than usual.** Nobody has watched a charm resolve
in a live game since the Slice B rewrite. Slice B's own in-game verification was
never completed: granting charm needs discovery or a save edit, and the save edit
hit a forced password-change flow. Everything since then rests on tests and
reasoning alone.

Boot-clean verifies the system. It has never once verified the experience.

- [ ] **Step 1: Build a reusable charm profile — OWNER RULING 2026-08-24**

Getting charm onto the tester is what defeated Slice B's attempt. The owner
chose the permanent fix over the quick one: **add a profile, so this and every
future charm test just works.** Do not attempt a save edit — that is the route
that hit a forced password-change flow and lost the run.

Profiles carry a `spellbook:` block, which is the whole mechanism. Model the new
file on `tools/playtest/profiles/specialist-caster.yaml`, which already grants
six spells this way.

**Create `tools/playtest/profiles/charmer.yaml`:**

```yaml
role: user
username: template-charmer
character:
  name: Bindsong
  description: >
    A manifestation specialist built to exercise charm end to end: enough
    charisma to win the contest, enough conviction to pay the cost AND hold
    the companion reservation, and a spare weapon to hand a charmed creature.
  roomid: 462
  zone: Thornwall City
  speciesid: 1
  stats:
    strength:
      base: 100
    dexterity:
      base: 110
    perception:
      base: 115
    vitality:
      base: 115
    willpower:
      base: 130
    charisma:
      base: 150
  health: 460
  stamina: 450
  conviction: 585
  gold: 400
  skills:
    manifestation: 30
    spellcasting: 15
    rhetoric: 10
    weapon-combat: 8
    search: 10
  spellbook:
    charm: 3
    heal: 2
  equipment:
    weapon:
      itemid: 10018
    body:
      itemid: 20008
    feet:
      itemid: 20003
  items:
    - itemid: 10018
```

**Why these numbers, so nobody "simplifies" them into a profile that cannot
charm anything:**

- `ConvictionMax = 5 + Charisma*3 + Willpower*1` = 5 + 450 + 130 = **585**.
- Charm costs **120** to cast and then reserves `CompanionReserveDefault` = **280**
  for as long as the bond holds.
- `WouldBreachReservationCap` refuses the charm if the reservation would exceed
  `PoolReservationCapPct` (0.66) of the pool. 0.66 × 585 = 386, and 280 < 386, so
  it passes — **with margin, deliberately.** Drop charisma much below 150 and the
  reserve starts breaching the cap, at which point every charm is **silently
  refused** and the run reads as a broken spell rather than an underpowered
  character. This is the same trap that cost a debugging cycle in Slice C's unit
  fixtures.
- The **second** `itemid: 10018` under `items:` is the weapon to hand a charmed
  creature in goal 5. Without a spare, the tester has to disarm itself first.

Verify the coefficients before trusting the arithmetic above — they are balance
knobs and may have moved:

```bash
grep -nE "^  (ConvictionBase|ConvictionPerCharisma|ConvictionPerWillpower|PoolReservationCapPct|CompanionReserveDefault):" _datafiles/config.yaml
```

**Expect `CompanionReserveDefault` to return NOTHING, and do not treat that as
zero.** It is absent from `config.yaml` and therefore falls back to its Go
default of 280 in `internal/configs/config.balance.mobs.go`. Absence is
meaningful in this project: a missing key means "use the Go default", not
"unset". The four that do appear are `ConvictionBase: 5` (line 974),
`ConvictionPerCharisma: 3` (975), `ConvictionPerWillpower: 1` (976) and
`PoolReservationCapPct: 0.66` (1409), which is where the 585 and the 386 above
come from.

The cap is `floor(poolMax × pct)` — `reservationCapFor` in
`internal/characters/reservation.go:47`, reached via `ReservationCap`. It floors,
so use `floor`, not round, if you recompute this for different stats.

- [ ] **Step 1b: Register and smoke-test the profile**

Check how `playtestrun` discovers profiles before assuming a bare file is enough:

```bash
grep -rn "profiles/" internal/playtestrun/*.go | head
cat tools/playtest/profiles/README.md
```

Then confirm the character actually boots with charm known — log in and type
`spells`. A profile that loads but grants nothing wastes the whole run.

- [ ] **Step 2: Write the goals file**

It must carry a top-level `ephemeral:` block or local `playtestrun` refuses it.
Copy the block shape from an existing local goals file:

```bash
sed -n '1,30p' tools/playtest/goals/2026-08-03-prepush-sweep.yaml
```

Goals to drive, in order:

1. Cast charm at a creature and read every line. **One contest, one verdict** — a
   resist line and a success line for the same cast is the exact double
   narration Slice B removed, and its return is the first thing to look for.
2. Cast charm at a **player**. Expect a clean refusal, not a swallowed command.
3. Cast charm at a **shopkeeper**. Expect the harm-target refusal.
4. Cast charm at a **sleeping** creature. It should land and hold long.
5. Hand the charmed creature a weapon; confirm it uses it.
6. Wait out a short bond and watch the break in the room. Is "breaks free of your
   control" legible as a threat, or does it scroll past?
7. `dismiss` a charmed creature and confirm it turns on you.
8. Confirm the conviction bar returns when the bond ends, on both break and
   dismiss.
9. `help charm` and `spells` — read the new copy as a confused human would.

- [ ] **Step 3: Run it with an explicitly adversarial mandate**

```text
/playtest local --checkout C:/Users/Calabe Davis/workspace/DOGMud bug-finder 2026-08-24-u10c-charm-arc.yaml
```

Read every line of output. Report every usability problem bluntly. A pass means
the tester played it and found nothing, not that the harness exited zero.

- [ ] **Step 4: Extract findings to memory before doing anything else**

Playtest reports are gitignored. A finding left in the report is a finding lost.

- [ ] **Step 5: Fix what it finds; re-run if the fixes are behavioural. Commit.**

---

### Task 7: Gates, PR, merge

- [ ] `gofmt -l internal/ modules/` prints nothing.
- [ ] `go build ./...`.
- [ ] `go test ./internal/actions/ ./internal/mobs/ ./internal/hooks/ ./internal/combat/ ./internal/characters/ ./internal/usercommands/ -count=1`.
- [ ] `golangci-lint run` — no **new** finding on a touched file. The repo carries
      73 pre-existing findings across these packages; only new ones matter.
- [ ] Boot in an isolated detached worktree. `Server Ready` = 1, panic patterns = 0.
      **Exit 124 is the success case.** Never grep the bare word `panic` —
      `GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`.
      This matters more than usual here: Tasks 3 and 4 both edit content files,
      and content YAML panics at startup rather than at build.
- [ ] Spec section 16: record Task 2's verdict, whichever way it went.
- [ ] PR with `--repo pruuk/DOGMud`. Confirm each job ran with **zero
      annotations** — a green check is not proof.
- [ ] Merge `--merge`, never `--squash`.
- [ ] Mark the arc closed in the U10c memory topic file and in the plans index.

---

## For whoever executes this

Slice D is where U10c stops being a code change and becomes something a player
meets. The two tests are small. **The helpfile and the playtest are the slice.**

If a `grep` or `sed` finds something this document did not predict, **stop and
report**. Three plans in this arc have been written against facts that were
wrong, and two were rejected for it. The section at the top of this plan exists
because the two tasks that looked most obvious were both aimed at the wrong
thing, and only checking caught it.

In particular: **do not treat the 38 non-combatant mobs without `charm_immune`
as a bug to fix in YAML.** They are covered by code. Verify that yourself if you
doubt it, then leave the data alone.
