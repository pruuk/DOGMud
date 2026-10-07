package items

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

var (
	attackMessages map[ItemSubType]*WeaponAttackMessageGroup = map[ItemSubType]*WeaponAttackMessageGroup{}
)

type SkillTier string

const (
	Beginner SkillTier = "beginner"
	Expert   SkillTier = "expert"
	Master   SkillTier = "master"
)

type WeaponAttackMessageGroup struct {
	OptionId ItemSubType `yaml:"optionid"`
	Options  AttackTypes `yaml:"options"`
}

type AttackTypes map[Intensity]AttackOptions

type AttackOptions struct {
	Together TogetherMessages `yaml:"together"`
	Separate SeparateMessages `yaml:"separate"`
}

// TogetherMessages is the authored shape of a blow whose participants share a
// room, so there is exactly one observer audience.
//
// The keys are the canonical role vocabulary (M4b-1), spelled
// toattacker/todefender/toroom until then. The Go field names still carry the
// old spelling, which is cosmetic and left for a later pass.
type TogetherMessages struct {
	ToAttacker SkillTieredMessages `yaml:"actor"`
	ToDefender SkillTieredMessages `yaml:"actee"`
	ToRoom     SkillTieredMessages `yaml:"observer"`
}

// SeparateMessages is the authored shape of a ranged blow whose participants
// are in different rooms, so there are two observer audiences: the one
// standing with the actor, and the REMOTE one standing with the actee.
//
// That second audience is why `toattackerroom` became plain `observer` while
// `todefenderroom` became `remote_observer`: the core has always called the
// observers with the actor the observer, and the far-room audience is the one
// that needs a name of its own (narration.Roles.ActeeObserver).
type SeparateMessages struct {
	ToAttacker     SkillTieredMessages `yaml:"actor"`
	ToDefender     SkillTieredMessages `yaml:"actee"`
	ToAttackerRoom SkillTieredMessages `yaml:"observer"`
	ToDefenderRoom SkillTieredMessages `yaml:"remote_observer"`
}

type SkillTieredMessages struct {
	Beginner MessageOptions `yaml:"beginner"`
	Expert   MessageOptions `yaml:"expert,omitempty"`
	Master   MessageOptions `yaml:"master,omitempty"`
}

type MessageOptions []ItemMessage

// Get chooses a message using the default picker (narration.DefaultPicker,
// which routes through util.Rand).
func (mo MessageOptions) Get() ItemMessage {
	return mo.GetWith(nil)
}

// GetWith is Get with an explicit picker, for the snapshot harness. A nil
// picker means production behaviour: narration.DefaultPicker, i.e. util.Rand.
//
// The seeded index override this used to carry went with
// ConsistentAttackMessages in M3 item 8, along with an
// `if seedNum[0] == 0 { return mo[0] }` branch that the guard above it had
// made unreachable on every input.
func (mo MessageOptions) GetWith(pick narration.Picker) ItemMessage {
	if pick == nil {
		pick = narration.DefaultPicker
	}

	if ct := len(mo); ct > 0 {
		return mo[pick(ct)]
	}

	return ItemMessage("")
}

