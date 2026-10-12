package items

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

var (
	defenseMessages map[DefencePool]*DefenseMessageGroup = map[DefencePool]*DefenseMessageGroup{}
)

// DefencePool is the KEY of the defense-messages/ store: the five defence
// pools, named from combatvocab.Defence so the files do not move, plus the
// five counter pools. It is not a defence type; that vocabulary lives in
// internal/combatvocab and this package only converts INTO its key.
type DefencePool string

// DefencePoolFor names the pool that narrates a defence. DefenceNone maps to
// the empty pool, which RenderDefenseMessage answers with an empty triad.
func DefencePoolFor(d combatvocab.Defence) DefencePool {
	return DefencePool(d)
}

// CounterPoolFor names the pool that narrates the counter EARNED by a
// defensive crit on d: the defence's own name under a counter- prefix, so a
// parry crit reads as a parry answered, a block crit as a block answered.
// DefenceNone maps to the empty pool, which RenderDefenseMessage answers
// with an empty triad; internal/combat then falls back to its generic
// counter lines, so the tier never goes silent.
func CounterPoolFor(d combatvocab.Defence) DefencePool {
	if d == combatvocab.DefenceNone {
		return ""
	}
	return DefencePool("counter-" + string(d))
}

const (
	// Counter-narration pools (U6b Task 11, re-keyed by the counters slice).
	// Not defences: each is the narration for the counter EARNED by a
	// defensive crit, named after the defence that won it, which is what
	// CounterPoolFor computes. They ride the same loader, shape and
	// validator as the defence pools. Band semantics differ: weak = the
	// counter is turned aside (no damage), normal = the counter lands,
	// heavy = the counter crits (or, for defy, the retort fails, lands,
	// crits).
	CounterPoolDodge DefencePool = "counter-dodge"
	CounterPoolParry DefencePool = "counter-parry"
	CounterPoolBlock DefencePool = "counter-block"
	CounterPoolQuell DefencePool = "counter-quell"
	CounterPoolDefy  DefencePool = "counter-defy"

	// CounterPoolDefyWordless is the wordless twin of CounterPoolDefy (M6
	// slice 3): the retort of a counterer with no voice for words. Not a
	// CounterPoolFor result; combat.BuildCounterTauntMessages picks it for a
	// speechless counterer and falls back to CounterPoolDefy without it.
	CounterPoolDefyWordless DefencePool = "counter-defy-wordless"
)

type DefenseMessageGroup struct {
	OptionId DefencePool      `yaml:"optionid"`
	Options  DefenseIntensity `yaml:"options"`
	// Deflected is the optional pool for a swing the defence won but that
	// still did damage. A group may carry it beside its bands, or carry only
	// it (the generic fallback group, DefencePoolDeflected).
	Deflected *DeflectedPool `yaml:"deflected,omitempty"`
}

// DefencePoolDeflected is the key of the generic deflected-swing group: a
// group with no bands, only a Deflected pool, used when the defence that won
// has none of its own.
const DefencePoolDeflected DefencePool = "deflected"

// minDeflectedVariants is the fewest pairs a deflected pool may hold.
const minDeflectedVariants = 4

// DeflectedPool is the authored shape of a deflected-swing pool: the attacker
// line and the defender line, paired by index.
type DeflectedPool struct {
	Actor []string `yaml:"actor"`
	Actee []string `yaml:"actee"`
}

func (p *DeflectedPool) validate(id DefencePool) error {
	if len(p.Actor) != len(p.Actee) {
		return fmt.Errorf("deflected actor and actee lists for %s must have equal lengths", id)
	}
	if len(p.Actor) < minDeflectedVariants {
		return fmt.Errorf("deflected pool for %s must contain at least %d variants", id, minDeflectedVariants)
	}
	for i := range p.Actor {
		if strings.TrimSpace(p.Actor[i]) == "" || strings.TrimSpace(p.Actee[i]) == "" {
			return fmt.Errorf("deflected[%d] for %s must be non-empty", i, id)
		}
	}
	return nil
}

