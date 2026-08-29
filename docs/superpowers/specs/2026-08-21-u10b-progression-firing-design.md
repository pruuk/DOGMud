> # 🛑 SUPERSEDED — this design was not built
>
> **Drafted 2026-08-21, abandoned the same day.** Kept as a record of an approach
> that did not work out. The plan built from it
> (`docs/superpowers/plans/2026-08-21-u10b-progression-firing.md`) failed its
> blind adversarial review with four blockers and was never executed.
>
> **The core premise died first.** This design assumed *use counters* drive
> progression rank. The owner replaced that with **training** as the rank, which
> invalidated several of the eight decisions recorded here and spawned the
> prerequisite slice U10b-0.
>
> **Its central proposal — three firing classes (contested / uncontested /
> bonus) — is NOT what shipped.** The settled rule is **best-of**: one event per
> resolved action, for the single highest-rolling candidate skill, full on a win
> and `ProgressionFailureFraction` on a loss, with crits and fumbles as a
> separate bonus layer that is never selected.
>
> **Read this instead:**
> `docs/superpowers/specs/2026-08-26-u10b-1-progression-firing-convention-design.md`,
> which carries a table of exactly what changed from this draft and why.

# U10b — progression firing consistency

**Status:** design drafted 2026-08-21 from a fresh current-state
verification (the source audit predates U9 and U10, both of which changed
this area). Owner taxonomy ratified 2026-08-21; see §7. Roadmap source:
`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` row U10b. Audit input:
`docs/audits/2026-08-19-progression-firing-audit.md` (read §1 below before
trusting any of its findings — two are already fixed).

## 1. Current state, verified at `d17836c64` (NOT the audit's state)

The August audit catalogued 135 sites and ten firing conventions. Since
then U9 and U10 shipped. Verified today:

| Audit finding | State now | Evidence |
|---|---|---|
| 1 — two melee defender sites disagreeing | **FIXED by U9** | `combat_helpers.go:1165-1174` is a tombstone comment; the per-swing site is gone; `processDefenderProgression` is the sole melee defence path and now awards skill AND stat |
| 2 — melee attacker firing twice | **FIXED by U9** | `internal/combat/combat.go` has **zero** progression calls (was 20); `applyCombatProgression` is the only attacker path |
| 3 — melee win-only vs channel win-or-lose | **STILL LIVE** | `defence_multiplier.go:364-367` names the divergence in source and defers it to U10b by name |
| 4 — Category C routing + `crafter.go` unscaled pair | **STILL LIVE, unchanged** | `mobs/crafter.go:505,546` still use unscaled `OnSkillUse` while every other crafting site uses `OnSkillUseScaled` |
| 5 — passive regen bypassing the guardrails | **PARTIALLY FIXED** | U9 added `TrackStatUse` (`progression.go:516`) and a rank floor (`regenDamperFactor`, `:409-420`), closing the rank-independence exploit. `CheckRegenProgression` still calls `IncreaseStat` **directly** (`:465`), still skips `CheckStatProgression`, still never applies `StatProgressionRate` (shipped 2.25) |

**The count moved the wrong way.** U9 unified what an event *carries* and
deleted duplicates; it never reduced how many *ways* events fire, and U10
added one more. Today there are **eleven** distinct firing shapes: nine of
the audit's ten survive (only "every defended swing" is gone), plus two
new ones — U10's success-only-on-a-secondary-contest, and U9's
bonus-tier-deduped-per-round (`claimBonusProgression`,
`progression.go:546-567`). Mechanical grep totals moved 138 sites/53
files → **98 sites/51 files** (106/53 counting `ApplyProgression`).

**Entry points no audit grep has ever covered** (found by walking every
exported `Character` method matching Progression/Skill/Stat/Regen):
`ApplyProgression` itself (8 production sites — the single most important
entry point in the system), and the no-roll grant paths `IncreaseStat`
(quest rewards `Quest_HandleQuestUpdate.go:363`, pack scaling
`pack_scaling.go:81`, quest engine `bridge.go:273`), `TrainSkill`, and
`SetSkill`. The grants are out of scope for a firing-consistency slice
(they are rewards, not use), but they are the only paths that move a stat
with no roll, no cap check, and no use tracking, and U10b's guard should
name them so the next audit does not rediscover them.

