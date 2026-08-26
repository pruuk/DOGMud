# Pre-deploy playtest crib sheet (unified resolution arc)

The owner's manual checklist for the end-of-arc playtest with Meirok on the
local server. Covers everything on master through U6 plus the caster-brain
fix (2026-08-15). U7-U12 items get appended as they land; do not deploy
until every section has been played, the two MANDATORY items pass, and
every section 0 BLOCKER is fixed.

Meirok baseline for reference: melee score ~455 (Dex 110 + weapon-combat 69
x SkillWeight 5), block 523, spellcasting 51, crit mult 5.45. Against trash
he one-shots everything, so most sections need the named instruments below,
not street fights.

## 0. BLOCKERS: fix before this arc deploys

Findings that must be **fixed**, not merely played around. Distinct from
section 9, which lists things that are accepted-open and should not be
re-reported.

An entry here can also be a **required investigation** rather than a required
fix, when the defect has not been reproduced and the test day is the cheapest
place to confirm it. Those are marked in their own status line. They are still
mandatory — the arc does not ship on an unanswered question.

### 0a. A corpse blocks targeting of the live mob sharing its name

**STATUS 2026-08-24: NOT REPRODUCED. This is now a REQUIRED INVESTIGATION TASK
for the test day, not a fix-first blocker.** Owner ruled 2026-08-24 that it is
easier to trigger in the **Ironwind Steppes** than in the city, so drive it
there: fast respawns plus same-prefix names make the needed coincidence likely.

Found 2026-08-22 in the U10b-0 Phase B playtest. Section 9's older "corpse
targeting can pick the wrong corpse" is the milder cousin of this; this one
refuses the live actor outright.

**Do not repeat this work — it has been ruled out:**

- `room.FindByName` is NOT the culprit. A live `Bandit Scout` plus a **real
  `Corpse`** of the same name in one room resolves `"scout"`, `"bandit scout"`
  and `"bandit"` all to the live mob, and `FindCorpse` still finds the corpse.
  (Note `respawn_targeting_test.go` fakes corpses as `Item`s and never
  populates `r.Corpses`; the throwaway repro used the real field.)
- Neither command consults corpses before live actors. `look` resolves the live
  actor at `look.go:81` and only reaches its corpse block at `:418` if that
  failed — so `look <name>` showing the corpse means **live resolution failed
  first**. The two repro lines are one bug, not two.
- No liveness filtering in `ResolveTargetActor` or `FindAll` would hide a live
  mob.
- Attempted in a running server: killed a mob and polled for "live mob + its own
  corpse present together". The respawn never coincided with the corpse inside
  the window watched. **That coincidence is the whole trigger.**

It may also simply be **fixed** — the stale-id work landed after this was filed.
Confirm it still reproduces before hunting further.

**What to capture if it does reproduce:** the exact zone and room, the mob name,
whether the corpse is that same mob's, and whether `look` and `consider` fail
together or separately.

**Workaround, and it is a real one:** `assess corpse` targets the last thing
killed, `assess 2.corpse` the one before it, and so on. Corpse interaction is
always available by index even when name targeting misbehaves.

