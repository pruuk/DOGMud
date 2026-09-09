package combat

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// TauntIntensity represents the outcome type of a taunt attempt.
type TauntIntensity string

const (
	TauntHit      TauntIntensity = "hit"
	TauntMiss     TauntIntensity = "miss"
	TauntCritical TauntIntensity = "critical"
	TauntFumble   TauntIntensity = "fumble"
)

// TauntMessages holds messages for a single intensity level.
type TauntMessages struct {
	ToAttacker []string `yaml:"toattacker"`
	ToDefender []string `yaml:"todefender"`
	ToRoom     []string `yaml:"toroom"`
}

// TauntMessageGroup is the top-level YAML structure for taunt messages.
type TauntMessageGroup struct {
	OptionId string                            `yaml:"optionid"`
	Options  map[TauntIntensity]*TauntMessages `yaml:"options"`
}

func (t *TauntMessageGroup) Id() string       { return t.OptionId }
func (t *TauntMessageGroup) Filepath() string { return fmt.Sprintf("%s.yaml", t.OptionId) }

// Validate enforces the contract a coordinated triad depends on.
//
// The equal-length rule is the important one, and it is the rule this store
// went without until 2026-09-09: variant N of toattacker, todefender and toroom
// describe the SAME moment, so a short pool means some indices have no line for
// that audience. The shipped data had exactly that, 8/8/6 in the hit and miss
// bands, so a coordinated index of 6 or 7 left the room silent. This mirrors
// the validator items.DefenseMessageGroup has had all along.
func (t *TauntMessageGroup) Validate() error {
	required := []TauntIntensity{TauntHit, TauntMiss, TauntCritical, TauntFumble}
	for _, r := range required {
		msgs, ok := t.Options[r]
		if !ok || msgs == nil {
			return fmt.Errorf("taunt messages %q: missing intensity %q", t.OptionId, r)
		}

		roles := []struct {
			name string
			pool []string
		}{
			{"toattacker", msgs.ToAttacker},
			{"todefender", msgs.ToDefender},
			{"toroom", msgs.ToRoom},
		}
		for _, role := range roles {
			if len(role.pool) < 5 {
				return fmt.Errorf("taunt messages %q: option[%q].%s must contain at least 5 variants, has %d",
					t.OptionId, r, role.name, len(role.pool))
			}
			for i, m := range role.pool {
				if strings.TrimSpace(m) == "" {
					return fmt.Errorf("taunt messages %q: option[%q].%s[%d] must be non-empty",
						t.OptionId, r, role.name, i)
				}
			}
		}

		if len(msgs.ToAttacker) != len(msgs.ToDefender) || len(msgs.ToAttacker) != len(msgs.ToRoom) {
			return fmt.Errorf("taunt messages %q: option[%q] audience lists must have equal lengths (toattacker=%d todefender=%d toroom=%d); variant N of each describes the SAME moment",
				t.OptionId, r, len(msgs.ToAttacker), len(msgs.ToDefender), len(msgs.ToRoom))
		}
	}
	return nil
}

var tauntMessages map[string]*TauntMessageGroup

// LoadTauntMessageFiles reads taunt message YAMLs from the data directory.
func LoadTauntMessageFiles() {
	start := time.Now()

	tmpAll, err := fileloader.LoadAllFlatFiles[string, *TauntMessageGroup](
		string(configs.GetFilePathsConfig().DataFiles) + `/taunt-messages`,
	)
	if err != nil {
		mudlog.Error("combat.LoadTauntMessageFiles()", "error", err)
		tauntMessages = make(map[string]*TauntMessageGroup)
		return
	}

	tauntMessages = tmpAll
	mudlog.Info("combat.LoadTauntMessageFiles()", "loadedCount", len(tauntMessages), "Time Taken", time.Since(start))
}

// TauntTriad is one taunt as its three audiences are told it. All three fields
// come from the SAME variant index.
type TauntTriad struct {
	ToAttacker string
	ToDefender string
	ToRoom     string
}

// GetTauntTriad renders one taunt for all three audiences from a single
// coordinated variant index.
//
// ONE INDEX FOR ALL THREE ROLES IS THE ENTIRE POINT. Until 2026-09-09 the
// caller in usercommands/taunt.go called a per-perspective getter three times,
// so the attacker, the defender and the room each drew an INDEPENDENT random
// index and were narrated three different moments of a single taunt. That is
// the same defect PR #112 fixed for melee defence, and the equal-length rule
// Validate now enforces is what keeps every index paired.
//
// Returns a zero TauntTriad when the store holds nothing for this intensity.
// usercommands/taunt.go detects that by testing ToAttacker for emptiness.
//
// The optional trailing picker overrides selection; production passes none and
// gets narration.DefaultPicker, which routes through the engine's util.Rand
// seam.
func GetTauntTriad(intensity TauntIntensity, source, target, sourceType, targetType, damageDesc string, picker ...narration.Picker) TauntTriad {
	pick := narration.DefaultPicker
	if len(picker) > 0 && picker[0] != nil {
		pick = picker[0]
	}

	group := tauntMessages["rhetoric"]
	if group == nil {
		return TauntTriad{}
	}

	msgs, ok := group.Options[intensity]
	if !ok || msgs == nil {
		return TauntTriad{}
	}

	if len(msgs.ToAttacker) == 0 {
		return TauntTriad{}
	}
	idx := pick(len(msgs.ToAttacker))

	// Validate guarantees the three pools are equal length, so the bounds
	// check only fires for a store seeded directly by a test.
	at := func(pool []string) string {
		if idx >= len(pool) {
			return ""
		}
		return replaceTauntTokens(pool[idx], source, target, sourceType, targetType, damageDesc)
	}

	return TauntTriad{
		ToAttacker: at(msgs.ToAttacker),
		ToDefender: at(msgs.ToDefender),
		ToRoom:     at(msgs.ToRoom),
	}
}

// replaceTauntTokens substitutes the five authored tokens in one pass.
//
// A single Replacer rather than five sequential ReplaceAll calls, so a value
// that happens to contain a token spelling cannot be substituted a second time
// by a later pass. No shipped name can currently do that; the one-pass form
// means none ever will.
func replaceTauntTokens(msg, source, target, sourceType, targetType, damageDesc string) string {
	return strings.NewReplacer(
		"{source}", source,
		"{target}", target,
		"{sourcetype}", sourceType,
		"{targettype}", targetType,
		"{damage}", damageDesc,
	).Replace(msg)
}