// RenderDeflectedSwing renders the attacker and defender lines for a swing the
// defence won but that still did damage, both from one variant index. It reads
// the defence's own deflected pool and falls back to the generic group; ok is
// false when neither is loaded, and the caller keeps its own literal text. An
// optional index is accepted for deterministic tests.
func RenderDeflectedSwing(d combatvocab.Defence, tokenReplacements map[TokenName]string, indexOverride ...int) (toAttacker, toDefender ItemMessage, ok bool) {
	var pool *DeflectedPool
	if g := defenseMessages[DefencePoolFor(d)]; g != nil && d != combatvocab.DefenceNone {
		pool = g.Deflected
	}
	if pool == nil {
		if g := defenseMessages[DefencePoolDeflected]; g != nil {
			pool = g.Deflected
		}
	}
	if pool == nil {
		return "", "", false
	}
	roles := narration.Render(
		narration.Variants{Actor: pool.Actor, Actee: pool.Actee},
		TokenStrings(tokenReplacements),
		nil,
		indexOverride...,
	)
	if roles.Actor == "" || roles.Actee == "" {
		return "", "", false
	}
	return ItemMessage(roles.Actor), ItemMessage(roles.Actee), true
}

type DefenseIntensity map[Intensity]DefenseOptions

type DefenseOptions struct {
	Together DefenseTogetherMessages `yaml:"together"`
}

// DefenseTogetherMessages is the authored shape of one defence band.
//
// The keys are the canonical role vocabulary (M4b-1). They were spelled
// todefender/toattacker/toroom until then; the Go field names still carry the
// old spelling, which is cosmetic and left for a later pass.
type DefenseTogetherMessages struct {
	ToDefender MessageOptions `yaml:"actee"`
	ToAttacker MessageOptions `yaml:"actor"`
	ToRoom     MessageOptions `yaml:"observer"`
}

// DefenseMessageTriad is one coordinated event rendered for its three
// audiences. All fields always come from the same variant index.
type DefenseMessageTriad struct {
	ToDefender ItemMessage
	ToAttacker ItemMessage
	ToRoom     ItemMessage
}

// Presumably to ensure the datafile hasn't messed something up.
func (d *DefenseMessageGroup) Id() DefencePool {
	return d.OptionId
}

