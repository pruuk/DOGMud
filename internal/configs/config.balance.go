package configs

// Balance holds every numeric gameplay-balance constant that was previously
// hardcoded in Go source.  All fields have defaults equal to the prior
// hardcoded values, so behaviour is unchanged unless a field is overridden
// in config.yaml or config-overrides.yaml.
type Balance struct {
	// ── ROLL SPREAD ──────────────────────────────────────────────────────────
	// Master randomness knob. Controls stdDev = stat * RollSpread for every
	// stat-based roll. Default 0.15 (15%). Valid range 0.05–0.50.
	RollSpread ConfigFloat `yaml:"RollSpread"`

	// ── COMBAT: ATTACK COSTS ─────────────────────────────────────────────────
	// U7 Task 7: an attack is charged PER SWING, through the same costs.Calc
	// formula the defences use -- base x encumbrance x inverse skill x modifier.
	// Before this, the attacker paid ONCE PER ROUND no matter how many weapons
	// and swings resolved, while the defender paid on every incoming swing. A
	// twelve-swing build therefore attacked twelve times for the price of one,
	// which is what made offence effectively free next to defence. Both sides of
	// an exchange are now priced on the same basis.
	//
	// The base is deliberately the same 1.0 as DefenceBaseStaminaCost, and the
	// modifier a neutral 1.0: attacking is the reference action the three defence
	// modifiers (1.25 / 1.15 / 1.10) are tuned against, so it starts at parity
	// and moves only when play says it should.
	//
	// This replaced UnarmedAttackStaminaCost and the per-weapon staminacost item
	// field. Weapon weight already prices a heavy weapon through the encumbrance
	// multiplier, so reading a per-weapon cost here as well would charge for the
	// same heaviness twice.
	AttackBaseStaminaCost ConfigFloat `yaml:"AttackBaseStaminaCost"` // Base stamina cost for ONE swing, before multipliers (default 1.0)
	AttackCostModifier    ConfigFloat `yaml:"AttackCostModifier"`    // Per-action cost modifier for attacking (default 1.0 — the neutral reference the defence modifiers are tuned against)

	// ── COMBAT: DEFENSE COSTS ────────────────────────────────────────────────
	// U7 Task 6 replaced the six per-defence knobs (DodgeBaseStaminaCost,
	// ParryBaseStaminaCost, BlockBaseStaminaCost and the three cost multipliers)
	// with ONE shared base and three per-action modifiers. The old pair-per-
	// defence shape let base and multiplier drift against each other, and the
	// integer truncation at the end of the old formula ate the difference anyway:
	// 2x0.9, 4x0.9 and 5x0.9 landed on 1, 3 and 4 with no way to express
	// anything between them. The three physical defences are now priced by
	// costs.Calc -- base x encumbrance x inverse skill x modifier -- and charged
	// through the fractional carry, so a fourteen percent gap between two
	// modifiers survives instead of rounding to nothing.
	//
	// Those six keys may still be present in config.yaml. yaml.Unmarshal is
	// non-strict, so an orphaned key is ignored rather than fatal.
	DefenceBaseStaminaCost ConfigFloat `yaml:"DefenceBaseStaminaCost"` // Shared base stamina cost for dodge/parry/block, before multipliers (default 1.0)
	DodgeCostModifier      ConfigFloat `yaml:"DodgeCostModifier"`      // Per-action cost modifier for dodge (default 1.25 — dearest; the whole body moves)
	ParryCostModifier      ConfigFloat `yaml:"ParryCostModifier"`      // Per-action cost modifier for parry (default 1.10 — cheapest; a weapon interposes)
	BlockCostModifier      ConfigFloat `yaml:"BlockCostModifier"`      // Per-action cost modifier for block (default 1.15)

	// U6 Task 12: quell and defy are paid in CONVICTION, not stamina, so they
	// get their own base costs rather than a fourth and fifth stamina knob. The
	// pairing lives in characters.DefensePool / Character.GetDefenseCostFloat,
	// which read the pool and the amount off the same defence name.
	//
	// U7 Task 6 left these FLAT while routing the physical three through
	// costs.Calc. The principled price for a mounted defence is a fraction of the
	// incoming action's cost, which needs the attacker's cost threaded through the
	// defence path; neither defence has been seen in live play yet, so that is
	// deferred rather than guessed at. A base cost alone is already tunable.
	QuellBaseConvictionCost ConfigInt `yaml:"QuellBaseConvictionCost"` // Base conviction cost for quell (default 2)
	DefyBaseConvictionCost  ConfigInt `yaml:"DefyBaseConvictionCost"`  // Base conviction cost for defy (default 2)

	// ── COMBAT: DEFENSE EFFECTIVENESS ────────────────────────────────────────
	DodgeEffectiveness ConfigFloat `yaml:"DodgeEffectiveness"` // Multiplier on dodge score before opposed roll (default 1.0)
	ParryEffectiveness ConfigFloat `yaml:"ParryEffectiveness"` // Multiplier on parry score before opposed roll (default 1.0)
	BlockEffectiveness ConfigFloat `yaml:"BlockEffectiveness"` // Multiplier on block score before opposed roll (default 1.0)
	QuellEffectiveness ConfigFloat `yaml:"QuellEffectiveness"` // Multiplier on quell score before opposed roll (default 1.0)
	DefyEffectiveness  ConfigFloat `yaml:"DefyEffectiveness"`  // Multiplier on defy score before opposed roll (default 1.0)
	// U6 deleted MinDefenseChance and MinAttackHitChance from here. They were the
	// melee-only floor pair, applied in resolveDefenseOutcomeCore AFTER crit
	// resolution had already returned on five branches, so the attack floor was
	// only ever evaluated on the swings a defence crit did not consume. Against a
	// defender who crits nearly every swing that made the floor dead code in
	// exactly the matchup it was written for. ContestFloor below replaces both.

	// ContestFloor is the single symmetric last-resort probability for EVERY
	// opposed contest in the game. A symmetric floor F yields the bound
	// [F, 1-F], so 0.125 means: hopelessly overmatched you still succeed one
	// attempt in eight, hopelessly overmatching you are still stopped one in
	// eight.
	//
	// It replaces eight per-channel knobs whose values encoded the cost of a
	// single failure. That distinction is deliberately discarded in favour of
	// one rule (U6).
	//
	// Governs OPPOSED contests only. Static-difficulty rolls (search, track,
	// forage) are roadmap category B/C and are not floored. Concentration
	// IS floored, by its own ConcentrationFloor below (U10).
	ContestFloor ConfigFloat `yaml:"ContestFloor"`

	// ConcentrationFloor is the symmetric last-resort flip probability for
	// concentration contests ONLY (all three triggers: damage, position,
	// throttle). The standard ContestFloor (0.125) would break a master's
	// concentration one disruption in eight; concentration gets its own,
	// much smaller mercy band. Read in exactly one place:
	// combat.RunConcentrationContest.
	ConcentrationFloor ConfigFloat `yaml:"ConcentrationFloor"` // default 0.02

	// ConcentrationDamageThresholdPct: damage below this percent of the
	// caster's health pool does not roll for concentration at all. Chip
	// damage should not generate rolls. Values below 1 are rewritten to
	// the default; "roll on any hit" is expressed as 1.
	ConcentrationDamageThresholdPct ConfigInt `yaml:"ConcentrationDamageThresholdPct"` // default 10

	// Crit floors (chunk 5.11e). Denominated in HITS, not swings, and applied
	// only after the hit outcome is final. Set either to 0 to disable it.
	MinAttackCritChance  ConfigFloat `yaml:"MinAttackCritChance"`  // Floor probability a landed hit is a crit (default 0.01)
	MinDefenseCritChance ConfigFloat `yaml:"MinDefenseCritChance"` // Floor probability a successful defense is a defensive crit (default 0.01)

	// Crit bar (U6b). combat.CritBarFor computes the attacker-side crit
	// threshold for EVERY channel from the channel's skill pair:
	//
	//	bar = clamp(2.0 - CritBarSkillSlope*(atkRank-defRank),
	//	            CritBarFloor, CritBarCeiling)
	//
	// The slope and floor were balance literals inside internal/combat before
	// U6b (0.05 and 1.5, melee-only). The ceiling is NEW: uncapped, a
	// gold-scaled skill-1 boss faced bar 5.4 vs a veteran and effectively
	// never crit. CritBarCeiling 0 is LEGAL and means UNCAPPED -- it is the
	// documented off-switch, restoring the pre-U6b unbounded bar.
	CritBarSkillSlope ConfigFloat `yaml:"CritBarSkillSlope"` // Bar shift per point of attacker skill advantage (default 0.05)
	CritBarFloor      ConfigFloat `yaml:"CritBarFloor"`      // Lowest the bar may fall (default 1.5)
	CritBarCeiling    ConfigFloat `yaml:"CritBarCeiling"`    // Highest the bar may rise (default 3.0); 0 = uncapped, and is legal

	// CounterDamagePercent: a defensive crit earns the defender one free
	// answering swing at this fraction of normal weapon damage (U6b counter
	// tier; the same knob melee riposte used at its old hardcoded 0.5).
	// 0 is LEGAL and disables counter damage entirely.
	CounterDamagePercent ConfigFloat `yaml:"CounterDamagePercent"` // Counter-swing damage fraction (default 0.5); 0 disables

	// ── COMBAT: PRONE & GRAPPLE ──────────────────────────────────────────────
	ProneAttackMultiplier        ConfigFloat `yaml:"ProneAttackMultiplier"`        // Multiplier on attack score while prone (default 0.80)
	ProneDodgePenalty            ConfigFloat `yaml:"ProneDodgePenalty"`            // Multiplier on dodge score while prone (default 0.70)
	ProneParryPenalty            ConfigFloat `yaml:"ProneParryPenalty"`            // Multiplier on parry score while prone (default 0.80)
	ProneBlockPenalty            ConfigFloat `yaml:"ProneBlockPenalty"`            // Multiplier on block score while prone (default 0.90)
	ProneDamagePenalty           ConfigFloat `yaml:"ProneDamagePenalty"`           // Damage multiplier while prone (default 0.80)
	ProneVulnerabilityMultiplier ConfigFloat `yaml:"ProneVulnerabilityMultiplier"` // Multiplier on attack score vs prone target (default 1.15)

	// Chunk 5.11c: grapple position moved off the crit threshold and onto the
	// attack score, mirroring how prone already worked. As threshold shifts the
	// ground-grapple pair self-cancelled to net zero, because BOTH participants
	// satisfy IsGroundGrapple() -- it is a position state, while IsController()
	// is a separate control fsm. As multipliers they compound, which was the
	// intent.
	GrappleGroundControlAttackMultiplier   ConfigFloat `yaml:"GrappleGroundControlAttackMultiplier"`   // Attack score multiplier for a ground-grapple controller (default 1.15)
	GrappleStandingControlAttackMultiplier ConfigFloat `yaml:"GrappleStandingControlAttackMultiplier"` // Attack score multiplier for a standing-grapple controller (default 1.08)
	GrappleGroundedVulnerabilityMultiplier ConfigFloat `yaml:"GrappleGroundedVulnerabilityMultiplier"` // Attack score multiplier vs a grounded, non-controlling target (default 1.15)
	// U6b Task 13: the AttemptGrapple prone literals, knobbed at identical
	// shipped values. NOTE the direction — an earlier draft had them swapped:
	// the DEFENDER-prone site multiplies the defense score by 0.3, the
	// ATTACKER-prone site multiplies the attack score by 0.5.
	GrappleProneAttackerMod ConfigFloat `yaml:"GrappleProneAttackerMod"` // Attack score multiplier when the grapple attacker is prone/supine (default 0.5)
	GrappleProneDefenderMod ConfigFloat `yaml:"GrappleProneDefenderMod"` // Defense score multiplier when the grapple defender is prone/supine (default 0.3)

	// U6b Task 14 (gate decision §5.4): the grapple-drift aggressor edge,
	// restored as a deliberate config knob after the accidental 2.2-vs-2.0
	// skill-coefficient edge was deleted in the SkillWeight reweight. The
	// value 1.038 was SOLVED, not guessed: it restores parity E[drift]
	// ≈ +0.196 steps/round under the √2-fixed + SkillWeight-reweighted
	// maths (tools/balance/u6b_model_counters_family_costs.py, drift
	// aggressor-bonus solve). Because the bonus is a multiplier on the
	// whole score, the restored parity drift is scale-free — the same
	// +0.196 at every parity tier.
	GrappleAggressorDriftBonus ConfigFloat `yaml:"GrappleAggressorDriftBonus"` // Multiplier on the grapple aggressor's whole drift score (default 1.038)

	StandStaminaCost         ConfigFloat `yaml:"StandStaminaCost"`         // Fraction of max stamina to stand up (default 0.15)
	StandMinStamina          ConfigFloat `yaml:"StandMinStamina"`          // Minimum fraction of max SP to stand (default 0.15)
	ThirdPartyGrapplePenalty ConfigFloat `yaml:"ThirdPartyGrapplePenalty"` // Defense multiplier when grappled vs third party (default 0.70)
	ClinchDodgePenalty       ConfigFloat `yaml:"ClinchDodgePenalty"`       // Dodge score multiplier while clinched (default 0.80)
	ClinchParryPenalty       ConfigFloat `yaml:"ClinchParryPenalty"`       // Parry score multiplier while clinched (default 0.83)
	ClinchBlockPenalty       ConfigFloat `yaml:"ClinchBlockPenalty"`       // Block score multiplier while clinched (default 0.85)
	GroundedDodgePenalty     ConfigFloat `yaml:"GroundedDodgePenalty"`     // Dodge score multiplier while grounded (default 0.75)
	GroundedParryPenalty     ConfigFloat `yaml:"GroundedParryPenalty"`     // Parry score multiplier while grounded (default 0.77)
	GroundedBlockPenalty     ConfigFloat `yaml:"GroundedBlockPenalty"`     // Block score multiplier while grounded (default 0.80)
	// GrappleStaminaLowThreshold is the stamina fraction (0.0–1.0) below
	// which a character is considered "low stamina" for grapple purposes.
	// Used by IsLowGrappleStamina() and the mob_low_grapple_stamina btree
	// primitive (T5) and Position_Messaging (T7).
	GrappleStaminaLowThreshold ConfigFloat `yaml:"GrappleStaminaLowThreshold"` // Stamina fraction floor for grapple stamina warning (default 0.25)

	// ── REACH UTILITY CURVE (chunk 4c) ───────────────────────────────────────
	// See internal/combat/reach.go for the formula and the design spec for
	// reasoning.

	// ReachStandingGrappleRadius is the effective radius (meters) at
	// which a weapon stops fitting in Clinch / BackStanding positions.
	// Default 0.5 — about chest-to-chest distance in a clinch.
	ReachStandingGrappleRadius ConfigFloat `yaml:"reach_standing_grapple_radius"`

	// ReachGroundGrappleRadius is the effective radius (meters) at
	// which a weapon stops fitting in any ground grapple (Mount,
	// SideControl, KneeOnBelly, NorthSouth, Crucifix, BackGround,
	// HalfGuard, Guard). Default 0.3 — body-on-body distance.
	ReachGroundGrappleRadius ConfigFloat `yaml:"reach_ground_grapple_radius"`

	// ReachUtilityFloor caps the minimum damage multiplier from the
	// reach curve. Without a floor, a pike in mount would multiply
	// damage by ~0.1 — a floor at 0.15 ensures even the longest
	// weapon can poke for chip damage (pommel jab, hilt-strike).
	// Tunable; smoke may push this lower.
	ReachUtilityFloor ConfigFloat `yaml:"reach_utility_floor"`

	// ── CHUNK 4D: SUBMISSION TICK ────────────────────────────────────────────
	// Per-round opportunistic submission attempts gated on the chunk-4b
	// control-axis drift roll. See spec
	// docs/superpowers/specs/2026-05-18-state-chunk-4d-submission-rework-design.md
	SubmissionAttemptAlpha ConfigFloat `yaml:"submission_attempt_alpha"`  // Min drift-margin (std devs) that opens a sub window (either side)
	SubmissionAttemptCritZ ConfigFloat `yaml:"submission_attempt_crit_z"` // Defender-side shortcut: drift z >= this opens a bottom-sub window regardless of margin
	SubBadZThreshold       ConfigFloat `yaml:"sub_bad_z_threshold"`       // Z-score below which the sub roll's bad-tier (attempter falls prone) fires
	SubGoldLossFraction    ConfigFloat `yaml:"sub_gold_loss_fraction"`    // Fraction of carried gold transferred to the aggressor on subdue/cripple
	BrokenLimbBuffDuration ConfigInt   `yaml:"broken_limb_buff_duration"` // Duration in rounds for the broken-limb buff; expires naturally via standard buff tick

	// ── CHUNK 4E: THIRD-PARTY INTERFERENCE ──────────────────────────────────
	// See docs/superpowers/specs/ chunk-4e design.

	// ControlDegradeOnOutsideHit enables chunk 4e §5: third-party damage on
	// a grapple controller shifts their ControlLevel one step toward Neutral
	// per disrupted round. Set false to disable the mechanic for tuning.
	ControlDegradeOnOutsideHit ConfigBool `yaml:"ControlDegradeOnOutsideHit"`

	// SubInterruptDamageThresholdPct is the fraction of HealthMax that
	// constitutes "above-threshold" third-party damage for chunk 4e §7
	// sub interrupt. Below this, damage doesn't break a sub setup; at or
	// above, the sub outcome is forced to Bad tier. A crit also triggers
	// the override regardless of threshold. Default 0.10 (10% of max HP).
	// Set 0 to disable threshold-path (crit-only).
	SubInterruptDamageThresholdPct ConfigFloat `yaml:"SubInterruptDamageThresholdPct"`

	// ── GRAPPLE CONTROL AXIS (chunk 4b) ──────────────────────────────────────
	// Per-round drift mechanics — see
	// docs/superpowers/specs/2026-05-16-state-chunk-4b-position-control-axis-design.md
	GrappleStaminaPenaltyMax        ConfigFloat `yaml:"GrappleStaminaPenaltyMax"`        // Max roll-mult reduction at 0% stamina (default 0.60)
	GrappleStaminaPenaltyCurve      ConfigFloat `yaml:"GrappleStaminaPenaltyCurve"`      // Exponent shape of stamina penalty curve (default 1.5)
	GrappleEncumbrancePenaltyMax    ConfigFloat `yaml:"GrappleEncumbrancePenaltyMax"`    // Max roll-mult reduction at max encumbrance (default 0.80)
	GrappleEncumbrancePenaltyCurve  ConfigFloat `yaml:"GrappleEncumbrancePenaltyCurve"`  // Exponent shape of encumbrance penalty curve (default 1.5)
	GrappleStaminaCostPerRound      ConfigFloat `yaml:"GrappleStaminaCostPerRound"`      // Ongoing grapple-maintenance base before role and shared cost multipliers (default 2)
	GrappleControllerCostMultiplier ConfigFloat `yaml:"GrappleControllerCostMultiplier"` // Controller's per-round cost multiplier (default 1.0)
	GrappleControlledCostMultiplier ConfigFloat `yaml:"GrappleControlledCostMultiplier"` // Controlled's per-round cost multiplier (default 2.0)
	PositionConsistencyCheckRounds  ConfigInt   `yaml:"PositionConsistencyCheckRounds"`  // How often the periodic invariant checker runs (default 10)

	// ── COMBAT: SPECIAL MOVES ────────────────────────────────────────────────
	SpecialMoveBaseStaminaCost       ConfigFloat `yaml:"SpecialMoveBaseStaminaCost"`       // Base stamina cost for special moves, including grapple initiation, before shared multipliers (default 4)
	SpecialMoveCooldown              ConfigInt   `yaml:"SpecialMoveCooldown"`              // Shared cooldown rounds for bash/trip/kick (default 5)
	FlightOpposedEdge                ConfigInt   `yaml:"FlightOpposedEdge"`                // Winged Flight: melee opposed-roll edge a flyer gets over the earthbound (default 25)
	FlightMoveStaminaMult            ConfigFloat `yaml:"FlightMoveStaminaMult"`            // Winged Flight: move-stamina cost multiplier while flying (default 0.5)
	FlightFleeStaminaMult            ConfigFloat `yaml:"FlightFleeStaminaMult"`            // Winged Flight: flee-stamina cost multiplier while flying (default 0.5)
	FleeStaminaCost                  ConfigInt   `yaml:"FleeStaminaCost"`                  // Base stamina charged for breaking off to flee (default 10). Charged PARTIALLY: an exhausted character still gets to flee. U7 may fold this into NonHarmContestBaseCost.
	TauntHoldRounds                  ConfigInt   `yaml:"TauntHoldRounds"`                  // Rounds a successful taunt pins the target's aggro onto the taunter (default 4)
	RhetoricActionBaseConvictionCost ConfigFloat `yaml:"RhetoricActionBaseConvictionCost"` // Base conviction cost for taunt/rally/warcry before shared multipliers (default 4)
	BashDamagePercent                ConfigFloat `yaml:"BashDamagePercent"`                // Fraction of normal melee damage (default 0.50)
	BashKnockdownFactor              ConfigFloat `yaml:"BashKnockdownFactor"`              // Knockdown score factor; intended-rate anchor 50% at parity (default 1.0)
	TripDamagePercent                ConfigFloat `yaml:"TripDamagePercent"`                // Fraction of normal melee damage (default 0.25)
	TripKnockdownFactor              ConfigFloat `yaml:"TripKnockdownFactor"`              // Knockdown score factor; intended-rate anchor 60% at parity (default 1.057)
	KickDamagePercent                ConfigFloat `yaml:"KickDamagePercent"`                // Fraction of normal melee damage (default 0.80)
	KickKnockdownFactor              ConfigFloat `yaml:"KickKnockdownFactor"`              // Knockdown score factor; intended-rate anchor 35% at parity (default 0.924)
	StompDamagePercent               ConfigFloat `yaml:"StompDamagePercent"`               // Stomp damage when target is prone (default 1.20)
	KneeDamagePercent                ConfigFloat `yaml:"KneeDamagePercent"`                // Knee damage in grapple (default 1.00)
	CoupDeGraceRounds                ConfigInt   `yaml:"CoupDeGraceRounds"`                // Rounds before mob finishes downed player (default 1; 0=disabled)
	DrainHealRatio                   ConfigFloat `yaml:"DrainHealRatio"`                   // Fraction of drain damage the attacker heals (lifesteal), default 0.75

	// ── COMBAT: RANGED ───────────────────────────────────────────────────────
	ShootBaseStaminaCost  ConfigFloat `yaml:"ShootBaseStaminaCost"`  // Base stamina cost to shoot before shared multipliers (default 2)
	ReloadBaseStaminaCost ConfigFloat `yaml:"ReloadBaseStaminaCost"` // Base stamina cost to reload before shared multipliers (default 1)
	RangedShotScale       ConfigFloat `yaml:"RangedShotScale"`       // Global multiplier on all ranged shot damage (default 1.0)

	// ── SKULLDUGGERY ─────────────────────────────────────────────────────────
	SneakBaseStaminaCost           ConfigFloat `yaml:"SneakBaseStaminaCost"`           // Base stamina cost to sneak before shared multipliers (default 2.5)
	SneakFailCooldown              ConfigInt   `yaml:"SneakFailCooldown"`              // Absent/zero means no failure cooldown; an invalid negative falls back to 3
	SurpriseAttackOffhandPenalty   ConfigFloat `yaml:"SurpriseAttackOffhandPenalty"`   // Hit penalty for offhand surprise attack (default 0.10)
	SurpriseAttackExtraArm1Penalty ConfigFloat `yaml:"SurpriseAttackExtraArm1Penalty"` // Hit penalty for extra arm 1 (default 0.25)
	SurpriseAttackExtraArm2Penalty ConfigFloat `yaml:"SurpriseAttackExtraArm2Penalty"` // Hit penalty for extra arm 2 (default 0.40)
	SurpriseAttackExtraArm3Penalty ConfigFloat `yaml:"SurpriseAttackExtraArm3Penalty"` // Hit penalty for extra arm 3 (default 0.55)
	SurpriseAttackExtraArm4Penalty ConfigFloat `yaml:"SurpriseAttackExtraArm4Penalty"` // Hit penalty for extra arm 4 (default 0.70)
	StealHiddenBonus               ConfigInt   `yaml:"StealHiddenBonus"`               // Bonus to attacker score when hidden (default 25)
	StealCooldown                  ConfigInt   `yaml:"StealCooldown"`                  // Steal/plant cooldown in real seconds (default 60)
	ShadowCooldown                 ConfigInt   `yaml:"ShadowCooldown"`                 // Rounds before re-shadowing (default 5)
	HiddenMoveStaminaMultiplier    ConfigFloat `yaml:"HiddenMoveStaminaMultiplier"`    // Extra stamina cost multiplier for moving while hidden (default 3.0)
	SneakModEmitsLightDarkRoom     ConfigFloat `yaml:"SneakModEmitsLightDarkRoom"`     // Sneak score multiplier: sneaker emits light, room dark (default 0.5)
	SneakModEmitsLightLitRoom      ConfigFloat `yaml:"SneakModEmitsLightLitRoom"`      // Sneak score multiplier: sneaker emits light, room lit (default 0.85)
	SneakModNoLightLitRoom         ConfigFloat `yaml:"SneakModNoLightLitRoom"`         // Sneak score multiplier: sneaker dark, room lit (default 0.9)

	// ── COMBAT: SPELL COSTS ──────────────────────────────────────────────────
	SpellConvictionCostMultiplier ConfigFloat `yaml:"SpellConvictionCostMultiplier"` // Global multiplier for spell conviction costs (default 1.0)
	SpellHealthCostMultiplier     ConfigFloat `yaml:"SpellHealthCostMultiplier"`     // Global multiplier for spell health costs (default 1.0)

	// ── COMBAT: DARKNESS ─────────────────────────────────────────────────────
	DarknessCombatPenalty ConfigFloat `yaml:"DarknessCombatPenalty"` // Multiplier on attack AND defense scores when fighting blind (default 0.80)

	// ── COMBAT: MESSAGES ─────────────────────────────────────────────────────
	ConsistentAttackMessages ConfigBool `yaml:"ConsistentAttackMessages"` // Whether each weapon has consistent attack messages

	// ── COMBAT: DAMAGE ───────────────────────────────────────────────────────
	// Legacy unarmed knobs — still used by GetDefaultDistributionDamage() for
	// attack count and crit buff calculation. Damage values are overridden by
	// the unified pipeline (UnarmedDamageMultiplier + CalcRawDamage).
	UnarmedBaseDamage       ConfigFloat `yaml:"UnarmedBaseDamage"`       // Base damage before stat bonuses (default 2.0)
	UnarmedStrengthDivisor  ConfigFloat `yaml:"UnarmedStrengthDivisor"`  // Str / this = damage bonus (default 25.0)
	UnarmedSkillDivisor     ConfigFloat `yaml:"UnarmedSkillDivisor"`     // Skill / this = damage bonus (default 10.0)
	UnarmedBaseVariance     ConfigFloat `yaml:"UnarmedBaseVariance"`     // Base randomness of unarmed hits (default 3.0)
	UnarmedDamageMultiplier ConfigFloat `yaml:"UnarmedDamageMultiplier"` // Fist damage multiplier for new pipeline (default 0.30)
	UnarmedSpeedMultiplier  ConfigFloat `yaml:"UnarmedSpeedMultiplier"`  // Unarmed attack speed — slightly faster than light weapons (default 1.4)
	HasteSwingMultiplier    ConfigFloat `yaml:"HasteSwingMultiplier"`    // Swing count multiplier when haste buff is active (default 1.50)
	SkillMultiplierBase     ConfigFloat `yaml:"SkillMultiplierBase"`     // Skill multiplier at rank 0 (default 1.0)
	SkillMultiplierMax      ConfigFloat `yaml:"SkillMultiplierMax"`      // Skill multiplier at soft cap (default 3.0)
	SkillWeight             ConfigFloat `yaml:"SkillWeight"`             // Global multiplier on skill contributions in additive formulas (default 2.0)
	CritDamageBase          ConfigFloat `yaml:"CritDamageBase"`          // Crit damage multiplier at skill rank 0, applied on top of the mitigation bypass (default 2.0)
	CritDamagePerSkill      ConfigFloat `yaml:"CritDamagePerSkill"`      // Added to the crit damage multiplier per rank of the attacking channel's skill (default 0.05)
	MeleeDamageScale        ConfigFloat `yaml:"MeleeDamageScale"`        // Physical damage scale. Stats ~100, so 0.30 yields ~30 raw per swing (default 0.30)
	SpellDamageScale        ConfigFloat `yaml:"SpellDamageScale"`        // Flat multiplier on spell damage output (default 1.0 = no change)
	RhetoricDamageScale     ConfigFloat `yaml:"RhetoricDamageScale"`     // Flat multiplier on conviction/taunt damage output (default 1.0 = no change)
	MobDamageMultiplier     ConfigFloat `yaml:"MobDamageMultiplier"`     // Extra multiplier applied to NPC melee damage only (default 1.0 = same as players)
	GlobalDamageMultiplier  ConfigFloat `yaml:"GlobalDamageMultiplier"`  // Master multiplier applied to ALL damage channels (default 1.0)
	PhysicalMitigationCap   ConfigFloat `yaml:"PhysicalMitigationCap"`   // Max physical mitigation % (default 0.75)
	MagicalMitigationCap    ConfigFloat `yaml:"MagicalMitigationCap"`    // Max magical mitigation % (default 0.75)
	ConvictionMitigationCap ConfigFloat `yaml:"ConvictionMitigationCap"` // Max conviction mitigation % (default 0.75)
	// U6 Task 12 deleted SpellAvoidanceDamageMultiplier and
	// RhetoricAvoidanceDamageMultiplier from here. They were the flat partial
	// multipliers returned by TrySpellDeflection and TryStoicResolve, the second
	// independent contest U6 removed. A defensive win on any channel is now
	// scaled by DefenceMitigation, whose 0.5 floor and crit-threshold ceiling are
	// STRUCTURAL rather than tunable -- see the comment on DefenceMitigation for
	// why moving either reintroduces the discontinuity it replaces.
	//
	// Both keys may still be present in config.yaml. yaml.Unmarshal is
	// non-strict, so an orphaned key is ignored rather than fatal; the lines can
	// be dropped from config.yaml whenever it is next edited.
	ResourcePenaltyCurve ConfigFloat `yaml:"ResourcePenaltyCurve"` // Exponent for resource depletion penalty curve (default 2.0)
	HealthPenaltyMax     ConfigFloat `yaml:"HealthPenaltyMax"`     // Max melee damage penalty at 0% HP (default 0.28)
	StaminaPenaltyMax    ConfigFloat `yaml:"StaminaPenaltyMax"`    // Max attack count + hit rate penalty at 0% SP (default 0.28)
	ConvictionPenaltyMax ConfigFloat `yaml:"ConvictionPenaltyMax"` // Max taunt/spell penalty at 0% CP (default 0.28)

	// ── REGEN RATES ──────────────────────────────────────────────────────────
	PlayerHealthRegenPct     ConfigFloat `yaml:"PlayerHealthRegenPct"`     // Fraction of HealthMax regen'd per tick — players (default 0.01)
	PlayerStaminaRegenPct    ConfigFloat `yaml:"PlayerStaminaRegenPct"`    // Fraction of StaminaMax regen'd per tick — players (default 0.01)
	PlayerConvictionRegenPct ConfigFloat `yaml:"PlayerConvictionRegenPct"` // Fraction of ConvictionMax regen'd per tick — players (default 0.01)
	MobHealthRegenPct        ConfigFloat `yaml:"MobHealthRegenPct"`        // Fraction of HealthMax regen'd per tick — NPCs (default 0.01)
	MobStaminaRegenPct       ConfigFloat `yaml:"MobStaminaRegenPct"`       // Fraction of StaminaMax regen'd per tick — NPCs (default 0.02)
	MobConvictionRegenPct    ConfigFloat `yaml:"MobConvictionRegenPct"`    // Fraction of ConvictionMax regen'd per tick — NPCs (default 0.02)

	// ── STAMINA & CONVICTION ──────────────────────────────────────────────────
	MovementBaseStaminaCost ConfigFloat `yaml:"MovementBaseStaminaCost"` // Flat cost to move on normal terrain, BEFORE encumbrance and skill (default 0.5)
	MovementMaxStaminaCost  ConfigFloat `yaml:"MovementMaxStaminaCost"`  // Ceiling for any single move action (default 20.0)
	// The U7 movement-banking change deleted MovementCostFloor. It was the
	// minimum stamina a single move could cost, and it existed only because
	// GetMovementStaminaCost returned an int: an unfloored sub-1.0 cost would
	// have rounded away to a free move. Movement now banks its fractional
	// remainder through ApplyCostFloatOrRefuse, so a sub-1.0 charge is not free,
	// it is a whole point every second or third room, and any floor at or above
	// 1 would flatten the encumbrance curve back into the single step with flat
	// shoulders that an in-game measurement found. The key may still be present
	// in config.yaml -- yaml.Unmarshal is non-strict, so an orphaned key is
	// ignored rather than fatal, and the line can be dropped whenever that file
	// is next edited.
	// U7 Task 10: movement is priced partly on the actor's search rank, so
	// travelling has to be able to EARN that discount -- but only barely.
	// This is the probability that a successful move RECORDS a search use;
	// it is deliberately not a multiplier on the odds of a use that is
	// counted every step (see movementTrainsSearch in usercommands/go.go).
	// Zero or negative switches travel-training off entirely.
	MovementSearchTrainChance ConfigFloat `yaml:"MovementSearchTrainChance"` // Chance a successful move records a search use (default 0.005, i.e. 1 in 200)
	// U7 Task 7 deleted UnarmedAttackStaminaCost. It was the fallback arm of a
	// per-round, per-weapon attack charge that no longer exists; attacking is
	// priced per swing by AttackBaseStaminaCost above, armed or not. The key may
	// still be present in config.yaml -- yaml.Unmarshal is non-strict, so an
	// orphaned key is ignored rather than fatal.

	// ── RESOURCE MAXIMUMS ─────────────────────────────────────────────────────
	HealthBase             ConfigInt `yaml:"HealthBase"`             // Flat HP before stat contribution (default 5)
	HealthPerStrength      ConfigInt `yaml:"HealthPerStrength"`      // Strength multiplier toward HealthMax (default 1)
	HealthPerVitality      ConfigInt `yaml:"HealthPerVitality"`      // Vitality multiplier toward HealthMax (default 3)
	StaminaBase            ConfigInt `yaml:"StaminaBase"`            // Flat stamina before stat contribution (default 5)
	StaminaPerStrength     ConfigInt `yaml:"StaminaPerStrength"`     // Strength multiplier toward StaminaMax (default 0)
	StaminaPerWillpower    ConfigInt `yaml:"StaminaPerWillpower"`    // Willpower multiplier toward StaminaMax (default 1)
	StaminaPerVitality     ConfigInt `yaml:"StaminaPerVitality"`     // Vitality multiplier toward StaminaMax (default 3)
	ConvictionBase         ConfigInt `yaml:"ConvictionBase"`         // Flat conviction before stat contribution (default 5)
	ConvictionPerCharisma  ConfigInt `yaml:"ConvictionPerCharisma"`  // Charisma multiplier toward ConvictionMax, primary (default 3)
	ConvictionPerWillpower ConfigInt `yaml:"ConvictionPerWillpower"` // Willpower multiplier toward ConvictionMax, secondary (default 1)

	// ── PROGRESSION ───────────────────────────────────────────────────────────
	SkillSoftCap             ConfigInt   `yaml:"SkillSoftCap"`             // Virtual ranks where progression slows sharply (default 50)
	StatProgressionSoftCap   ConfigInt   `yaml:"StatProgressionSoftCap"`   // Trained points at which stat progression slows sharply (default 50). NOT a cap on stat values, and no longer an anti-exploit floor: rank IS the trained points, so a high value cannot buy a cheap rank.
	UsesPerRank              ConfigInt   `yaml:"UsesPerRank"`              // Skill/stat uses that equal one virtual rank (default 25)
	BaseProgressionChance    ConfigFloat `yaml:"BaseProgressionChance"`    // Starting chance to progress at rank 0 (default 0.30)
	StatProgressionRate      ConfigFloat `yaml:"StatProgressionRate"`      // Global multiplier on STAT progression chance; skills unaffected (default 1.0)
	ProgressionDecayBelowCap ConfigFloat `yaml:"ProgressionDecayBelowCap"` // Exponential steepness below soft cap (default 3.0)
	ProgressionDecayAboveCap ConfigFloat `yaml:"ProgressionDecayAboveCap"` // Exponential steepness above soft cap (default 2.0)
	// ProgressionChanceFloor is the smallest chance a rank-driven progression
	// roll may present. Below it a stat or skill is not merely slow, it is
	// sealed: the roll quantises to zero and can never fire again. Two of a
	// live character's six stats were in that state before this knob existed.
	//
	// Applied ONLY to the two rank-driven sites (CheckStatProgression,
	// CheckSkillProgression). CheckRegenProgression is deliberately excluded:
	// its chance is proportional to pool depletion and is SUPPOSED to vanish as
	// the pool fills.
	ProgressionChanceFloor ConfigFloat `yaml:"ProgressionChanceFloor"` // Smallest chance a rank-driven progression roll may present (default 0.00001)

	CritProgressionBonus         ConfigFloat `yaml:"CritProgressionBonus"`         // Progression multiplier for the party who DID a crit or fumble (default 2.0; 0 disables)
	ObservedCritProgressionBonus ConfigFloat `yaml:"ObservedCritProgressionBonus"` // Progression multiplier for the party who RECEIVED one (default 0.5; 0 disables)

	MobProgressionEnabled ConfigBool  `yaml:"MobProgressionEnabled"` // Enable mob stat/skill progression (default true)
	MobProgressionRate    ConfigFloat `yaml:"MobProgressionRate"`    // Multiplier on progression chance vs players (default 0.5)
	MobStatCap            ConfigInt   `yaml:"MobStatCap"`            // Legacy VALUE cap on mob stats. Superseded by MobStatTrainingCap; see config.yaml.
	// MobStatTrainingCap caps how many points a mob may GAIN, rather than the
	// value it may reach. The old value cap was asymmetric: a mob with base 250
	// could gain nothing while one with base 180 could gain 20, purely because
	// of how big it was authored.
	MobStatTrainingCap ConfigInt `yaml:"MobStatTrainingCap"` // Max points a mob may gain per stat (default 50)
	MobSkillCap        ConfigInt `yaml:"MobSkillCap"`        // Legacy hard cap on mob skill level. Superseded by MobSkillTrainingCap.
	// Mobs get their own soft cap because they fight far more often than
	// players do, so the shared 50 would leave their curve too flat for too
	// long. The hard cap is what actually bounds them.
	MobSkillSoftCap               ConfigInt   `yaml:"MobSkillSoftCap"`               // Skill level at which mob progression slows sharply (default 20)
	MobSkillTrainingCap           ConfigInt   `yaml:"MobSkillTrainingCap"`           // Hard cap on mob skill level from progression (default 25)
	MobSaveIntervalRounds         ConfigInt   `yaml:"MobSaveIntervalRounds"`         // Rounds between periodic mob instance saves (default 100)
	MobInstanceMaxAgeDays         ConfigInt   `yaml:"MobInstanceMaxAgeDays"`         // Max age in days before stale instance files are pruned (default 7)
	DamageProgressionThresholdPct ConfigFloat `yaml:"DamageProgressionThresholdPct"` // Fraction of a pool's reachable max a single hit must remove to train that pool's stats (default 0.05)
	RegenProgressionBase          ConfigFloat `yaml:"RegenProgressionBase"`          // Max chance at 0% resource per stat per tick (default 0.005)
	RegenProgressionCurve         ConfigFloat `yaml:"RegenProgressionCurve"`         // Exponent shaping the depletion→chance curve (default 3.0)

	// ── COSTS: INVERSE-SKILL MULTIPLIER (U7) ─────────────────────────────────
	// costs.SkillCostMultiplier runs INVERSE to skill: a practised fighter
	// spends less stamina/conviction on the same action than an untrained
	// one. Two linear segments joined at CostSkillMidRank, clamped flat below
	// rank 0 and at/above CostSkillCapRank. NOT combat.SkillMultiplier (a
	// sqrt curve scaling damage UPWARD) — same-shaped signature, opposite
	// direction, different job; named SkillCostMultiplier specifically so an
	// unqualified in-package call inside combat can never resolve to it.
	//
	// Unlike some knobs elsewhere in this file (e.g. StaminaPerStrength: 0),
	// 0 is NOT a usable value for any of the five fields below — each is
	// replaced by its default at load if left at or below 0 (the two rank
	// fields if left below 1).
	CostSkillMultAtZero ConfigFloat `yaml:"CostSkillMultAtZero"` // Cost multiplier at rank 0 (default 1.10)
	CostSkillMultAtMid  ConfigFloat `yaml:"CostSkillMultAtMid"`  // Cost multiplier at CostSkillMidRank, neutral (default 1.00)
	CostSkillMultAtCap  ConfigFloat `yaml:"CostSkillMultAtCap"`  // Cost multiplier at/above CostSkillCapRank (default 0.40)
	CostSkillMidRank    ConfigInt   `yaml:"CostSkillMidRank"`    // Virtual rank where the multiplier is neutral (default 25)
	CostSkillCapRank    ConfigInt   `yaml:"CostSkillCapRank"`    // Virtual rank where the discount maxes out (default 100); must exceed CostSkillMidRank, enforced in validateProgression

	// ── COSTS: ENCUMBRANCE MULTIPLIER (U7) ───────────────────────────────────
	// costs.EncumbranceMultiplier prices carried weight into PHYSICAL actions
	// only. Two linear segments joined at CostEncumbranceKnee: gentle from
	// empty to the knee, steep from the knee to capacity, flat at/above it.
	CostEncumbranceKnee     ConfigFloat `yaml:"CostEncumbranceKnee"`     // Fraction of capacity where the curve steepens (default 0.75)
	CostEncumbranceKneeMult ConfigFloat `yaml:"CostEncumbranceKneeMult"` // Multiplier at the knee (default 1.5)
	CostEncumbranceMax      ConfigFloat `yaml:"CostEncumbranceMax"`      // Multiplier at/above capacity (default 5.0)

	// ── COSTS: COMPOSED-TOTAL CEILING (U7) ───────────────────────────────────
	// costs.Calc clamps the PRODUCT of its multipliers (encumbrance x inverse
	// skill x per-action modifier) to this, NOT each factor in turn. Clamping
	// factors individually still lets encumbrance 5.0, a rank-0 penalty 1.10
	// and a defence premium 1.25 stack to 6.875x, which a laden novice cannot
	// pay — and "cannot pay" reads to the player as autofail-everything, not as
	// expensive. The action's Base price sits OUTSIDE this clamp.
	CostTotalMultiplierMax ConfigFloat `yaml:"CostTotalMultiplierMax"` // Ceiling on the composed cost multiplier (default 6.0)

	// ── PROGRESSION MULTIPLIERS ──────────────────────────────────────────────
	// Per-stat and per-skill multipliers on progression chance.
	// Use plain float64 maps (not ConfigFloat) for native YAML unmarshaling.
	StatProgressionMultipliers  map[string]float64 `yaml:"StatProgressionMultipliers"`  // Per-stat multiplier on progression chance (default 1.0; dex 0.5)
	SkillProgressionMultipliers map[string]float64 `yaml:"SkillProgressionMultipliers"` // Per-skill multiplier on progression chance — overrides hardcoded defaults

	// ── CHARACTER CREATION ────────────────────────────────────────────────────
	StatRollMean   ConfigFloat `yaml:"StatRollMean"`   // Mean for stat rolls at character creation (default 100.0)
	StatRollStdDev ConfigFloat `yaml:"StatRollStdDev"` // Std dev for stat rolls (default 15.0)
	StatRollMin    ConfigFloat `yaml:"StatRollMin"`    // Minimum stat value from rolls (default 70.0)
	StatRollMax    ConfigFloat `yaml:"StatRollMax"`    // Maximum stat value from rolls (default 130.0)
	StartingHealth ConfigInt   `yaml:"StartingHealth"` // Initial health points at character creation (default 10)

	// ── CRAFTING ──────────────────────────────────────────────────────────────
	CraftingBaseSuccessChance  ConfigInt `yaml:"CraftingBaseSuccessChance"`  // % before skill adjustment (default 50)
	CraftingSkillBonusPerLevel ConfigInt `yaml:"CraftingSkillBonusPerLevel"` // +% per skill level above recipe minimum (default 5)
	CraftingMinSuccessChance   ConfigInt `yaml:"CraftingMinSuccessChance"`   // Floor (default 5)
	CraftingMaxSuccessChance   ConfigInt `yaml:"CraftingMaxSuccessChance"`   // Ceiling (default 95)

	// ── RECIPE DISCOVERY ─────────────────────────────────────────────────────
	RecipeDiscoveryBaseChance ConfigFloat `yaml:"RecipeDiscoveryBaseChance"` // Base % to discover a new recipe per successful craft (default 8.0)
	RecipeDiscoveryDecayRate  ConfigFloat `yaml:"RecipeDiscoveryDecayRate"`  // Decay per known recipe: chance = base / (1 + known*this) (default 0.1)

	// ── SALVAGE ──────────────────────────────────────────────────────────────
	SalvageMinChance    ConfigFloat `yaml:"SalvageMinChance"`    // Per-ingredient recovery chance at skill 1 (default 0.15)
	SalvageMaxChance    ConfigFloat `yaml:"SalvageMaxChance"`    // Hard cap on per-ingredient chance (default 0.85)
	SalvageSoftCap      ConfigInt   `yaml:"SalvageSoftCap"`      // Skill level for max curve (default 50)
	SalvageGoldPerRound ConfigInt   `yaml:"SalvageGoldPerRound"` // Ingredient gold value per salvage round (default 10)
	SalvageMaxRounds    ConfigInt   `yaml:"SalvageMaxRounds"`    // Maximum salvage rounds (default 5)

	// ── QUEST ENGINE ─────────────────────────────────────────────────────────
	QuestLogLevel          string    `yaml:"QuestLogLevel"`          // verbose, medium, minimal (default verbose)
	QuestChainDepthLimit   ConfigInt `yaml:"QuestChainDepthLimit"`   // max chained grant evaluations per event (default 10)
	QuestPerformanceWarnMs ConfigInt `yaml:"QuestPerformanceWarnMs"` // warn if trigger evaluation exceeds this (default 50)

	// ── MUTATIONS ─────────────────────────────────────────────────────────────
	MutationBaseProgress         ConfigFloat `yaml:"MutationBaseProgress"`         // Progress needed for first mutation (default 50.0)
	MutationProgressScale        ConfigFloat `yaml:"MutationProgressScale"`        // Each additional mutation costs Scale^n more (default 1.5)
	MutationMaxCount             ConfigInt   `yaml:"MutationMaxCount"`             // Max simultaneous mutations per character (default 5)
	MutationMaxLevel             ConfigInt   `yaml:"MutationMaxLevel"`             // Max level any single mutation can reach (default 3)
	MutationDeepenChance         ConfigFloat `yaml:"MutationDeepenChance"`         // Probability of deepening vs new discovery when both possible (default 0.70)
	MutationProgressGainPerRound ConfigFloat `yaml:"MutationProgressGainPerRound"` // Progress added per combat round (default 1.0)
	// ── MUTATION GRAPH (drift + opposition) ───────────────────────────────────
	MutationAffinityPerSkillUse    ConfigFloat `yaml:"MutationAffinityPerSkillUse"`    // drift affinity added per cluster-relevant skill use (default 1.0)
	MutationAffinityPerCombatEvent ConfigFloat `yaml:"MutationAffinityPerCombatEvent"` // drift affinity per combat behavior event — tank/dodge/control (default 1.0)
	MutationAffinityPerRarity      ConfigFloat `yaml:"MutationAffinityPerRarity"`      // coefficient of the QUADRATIC depth gate: threshold = rarity^2 * this (default 2.0 → r3 ~18, r6 ~72, r8 bridge ~128, r9 apex ~162)
	MutationAffinityDecay          ConfigFloat `yaml:"MutationAffinityDecay"`          // per-tick multiplicative decay of cluster affinity (default 0.98)
	MutationBodyConvictionDecayMax ConfigFloat `yaml:"MutationBodyConvictionDecayMax"` // max fraction of ConvictionMax lost to deep Body commitment (default 0.9)
	MutationBeliefGearDecayMax     ConfigFloat `yaml:"MutationBeliefGearDecayMax"`     // max fraction of gear effectiveness lost to deep Belief commitment (default 0.9)
	MutationPoleDecayRef           ConfigFloat `yaml:"MutationPoleDecayRef"`           // pole-depth at which decay reaches half its max (default 60.0)
	MutationLevel2Multiplier       ConfigFloat `yaml:"MutationLevel2Multiplier"`       // Effect scaling at level 2 (default 1.5)
	MutationLevel3Multiplier       ConfigFloat `yaml:"MutationLevel3Multiplier"`       // Effect scaling at level 3 (default 2.0)
	MutationLevel4Multiplier       ConfigFloat `yaml:"MutationLevel4Multiplier"`       // Effect scaling at level 4 (default 2.5)

	// ── SPELLCASTING ─────────────────────────────────────────────────────
	SpellDiscoveryBaseChance ConfigFloat `yaml:"SpellDiscoveryBaseChance"` // Base % to discover a new spell per successful cast (default 5.0)
	SpellDiscoveryDecayRate  ConfigFloat `yaml:"SpellDiscoveryDecayRate"`  // Decay per known spell: chance = base / (1 + known*this) (default 0.1)

	// ── DISCOVERY OFFSET (shared: spells + recipes) ──────────────────────────
	DiscoveryPerceptionScale        ConfigFloat `yaml:"DiscoveryPerceptionScale"`        // Raw Per contribution reaches 1.0 at (Per - 100) / this (default 200)
	DiscoverySkillScale             ConfigFloat `yaml:"DiscoverySkillScale"`             // Raw skill contribution reaches 1.0 at rank / this (default 100)
	DiscoveryMaxDecayOffset         ConfigFloat `yaml:"DiscoveryMaxDecayOffset"`         // Hard ceiling on combined offset; effective decay floor = Decay × (1 - this) (default 0.8)
	SpellFoldsSkillFactor           ConfigInt   `yaml:"SpellFoldsSkillFactor"`           // Skill * this in folds-per-round calc (default 25)
	SpellProficiencyCastsPerPoint   ConfigInt   `yaml:"SpellProficiencyCastsPerPoint"`   // Casts needed per 1 proficiency point (default 50)
	ConjureCooldown                 ConfigInt   `yaml:"ConjureCooldown"`                 // Rounds between corpse-free conjure/summon casts, on their own key (default 36)
	SpellDifficultyProgressionScale ConfigFloat `yaml:"SpellDifficultyProgressionScale"` // Per-point spell difficulty bonus to skill progression (default 0.01)
	CraftDifficultyProgressionScale ConfigFloat `yaml:"CraftDifficultyProgressionScale"` // Per-point recipe skill_minimum bonus to skill progression (default 0.02)
	SelfCastProgressionMultiplier   ConfigFloat `yaml:"SelfCastProgressionMultiplier"`   // Progression multiplier when spell only targets self (default 0.5)

	// ── POOL RESERVATION ─────────────────────────────────────────────────────
	PoolReservationCapPct ConfigFloat `yaml:"PoolReservationCapPct"` // Ceiling on TOTAL reservation per pool, as a fraction of that pool's max (default 0.66)

	// ── ENCHANTMENTS ─────────────────────────────────────────────────────────
	EnchantTierUpBaseChance     ConfigFloat `yaml:"EnchantTierUpBaseChance"`     // Chance per use (once threshold met) to advance tier (default 0.02)
	EnchantTierUsesBase         ConfigInt   `yaml:"EnchantTierUsesBase"`         // Uses needed for tier 0→1 (default 25)
	EnchantTierUsesScale        ConfigFloat `yaml:"EnchantTierUsesScale"`        // Multiplier per tier for uses threshold (default 2.5)
	EnchantRemovalPenaltyRounds ConfigInt   `yaml:"EnchantRemovalPenaltyRounds"` // Rounds of withdrawal after disenchant (default 50)
	EnchantMaxTier              ConfigInt   `yaml:"EnchantMaxTier"`              // Maximum tier enchantments can reach (default 4)

	// ── ENCHANT SALVAGE BANDS ─────────────────────────────────────────────────
	// Tiered potion → enchanting-mat mapping. Potions are bucketed by their
	// alchemy-recipe skill_minimum into four bands, each with per-mat drop %s.
	EnchantSalvageBand2Min         ConfigInt `yaml:"EnchantSalvageBand2Min,omitempty"`         // potion skill_min >= this → band 2 (default 10)
	EnchantSalvageBand3Min         ConfigInt `yaml:"EnchantSalvageBand3Min,omitempty"`         // band 3 (default 18)
	EnchantSalvageBand4Min         ConfigInt `yaml:"EnchantSalvageBand4Min,omitempty"`         // band 4 (default 28)
	EnchantSalvageBand2SettingPct  ConfigInt `yaml:"EnchantSalvageBand2SettingPct,omitempty"`  // % chance to yield chrysalis-setting in band 2 (default 25)
	EnchantSalvageBand3SettingPct  ConfigInt `yaml:"EnchantSalvageBand3SettingPct,omitempty"`  // % chance to yield chrysalis-setting in band 3 (default 35)
	EnchantSalvageBand3CatalystPct ConfigInt `yaml:"EnchantSalvageBand3CatalystPct,omitempty"` // % chance to yield mutation-catalyst in band 3 (default 12)
	EnchantSalvageBand4CatalystPct ConfigInt `yaml:"EnchantSalvageBand4CatalystPct,omitempty"` // % chance to yield mutation-catalyst in band 4 (default 40)
	EnchantSalvageBand4SettingPct  ConfigInt `yaml:"EnchantSalvageBand4SettingPct,omitempty"`  // % chance to yield chrysalis-setting in band 4 (default 30)
	EnchantSalvageBand4CorePct     ConfigInt `yaml:"EnchantSalvageBand4CorePct,omitempty"`     // % chance to yield chrysalis-core in band 4 (default 8)

	// ── WORLD EVENTS ─────────────────────────────────────────────────────────
	WorldEventBufferSize ConfigInt `yaml:"WorldEventBufferSize"` // Max events in the ring buffer (default 200)

	// ── CHARACTER MANAGEMENT ──────────────────────────────────────────────────
	CharacterRenameCooldownHours ConfigInt `yaml:"CharacterRenameCooldownHours"` // Hours between player renames (0 disables; default 168 = 7 days)

	// ── MOB MUTATIONS ────────────────────────────────────────────────────────
	MobMutationEnabled ConfigBool  `yaml:"MobMutationEnabled"` // Enable mob mutation acquisition in combat (default false)
	MobMutationRate    ConfigFloat `yaml:"MobMutationRate"`    // Multiplier on mutation progress vs players (default 0.3)

	// ── MOB AI ───────────────────────────────────────────────────────────────
	CombatMemoryDuration            ConfigInt   `yaml:"CombatMemoryDuration"`            // Rounds before combat memory expires (default 300)
	MobAIEnabled                    ConfigBool  `yaml:"MobAIEnabled"`                    // Global toggle for reactive AI system (default true)
	MobReactionDelayMin             ConfigFloat `yaml:"MobReactionDelayMin"`             // Min reaction delay in seconds (default 0.25)
	MobReactionDelayMax             ConfigFloat `yaml:"MobReactionDelayMax"`             // Max reaction delay in seconds (default 4.0)
	MobBTreeReactionBase            ConfigFloat `yaml:"MobBTreeReactionBase"`            // Base reaction delay in seconds for behavior tree mobs (default 3.0)
	MobBTreeReactionPerceptionScale ConfigInt   `yaml:"MobBTreeReactionPerceptionScale"` // Perception divisor for reaction delay (default 100)

	// ── MOB SCHEDULES (chunk 3.2) ────────────────────────────────────────────
	// ScheduleMaxPathRetries is the number of consecutive failed pathto
	// attempts a scheduled mob will tolerate before falling back to
	// `pathto home`. Default 20 (≈80 seconds at the default tick rate).
	// See chunk 3.2 spec.
	ScheduleMaxPathRetries ConfigInt `yaml:"ScheduleMaxPathRetries"`

	// ── MOB SLEEP & WAKE (chunk 3.3) ──────────────────────────────────────────
	// SleepRegenMultiplier multiplies HP/SP/CP per-round percentage regen
	// when the bearer has the Sleeping flag. Default 5.0 — sleep is the
	// dominant recovery mechanic. Chunk 3.3.
	SleepRegenMultiplier ConfigFloat `yaml:"SleepRegenMultiplier"`

	// ScheduleWakeGraceRounds is the cooldown (in rounds) during which a
	// scheduled mob will not re-sleep after a forced wake. Prevents the
	// schedule executor from immediately re-applying Sleeping when the
	// player interacts with a sleeping NPC. Default 50 (~200 sec real-time
	// at default tick rate). Chunk 3.3.
	ScheduleWakeGraceRounds ConfigInt `yaml:"ScheduleWakeGraceRounds"`

	// ── NPC-NPC IDLE CONVERSATIONS (chunk 3.6) ───────────────────────────────────
	// ConversationBaseChancePct is the per-tick percentage chance that a
	// fully-idle NPC will attempt to start an idle conversation with an
	// in-room partner that has a relationship edge. Default 1.0 → ~once
	// per 100 ticks per NPC. Chunk 3.6.
	ConversationBaseChancePct ConfigFloat `yaml:"ConversationBaseChancePct"`

	// ConversationPlayerArrivalBoostPct is the percentage chance that
	// a conversation will start when a player arrives in a room with 2+
	// relateable, idle NPCs. Default 25. Chunk 3.6.
	ConversationPlayerArrivalBoostPct ConfigInt `yaml:"ConversationPlayerArrivalBoostPct"`

	// ConversationCooldownRounds is the cooldown applied to both NPCs
	// after a conversation completes, before either can initiate another.
	// Default 50 (~200 sec real-time). Chunk 3.6.
	ConversationCooldownRounds ConfigInt `yaml:"ConversationCooldownRounds"`

	// ── GOAL SELECTION (chunk 4.2) ────────────────────────────────────────────
	// GoalSelectSwitchMargin is the minimum effective-score advantage a
	// challenger goal must have over the current goal to displace it.
	// Hysteresis safety against goal-thrash on noisy scoring inputs.
	// Default 5.0; floats so weights/contextMod can produce fractional
	// scores. Chunk 4.2.
	GoalSelectSwitchMargin ConfigFloat `yaml:"GoalSelectSwitchMargin"`

	// GoalSelectMinHoldRounds is the minimum number of rounds the
	// current goal must be held before any switch is allowed. ≈ 5 min
	// at default tick rate (default 100). Chunk 4.2.
	GoalSelectMinHoldRounds ConfigInt `yaml:"GoalSelectMinHoldRounds"`

	// GoalSelectTickEnabled is the master kill-switch for the tick-driven
	// recompute path. Eager recompute on Add/Remove/Clear still fires
	// when false (cache stays consistent). Default true. Chunk 4.2.
	GoalSelectTickEnabled ConfigBool `yaml:"GoalSelectTickEnabled"`

	// GoalPruneIntervalRounds is how often (in rounds) the per-mob goal
	// prune sweep runs. Staggered per mob to avoid a synchronized spike.
	// chunk 4.6.
	GoalPruneIntervalRounds ConfigInt `yaml:"GoalPruneIntervalRounds"`

	// GoalAbandonDormantRounds is how many consecutive rounds a goal's
	// context score may stay at ~0 before it is abandoned (pruned).
	// chunk 4.6.
	GoalAbandonDormantRounds ConfigInt `yaml:"GoalAbandonDormantRounds"`

	// GuardWarnGraceRounds is how many rounds a warned (Cold-rep) player may
	// remain present before a guard escalates the warning to an attack. 5.1a.
	GuardWarnGraceRounds ConfigInt `yaml:"GuardWarnGraceRounds"`

	// JusticeCrimeLookbackRounds is the recency window for the unresolved-crime
	// "wanted" signal used by guard enforcement. 5.1a.
	JusticeCrimeLookbackRounds ConfigInt `yaml:"JusticeCrimeLookbackRounds"`

	// JusticeBountyExpiryRounds is how long an auto-declared town-faction
	// bounty stays open before lapsing. 5.1b.
	JusticeBountyExpiryRounds ConfigInt `yaml:"JusticeBountyExpiryRounds"`

	// JusticeBountyMurderMult scales an auto-bounty's gold when triggered by
	// an identified murder. 5.1b.
	JusticeBountyMurderMult ConfigFloat `yaml:"JusticeBountyMurderMult"`

	// JusticeBountyRepMultMax is the max gold multiplier at maximum hostility
	// (rep -100) for rep-triggered auto-bounties. 5.1b.
	JusticeBountyRepMultMax ConfigFloat `yaml:"JusticeBountyRepMultMax"`

	// ArrestResistGraceRounds is the window between a guard declaring an arrest
	// and hauling the player off, during which the player may fight back. 5.1c.
	ArrestResistGraceRounds ConfigInt `yaml:"ArrestResistGraceRounds"`

	// JusticeFineDecayPerRound is gold the arrest fine drops per round served;
	// sentence length = fine / this. 5.1c.
	JusticeFineDecayPerRound ConfigInt `yaml:"JusticeFineDecayPerRound"`

	// JusticeArrestRepReset is the reputation floor restored with the issuing
	// faction when a sentence is served/paid (only raises, never lowers). 5.1c.
	JusticeArrestRepReset ConfigInt `yaml:"JusticeArrestRepReset"`

	// ── PACK SCALING ─────────────────────────────────────────────────────────
	PackScalingEnabled   ConfigBool `yaml:"PackScalingEnabled"`   // Enable pack survival bonuses (default true)
	PackSurvivalRounds   ConfigInt  `yaml:"PackSurvivalRounds"`   // Consecutive rounds together before bonus (default 10)
	PackBonusTrainingPts ConfigInt  `yaml:"PackBonusTrainingPts"` // Training points awarded per pack bonus (default 1)
	PackMaxBonus         ConfigInt  `yaml:"PackMaxBonus"`         // Max total pack bonus training points (default 5)

	// ── PACK ROAMING ────────────────────────────────────────────────────────
	PackRoamingEnabled ConfigBool `yaml:"PackRoamingEnabled"` // Enable alpha-follow pack movement (default true)
	PackMaxSize        ConfigInt  `yaml:"PackMaxSize"`        // Max followers per alpha (-1 = unlimited, default -1)
	PackScatterRounds  ConfigInt  `yaml:"PackScatterRounds"`  // Rounds mobs skip wandering after alpha death (default 2)

	// ── CRAFTER MOBS ─────────────────────────────────────────────────────────
	CrafterEnabled       ConfigBool `yaml:"CrafterEnabled"`       // Enable mob autonomous crafting (default true)
	CrafterRareThreshold ConfigInt  `yaml:"CrafterRareThreshold"` // SkillMinimum at or above which a craft is considered rare (default 3)

	// Deprecated: replaced by RestockCadenceTier{50,40,30,20,10}*. Kept
	// only so old config.yaml files load without error. Remove after
	// one deploy cycle.
	CrafterMaterialRestockRate ConfigInt `yaml:"CrafterMaterialRestockRate"`

	// Per-rarity-tier restock cadences (game-time hours). Replaces the
	// single CrafterMaterialRestockRate. Higher rarity tiers (= more
	// common) fire faster; lower tiers (= rarer) fire slowly as a
	// backstop on top of forager / player-sale input.
	RestockCadenceTier50Hours ConfigInt `yaml:"RestockCadenceTier50Hours"`
	RestockCadenceTier40Hours ConfigInt `yaml:"RestockCadenceTier40Hours"`
	RestockCadenceTier30Hours ConfigInt `yaml:"RestockCadenceTier30Hours"`
	RestockCadenceTier20Hours ConfigInt `yaml:"RestockCadenceTier20Hours"`
	RestockCadenceTier10Days  ConfigInt `yaml:"RestockCadenceTier10Days"`

	// ── GOSSIP SYSTEM ────────────────────────────────────────────────────────
	GossipIntervalRounds ConfigInt `yaml:"GossipIntervalRounds"` // Rounds between gossip broadcasts for "gossiper" group mobs (default 75)

	// ── MOON PHASES ───────────────────────────────────────────────────────────
	MoonStatModMax          ConfigFloat `yaml:"MoonStatModMax"`          // Max fractional stat modifier from moon phases, e.g. 0.05 = ±5% (default 0.05)
	CarryCapacityMultiplier ConfigFloat `yaml:"CarryCapacityMultiplier"` // Strength multiplier for carry capacity in lbs (default 0.65)

	// ── TOXICITY ────────────────────────────────────────────────────────────
	ToxicityDecayPerTick      ConfigFloat `yaml:"ToxicityDecayPerTick"`      // Points decayed per regen tick (default 1.0)
	ToxicityBaseMax           ConfigFloat `yaml:"ToxicityBaseMax"`           // Base max before vitality bonus (default 100)
	ToxicityVitalityScale     ConfigFloat `yaml:"ToxicityVitalityScale"`     // Vitality divisor for max bonus (default 5)
	ToxicitySicknessDamagePct ConfigFloat `yaml:"ToxicitySicknessDamagePct"` // % max-HP/tick acute harm at top band (default 0.02)
	ToxicityHighDecaySlowMult ConfigFloat `yaml:"ToxicityHighDecaySlowMult"` // decay multiplier when toxicity >= 75% (default 0.5 = clears slower)

	// ── BLOOM ───────────────────────────────────────────────────────────────
	BloomAddictionPerDose      ConfigInt   `yaml:"BloomAddictionPerDose"`      // addiction gained per dose (default 1)
	BloomAddictionDecayRounds  ConfigInt   `yaml:"BloomAddictionDecayRounds"`  // rounds of abstinence per -1 addiction (default 300)
	BloomWithdrawalOnsetRounds ConfigInt   `yaml:"BloomWithdrawalOnsetRounds"` // abstinence rounds before withdrawal (default 60)
	BloomMutationAdvanceChance ConfigFloat `yaml:"BloomMutationAdvanceChance"` // chance/dose to advance strongest mutation (default 0.50)
	BloomNewMutationChance     ConfigFloat `yaml:"BloomNewMutationChance"`     // chance/dose to instead grant a new mutation (default 0.10)
	BloomCommunionRounds       ConfigInt   `yaml:"BloomCommunionRounds"`       // communion high duration in rounds (default 30)
	BloomCrashRoundsMult       ConfigFloat `yaml:"BloomCrashRoundsMult"`       // crash duration = communion * this (default 2.5)

	// ── MANIFESTATION / COMPANION SCALING ───────────────────────────────────
	ManifestStatScaleChaFactor   ConfigInt   `yaml:"ManifestStatScaleChaFactor"`   // Charisma divisor for companion stat scaling (default 150)
	ManifestStatScaleSkillFactor ConfigFloat `yaml:"ManifestStatScaleSkillFactor"` // Manifestation skill additive factor (default 0.02)

	// ── COMPANION CONVICTION ECONOMY ────────────────────────────────────────
	CompanionReserveSkillPct      ConfigFloat `yaml:"CompanionReserveSkillPct"`      // Reservation reduction per manifestation skill point (default 0.01)
	CompanionReserveSkillCap      ConfigFloat `yaml:"CompanionReserveSkillCap"`      // Max reduction from skill (default 0.55, cap reached at manifestation 55)
	CompanionReserveMutPctPerRank ConfigFloat `yaml:"CompanionReserveMutPctPerRank"` // Reservation reduction per Manifester mutation rank (default 0.06)
	CompanionReserveMutCap        ConfigFloat `yaml:"CompanionReserveMutCap"`        // Max reduction from the Manifester mutation (default 0.24)
	CompanionReserveTotalCap      ConfigFloat `yaml:"CompanionReserveTotalCap"`      // Combined reduction ceiling (default 0.79)
	CompanionSoftCap              ConfigInt   `yaml:"CompanionSoftCap"`              // Soft count backstop; real limit is the reservation ceiling (default 5)
	CompanionSoftCapApex          ConfigInt   `yaml:"CompanionSoftCapApex"`          // Soft count backstop with the Manifester apex (default 7)
	CompanionReserveDefault       ConfigInt   `yaml:"CompanionReserveDefault"`       // Base reserve a companion costs, scaled by the spell's summon_pet_multiplier (default 280)
	CharmDurationMinRounds        ConfigInt   `yaml:"CharmDurationMinRounds"`        // Rounds a barely-won charm holds, and what a floored win takes (default 30)
	CharmDurationMaxRounds        ConfigInt   `yaml:"CharmDurationMaxRounds"`        // Rounds a charm won by two sigma or better holds (default 450)
	HomunculusCraftScale          ConfigFloat `yaml:"HomunculusCraftScale"`          // Chrysifier: homunculus statpool = (sum of crafting-skill levels) * this (default 4.0)
	HomunculusConvictionReserve   ConfigInt   `yaml:"HomunculusConvictionReserve"`   // Chrysifier: base Conviction the homunculus reserves before reduction (default 300; was 1000 before the U7b cap made that unfieldable by its own crafter)

	// ── SHOP ECONOMY ─────────────────────────────────────────────────────────
	ShopBuyRatio              ConfigFloat `yaml:"ShopBuyRatio,omitempty"`              // Base buy/sell spread: NPC buy offer = baseValue * BuyRatio * scarcityMult (default 0.50)
	ShopPriceFloor            ConfigFloat `yaml:"ShopPriceFloor,omitempty"`            // Minimum scarcity multiplier when stock is very high (default 0.25)
	ShopPriceCeiling          ConfigFloat `yaml:"ShopPriceCeiling,omitempty"`          // Maximum scarcity multiplier when stock is zero (default 5.0)
	ShopAbundanceThreshold    ConfigFloat `yaml:"ShopAbundanceThreshold,omitempty"`    // Stock/restock ratio at which price hits the floor (default 3.0)
	ShopMaterialReserve       ConfigInt   `yaml:"ShopMaterialReserve,omitempty"`       // Units of each material a crafter mob reserves before selling (default 1)
	DefaultPricingBaselineQty ConfigInt   `yaml:"DefaultPricingBaselineQty,omitempty"` // Pricing baseline (scarcity-curve denominator) for stock entries with RestockQty==0, e.g. crafted/caravan-delivered goods (default 3)
	// CrafterIngredientReservePct is the fraction of an ingredient's
	// MaxStock the crafter mob keeps in reserve when deciding whether to
	// craft. Prevents the crafter from consuming its own stock to a level
	// where players can't buy. Per-ingredient check, floor of 1.
	CrafterIngredientReservePct ConfigFloat `yaml:"CrafterIngredientReservePct"`        // Fraction of MaxStock kept as reserve (default 0.25)
	ShopGoldReserveRatio        ConfigFloat `yaml:"ShopGoldReserveRatio,omitempty"`     // Fraction of gold pool a shop keeps in reserve before buying (default 0.50)
	ShopMaxStockMultiplier      ConfigFloat `yaml:"ShopMaxStockMultiplier,omitempty"`   // Global multiplier on EffectiveMaxStock — chunk 3.8 bumped to 2.0 to give the cross-city caravan room to build surplus (default 2.0)
	ShopOverstockDecayRounds    ConfigInt   `yaml:"ShopOverstockDecayRounds,omitempty"` // Rounds an over-baseline stock entry must sit un-grown before one unit decays (default 21600 ≈ several in-game days)
	ShopOverstockDecayQty       ConfigInt   `yaml:"ShopOverstockDecayQty,omitempty"`    // Units removed per decay fire (default 1)
	BarterMaxDiscount           ConfigFloat `yaml:"BarterMaxDiscount,omitempty"`        // Max fractional price reduction a player can get via bartering (default 0.15)
	BarterMaxBonus              ConfigFloat `yaml:"BarterMaxBonus,omitempty"`           // Max fractional sell-price bonus a player can get via bartering (default 0.15)
	StorageFeePerItem           ConfigInt   `yaml:"StorageFeePerItem"`                  // Gold charged per stored item per game month (default 1)
	StorageSeizureMinValue      ConfigInt   `yaml:"StorageSeizureMinValue"`             // Min aggregate stack value (spec.Value*Count) for a seized slot to be auctioned vs. disposed (default 250). Set very high to disable seizure-auction (dispose all).
	MailSendCooldownRounds      ConfigInt   `yaml:"MailSendCooldownRounds"`             // Rounds a player must wait between sending mail (anti-spam; default 10). Set to 1 for effectively no cooldown.
	AchievementPollRounds       ConfigInt   `yaml:"AchievementPollRounds"`              // How often (rounds) the achievement poll evaluates online players (default 10).
	GuildFoundingCost           ConfigInt   `yaml:"GuildFoundingCost"`                  // One-time gold cost (from bank) to found a guild (default 5000).
	GuildVaultCapacity          ConfigInt   `yaml:"GuildVaultCapacity"`                 // Max items a guild vault holds (default 100).

	// ── WAREHOUSES (Stage 3 ferry system) ────────────────────────────────────
	WarehouseItemCap      ConfigInt `yaml:"WarehouseItemCap,omitempty"`      // Per-item stock cap in city warehouses (default 4,000,000 — effectively unbounded)
	WarehouseAccrualHours ConfigInt `yaml:"WarehouseAccrualHours,omitempty"` // Game-hours between ambient accrual ticks (default 2)

	// ── LOOT ──────────────────────────────────────────────────────────────────
	LootBudgetScalar    ConfigFloat `yaml:"LootBudgetScalar"`    // Multiplier for sqrt(goldPaid) loot budget (default 7.0)
	GoldPerAffixPoint   ConfigFloat `yaml:"GoldPerAffixPoint"`   // Gold value per affix cost-point on instance/affixed loot (default 3.0)
	ShopAffixedStockCap ConfigInt   `yaml:"ShopAffixedStockCap"` // Max per-instance affixed items a shop resells before evicting oldest (default 8)

	// ── INSTANCES ────────────────────────────────────────────────────────────
	InstanceStatPoolCap ConfigInt `yaml:"InstanceStatPoolCap"` // Max stat pool per mob in instances (default 50000, 0=uncapped)

	// ── CRASH SITE (#22) ─────────────────────────────────────────────────────
	CrashSiteSuppressionFactor ConfigFloat `yaml:"CrashSiteSuppressionFactor"` // inside the buried hull (#22), spell power and mutation combat bonuses are scaled to this fraction; 0=fully suppressed, 1=no effect (default 0.35)

	// ── BOSS INTERRUPTS ──────────────────────────────────────────────────────
	// General-purpose disruptor allowlists (not crash-site-specific). Any mob
	// mid-fold-cast (Character.IsCasting() / Activity.IsCasting()) has its
	// cast interrupted via actions.InterruptTargetCast when hit by a thrown
	// item whose id is in BossInterruptItemIds, or a player spell whose id is
	// in BossInterruptSpellIds. Generic melee never interrupts. Defaults set
	// in validateMisc(); see IsBossInterruptItem / IsBossInterruptSpell.
	BossInterruptItemIds  []int    `yaml:"BossInterruptItemIds"`
	BossInterruptSpellIds []string `yaml:"BossInterruptSpellIds"`

	// ── CARAVAN SYSTEM ───────────────────────────────────────────────────────
	// CaravanServedZones lists zone display names whose vendor mobs do NOT
	// auto-restock — they restock only on caravan visit. Mobs in zones not
	// in this list keep the legacy per-mob restock tick.
	CaravanServedZones []string `yaml:"CaravanServedZones"`

	// CaravanDepotDwellRounds is the number of rounds the caravan rests at
	// each depot between transit legs. ~720 ≈ 48 min real ≈ a full game
	// day. Stage 3.1 doubled this from 360 so foragers are the day-to-day
	// supply pipeline; caravans now arrive about once per game day.
	CaravanDepotDwellRounds ConfigInt `yaml:"CaravanDepotDwellRounds"`

	// FernwayPickupDwellRounds is the dwell time at the Fernway forager
	// meeting point (North Road 4038) on each transit leg. Default 6.
	FernwayPickupDwellRounds ConfigInt `yaml:"FernwayPickupDwellRounds"`

	// ── FORAGER SYSTEM (Stage 3.1) ───────────────────────────────────────────

	// ForagerCarryThresholdPct is the carry-capacity ratio (0.0-1.0) at
	// which the forager heads home for delivery. Default 0.75.
	ForagerCarryThresholdPct ConfigFloat `yaml:"ForagerCarryThresholdPct"`

	// ForagerHPRecallThresholdPct is the HP ratio (0.0-1.0) below which
	// the forager casts fold-recall as an emergency escape. Default 0.50.
	ForagerHPRecallThresholdPct ConfigFloat `yaml:"ForagerHPRecallThresholdPct"`

	// ForagerHealPotionThresholdPct is the HP ratio (0.0-1.0) below which
	// the forager auto-drinks a healing salve. Default 0.75.
	ForagerHealPotionThresholdPct ConfigFloat `yaml:"ForagerHealPotionThresholdPct"`

	// ForagerWaitTimeoutRounds is the maximum rounds the Fernway forager
	// idles at the meeting point waiting for the caravan before recalling
	// home with the satchel. Default 150.
	ForagerWaitTimeoutRounds ConfigInt `yaml:"ForagerWaitTimeoutRounds"`

	// ForagerRestCarryThreshold (Stage 3.4) is the carry-capacity ratio
	// (0.0-1.0) above which the forager stays resting at home instead of
	// cycling back to forage. Prevents futile foraging loops when local
	// vendors are saturated (MaxStock cap reached). Default 0.5.
	ForagerRestCarryThreshold ConfigFloat `yaml:"ForagerRestCarryThreshold"`

	// ChestBackpressureResumePct (Stage 5.4) is the storage-lockbox fill
	// fraction (0.0-1.0) at/below which a rested forager is allowed to
	// start a new gather cycle. While the chest is fuller than this, the
	// forager stays resting until the vendor-backfill drains it. Default 0.9.
	ChestBackpressureResumePct ConfigFloat `yaml:"ChestBackpressureResumePct,omitempty"`

	// ForagerRestDurationRounds is how long a forager stays at sanctuary
	// before re-entering the territory. Gated by HP-full and carry-ratio
	// checks too, so the actual rest can be longer if the forager
	// arrived hurt or over-encumbered. Default 40 rounds (~3 real
	// minutes / ~1 game-hour at the default round cadence).
	ForagerRestDurationRounds ConfigInt `yaml:"ForagerRestDurationRounds"`

	// ForagerLockboxCapacity caps how many items a sanctuary lockbox
	// can hold. When the box is full, the forager falls back to the
	// Stage 3.4 rest-extension behavior until a player picks the box
	// open and clears space.
	ForagerLockboxCapacity ConfigInt `yaml:"ForagerLockboxCapacity"`

	// ForagerStuckThresholdRounds is the watchdog timeout. If a forager
	// sits in the same state for more than this many rounds, it is
	// force-reset to Recalling so it heads home, dumps satchel, and
	// re-cycles. Logs a Warn on reset for ops visibility.
	ForagerStuckThresholdRounds ConfigInt `yaml:"ForagerStuckThresholdRounds"`

	// ForagerStoringWatchdogRounds is the maximum number of rounds a
	// forager will spend in StateStoring before bailing to StateRecalling.
	// Prevents infinite loops if the chest workflow stalls (e.g.,
	// persistently full chest or unreachable chest room).
	ForagerStoringWatchdogRounds ConfigInt `yaml:"ForagerStoringWatchdogRounds"`

	// ── ECONOMY HEALTH DASHBOARD ─────────────────────────────────────────────
	EconomySnapshotIntervalHours ConfigInt   `yaml:"EconomySnapshotIntervalHours"` // Wall-clock cadence (default 1)
	EconomySnapshotRetentionDays ConfigInt   `yaml:"EconomySnapshotRetentionDays"` // Auto-snapshot retention (default 30)
	EconomyScoreWeightShop       ConfigFloat `yaml:"EconomyScoreWeightShop"`       // Overall-score weight for shops (default 0.6)
	EconomyScoreWeightCaravan    ConfigFloat `yaml:"EconomyScoreWeightCaravan"`    // (default 0.2)
	EconomyScoreWeightForager    ConfigFloat `yaml:"EconomyScoreWeightForager"`    // (default 0.2)

	// ── ECONOMY SCORING — TtR TARGETS ────────────────────────────────────────
	// Time-to-Refill targets per rarity tier (game-time). Throughput score
	// penalizes items that take longer than their tier's target to refill.
	// Tier 50 (common) should refill in ~3 game-hours; tier 10 (rare) in ~7 game-days.
	TtRTargetTier50Hours ConfigInt `yaml:"TtRTargetTier50Hours"`
	TtRTargetTier40Hours ConfigInt `yaml:"TtRTargetTier40Hours"`
	TtRTargetTier30Hours ConfigInt `yaml:"TtRTargetTier30Hours"`
	TtRTargetTier20Days  ConfigInt `yaml:"TtRTargetTier20Days"`
	TtRTargetTier10Days  ConfigInt `yaml:"TtRTargetTier10Days"`

	// TtRWindowGameDays is the rolling window of completed depletion→refill
	// events used by ThroughputScore (default 7 game-days).
	TtRWindowGameDays ConfigInt `yaml:"TtRWindowGameDays"`

	// ── ECONOMY SCORING — LOGISTICS ──────────────────────────────────────────
	// LogisticsStuckRounds is the round count after which a caravan or forager
	// is considered stuck; the stuck multiplier is applied to its health score.
	// Default 3000 (more aggressive than the MVP 5000).
	LogisticsStuckRounds ConfigInt `yaml:"LogisticsStuckRounds"`
	// LogisticsStuckMultiplier scales the base logistics score when stuck
	// (default 0.4 — a stuck entity reads ~40% of its healthy score).
	LogisticsStuckMultiplier ConfigFloat `yaml:"LogisticsStuckMultiplier"`

	// ── ECONOMY SCORING — OVERALL BLEND ─────────────────────────────────────
	// Weights for the five-axis OverallScore blend. Must sum to 1.0.
	// Logistics is not blended here; it is displayed as a standalone panel.
	ScoreWeightStock      ConfigFloat `yaml:"ScoreWeightStock"`
	ScoreWeightInput      ConfigFloat `yaml:"ScoreWeightInput"`
	ScoreWeightThroughput ConfigFloat `yaml:"ScoreWeightThroughput"`
	ScoreWeightShopGold   ConfigFloat `yaml:"ScoreWeightShopGold"`

	// ── OPINIONS / DISPOSITION ───────────────────────────────────────────────
	OpinionAttackBump              ConfigInt `yaml:"OpinionAttackBump"`              // Disposition delta when a player initiates aggression on a mob (default -15)
	DispositionDecayHalfLifeRounds ConfigInt `yaml:"DispositionDecayHalfLifeRounds"` // Rounds for one half-life of disposition decay toward default (default 100000; 0 disables decay)

	// ── FACTIONS ─────────────────────────────────────────────────────────────
	FactionMemberKillRep       ConfigInt `yaml:"FactionMemberKillRep"`       // Rep delta when a player kills a member of a defined faction (default -10) — DEPRECATED, retained for any non-citizen faction fallback path. Citizen factions use CrimeRepDeltaMurder via internal/crimes.
	CrimeRepDeltaMurder        ConfigInt `yaml:"CrimeRepDeltaMurder"`        // Rep delta on murder crime with identified perpetrator (default -25)
	CrimeRepDeltaAssault       ConfigInt `yaml:"CrimeRepDeltaAssault"`       // Rep delta on assault crime with identified perpetrator (default -10)
	CrimeRepDeltaTheft         ConfigInt `yaml:"CrimeRepDeltaTheft"`         // Rep delta on theft crime with identified perpetrator (default -5)
	CrimeStaleAfterRounds      ConfigInt `yaml:"CrimeStaleAfterRounds"`      // Rounds after which an unresolved crime is auto-snapped to stale (default 7884000 — ~365 game-days at 4-second rounds)
	KnowledgeObservationLogMax ConfigInt `yaml:"KnowledgeObservationLogMax"` // Max observation log entries per record (FIFO) (default 32)

	// ── BOUNTIES ────────────────────────────────────────────────────────────
	BountyGoldDefaultMultiplier ConfigFloat `yaml:"BountyGoldDefaultMultiplier"` // Multiplier on statpool for default gold reward (default 0.5)
	BountyGoldFloor             ConfigInt   `yaml:"BountyGoldFloor"`             // Minimum gold floor for any bounty (default 50)

	// ── BOUNTY HUNTING (5.2) ────────────────────────────────────────────
	BountyHunterGoldThreshold      ConfigInt   `yaml:"BountyHunterGoldThreshold"`      // Single open bounty >= this dispatches a hunter (default 500)
	BountyHunterBaseStatpool       ConfigInt   `yaml:"BountyHunterBaseStatpool"`       // Base of hunter scaled statpool (default 250)
	BountyHunterStatpoolPerGold    ConfigFloat `yaml:"BountyHunterStatpoolPerGold"`    // Statpool added per gold of triggering bounty (default 0.25)
	BountyHunterMinStatpool        ConfigInt   `yaml:"BountyHunterMinStatpool"`        // Clamp floor for hunter statpool (default 300)
	BountyHunterMaxStatpool        ConfigInt   `yaml:"BountyHunterMaxStatpool"`        // Clamp ceiling for hunter statpool (default 500)
	BountyHunterRedispatchCooldown ConfigInt   `yaml:"BountyHunterRedispatchCooldown"` // Rounds before re-dispatch after a hunter dies (default 500)
	BountyHunterGearGoldDivisor    ConfigInt   `yaml:"BountyHunterGearGoldDivisor"`    // gearGold = statpool / this, fed to GenerateAffixedItem (default 5)

	// ── EQUIPMENT-AWARE SHOPPING (5.3) ──────────────────────────────────
	MobUpgradeGoldReserve ConfigInt   `yaml:"MobUpgradeGoldReserve"` // Gold a shopping mob keeps in reserve, won't spend below (default 50)
	MobUpgradeMinDelta    ConfigFloat `yaml:"MobUpgradeMinDelta"`    // Minimum itemvalue swap delta worth buying (default 1.0)

	// ── FACTS & EVENTS ──────────────────────────────────────────────────────
	FactsHeardEventsMax ConfigInt `yaml:"FactsHeardEventsMax"` // Max facts_heard events per mobs instance (default 32)

	// ── PINNACLE ITEMS ───────────────────────────────────────────────────
	BandolierAttuneRounds         ConfigInt `yaml:"BandolierAttuneRounds"`         // Rounds of re-attunement after bandolier contents change (default 100)
	SentientChatterCooldownRounds ConfigInt `yaml:"SentientChatterCooldownRounds"` // Min rounds between sentient item lines (default 20)
	SentientChatterChancePct      ConfigInt `yaml:"SentientChatterChancePct"`      // Percent chance per eligible round that a sentient item speaks (default 15)
}