**Dead already:** `Character.OnCriticalSuccess` and `OnCriticalFailure` no
longer exist as methods (U9 replaced them with the bonus tier), but
no-op implementations survive on test fakes in nine `_test.go` files.
`Character.OnCritReceived` still exists (`progression.go:352`) with **zero
production callers** — `Outcome.ToughenStat` supplanted it.

## 2. The rule (owner, 2026-08-21)

Three classes and one bonus layer. Every production progression site is
assigned to exactly one class, and a guard test pins the assignment.

| Class | Fires | Rate |
|---|---|---|
| **Contested** | Only on a **win**. A roll happened and the actor came out ahead. Includes rolls against a static difficulty — this codebase already calls those contests (`contest.AgainstDifficulty`). | Standard progression curve |
| **Uncontested** | On the action, or on the tick. No roll, no opposition. | **Low chance, high frequency** — its own multiplier |
| **Bonus** | Layered on top of a contested event when it crit or critically failed. Never an independent event. | Existing `BonusEvents` machinery |

**Deleted outright:** first-kill-of-a-type progression (§5).

Two consequences, both **accepted by the owner 2026-08-21**:

- **Losing no longer trains.** Failed channel resists, failed steals,
  fruitless searches, and zero-yield salvage all stop awarding. The
  defence is still *charged* on a loss (U7/U8) — you pay for a failed
  dodge and learn nothing from it. Owner: acceptable, because the contest
  floors guarantee everyone wins sometimes and early progression is rapid.
- **Uncontested actions get much rarer progression.** `consider` and
  `look` currently roll perception at the *full* contested rate (~27% per
  use at rank 0). Under the uncontested multiplier they become the low
  trickle their triggering cost implies. Owner: acceptable — spamming a
  free look is a cheesy way to raise a stat.

**The one deliberate exception to "losing doesn't train you"** is the
toughen path (§3.4): being critically hit trains the stat that ate the
blow, and it fires for the side that just *lost* the exchange. It is a
bonus-class event, not a contested-class one, and it is carved out on
purpose. An implementer reading rule 1 alone would delete it as
inconsistent; §3.4 exists so nobody does.

## 3. Class assignment — every production site

### 3.1 Contested (fires on win only)

| Site | Today | Change |
|---|---|---|
| Melee attacker skill/crit (`NewRound_DoCombat_unified.go:666-679`) | `wh.CleanHit` | none |
| Melee attacker stat (`emitAttackerStatGain`, `:659-660`) | **always, every swing** | gate on a clean hit |
| Melee defence (`processDefenderProgression`) | win-only | none |
| **Channel defence** (`defence_multiplier.go:371-376`) | **contested, win or lose** | **gate on `res.Success`** |
| Special moves ×11 (`combat_bash/drain/gore/grapple/hamstring/kick/maul/pounce/rake/throttle/trip`) | `result.Hit` | none |
| U10's five sites | success-only | none |
| Taunt (`combat_taunt.go:183,270,325`) | **all three outcomes** | success branch only |
| Throw (`throw.go:454`) | **always** | gate on hit |
| Shoot skill (`shoot.go:199`) | on hit | none |
| Shoot perception stat (`shoot.go:197`) | **always** | gate on hit |
| Steal, plant, defuse | **fire BEFORE the roll** | move after resolution, gate on success |
| Shadow (`shadow.go:101,150`) | **on begin** | gate on success |
| Sneak | roll-happened | gate on success |
| Flee | contested | verify, then none |
| Search (`search.go:243`) | **roll-happened** | gate on a find |
| Track (`track.go:128`) | **roll-happened** | gate on success |
| Forage (`forage.go:142`) | on find | none |
| Salvage (`salvage.go:166,252`) | **always once committed** | gate on recovering at least one material |
| Crafting ×4 (`craft.go` ×2, both round-ticks) | success, difficulty-scaled | none |
| **`mobs/crafter.go:505,546`** | success but **unscaled** | use `OnSkillUseScaled` like the rest of the cluster |
| Bartering (`buy.go:786`, `sell.go:377`) | caller-decided success | verify, then none |