Related fix already merged (`eda8a958f`, PR #63): `consider` truncated its
target to the first word, so `consider bandit archer` silently considered a
Bandit **Scout**. That explains the `consider bandit scout` line of the repro
below but not the headline symptom.

One stale claim in the original entry: it said this "silently starves
progression" because look/consider are perception faucets. **Phase D unhooked
look and consider from perception**, so that reasoning no longer applies.

Reproduction, in a room holding both:

```
Also here: Bandit Scout (100%)
On the Ground: Bandit Scout corpse

consider scout        -> You don't see them here.
consider bandit scout -> You don't see them here.
look scout            -> [the CORPSE] "This is a corpse. They are dead."
```

Control, live NPC with no matching corpse in the room:

```
consider garve -> You consider Caravan Guard Garve...
```

Eighteen consecutive refusals across two rooms. It is common rather than
exotic: after any fight the mob you most want to consider or look at is exactly
the one whose corpse is now on the floor, and same-type mobs respawn into the
same room.

It also silently starves progression. `look <target>` and `consider` are two of
only three production callers of `OnStatUse("perception")` (the third is
`shoot`), so wherever a corpse is lying around, perception has almost no faucet.

When checking the fix: confirm `consider <mob>` and `look <mob>` both reach the
live creature with its own corpse present, and that `look <mob> corpse` still
reaches the corpse.

## 1. MANDATORY: the Elemental Queen (planar oasis instance, ~300g)

She casts for the first time ever (dead archetype until 2026-08-15). Buy in
at ~300g and fight her seriously.

- [ ] Difficulty: does the fight feel fair for the gold paid? She should
      open with a chrysalis-cocoon shield, throw conviction-spike at you,
      conviction-barrage when you bring company, and flee below 30% health.
- [ ] QUELL answers EVERY cast now. She was picked as the quell instrument
      back when a royal crit skipped quell entirely; U6b removed that skip,
      so there is no longer any cast your quell does not contest. Expect
      visible quell narration on every cast: your CONVICTION dropping when
      you defend (win or lose), a barely-quelled spell still stinging with
      damage words, and a decisively-quelled one doing nothing. A cast that
      resolves in silence is a finding.
- [ ] Far fewer royal crits at 300g. Her crits used to ignore your training;
      the crit bar now weighs her skill against yours, and she is no
      swordsman, so at 300g the crits should be rare. Note the deliberate
      repricing that came with this: roughly 500g now buys the threat 300g
      used to. If her fight feels flat for the gold, that is the repricing
      (tunable via the instance stat-pool multipliers and cap in config),
      not a resolution bug.
- [ ] Floor and ceiling feel: in a long fight you should occasionally be
      stopped while dominating, and she should occasionally get through
      while losing. Neither should feel like the game cheating.
- [ ] Known latent hazard: mob area spells have no friendly filter. If
      royals share her room, her barrage may hit her own pack. Note whether
      it happens and how it reads.
- [ ] While casting she should stop meleeing; between casts she fights.

## 2. Partial deflection (any near-even melee fight)

A defensive win now deflects instead of erasing: a bare win removes half
the blow, scaling to full negation at a defensive crit.

- [ ] Every deflected swing reads as ONE line per viewer, naming the
      defence and the damage it let through ("X dodges your swing, but you
      still connect! (light wounds)"). No "misses you completely! The blow
      still catches you." composites, no dodge line plus hit line for the
      same swing.
- [ ] Damage words scale sensibly: graze on a good defence, full band on a
      clean hit, nothing on a defensive crit.
- [ ] Defensive crits still fully negate and still fire RIPOSTE / SWEEP /
      SHIELD SLAM counters, in both directions.
- [ ] Being blocked-but-hurt feels intentional, not like a bug.
- [ ] Sounds: clean hits play the hit sound; deflections play the miss
      sound alongside the deflection text.

## 3. Maneuvers (bash, trip, kick, and beast moves)

- [ ] A DEFENDED trip/bash/kick can deal partial damage but never the
      knockdown, and its message says so ("fails to take them down, but
      still catches them"). Health must move when damage words print.
- [ ] Kick variants still auto-select (kick standing, stomp on prone, knee
      in grapple). Bash still demands a shield.
- [ ] Mob-side beast moves (gore, maul, pounce, rake) narrate partials the
      same way when you defend them.

## 4. Quell and defy as living defences

- [ ] Defy: taunt something tough and watch "Your target defies you, and
      the barb loses its edge." Get taunted back if you can find a taunting
      mob; your conviction should drain from defending.
- [ ] Read `help quell`, `help defy`, `help defense`, `help combat` after
      playing: anything they promise that the game did not do is a finding.
- [ ] KNOWN forward reference: the help copy says running a pool dry makes
      its defences collapse. That is TRUE ONLY AFTER U8 lands. If U8 gets
      descoped or reordered, those two help files must be revised.

## 5. Physical-flavoured spells vs a melee build

Spells like kinetic-hurl are now answered by dodge and block (stamina),
not by a Perception roll. Meirok's block should shine here.

- [ ] Find a caster throwing physical spells; confirm your dodge/block
      answer them, your STAMINA (not conviction) pays for it, and partial
      mitigation reads correctly.

## 6. The North Road bandit camp

The whole camp got its brains back (party overlay used to shadow every
combat archetype).

- [ ] The lookout whistles and calls for help; the camp arrives within a
      couple of rounds and focuses one target.
- [ ] The caster telegraphs, weaves, lands mental spells, heals herself
      when hurt, and holds melee while casting.
- [ ] Soren rallies with a warcry that visibly buffs packmates, and fights
      like a leader rather than a fourth melee body.

## 7. Balance riders needing your explicit sign-off (shipped in PR #47)

1. Mounting quell/defy costs conviction (2 each; previously free).
2. Physical-flavoured spells face dodge + block and charge defender
   stamina.
3. The deflection partial is a curve (decisive defence takes far less than
   the old flat 50%).
4. Every hostile spell drains a little defender conviction win-or-lose
   (compounds in AoE).
5. Quell/defy effectiveness knobs ship neutral at 1.0.
6. Drain's lifesteal feeds on partial damage from defended attempts.
7. Attacker skill progression no longer accrues on deflected swings.
8. (Rider) The Elemental Queen actually casts; see section 1.

## 8. Smaller checks worth thirty seconds each

- [ ] Sniper reveal: a hidden shooter whose deflected arrow still draws
      blood is revealed; a clean zero-damage miss keeps them hidden.
- [ ] The windscour wyrm (Ironwind Steppe) enrages in phase 2 for the
      first time (trip/bash under 50% health; its enrage was wired dead
      since authoring).
- [ ] Dewey and the ferry/passage agents greet you normally; none of them
      says "pays you little mind." on entry.
- [ ] Set-piece bosses (Sentinel, Core Guardian, Warden Prime, Old Edrin,
      beetle queen) still stand and fight; none of them flees its arena.
- [ ] A sleeping target still takes auto-crits for the whole first round.

## 9. Known-open findings: do NOT re-report these

Crit lines lack damage words and repeat; surprise/ranged attacks double
print; `consider` calls one-round walkovers "even"; ASCII charset leaks
UTF-8 symbols; AI-port batched commands drop silently; corpse targeting by
mob name can pick the wrong corpse; "looks a little confused (east )"
leaks a direction token; Dense Muscles hides its conviction/charisma cost.
All filed. Anything NOT on this list that reads wrong is a fresh finding.

Note the corpse-targeting entry has a sharper sibling that is a **deploy
blocker**, not an accepted-open finding: see section 0a.

## 10. Reserved for U7-U12

Append checks here as each chunk lands (defence costs and encumbrance in
U7, exhaustion consequences in U8, concentration and knockdown reworks in
U10, the help sweep in U11, targeting simplification in U12).

### 10a. U7: the unified cost model

Every action is now priced by one formula, and two of the inputs (defending
at all, and carried weight on anything physical) have NEVER been charged
before. This section is about FEEL, not arithmetic: no number in the game
should be visible to you, so judge it on whether stamina moves when it
should and whether the choices it forces are interesting.

Setup note: to arrive laden, carry Heavy Weighted Stones (item 12). Walking
unloaded GAINS stamina, so drain first and engage immediately.

- [ ] Long fight, stamina watched: pick something that survives many rounds
      and watch your stamina across the whole fight. It should drain
      steadily rather than in one cliff, and it should drain whether you
      are swinging or being swung at. If you can stand in a fight
      indefinitely without your stamina moving, that is a finding.
- [ ] Four-mob room: `3078 Wolf Den Approach` in Ironwind Steppe is the
      harshest spawn in the world (four wolves). Fight it and watch what
      four incoming attackers do to a defender's stamina. Defending is now
      charged on every swing you answer, so this is the worst case in the
      game by design. Verify it is punishing without being unwinnable for
      Meirok, and note where the line sits for a weaker character.
- [ ] Laden vs unladen defence, same character: fight the same opponent
      twice, once carrying almost nothing and once close to your carrying
      limit. The laden run should cost noticeably more stamina per round.
      The curve is gentle with room to spare and steep near the limit, so
      a moderate pack should barely register and a near-full one should
      hurt. If moderate and heavy feel the same, that is a finding.
- [ ] Skilled vs novice discount: compare a well-practised action against
      a barely-trained one. Meirok's weapon-combat swings should feel
      markedly cheaper than a fresh character's, and a defence you rarely
      use should cost more than one you lean on. Spawn a fresh character
      if you need the contrast; the difference should be obvious over a
      fight, not a rounding artefact.
- [ ] Travel at ordinary load vs travel near capacity: walk a long route
      with a light pack, then walk it again near your carrying limit. The
      ordinary walk should be slightly cheaper than you remember. The
      laden walk should be markedly dearer, to the point of changing
      whether you make the trip in one go. Also confirm no move ever costs
      nothing, and that no single move drains an absurd share of the pool.
- [ ] Reserved-pool stand-up: take a character carrying heavy pool
      reservation (fielded companions, or Chrysalis / pinnacle gear with
      reserve percentages), get knocked prone, and stand. This was a
      permanent lockout: the gate asked for a share of a pool the
      character could never fill, refused, and blamed exhaustion, which
      resting could not fix. Confirm standing works, and that if it does
      refuse, the refusal names the reservation descriptively rather than
      just calling you tired.
- [ ] Walking rarely trains search: this is deliberately very rare, so
      DO NOT treat its absence over a session as a finding. Note it only
      if you see it fire suspiciously often.
- [ ] Quell and defy ignore weight: mount a quell or a defy while heavily
      laden. It should cost conviction and the pack should make no
      difference at all. If a heavy load makes holding your nerve dearer,
      that is a finding.

### 10b. U7b: the reservation ceiling

Total reservation on each pool is now capped, the breaching action is
refused rather than allowed through and clamped, and nothing is ever taken
from a character who is already past the cap. Alongside it, every summoned
and raised creature was re-priced and re-powered. Judge this on whether the
refusals are legible and whether the pet tiers feel genuinely different.

- [ ] `status` on Meirok before anything else. His conviction should read
      `notable` or `heavy`, not `at limit`: his two golems rebase on login
      and he should land comfortably under the ceiling with both kept. If
      it says `at limit`, the login refresh did not run.
- [ ] Try to breach it deliberately. Stack reserving gear until `status`
      says `at limit`, then try to wear one more reserving item. Confirm
      the refusal names reservation rather than exhaustion, carries no
      numbers, and leaves the previous item still worn and the new one
      still in your pack.
- [ ] Sidegrade while at the limit. With one pool reading `at limit`, swap
      one reserving item for another of the same weight. It must be
      ALLOWED. If it is refused, grandfathering is broken and the character
      has been forced to strip.
- [ ] Conjure a magma elemental as a mid-level summoner. It used to be
      uncastable. Confirm the cast lands, then confirm `status` shows the
      reservation it now costs to keep.
- [ ] Raise a skeleton and a flesh golem off comparable remains, one after
      the other. The golem must be visibly the stronger of the two at every
      size of remains, not merely marginally so at large ones. Try it once
      over something weak and once over something formidable.
- [ ] Fight alongside a water elemental, a skeleton and a hive swarm. Each
      should act on nearly every round. If any of the three regularly
      stands there doing nothing, its archetype fix did not take.
- [ ] Fight alongside a fire elemental. It should keep acting all fight,
      not ward itself once and then idle.
- [ ] A Chrysifier with the homunculus apex. Confirm the homunculus still
      manifests. If it refuses and speaks the reservation message, report
      it immediately: the homunculus is a crafting apex whose owner has no
      reason to have invested in manifestation, and it carries the heaviest
      base reserve in the game, so this is the interaction most likely to
      bite. Its base was already lowered once for exactly this reason.
- [ ] Fight long enough for an enchant tier-up while near the limit.
      Confirm the skip message arrives, that it does not repeat every
      round, and that the item still tiers up later once you remove
      something.
- [ ] Log in after selling or losing a piece of gear that grants
      manifestation. You should be told your bonds were re-priced, in
      words, and nothing should be dismissed.
- [ ] A brand-new character who is both a novice enchanter and a novice
      summoner pays a small premium on each. Each half is deliberate, the
      compounding is not designed. Note whether it is noticeable in play or
      purely theoretical.

## U9 progression layer (2026-08-19)

Rates moved in BOTH directions in U9, so the felt frequency is what matters,
not the arithmetic. Four duplications were removed and one faucet was closed;
the crit and fumble bonus tiers were added or widened at the same time.

- [ ] Fight for ten or more rounds and count how often a skill or stat
      improvement banner appears. Before U9 a melee round awarded its
      attacker progression twice and its defender once per defended swing,
      so the honest expectation is FEWER banners than you are used to, not
      more. If it feels dead rather than merely slower, say so.
- [ ] Land a critical hit and watch for the brilliance message. Take one and
      confirm you are told nothing (receiving is deliberately silent) but
      that vitality still creeps up over a long fight.
- [ ] Fumble deliberately, repeatedly, and confirm the learn-from-mistakes
      message still arrives. Fumbles now pay the same bonus a crit does,
      which is new.
- [ ] Cast a summon, a conjure, or a raise and confirm it trains presence
      rather than resolve. These were mislabelled in data for their whole
      life.
- [ ] Sit at low health and let yourself regenerate for a long stretch on a
      VETERAN character. Passive growth should now tail off noticeably. On a
      NEW character it must feel exactly as it always did; if early growth
      got slower, the damping is wrong and that is a bug, not tuning.

## U6b: finish the flip (2026-08-19)

Every attack now runs the same single contest, every attack can crit, and a
fully turned-aside attack earns an answer. Judge these on whether the new
moments read clearly, not on rates.

- [ ] Bash and shot crits land and are narrated: special moves and aimed
      shots could never crit before U6b. Bash something repeatedly and put
      arrows into something tough until a critical lands on each. The crit
      should be unmistakable in the text and visibly harder-hitting; if a
      whole session produces none on either, or one lands without crit
      wording, that is a finding.
- [ ] A counter fires after a decisive quell: in the queen fight (or against
      any caster), watch for a cast you quell so completely that you answer
      it in the same breath. The counter should read as YOUR move, arrive
      immediately after the quell line, and never chain into a second
      counter. Defy works the same way against taunts, answering with a barb
      instead of a blow.
- [ ] Boss-kiting from the adjacent room, as a FEEL item: the cross-room
      shot is deliberately the one attack that cannot be countered, which
      makes shooting a boss from next door strictly safer than standing in
      the arena. Kite something dangerous from the adjacent room for a few
      rounds and report how it feels: legitimate tactics, or an exploit
      that needs a design answer. No verdict is baked in yet; your read
      here decides whether it goes to the backlog.

## U10: disruption model (2026-08-21)

Concentration, knockdown, and prone recovery all moved from flat chances (or,
for knockdown, thresholds that quietly delivered the wrong rates) onto opposed
contests. Judge whether skill now visibly matters in each of these moments.

- [ ] Hold a cast under fire at both low and high spellcasting skill. A chip
      hit should never even threaten the cast (no roll, no message). A big
      hit should visibly risk it, and a high-skill caster should hold through
      damage that breaks a novice's concentration in the same fight.
- [ ] Cast while prone, and separately while grappled. Both should be
      noticeably harder to hold together than casting on your feet, with
      grapple the harsher of the two. This is intended to make grapple a
      real anti-caster tool, not an oversight.
- [ ] Get tripped and kicked across a skill gap (a novice vs. a trained
      defender, or vice versa). Trip should now be resistible sometimes
      instead of landing almost every time, and kick's knockdown should
      actually happen sometimes instead of almost never. Report the felt
      rate, not just whether it changed.
- [ ] Get knocked down with an aggressor still on you, and separately with
      no one attacking you. Pinned recovery should read as a real contest
      (sometimes you get up free, sometimes you don't) while an unopposed
      knockdown should still let you scramble up automatically. Compare
      both against paying stamina for the guaranteed manual `stand`.
- [ ] Let a fanged beast throttle you while you are mid-cast. The choke
      should now test your training against its grip rather than rolling
      a flat coin: a practiced caster should sometimes hold the cast
      together through a throttle that would break a novice.

## U10b-0: progression rank from training (2026-08-24, arc complete)

Rank is now trained points for a stat and the skill level for a skill, never a
use counter. Mob and companion stat pools moved from `Training` into `Base`
behind a save migration. Every multiplier was re-solved on measured play data.

**The companion round trip is the one item that was never driven live.** Its
arithmetic is verified against all four of your real legacy pets, but the
login path itself was not exercised inside the playtest wall clock, and it is
the branch most likely to hide a `Base`-assignment bug. Do this one first.

- [ ] **Log in with Meirok and look hard at Rocky and Fleshy before doing
      anything else.** Note what `companion` and `status` report about each:
      how strong they look, their health, what they can do. Then log out, log
      back in, and compare. **They must come back identical.** Any drift in
      power or health is a serious finding and a deploy blocker. Expected
      result: unchanged stat values, unchanged HP, and a training figure that
      no longer includes what they were born with.
- [ ] **Rocky and Fleshy will never gain another stat point, and that is the
      accepted design.** Migrated legacy pets carry more trained points than
      the per-stat gain cap allows, so they arrive already at it. They keep
      everything they earned and simply stop growing. Confirm they feel no
      *weaker* than you remember. Weaker is a bug; merely static is not.
      A pet raised from here on will grow normally.
- [ ] Summon or raise a **fresh** companion and fight with it a while. It
      should visibly improve, unlike the two old golems. That contrast is the
      check that the cap is landing on legacy accumulation rather than on
      everything.
- [ ] Fight until stats and skills advance. Early progress should feel quick.
      Confirm the banners name the right thing and never show a raw number.
- [ ] Take real hits and keep fighting. Toughness is now built by being hit;
      a glancing blow should not count. Say whether it ever felt like it
      happened.
- [ ] `assess` a corpse, then actually raise it with the spell
      (`cast raise-skeleton corpse`). The forms assess advertises must be the
      forms the spells accept.
- [ ] Spend time on forage, search and a craft. Judge them against combat for
      the same effort. The retune was solved on measured rates, so anything
      that feels dramatically out of line is worth reporting.
- [ ] Read `help progression` as a player would and say whether it actually
      answers "how do I get better".

## U10d: surprise attack redesign (2026-08-25)

Surprise attack is now **one contested opening blow**, not a free multi-weapon
volley. Stealth breaks the moment you strike. A same-room bow shot from cover
gets its own smaller version and burns the shared special-move recovery. Bows
were detuned hard and given a large bonus for shooting something that is not
already fighting you.

The adversarial playtest passed every mechanical check, but **four things could
not be reached** because the fixture mob died in a single round. Every item
below needs **a target that survives at least one full round against you**, so
do these against something real (a North Road bandit, an Ironwind Steppes mob,
or the Queen's escort), never against a training dummy.

- [ ] **Get an opening strike DEFENDED.** Ambush something skilled enough to
      turn it. The line should name which defence stopped you (dodge, parry,
      block, quell or defy) and you should still do reduced damage rather
      than nothing. Neither half of that has ever been seen in play.
- [ ] **Try to ambush while the recovery is still spent** (ambush, then
      immediately try again, in melee and with a bow). You should get a clear
      refusal sentence in both cases. That copy has never rendered on screen.
- [ ] **Does an ambush actually feel harder-hitting?** 🔴 This is the known
      open finding. Against a one-round target, an ordinary shot and an ambush
      shot print the **same damage words**, so the deliberate ordering of the
      three new multipliers is invisible. Against a target that survives, the
      wording should separate. Tell me whether it does. If it still does not,
      the fix is the damage vocabulary, not the multipliers.
- [ ] **Shoot something that is not fighting anyone, then shoot something
      already swinging at you.** The first should feel clearly better. This is
      the whole compensation for the bow detune, so if it is not felt, archery
      is now simply worse than it was.
- [ ] **Fire a bow you already owned before this arc.** Your saved bows are
      rescaled on login (backpack, component bag, bandolier, pet inventory,
      worn gear and the account bank). Confirm an old bow still feels like a
      bow and not like a club. Bows sitting in shops, containers or on mob
      instances were deliberately left alone and will age out, so a
      briefly overpowered shop bow is expected, not a bug.
- [ ] A successful melee ambush sends **no** reveal sentence, by design. The
      marker on the blow and your hidden status quietly dropping are the whole
      feedback. Only the refusal path speaks. Say whether that reads as
      intentional or as a missing message.