// PoolFor returns the tier union for a skill level as the core's plain-string
// form. The union is cumulative and matches what GetForSkillLevelWith built:
// beginner always, plus expert at 34, plus master at 67.
//
// Assembly stays in the store. The core coordinates the index and substitutes
// tokens; it knows nothing about tiers (internal/narration/context.md).
//
// Returns nil rather than an empty slice when nothing is authored, so a role
// the core sees as absent is absent rather than present-and-empty.
func (stm SkillTieredMessages) PoolFor(skillLevel int) []string {
	out := make([]string, 0, len(stm.Beginner)+len(stm.Expert)+len(stm.Master))
	for _, m := range stm.Beginner {
		out = append(out, string(m))
	}
	if skillLevel >= 34 {
		for _, m := range stm.Expert {
			out = append(out, string(m))
		}
	}
	if skillLevel >= 67 {
		for _, m := range stm.Master {
			out = append(out, string(m))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Render renders one coordinated attacker/defender/room triad for a blow whose
// participants share a room.
//
// ALL ROLES COME FROM ONE VARIANT INDEX. The authored pools pair up by index,
// so picking per role narrates three different events to three audiences. That
// is the defect this migration removes, and it shipped twice before: melee
// defence (PR #112) and taunt (PR #115).
//
// The role mapping is the one thing here worth reading slowly. An attacker
// ACTS and a defender is ACTED UPON, so ToAttacker (authored `actor`) is the
// Actor and ToDefender (authored `actee`) is the Actee. Swapping those two
// lines inverts every combat message in the game, and three separately named
// pools becoming adjacent fields of one struct literal is exactly how that
// mistake gets made.
// combat_messages.golden keys its rows by the AUTHORED name, which is what
// catches it.
//
// ActeeObserver is deliberately left empty: when the participants share a room
// there is only one observer audience.
//
// A nil picker means production behaviour (narration.DefaultPicker).
func (m TogetherMessages) Render(skillLevel int, tokenReplacements map[TokenName]string, pick narration.Picker) narration.Roles {
	return narration.Render(
		narration.Variants{
			Actor:    m.ToAttacker.PoolFor(skillLevel),
			Actee:    m.ToDefender.PoolFor(skillLevel),
			Observer: m.ToRoom.PoolFor(skillLevel),
		},
		TokenStrings(tokenReplacements),
		pick,
	)
}

// Render renders one coordinated quartet for a ranged blow where attacker and
// defender are in different rooms, so there are genuinely two observer
// audiences. ActeeObserver is the observers where the DEFENDER is; this is the
// case narration.Roles grew its fourth field for
// (internal/narration/render.go:35-38).
//
// The two room roles are different audiences seeing different things and must
// never be treated as interchangeable.
//
// The lines put the exit after "the" ("from the {entrancename} direction"),
// so each role goes through exit.PhraseVerticalExits and an up or down exit
// reads "from above" or "upward" rather than "from the up" (#430).
//
// A nil picker means production behaviour (narration.DefaultPicker).
func (m SeparateMessages) Render(skillLevel int, tokenReplacements map[TokenName]string, pick narration.Picker) narration.Roles {
	roles := narration.Render(
		narration.Variants{
			Actor:         m.ToAttacker.PoolFor(skillLevel),
			Actee:         m.ToDefender.PoolFor(skillLevel),
			Observer:      m.ToAttackerRoom.PoolFor(skillLevel),
			ActeeObserver: m.ToDefenderRoom.PoolFor(skillLevel),
		},
		TokenStrings(tokenReplacements),
		pick,
	)
	exits := []string{tokenReplacements[TokenExitName], tokenReplacements[TokenEntranceName]}
	roles.Actor = exit.PhraseVerticalExits(roles.Actor, exits...)
	roles.Actee = exit.PhraseVerticalExits(roles.Actee, exits...)
	roles.Observer = exit.PhraseVerticalExits(roles.Observer, exits...)
	roles.ActeeObserver = exit.PhraseVerticalExits(roles.ActeeObserver, exits...)
	return roles
}

// Presumably to ensure the datafile hasn't messed something up.
func (w *WeaponAttackMessageGroup) Id() ItemSubType {
	return w.OptionId
}

// Validate checks that every required intensity is present and that every
// authored group can be rendered from ONE coordinated index.
//
// Equality is checked PER TIER, not on the union totals. The runtime union is
// cumulative (beginner, plus expert at 34, plus master at 67), so equal totals
// with unequal tiers would still pair an expert line against a master one at
// the same index. Per-tier equality is exactly equivalent to union equality at
// all three skill levels.
//
// minVariants is 1, not the defence store's 5: the smallest authored group in
// the shipped store is generic/coupdegrace/separate/beginner at one line, and
// 414 of 534 groups hold fewer than five.
//
// The attack store had none of this until M3 item 8. It checked intensity
// presence and nothing else, while the defence sibling next door
// (defensive_messages.go) has always checked emptiness and equal lengths.
func (w *WeaponAttackMessageGroup) Validate() error {

	// Make sure all important options are present.
	optionsToCheck := []Intensity{Prepare, Wait, Miss, Weak, Normal, Heavy, Critical, Fumble}
	for _, option := range optionsToCheck {
		if _, ok := w.Options[option]; !ok {
			return fmt.Errorf("missing option[`%s`] for %s", option, w.OptionId)
		}
	}

	tiers := []struct {
		name string
		get  func(SkillTieredMessages) MessageOptions
	}{
		{"beginner", func(s SkillTieredMessages) MessageOptions { return s.Beginner }},
		{"expert", func(s SkillTieredMessages) MessageOptions { return s.Expert }},
		{"master", func(s SkillTieredMessages) MessageOptions { return s.Master }},
	}

	// Sorted so a file with several faults always reports the same one first,
	// rather than whichever the map happened to yield.
	intensities := make([]string, 0, len(w.Options))
	for intensity := range w.Options {
		intensities = append(intensities, string(intensity))
	}
	sort.Strings(intensities)

	for _, name := range intensities {
		intensity := Intensity(name)
		opts := w.Options[intensity]

		for _, tier := range tiers {
			together := narration.Variants{
				Actor:    messageStrings(tier.get(opts.Together.ToAttacker)),
				Actee:    messageStrings(tier.get(opts.Together.ToDefender)),
				Observer: messageStrings(tier.get(opts.Together.ToRoom)),
			}
			if anyPool(together) {
				if err := narration.ValidateVariants(together, 1,
					narration.RoleActor, narration.RoleActee, narration.RoleObserver); err != nil {
					return fmt.Errorf("%s option[`%s`].together.%s: %w", w.OptionId, intensity, tier.name, err)
				}
			}

			separate := narration.Variants{
				Actor:         messageStrings(tier.get(opts.Separate.ToAttacker)),
				Actee:         messageStrings(tier.get(opts.Separate.ToDefender)),
				Observer:      messageStrings(tier.get(opts.Separate.ToAttackerRoom)),
				ActeeObserver: messageStrings(tier.get(opts.Separate.ToDefenderRoom)),
			}
			if anyPool(separate) {
				if err := narration.ValidateVariants(separate, 1,
					narration.RoleActor, narration.RoleActee,
					narration.RoleObserver, narration.RoleActeeObserver); err != nil {
					return fmt.Errorf("%s option[`%s`].separate.%s: %w", w.OptionId, intensity, tier.name, err)
				}
			}
		}
	}

	return nil
}

// anyPool reports whether a group was authored at all, so a subtype with no
// separate block is skipped rather than reported as four missing roles.
func anyPool(v narration.Variants) bool {
	return len(v.Actor) > 0 || len(v.Actee) > 0 ||
		len(v.Observer) > 0 || len(v.ActeeObserver) > 0
}

func (w *WeaponAttackMessageGroup) Filepath() string {
	return fmt.Sprintf("%s.yaml", w.OptionId)
}

func GetPreAttackMessage(subType ItemSubType, messageType Intensity) AttackOptions {

	// Check whether this item subtype has any attack messages
	if attackMsgOptions, ok := attackMessages[subType]; ok {
		if attackMsgOptions, ok := attackMsgOptions.Options[messageType]; ok {
			// return a random message
			return attackMsgOptions
		}
	}

	// Fall back to generic, but NEVER recurse into ourselves. If Generic itself
	// lacks the key this would call itself forever and overflow the stack,
	// taking the server down. That was survivable only because generic.yaml
	// happened to define every intensity in use, which is a property of the data
	// rather than of the code. Returning the zero value degrades to no message.
	if subType == Generic {
		return AttackOptions{}
	}

	return GetPreAttackMessage(Generic, messageType)
}

func GetAttackMessage(subType ItemSubType, pctDamage int) AttackOptions {

	// 101 and the zero floor are STRUCTURAL and stay here: combat.attackMessagePct
	// forces a crit to 101 and caps a non-crit at 100, which is what pairs the
	// crit-worded pool with the *** banner, and 0 means nothing landed. Only the
	// two middle cutoffs are authorable.
	balance := configs.GetBalanceConfig()
	var intensity Intensity
	if pctDamage >= 101 {
		intensity = Critical
	} else if pctDamage >= int(balance.AttackBandHeavyThresholdPct) {
		intensity = Heavy
	} else if pctDamage >= int(balance.AttackBandNormalThresholdPct) {
		intensity = Normal
	} else if pctDamage >= 1 {
		intensity = Weak
	} else {
		intensity = Miss
	}

	// Check whether this item subtype has any attack messages
	if attackMsgOptions, ok := attackMessages[subType]; ok {
		if attackMsgOptions, ok := attackMsgOptions.Options[intensity]; ok {
			// return a random message
			return attackMsgOptions
		}
	}
	// Fall back to generic, but NEVER recurse into ourselves -- the same guard
	// GetPreAttackMessage carries above, and for the same reason. If Generic
	// itself lacks the intensity (or attackMessages was never loaded at all)
	// this calls itself forever and overflows the stack, taking the process
	// down. Returning the zero value degrades to no message instead.
	if subType == Generic {
		return AttackOptions{}
	}

	return GetAttackMessage(Generic, pctDamage)
}