### 3.2 Uncontested (low chance, high frequency)

| Site | Today | Change |
|---|---|---|
| `consider` perception (`consider.go:27`) | full contested rate | uncontested rate |
| `look` perception (`look.go:85`) | full rate | uncontested rate |
| Venom-coat weapon-combat (`mutation_venom_coat.go:34`) | full rate | uncontested rate |
| Warcry/rally rhetoric (`skill_helpers.go:102`) | **in-combat always / out-of-combat 50%** | uncontested rate, one rule |
| Movement-trains-search (`go.go:388`) | own config probability | uncontested rate (this site is the class prototype) |
| Shop-mob charisma (`buy.go:789`, `sell.go:378`) | full rate | uncontested rate |
| **Regen ticks ×6** (`NewRound_AutoHeal.go:272,275,278,368,371,374`) | separate mechanism calling `IncreaseStat` directly | **join this class** — §4 |

### 3.3 Bonus (crit / critical failure)

Already the right shape: `progression.BonusEvents` + `claimBonusProgression`
dedupe per `skill|stat|class` per round, with `ClassCrit`/`ClassFumble`
deliberately not tracking use (tracking the doer punishes the achievement,
since the curve decays with rank) and `ClassObserved` tracking. Keep as-is.
U10b's only job here: ensure every contested site that can crit or fumble
routes its bonus through this layer rather than an ad-hoc second call, and
that no site emits a bonus event without a base event.

### 3.4 The toughen path — "I took a big hit" (audit only, no change expected)

Added to this spec 2026-08-21 at the owner's request: it was missing from
the audit's convention list and from the first draft of this spec, and it
is the one place progression rewards the loser of an exchange.

**What it does today**, verified at `d17836c64`:

- It is **not a standalone trigger**. It rides the bonus layer: on an
  `ExcAttackCrit` outcome, `BonusEvents` (`internal/progression/event.go:174-186`)
  emits `{SideDefender, DefenderSkill, ToughenStat, ClassObserved}`. The
  source calls it "the one cell that swaps the stat."
- **`ToughenStatFor(channel)`** (`progression.go:383-393`) maps the damage
  channel to the stat that eating the blow trains: physical → **vitality**,
  magical → **willpower**, conviction → **charisma**. Exported precisely so
  the seam and the applier cannot drift into two copies.
- Two production sites fill it: melee hardcodes `"physical"`
  (`NewRound_DoCombat_unified.go:709`) and the channel path maps the live
  channel (`defence_multiplier.go:525`, with a comment at `:559` warning
  that an empty channel string would silently toughen the wrong stat on a
  bash or trip).
- `ClassObserved` **does** track use, unlike `ClassCrit`/`ClassFumble` —
  correct, since being hit repeatedly is what moves the rank.
- A **floored** outcome emits nothing (`event.go:170-172`): a crit the
  floor granted rather than the dice teaches nobody.

