# Pre-deploy playtest crib sheet (unified resolution arc)

The owner's manual checklist for the end-of-arc playtest with Meirok on the
local server. Covers everything on master through U6 plus the caster-brain
fix (2026-08-15). U7-U12 items get appended as they land; do not deploy
until every section has been played and the two MANDATORY items pass.

Meirok baseline for reference: melee score ~455 (Dex 110 + weapon-combat 69
x SkillWeight 5), block 523, spellcasting 51, crit mult 5.45. Against trash
he one-shots everything, so most sections need the named instruments below,
not street fights.

## 1. MANDATORY: the Elemental Queen (planar oasis instance, ~300g)

She casts for the first time ever (dead archetype until 2026-08-15). Buy in
at ~300g and fight her seriously.

- [ ] Difficulty: does the fight feel fair for the gold paid? She should
      open with a chrysalis-cocoon shield, throw conviction-spike at you,
      conviction-barrage when you bring company, and flee below 30% health.
- [ ] QUELL, finally observed live: her conviction spells are mental, so
      your quell (Willpower + Spellcasting) answers them. Watch for: a
      resisted cast being narrated, your CONVICTION pool dropping when you
      defend (win or lose), a barely-resisted spell still stinging with
      damage words, and a decisively-resisted one doing nothing.
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
