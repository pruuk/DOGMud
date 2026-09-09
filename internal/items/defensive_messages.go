package items

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

var (
	defenseMessages map[DefenseType]*DefenseMessageGroup = map[DefenseType]*DefenseMessageGroup{}
)

// DefenseType identifies the type of defensive action
type DefenseType string

const (
	DefenseDodge DefenseType = "dodge"
	DefenseParry DefenseType = "parry"
	DefenseBlock DefenseType = "block"
	DefenseQuell DefenseType = "quell"
	DefenseDefy  DefenseType = "defy"

	// Counter-narration pools (U6b Task 11). Not defences themselves: each is
	// the channel-correct narration for the counter EARNED by a defensive
	// crit on that channel. They ride the same loader, shape, and validator
	// as the defence pools. Band semantics differ from the defence pools:
	// weak = the counter is turned aside (no damage), normal = the counter
	// lands, heavy = the counter crits.
	DefenseCounterMelee  DefenseType = "counter-melee"
	DefenseCounterRanged DefenseType = "counter-ranged"
	DefenseCounterQuell  DefenseType = "counter-quell"
	DefenseCounterDefy   DefenseType = "counter-defy"
)

type DefenseMessageGroup struct {
	OptionId DefenseType      `yaml:"optionid"`
	Options  DefenseIntensity `yaml:"options"`
}

type DefenseIntensity map[Intensity]DefenseOptions

type DefenseOptions struct {
	Together DefenseTogetherMessages `yaml:"together"`
}

type DefenseTogetherMessages struct {
	ToDefender MessageOptions `yaml:"todefender"`
	ToAttacker MessageOptions `yaml:"toattacker"`
	ToRoom     MessageOptions `yaml:"toroom"`
}

// DefenseMessageTriad is one coordinated event rendered for its three
// audiences. All fields always come from the same variant index.
type DefenseMessageTriad struct {
	ToDefender ItemMessage
	ToAttacker ItemMessage
	ToRoom     ItemMessage
}

// Presumably to ensure the datafile hasn't messed something up.
func (d *DefenseMessageGroup) Id() DefenseType {
	return d.OptionId
}

// Presumably to ensure the datafile hasn't messed something up.
func (d *DefenseMessageGroup) Validate() error {

	// Make sure all important options are present.
	optionsToCheck := []Intensity{Weak, Normal, Heavy}
	for _, option := range optionsToCheck {
		defenseOptions, ok := d.Options[option]
		if !ok {
			return fmt.Errorf("missing option[`%s`] for %s", option, d.OptionId)
		}
		audiences := []struct {
			name     string
			messages MessageOptions
		}{
			{"todefender", defenseOptions.Together.ToDefender},
			{"toattacker", defenseOptions.Together.ToAttacker},
			{"toroom", defenseOptions.Together.ToRoom},
		}
		for _, audience := range audiences {
			if len(audience.messages) < 5 {
				return fmt.Errorf("option[`%s`].%s for %s must contain at least 5 variants", option, audience.name, d.OptionId)
			}
			for index, message := range audience.messages {
				if strings.TrimSpace(string(message)) == "" {
					return fmt.Errorf("option[`%s`].%s[%d] for %s must be non-empty", option, audience.name, index, d.OptionId)
				}
			}
		}
		if len(defenseOptions.Together.ToDefender) != len(defenseOptions.Together.ToAttacker) ||
			len(defenseOptions.Together.ToDefender) != len(defenseOptions.Together.ToRoom) {
			return fmt.Errorf("option[`%s`] audience lists for %s must have equal lengths", option, d.OptionId)
		}
	}

	return nil
}

// RenderDefenseMessage chooses an outcome-appropriate band and renders one
// coordinated defender/attacker/room triad. Defensive crits alone use Heavy;
// ordinary defensive wins cap at Normal because they still let an effect
// through. An optional index is accepted for deterministic tests.
func RenderDefenseMessage(defenseType DefenseType, defensiveCrit bool, normalizedDefenceMargin float64, tokenReplacements map[TokenName]string, indexOverride ...int) DefenseMessageTriad {
	intensity := Weak
	if defensiveCrit {
		intensity = Heavy
	} else if normalizedDefenceMargin >= 0.5 {
		intensity = Normal
	}

	group := defenseMessages[defenseType]
	if group == nil {
		return DefenseMessageTriad{}
	}
	options, ok := group.Options[intensity]
	if !ok {
		return DefenseMessageTriad{}
	}

	return options.RenderTriad(tokenReplacements, nil, indexOverride...)
}

// RenderTriad renders one coordinated defender/attacker/room triad from an
// already-selected band. ALL THREE ROLES COME FROM THE SAME VARIANT INDEX:
// the authored pools pair up by index, so picking per role produces three
// descriptions of three different events.
//
// A nil picker means production behaviour (narration.DefaultPicker).
// The coordination itself now lives in narration.Render. What remains here is
// the ADAPTER, and its role mapping is the one thing in this file worth
// reading slowly: an attacker ACTS and a defender is ACTED UPON, so toattacker
// is the Actor and todefender is the Actee. Swapping those two lines would
// invert every defence message in the game, and it is exactly the mistake this
// refactor makes easiest, because three separately named pools become adjacent
// fields of one literal differing only by role.
//
// defense_messages.golden is what catches it: that file keys its rows by the
// AUTHORED name, so a swap puts the attacker's sentence in a todefender row.
func (o DefenseOptions) RenderTriad(tokenReplacements map[TokenName]string, pick narration.Picker, indexOverride ...int) DefenseMessageTriad {
	roles := narration.Render(
		narration.Variants{
			Actor:    messageStrings(o.Together.ToAttacker),
			Actee:    messageStrings(o.Together.ToDefender),
			Observer: messageStrings(o.Together.ToRoom),
		},
		tokenStrings(tokenReplacements),
		pick,
		indexOverride...,
	)

	return DefenseMessageTriad{
		ToAttacker: ItemMessage(roles.Actor),
		ToDefender: ItemMessage(roles.Actee),
		ToRoom:     ItemMessage(roles.Observer),
	}
}

// messageStrings converts an authored pool to the core's plain-string form.
func messageStrings(pool MessageOptions) []string {
	if len(pool) == 0 {
		return nil
	}
	out := make([]string, len(pool))
	for i, m := range pool {
		out[i] = string(m)
	}
	return out
}

// tokenStrings converts a token map to the core's plain-string form.
func tokenStrings(tokens map[TokenName]string) map[string]string {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]string, len(tokens))
	for name, value := range tokens {
		out[string(name)] = value
	}
	return out
}

func (d *DefenseMessageGroup) Filepath() string {
	return fmt.Sprintf("%s.yaml", d.OptionId)
}

// GetDefenseMessage returns the appropriate defense message based on defense type and intensity
func GetDefenseMessage(defenseType DefenseType, zScore float64) DefenseOptions {

	var intensity Intensity
	// Map z-score to intensity:
	// High z-score = easy defense (opponent barely came close)
	// Low z-score = narrow defense (opponent almost hit)
	if zScore >= 2.0 {
		intensity = Heavy // Easy/decisive defense
	} else if zScore >= 0.5 {
		intensity = Normal // Standard defense
	} else {
		intensity = Weak // Narrow/close defense
	}

	// Check whether this defense type has any messages
	if defenseMsgOptions, ok := defenseMessages[defenseType]; ok {
		if defenseMsgOptions, ok := defenseMsgOptions.Options[intensity]; ok {
			return defenseMsgOptions
		}
	}

	// Return empty if not found (caller should fallback to generic messages)
	return DefenseOptions{}
}
