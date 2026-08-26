package combat

import (
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// DefenseType represents the type of defense used (Stage 7.1)
type DefenseType string

const (
	DefenseNone  DefenseType = ""
	DefenseDodge DefenseType = "dodge"
	DefenseParry DefenseType = "parry"
	DefenseBlock DefenseType = "block"
)

// WeaponHitInfo tracks whether a specific weapon landed at least one hit
// during a combat round, for per-weapon skill progression.
//
// U6 Task 14: Hit means the weapon dealt damage on at least one swing, a
// deflected swing included. CleanHit means at least one swing actually WON
// the contest (a deflected swing has Hit true but the defence won it, so
// attacker-side progression keys on CleanHit).
type WeaponHitInfo struct {
	SkillTag string // e.g., "weapon-combat", "unarmed-combat", "ranged-combat"
	Hit      bool
	CleanHit bool
	Crit     bool
	Fumble   bool
}

// SwingEvent captures per-swing analytics data for accurate hit rate tracking.
type SwingEvent struct {
	Hit           bool
	Crit          bool
	Fumble        bool
	DoubleFumble  bool
	DefenseCrit   bool
	Damage        int
	DamageReduced int
	DefenseUsed   DefenseType
	AttackZScore  float64
	DefenseZScore float64
	AttackType    string // "weapon", "unarmed", "ranged" — per-swing weapon type for analytics
}

// SwingDefence is ONE swing's defence record for the round's defender
// progression award: which defence ROLLED BEST in that swing's best-of-all
// contest, what it rolled, and whether it beat the attack.
//
// THREE defence records on AttackResult mean three different things and none
// substitutes for another:
//
//   - DefenseUsed     -- a defence that WON. Stamped by sendDefenseMessages,
//     which runs only on a defensive win.
//   - DefenseAttempts -- every defence that was TRIED, in sequence.
//   - SwingDefences   -- the one defence per swing that ROLLED BEST, win or
//     lose. It is the only one of the three populated on a defence that lost,
//     which is what lets U10b-1 award a lost defence at
//     ProgressionFailureFraction instead of awarding nothing.
//
// It is NOT index-parallel to SwingEvents. An UNCONTESTED swing -- the defender
// had no defence available at all -- quotes nothing and appends no entry here,
// because an empty defence name awards nothing (AwardDefenceProgression returns
// early on it) and an empty entry could only displace a real candidate in the
// Best-of.
type SwingDefence struct {
	// Defence is contest.Result.Winner for that swing: the entry that defended
	// best, whether or not it beat the attack roll. Never empty -- an
	// uncontested swing appends no SwingDefence at all.
	Defence DefenseType
	// Roll is that entry's rolled value, the roll that ALREADY happened.
	// Nothing re-rolls to compare swings against each other.
	Roll float64
	// Won is the DEFENCE's win, not the swing's hit: true on a deflection and
	// on a defensive crit, false on every attack win, on a forced crit against
	// a sleeping victim, and on all THREE fumble paths (double fumble, attack
	// fumble, defence fumble). That last row diverges from the channel seam,
	// which has no fumble branch before its award -- see hitResolution.defenceWon,
	// which records the divergence and why U10b-1b owns it.
	Won bool
}

// TaggedMessage pairs a combat narration line with the messaging
// Category that should color it. Multi-weapon characters (e.g., the
// 6-arm test character) produce heterogeneous batches — some swings
// hit, some are defended, some fumble — so per-line categorization
// is necessary for the new palette to render correctly. The hook
// drain iterates these tagged messages instead of guessing one
// Category for the whole batch.
type TaggedMessage struct {
	Category messaging.Category
	Text     string
}

type AttackResult struct {
	// Hit means damage was dealt by at least one swing this round — a
	// DEFLECTED swing included, since U6 Task 10 made a defensive win deal
	// partial damage. CleanHit means at least one swing actually won the
	// contest (crit, normal win, defence fumble, forced crit). Both
	// accumulate across the whole round and are never reset per swing.
	// Consumers that mean "the attack got through the defence" (progression,
	// sounds, weapon break) key on CleanHit; consumers that mean "damage was
	// dealt" (lifesteal, on-hit procs, wimpy) key on Hit. Momentum is
	// per-swing, not per-round: it keys on hitResolution's hit && !defended
	// inside the swing loop, never on this aggregate.
	Hit      bool // defaults false
	CleanHit bool // defaults false
	// WasSurpriseAttack records that this round armed a surprise opening
	// strike. Carried out because calculateCombat DEMOTES Aggro.Type to
	// DefaultAttack while resolving, so every consumer running after the attack
	// -- progression at Phase 5, messaging, analytics -- would otherwise see no
	// trace that the round was an ambush at all.
	WasSurpriseAttack bool
	// SwingsThrown counts every swing resolved this round, ACROSS ALL WEAPONS,
	// landed or not. Like Hit and CleanHit it accumulates for the whole round and
	// is never reset by the per-swing flag reset (which clears Crit, Fumble and
	// DoubleFumble only). U7 Task 7 charges the attacker
	// SwingsThrown x per-swing cost, so a reset here would make a multi-weapon
	// round cost the same as its last weapon's swings and quietly restore the
	// free-offence bug this field exists to fix.
	SwingsThrown            int             // defaults 0
	Crit                    bool            // defaults false
	Fumble                  bool            // defaults false
	DoubleFumble            bool            // defaults false
	BuffSource              []int           // defaults 0
	BuffTarget              []int           // defaults 0
	DamageToTarget          int             // defaults 0
	DamageToTargetReduction int             // defaults 0
	DamageToSource          int             // defaults 0
	DamageToSourceReduction int             // defaults 0
	DefenseUsed             DefenseType     // Which defense avoided the hit (Stage 7.1)
	DefenseAttempts         []DefenseType   // Sequence of defenses attempted (Stage 7.1)
	DefenseZScore           float64         // Defense roll z-score (Stage 8.4)
	AttackZScore            float64         // Attack roll z-score (Stage 8.4)
	ParryCritDetected       bool            // Flag for parry crit → riposte
	DodgeCritDetected       bool            // Flag for dodge crit → auto-trip
	BlockCritDetected       bool            // Flag for block crit → auto-bash
	SwingEvents             []SwingEvent    // Per-swing analytics (Stage 30.2)
	WeaponHits              []WeaponHitInfo // Per-weapon hit tracking for skill progression
	DefenderWasAttacked     bool            // True if any swing was attempted against defender
	MessagesToSource        []TaggedMessage
	MessagesToTarget        []TaggedMessage
	MessagesToSourceRoom    []TaggedMessage
	MessagesToTargetRoom    []TaggedMessage
	MessagesToRoomOld       []TaggedMessage

	// SwingDefences carries the per-swing defence that the round's SINGLE
	// defender progression award is chosen from. See the SwingDefence doc
	// comment for how it differs from DefenseUsed and DefenseAttempts.
	SwingDefences []SwingDefence
}

func (a *AttackResult) SendToSource(cat messaging.Category, msg string) {
	a.MessagesToSource = append(a.MessagesToSource, TaggedMessage{Category: cat, Text: msg})
}

func (a *AttackResult) SendToTarget(cat messaging.Category, msg string) {
	a.MessagesToTarget = append(a.MessagesToTarget, TaggedMessage{Category: cat, Text: msg})
}

func (a *AttackResult) SendToSourceRoom(cat messaging.Category, msg string) {
	a.MessagesToSourceRoom = append(a.MessagesToSourceRoom, TaggedMessage{Category: cat, Text: msg})
}

func (a *AttackResult) SendToTargetRoom(cat messaging.Category, msg string) {
	a.MessagesToTargetRoom = append(a.MessagesToTargetRoom, TaggedMessage{Category: cat, Text: msg})
}

func (a *AttackResult) SendToRoomOld(cat messaging.Category, msg string) {
	a.MessagesToRoomOld = append(a.MessagesToRoomOld, TaggedMessage{Category: cat, Text: msg})
}

// CategoryForWeaponSubtype maps an items.ItemSubType to the right
// CategoryHit* damage-band. Used by the per-swing senders so each
// weapon's narration renders in its band color (warhammer → blunt,
// hook-spear → melee, knuckles → unarmed, etc.).
//
// Includes the literal "piercing" subtype as a synonym for Stabbing —
// the YAML data files use both spellings ("piercing" is the dominant
// term across spears / hooks / smuggler blades; the enum constant is
// historically "stabbing"). Keep both mapped to CategoryHitMelee until
// the YAML naming is normalized.
func CategoryForWeaponSubtype(sub items.ItemSubType) messaging.Category {
	switch sub {
	case items.Bludgeoning, items.Slam:
		return messaging.CategoryHitBlunt
	case items.Cleaving, items.Stabbing, items.Slashing, "piercing":
		return messaging.CategoryHitMelee
	case items.Claws, items.Bite, items.Sting, items.Gore:
		return messaging.CategoryHitNaturalSharp
	case items.Shooting, items.Whipping:
		return messaging.CategoryHitRanged
	case items.Wand, items.Sceptre, items.Staff:
		return messaging.CategoryHitCaster
	case items.Unarmed, items.Fist:
		return messaging.CategoryHitUnarmed
	}
	return messaging.CategoryHitMelee
}

// CategoryForDefenseVerb maps a defense verb ("dodge"/"parry"/"block")
// to the matching CategoryDodge/Parry/Block. Falls back to
// CategoryDodge for unknown verbs (e.g., "avoid"); not visually
// critical since unknown verbs already use the catch-all defense
// flavor.
func CategoryForDefenseVerb(verb string) messaging.Category {
	switch verb {
	case "dodge":
		return messaging.CategoryDodge
	case "parry":
		return messaging.CategoryParry
	case "block":
		return messaging.CategoryBlock
	}
	return messaging.CategoryDodge
}
