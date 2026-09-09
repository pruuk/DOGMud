package combat

import (
	"fmt"
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

// Validate enforces the contract a coordinated triad depends on, by handing
// each band to the shared primitive.
//
// The equal-length rule is the important one, and it is the rule this store
// went without until 2026-09-09: variant N of toattacker, todefender and toroom
// describe the SAME moment, so a short pool means some indices have no line for
// that audience. The shipped data had exactly that, 8/8/6 in the hit and miss
// bands, so a coordinated index of 6 or 7 left the room silent.
//
// The three roles are DECLARED rather than inferred. Without that,
// ValidateVariants cannot tell a band that never had a toroom pool from one
// that lost it, since both are an empty slice, and this store would boot
// happily while narrating a real taunt to two audiences and silence to the
// third.
func (t *TauntMessageGroup) Validate() error {
	required := []TauntIntensity{TauntHit, TauntMiss, TauntCritical, TauntFumble}
	for _, r := range required {
		msgs, ok := t.Options[r]
		if !ok || msgs == nil {
			return fmt.Errorf("taunt messages %q: missing intensity %q", t.OptionId, r)
		}
		if err := narration.ValidateVariants(msgs.variants(), minTauntVariants,
			narration.RoleActor, narration.RoleActee, narration.RoleObserver); err != nil {
			return fmt.Errorf("taunt messages %q: option[%q]: %w", t.OptionId, r, err)
		}
	}
	return nil
}

// minTauntVariants matches defence's floor. The shipped data carries 6 to 8
// per pool, so this catches a pool being gutted rather than constraining an
// author.
const minTauntVariants = 5

// variants maps the AUTHORED role names onto the core's vocabulary.
//
// This is the one line in this file worth reading slowly, and it is the same
// aliasing the defence store does: a taunter ACTS and a target is ACTED UPON,
// so toattacker is the Actor and todefender is the Actee. Swapping them would
// invert every taunt in the game, and taunt_messages.golden catches it because
// that file keys its rows by the AUTHORED name.
func (m *TauntMessages) variants() narration.Variants {
	return narration.Variants{
		Actor:    m.ToAttacker,
		Actee:    m.ToDefender,
		Observer: m.ToRoom,
	}
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
	group := tauntMessages["rhetoric"]
	if group == nil {
		return TauntTriad{}
	}

	msgs, ok := group.Options[intensity]
	if !ok || msgs == nil {
		return TauntTriad{}
	}

	var pick narration.Picker
	if len(picker) > 0 {
		pick = picker[0]
	}

	// The authored token vocabulary is ALIASED, not rewritten. rhetoric.yaml
	// says {source} and {target} where the defence store says {attacker} and
	// {defender}; both mean the same two people. Resolving that here rather
	// than in the core is what lets the file stay as its author wrote it.
	roles := narration.Render(msgs.variants(), map[string]string{
		"{source}":     source,
		"{target}":     target,
		"{sourcetype}": sourceType,
		"{targettype}": targetType,
		"{damage}":     damageDesc,
	}, pick)

	// ALL THREE OR NOTHING. The old hand-rolled renderer required every pool
	// non-empty and equal before it would render, and callers depend on that:
	// usercommands/taunt.go treats an empty ToAttacker as "the store said
	// nothing" and falls back to its literals, and mobcommands does the same
	// with ToRoom. A partial triad would satisfy neither sentinel while
	// narrating to some audiences and not others, which is the defect this
	// whole arc exists to prevent.
	//
	// Validate makes this unreachable through loaded world data. It is
	// reachable through SeedTauntMessagesForTest, which is exactly the sort of
	// bypass a future test will use.
	if roles.Actor == "" || roles.Actee == "" || roles.Observer == "" {
		return TauntTriad{}
	}

	return TauntTriad{
		ToAttacker: roles.Actor,
		ToDefender: roles.Actee,
		ToRoom:     roles.Observer,
	}
}

// SeedTauntMessagesForTest swaps the store for a test-supplied set of bands,
// returning a restore func for the caller to defer. Mirrors
// itemvoices.SeedVoicesForTest.
//
// It exists so tests OUTSIDE this package can exercise taunt rendering without
// loading world data, which is what lets internal/usercommands assert that a
// rendered taunt line is anonymizable.
func SeedTauntMessagesForTest(bands map[TauntIntensity]*TauntMessages) func() {
	prev := tauntMessages
	tauntMessages = map[string]*TauntMessageGroup{
		"rhetoric": {OptionId: "rhetoric", Options: bands},
	}
	return func() { tauntMessages = prev }
}
