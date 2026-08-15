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
