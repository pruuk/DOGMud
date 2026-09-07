package items

import (
	"fmt"
	"strings"

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

type TogetherMessages struct {
	ToAttacker SkillTieredMessages `yaml:"toattacker"`
	ToDefender SkillTieredMessages `yaml:"todefender"`
	ToRoom     SkillTieredMessages `yaml:"toroom"`
}

type SeparateMessages struct {
	ToAttacker     SkillTieredMessages `yaml:"toattacker"`
	ToDefender     SkillTieredMessages `yaml:"todefender"`
	ToAttackerRoom SkillTieredMessages `yaml:"toattackerroom"`
	ToDefenderRoom SkillTieredMessages `yaml:"todefenderroom"`
}

type SkillTieredMessages struct {
	Beginner MessageOptions `yaml:"beginner"`
	Expert   MessageOptions `yaml:"expert,omitempty"`
	Master   MessageOptions `yaml:"master,omitempty"`
}

type MessageOptions []ItemMessage

func (am ItemMessage) SetTokenValue(tokenName TokenName, tokenValue string) ItemMessage {
	return ItemMessage(strings.Replace(string(am), string(tokenName), tokenValue, -1))
}

// Get chooses a message using the default picker (narration.DefaultPicker,
// which routes through util.Rand).
func (mo MessageOptions) Get(seedNum ...int) ItemMessage {
	return mo.GetWith(nil, seedNum...)
}

// GetWith is Get with an explicit picker, for the snapshot harness. A nil
// picker means production behaviour: narration.DefaultPicker, i.e. util.Rand.
//
// The picker is consulted only in the no-seed branch, where a random pick
// used to happen directly. seedNum's explicit-index-override behaviour is
// untouched.
func (mo MessageOptions) GetWith(pick narration.Picker, seedNum ...int) ItemMessage {
	if pick == nil {
		pick = narration.DefaultPicker
	}

	if ct := len(mo); ct > 0 {

		if len(seedNum) == 0 || seedNum[0] == 0 {
			return mo[pick(ct)]
		}

		if seedNum[0] == 0 {
			return mo[0]
		}

		return mo[seedNum[0]%len(mo)]
	}

	return ItemMessage("")
}

// GetForSkillLevel selects a message based on character's skill level, using
// the default picker (narration.DefaultPicker, which routes through
// util.Rand). Returns messages from available tiers based on skill:
//   - Skill 1-33: beginner only
//   - Skill 34-66: beginner + expert
//   - Skill 67-100: beginner + expert + master
func (stm SkillTieredMessages) GetForSkillLevel(skillLevel int, msgSeed ...int) ItemMessage {
	return stm.GetForSkillLevelWith(nil, skillLevel, msgSeed...)
}

// GetForSkillLevelWith is GetForSkillLevel with an explicit picker, for the
// snapshot harness. A nil picker means production behaviour:
// narration.DefaultPicker, i.e. util.Rand.
//
// This is the store the core combat loop calls for all three viewpoints at
// once (ToAttacker, ToDefender, ToRoom), so it is the store that most needed
// a picker reachable without mutating shared state.
//
// The picker is consulted only in the no-seed branch, where a random pick
// used to happen directly. msgSeed's explicit-index-override behaviour is
// untouched.
func (stm SkillTieredMessages) GetForSkillLevelWith(pick narration.Picker, skillLevel int, msgSeed ...int) ItemMessage {
	if pick == nil {
		pick = narration.DefaultPicker
	}

	// Collect available message pools
	var allMessages []ItemMessage

	// Always include beginner messages
	allMessages = append(allMessages, stm.Beginner...)

	// Add expert if skill >= 34
	if skillLevel >= 34 {
		allMessages = append(allMessages, stm.Expert...)
	}

	// Add master if skill >= 67
	if skillLevel >= 67 {
		allMessages = append(allMessages, stm.Master...)
	}

	// Select from combined pool
	if len(allMessages) == 0 {
		return ItemMessage("")
	}

	// Use seed if provided
	if len(msgSeed) > 0 && msgSeed[0] != 0 {
		return allMessages[msgSeed[0]%len(allMessages)]
	}

	return allMessages[pick(len(allMessages))]
}

// Presumably to ensure the datafile hasn't messed something up.
func (w *WeaponAttackMessageGroup) Id() ItemSubType {
	return w.OptionId
}

// Presumably to ensure the datafile hasn't messed something up.
func (w *WeaponAttackMessageGroup) Validate() error {

	// Make sure all important options are present.
	optionsToCheck := []Intensity{Prepare, Wait, Miss, Weak, Normal, Heavy, Critical, Fumble}
	for _, option := range optionsToCheck {
		if _, ok := w.Options[option]; !ok {
			return fmt.Errorf("missing option[`%s`] for %s", option, w.OptionId)
		}
	}

	return nil
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

	var intensity Intensity
	if pctDamage >= 101 {
		intensity = Critical
	} else if pctDamage >= 75 {
		intensity = Heavy
	} else if pctDamage >= 30 {
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