// Presumably to ensure the datafile hasn't messed something up.
func (d *DefenseMessageGroup) Validate() error {

	if d.Deflected != nil {
		if err := d.Deflected.validate(d.OptionId); err != nil {
			return err
		}
		// A group that is only a deflected pool (the generic fallback) has no
		// bands to check.
		if len(d.Options) == 0 {
			return nil
		}
	}

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
			// The names are the AUTHORED keys, so the error points an author
			// straight at the line to fix. M4b-1 renamed them from
			// todefender/toattacker/toroom.
			{"actee", defenseOptions.Together.ToDefender},
			{"actor", defenseOptions.Together.ToAttacker},
			{"observer", defenseOptions.Together.ToRoom},
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
func RenderDefenseMessage(defenseType DefencePool, defensiveCrit bool, normalizedDefenceMargin float64, tokenReplacements map[TokenName]string, indexOverride ...int) DefenseMessageTriad {
	return renderDefenseMessage(defenseType, defensiveCrit, normalizedDefenceMargin, tokenReplacements, false, indexOverride...)
}

// RenderMoveDefenseMessage is RenderDefenseMessage for an {attack} that names
// a move rather than an item: the unarmed "strike", a kick, an aimed shot, a
// spell. The authored lines tag {attack} and {weapon} as an item, which is
// right for an armed swing; for a move that tag would make the sight pipeline
// read the move as a weapon (spec F2: "an unarmed name is not an item"), so
// this strips the item tag from around those two tokens before substitution.
func RenderMoveDefenseMessage(defenseType DefencePool, defensiveCrit bool, normalizedDefenceMargin float64, tokenReplacements map[TokenName]string, indexOverride ...int) DefenseMessageTriad {
	return renderDefenseMessage(defenseType, defensiveCrit, normalizedDefenceMargin, tokenReplacements, true, indexOverride...)
}

func renderDefenseMessage(defenseType DefencePool, defensiveCrit bool, normalizedDefenceMargin float64, tokenReplacements map[TokenName]string, moveNamed bool, indexOverride ...int) DefenseMessageTriad {
	intensity := Weak
	if defensiveCrit {
		intensity = Heavy
	} else if normalizedDefenceMargin >= float64(configs.GetBalanceConfig().DefenceBandNormalThreshold) {
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
	if moveNamed {
		options = options.untagged(TokenAttack, TokenWeapon)
	}

	return options.RenderTriad(tokenReplacements, nil, indexOverride...)
}

// untagged returns a copy of o whose lines carry tokens bare, without the
// item colour tag the authored lines wrap them in. The stored pools are never
// edited.
func (o DefenseOptions) untagged(tokens ...TokenName) DefenseOptions {
	strip := func(pool MessageOptions) MessageOptions {
		if len(pool) == 0 {
			return pool
		}
		out := make(MessageOptions, len(pool))
		for i, m := range pool {
			out[i] = ItemMessage(UntagItemTokens(string(m), tokens...))
		}
		return out
	}
	o.Together = DefenseTogetherMessages{
		ToDefender: strip(o.Together.ToDefender),
		ToAttacker: strip(o.Together.ToAttacker),
		ToRoom:     strip(o.Together.ToRoom),
	}
	return o
}

// UntagItemTokens rewrites each `<ansi fg="item">{token}</ansi>` in template
// as a bare `{token}`, for a token that holds a move or body part rather than
// an item. messaging.HideWeapons reads every item-tagged name as a weapon, so
// a non-item must not wear the tag.
func UntagItemTokens(template string, tokens ...TokenName) string {
	for _, tok := range tokens {
		template = strings.ReplaceAll(template, `<ansi fg="item">`+string(tok)+`</ansi>`, string(tok))
	}
	return template
}

// RenderTriad renders one coordinated defender/attacker/room triad from an
// already-selected band. ALL THREE ROLES COME FROM THE SAME VARIANT INDEX:
// the authored pools pair up by index, so picking per role produces three
// descriptions of three different events.
//
// A nil picker means production behaviour (narration.DefaultPicker).
// The coordination itself now lives in narration.Render. What remains here is
// the ADAPTER, and its role mapping is the one thing in this file worth
// reading slowly: an attacker ACTS and a defender is ACTED UPON, so ToAttacker
// (authored `actor`) is the Actor and ToDefender (authored `actee`) is the
// Actee. Swapping those two lines would invert every defence message in the
// game, and it is exactly the mistake this refactor makes easiest, because
// three separately named pools become adjacent fields of one literal differing
// only by role.
//
// defense_messages.golden is what catches it: that file keys its rows by the
// AUTHORED name, so a swap puts the attacker's sentence in an actee row.
func (o DefenseOptions) RenderTriad(tokenReplacements map[TokenName]string, pick narration.Picker, indexOverride ...int) DefenseMessageTriad {
	roles := narration.Render(
		narration.Variants{
			Actor:    messageStrings(o.Together.ToAttacker),
			Actee:    messageStrings(o.Together.ToDefender),
			Observer: messageStrings(o.Together.ToRoom),
		},
		TokenStrings(tokenReplacements),
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

// TokenStrings converts a token map to the core's plain-string form, which is
// what narration.Substitute and narration.Render take. Exported in M4a so
// internal/combat's non-pool paths (the opening-strike/deflected fallback and
// the feint lines) substitute through the same door as everything else,
// instead of through a per-token strings.Replace loop of their own.
func TokenStrings(tokens map[TokenName]string) map[string]string {
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