**Why it is consistent with the taxonomy, despite rewarding a loss.** It is
a class-3 (bonus) event, and the bonus class's rule is *"something
exceptional actually happened, so both parties learn from it"* — the winner
via `ClassCrit`, the loser via `ClassObserved`. Rule 1 ("contests fire on a
win") governs class-1 base events only. Nothing here changes.

**Audit observations to carry into the plan** (findings, not change
requests):

1. **It is "took a CRITICAL hit," not "took a big hit."** A huge non-crit
   blow trains nothing. Given crits are margin-derived rather than
   damage-derived, a low-margin crit for modest damage trains vitality
   while a massive clean hit does not. **Owner ruled 2026-08-21: keep
   crit-only, unchanged** (§7 decision 8). There is **no damage-magnitude
   gate anywhere in this path** — verified at `d17836c64`, the whole gate
   is `if o.Floored || o.Exceptional == ExcNone { return nil }`
   (`event.go:169-172`), and `Outcome` carries no damage amount into
   `BonusEvents`. Two things that resemble one and are not: `o.Floored` is
   the U6 **contest** floor (a floor-granted crit, not a damage
   threshold), and the "old hard 25%-threshold system" named at
   `config.yaml:1040` was the **regen** path's pool-depletion gate, since
   replaced by the smooth curve. No commit touching `ToughenStat` ever
   added a percent-of-health floor.
2. **Vitality's only other source is the regen tick** (§4). Since U10b
   moves regen into the uncontested class, the toughen path becomes one of
   exactly two vitality sources, and the only *active* one. Any change to
   the uncontested multiplier therefore shifts vitality's growth curve
   harder than any other stat's — the modelling step must check vitality
   specifically, not just a representative stat.
3. **Only `ExcAttackCrit` toughens.** On `ExcAttackFumble` the defender
   receives their ordinary `DefenderStat`, not the toughen stat, which is
   coherent (nobody toughens from a flailing miss) but should be pinned so
   a future refactor does not "symmetrise" it.
4. **`OnCritReceived`** (`progression.go:352-378`) is this path's dead
   predecessor: zero production callers, superseded by `Outcome.ToughenStat`.
   Its deletion is already in §5 — the plan must confirm the toughen path
   is genuinely the live replacement before deleting, so the mechanic is
   not removed along with its corpse.
5. **The curve requirement is already satisfied** (owner condition,
   2026-08-21). The live chain is `BonusEvents` → `ApplyProgression`
   (`progression.go:613-618`) → `applyBonusProgression` → `TrackStatUse`
   **before** rolling → `CheckStatProgression` → `statProgressionChance`
   (`:157`) → `CalculateProgressionChance(virtualRank, softCap)`
   (`:47-64`) — the same exponential-decay curve every other progression
   roll uses, with `virtualRank = useCount / UsesPerRank` and the
   anti-exploit floor that promotes a stat **value** above
   `StatProgressionSoftCap` into the rank. This is what U9's
   `8aca31341 fix(u9): put crit-received progression on the decayed curve`
   bought. **Nothing to build — but nothing pins it either**, so §8
   criterion 5 is extended to lock it.
6. **The existing curve test is named after the corpse.**
   `TestCritReceivedProgression_DecaysWithRank`
   (`progression_faucet_test.go:47`) does pin production's real
   expression — its helper calls `statProgressionChance`, which the live
   seam also uses — but its name and its entire doc comment describe
   `OnCritReceived`. When §5 deletes that method the test reads as dead
   scaffolding and the next sweep will delete it. **Re-anchor it to the
   seam, do not remove it with its namesake.** This is observation 4's
   hazard in its second form: the corpse's *test* is load-bearing too.

**Channel coverage — spell and taunt crits DO toughen** (owner asked
2026-08-21; verified at `d17836c64`, every production site walked):

| Attack | Channel | Toughens | Entry point |
|---|---|---|---|
| Melee / special attacks | `ChannelMelee` | vitality | `NewRound_DoCombat_unified.go:709` (hardcoded) + `skill_moves.go` via the seam |
| Ranged, `throw` | `ChannelRanged` | vitality | `throw.go:312` → seam |
| Spell, player→mob | `ChannelSpell*` | **willpower** | `spell_resolution.go:332` |
| Spell, player→player | `ChannelSpell*` | **willpower** | `spell_resolution.go:853` |
| Spell, mob→mob | `ChannelSpell*` | **willpower** | `spell_resolution.go:1364` |
| Spell, mob→player | `ChannelSpell*` | **willpower** | `spell_resolution.go:1391` |
| Taunt (player and mob) | `ChannelSocial` | **charisma** | `combat_taunt.go:176`, both `usercommands`/`mobcommands` through the one `ExecuteTaunt` |
| Counter-taunt | `ChannelSocial` | **charisma** | `combat_counter.go:162` |

All non-melee rows reach `ToughenStat` through one function,
`channelDamageChannel` (`defence_multiplier.go:562-573`), so there is no
gap to fix. Three hazards in it that the plan must pin, because all three
are the kind a well-meaning refactor breaks:

7. **`ChannelSpellPhysical` deliberately answers `"magical"`.** A spell
   with `TargetDefenseType: physical` is *defended* by dodge/block instead
   of quell, but its damage is still cast off willpower, so eating a crit
   from it must toughen **willpower, not vitality**. The mapping looks
   like a bug and is not. Pin it by name.
8. **The `default: return ""` arm degrades silently, it does not fail.**
   `ToughenStatFor("")` returns `""`, and `BonusEvents` then falls back to
   `toughen = o.DefenderStat` (`event.go:174-177`) — so an unmapped or
   newly-added channel quietly stops toughening and trains the ordinary
   defence stat instead, with no error and no test failure. The source
   comment at `:559` warns about exactly this for bash and bolt crits. The
   class guard should assert every `AttackChannel` constant maps to a
   non-empty damage channel, so adding a channel without a mapping breaks
   the build rather than the mechanic.
9. **Melee is a second source of truth.**
   `NewRound_DoCombat_unified.go:709` hardcodes `ToughenStatFor("physical")`
   rather than calling `channelDamageChannel(ChannelMelee)`. It is
   currently correct and it is the exact drift `ToughenStatFor` was
   exported to prevent. Either route it through the mapper or pin the two
   against each other.

Note that willpower and charisma are **not** in vitality's two-source
predicament (obs 2) — both have ordinary skill-driven sources — so the
criterion-4 modelling worry stays vitality-specific. But all three
channels share this one code path, so the pins below cost nothing extra.

## 4. Regen joins the uncontested class

Owner ruling: *"unify it with the uncontested rolls, low chance, high
frequency, no opposition."*

- `CheckRegenProgression`'s direct `IncreaseStat` call
  (`progression.go:465`) is deleted. Regen ticks emit an ordinary
  uncontested event through the same applier as every other uncontested
  site, so the soft-cap and anti-exploit floor apply structurally rather
  than by `regenDamperFactor` reproducing them by hand.
- Pool depletion stays as the *magnitude* input (a fuller pool trains
  less), but it stops being a bespoke chance formula.
- **The shipped uncontested multiplier must be chosen so today's effective
  regen progression rate is preserved.** Regen currently skips
  `StatProgressionRate` (2.25); routing it through the standard path
  without a compensating multiplier would silently multiply passive stat
  training world-wide. The number is derived in the plan's modelling step,
  not guessed. If one multiplier cannot serve both regen and the
  action-triggered uncontested sites, the class takes two knobs and the
  spec says so rather than fudging one.
- `regenDamperFactor` and `CheckRegenProgression` are deleted once the
  class carries their job.

## 5. Deletions

- **First-kill-of-a-type progression** (owner). Delete
  `Character.OnFirstMobKill` (`progression.go:322-332`), both call sites
  (`Death_MobKillCredit.go:61,86` — killer and party members), and the
  player-facing message *"Defeating a new foe hones your combat
  instincts!"*. **Keep `KD.AddMobKill`** and the kill-tracking around it:
  that bookkeeping feeds the kill/bestiary displays and is not
  progression.
- **`Character.OnCritReceived`** — zero production callers; superseded by
  `Outcome.ToughenStat`/`ToughenStatFor`.
- **Stale test-fake `OnCriticalSuccess`/`OnCriticalFailure`** no-ops in
  nine `_test.go` files, for methods that no longer exist on `Character`.

## 6. Mob-spell gate asymmetry (in scope)

The player spell path applies a self-cast progression penalty, zeroes
progression for an area cast that found no targets, and gates on
`spellBonus > 0` (`NewRound_DoCombat_helpers.go:357-396`). The mob path
(`:531-552`) has **none of the three** and fires unconditionally on
`CastComplete`. That is a firing-condition inconsistency, so it belongs
here: the mob path adopts the player path's gates.

## 7. Owner decisions (2026-08-21)

1. **Three classes**: contested (win only), crit/critical-failure bonus,
   uncontested-and-regen-ticks merged. Not one literal rule, not eleven.
2. **Defence timing: success only, everywhere** — the channel path
   converges on melee's win-only shape.
3. **Skullduggery gates on success** — defuse, plant, and steal stop
   training on failed attempts.
4. **Regen unifies with uncontested**: low chance, high frequency, no
   opposition.
5. **First-kill-of-a-type progression is deleted.**
6. **Both §2 consequences accepted**: losing stops training (floors
   guarantee everyone wins sometimes; early progression is rapid), and
   free-action stat training drops to the uncontested trickle (spamming
   `look`/`consider` was a cheesy way to raise a stat).
7. **The toughen path is audited, not changed** (§3.4) — it was missing
   from the audit and from this spec's first draft.
8. **Toughening stays crit-only, with no damage-magnitude gate**, on the
   condition that it decays toward the soft cap on the standard curve.
   Verified already true (§3.4 observations 1 and 5); the condition
   becomes a test rather than a build (§8 criterion 5).

## 8. Done when

Each criterion ships as a test, not prose (the U6b lesson):

1. A **class guard** enumerates every production progression call site and
   the class it belongs to, and fails on any site that is unassigned, or
   whose gate does not match its class. It must also name the no-roll
   grant paths (`IncreaseStat`, `TrainSkill`, `SetSkill`) as deliberately
   out of scope so the next audit does not rediscover them as gaps.
2. No contested site fires on a loss: a property test drives representative
   contested actions to failure and asserts a zero progression delta.
3. No uncontested site fires at the contested rate: the uncontested
   multiplier is applied at every §3.2 site.
4. Regen progression's effective rate is unchanged from `d17836c64` within
   a stated tolerance, proven by a statistical test, and no production code
   calls `IncreaseStat` from a progression path. **Vitality is checked
   explicitly** (§3.4 observation 2): with regen moved into the uncontested
   class, vitality has exactly two sources and the toughen path is its only
   active one, so it feels a multiplier change harder than any other stat.
5. The toughen path still fires: a defender who takes a real (non-floored)
   critical hit gains one `ClassObserved` event on the channel's toughen
   stat, and gains nothing when the crit was floor-granted. Pinned per
   channel (physical→vitality, magical→willpower, conviction→charisma).
   Three further pins, all owner conditions from §7 decision 8:
   **(a)** the roll runs on the shared decayed curve — a high-rank
   defender's toughen chance is strictly below a rank-0 defender's, and
   the chance expression is production's `statProgressionChance`, not a
   duplicate;
   **(b)** the event tracks use **before** rolling, so vitality's virtual
   rank actually moves (untracked, the rank sits at 0 forever regardless
   of value — the fyttyn exploit);
   **(c)** damage magnitude does **not** gate it — a crit for trivial
   damage toughens and a huge non-crit does not, asserted directly so a
   future reader does not "fix" it into a threshold;
   **(d)** the per-channel pin is driven from the `AttackChannel`
   constants, not a hand-written list, and asserts every constant maps to
   a non-empty damage channel (§3.4 observation 8) — so a channel added
   without a mapping fails the build instead of silently falling back to
   the ordinary defence stat. `ChannelSpellPhysical → willpower` is
   asserted by name (observation 7), and the spell rows cover **mob-cast
   at a player** as well as player-cast, since those are separate call
   sites (`spell_resolution.go:1391` vs `:853`).
   The test that covers (a) today is `TestCritReceivedProgression_DecaysWithRank`;
   §3.4 observation 6 requires it be re-anchored to the seam, not deleted
   alongside `OnCritReceived`.
6. `OnFirstMobKill`, `OnCritReceived`, `CheckRegenProgression`,
   `regenDamperFactor`, and the nine stale test-fake methods do not exist;
   `KD.AddMobKill` still does.
7. The mob spell path applies the same three gates as the player path.
8. Every crafting site uses the difficulty-scaled call.
9. An adversarial playtest gate closes the slice, with the cribsheet rows
   it needs (a failed steal trains nothing; resting trains at the old
   rate; `consider` spam no longer trains perception; taking a critical
   hit still toughens).