func (b *Balance) Validate() {
	b.validateCombat()
	b.validateProgression()
	b.validateMobs()
	b.validateSpells()
	b.validateDiscovery()
	b.validateShops()
	b.validateMisc()
}

func GetBalanceConfig() Balance {
	ensureConfigValidated()

	configDataLock.RLock()
	defer configDataLock.RUnlock()

	return configData.Balance
}

// GetStatProgressionMultiplier returns the per-stat progression multiplier
// from config, or 1.0 if the stat has no override.
func (b *Balance) GetStatProgressionMultiplier(statName string) float64 {
	if b.StatProgressionMultipliers != nil {
		if mult, ok := b.StatProgressionMultipliers[statName]; ok {
			return mult
		}
	}
	return 1.0
}

// GetSkillProgressionMultiplier returns the per-skill progression multiplier
// from config, or 0 to signal "use hardcoded default".
func (b *Balance) GetSkillProgressionMultiplier(skillName string) (float64, bool) {
	if b.SkillProgressionMultipliers != nil {
		if mult, ok := b.SkillProgressionMultipliers[skillName]; ok {
			return mult, true
		}
	}
	return 0, false
}

// IsCaravanServedZone reports whether the named zone is in the
// CaravanServedZones list. Case-sensitive — match the zone display
// name exactly.
func (b Balance) IsCaravanServedZone(zone string) bool {
	for _, z := range b.CaravanServedZones {
		if z == zone {
			return true
		}
	}
	return false
}

// IsBossInterruptItem reports whether the given item id is a configured
// boss-interrupt disruptor (see BossInterruptItemIds).
func (b Balance) IsBossInterruptItem(itemId int) bool {
	for _, id := range b.BossInterruptItemIds {
		if id == itemId {
			return true
		}
	}
	return false
}

// IsBossInterruptSpell reports whether the given spell id is a configured
// boss-interrupt disruption spell (see BossInterruptSpellIds).
func (b Balance) IsBossInterruptSpell(spellId string) bool {
	for _, id := range b.BossInterruptSpellIds {
		if id == spellId {
			return true
		}
	}
	return false
}
