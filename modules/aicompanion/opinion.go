package aicompanion

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// Opinion is the companion's standing view of its owner, each dimension on
// a bounded -100..+100 scale. Scores are never shown to players; the model
// only ever sees them as words (opinionWords).
type Opinion struct {
	Trust     int `yaml:"trust"`
	Respect   int `yaml:"respect"`
	Affection int `yaml:"affection"`
}

func (o Opinion) clamped() Opinion {
	return Opinion{
		Trust:     clampInt(o.Trust, -100, 100),
		Respect:   clampInt(o.Respect, -100, 100),
		Affection: clampInt(o.Affection, -100, 100),
	}
}

// OpinionChange is one audited change (F5.10): what caused it, the delta
// actually applied, and the result.
type OpinionChange struct {
	Unix      int64   `yaml:"t"`
	Trigger   string  `yaml:"trigger"`
	Source    string  `yaml:"source"` // model, rule
	Delta     Opinion `yaml:"delta"`
	After     Opinion `yaml:"after"`
	Reason    string  `yaml:"reason,omitempty"`
	FromWords bool    `yaml:"from_words,omitempty"` // conversation rather than deeds
}

// envelope is the most one decision may move each dimension, in either
// direction. The model proposes; code decides how much of it lands (F5.4).
type envelope struct {
	Trust     int
	Respect   int
	Affection int
	// NoGain forbids any positive trust or affection change, for decisions
	// triggered by the owner hurting the companion.
	NoGain bool
}

// envelopes per stimulus kind. Words move opinion a little; deeds more.
var envelopes = map[string]envelope{
	`heard`:         {Trust: 2, Respect: 2, Affection: 2},
	`asked`:         {Trust: 2, Respect: 2, Affection: 2},
	`emote`:         {Trust: 3, Respect: 2, Affection: 3},
	`gift`:          {Trust: 3, Respect: 2, Affection: 5},
	`attacked`:      {Trust: 15, Respect: 5, Affection: 15, NoGain: true},
	`first_meeting`: {Trust: 1, Respect: 1, Affection: 1},
	`session_start`: {Trust: 1, Respect: 1, Affection: 1},
	`farewell`:      {Trust: 1, Respect: 1, Affection: 1},
	`recovered`:     {Trust: 1, Respect: 1, Affection: 1},
	`quiet`:         {Trust: 1, Respect: 1, Affection: 1},
	`healed`:        {Trust: 2, Respect: 1, Affection: 3},
	`party`:         {Trust: 1, Respect: 1, Affection: 1},
	`fight_over`:    {Trust: 2, Respect: 3, Affection: 2},
	`romance`:       {Trust: 2, Respect: 1, Affection: 3},
	`romance_yes`:   {Trust: 4, Respect: 2, Affection: 6},
	`romance_no`:    {Trust: 1, Respect: 1, Affection: 4},
	`night`:         {Trust: 3, Respect: 1, Affection: 5},
	`witnessed`:     {Trust: 6, Respect: 8, Affection: 6, NoGain: true},
	`ailing`:        {Trust: 1, Respect: 1, Affection: 1},
	`trouble`:       {Trust: 1, Respect: 1, Affection: 1},
}

// envelopeFor combines the envelopes of every stimulus that came from the
// owner. Stimuli from other people never move the owner opinion.
func envelopeFor(stims []stimulus) (envelope, bool) {
	var out envelope
	found := false
	for _, s := range stims {
		if !s.FromOwner {
			continue
		}
		e, ok := envelopes[s.Kind]
		if !ok {
			continue
		}
		found = true
		if e.Trust > out.Trust {
			out.Trust = e.Trust
		}
		if e.Respect > out.Respect {
			out.Respect = e.Respect
		}
		if e.Affection > out.Affection {
			out.Affection = e.Affection
		}
		if e.NoGain {
			out.NoGain = true
		}
	}
	return out, found
}

// wordsOnly reports whether every owner stimulus was talk rather than deeds.
func wordsOnly(stims []stimulus) bool {
	for _, s := range stims {
		if !s.FromOwner {
			continue
		}
		switch s.Kind {
		case `heard`, `asked`, `emote`, `quiet`, `party`:
		default:
			return false
		}
	}
	return true
}

// boundDelta applies the envelope, the personality sensitivity and the
// diminishing return on repeated praise (F5.5) to a proposed delta.
func boundDelta(proposed Opinion, env envelope, sens Sensitivity, positiveWordsRecently int) Opinion {
	scale := func(v int) int {
		f := float64(v)
		if v > 0 {
			f *= sens.Positive
		} else if v < 0 {
			f *= sens.Negative
		}
		return int(math.Round(f))
	}
	d := Opinion{
		Trust:     clampInt(scale(proposed.Trust), -env.Trust, env.Trust),
		Respect:   clampInt(scale(proposed.Respect), -env.Respect, env.Respect),
		Affection: clampInt(scale(proposed.Affection), -env.Affection, env.Affection),
	}
	if env.NoGain {
		if d.Trust > 0 {
			d.Trust = 0
		}
		if d.Affection > 0 {
			d.Affection = 0
		}
	}
	// Three warm conversational changes in the last hour and further warm
	// words stop counting until deeds back them up.
	if positiveWordsRecently >= 3 {
		if d.Trust > 0 {
			d.Trust = 0
		}
		if d.Respect > 0 {
			d.Respect = 0
		}
		if d.Affection > 0 {
			d.Affection = 0
		}
	}
	return d
}

// positiveWordChangesSince counts recent opinion gains that came from words.
func (m *Mind) positiveWordChangesSince(unix int64) int {
	n := 0
	for _, c := range slices.Backward(m.OpinionLog) {
		if c.Unix < unix {
			break
		}
		if c.FromWords && (c.Delta.Trust > 0 || c.Delta.Respect > 0 || c.Delta.Affection > 0) {
			n++
		}
	}
	return n
}

// applyOpinion adds a delta, clamps, and records it. A zero delta is not
// recorded. Returns the delta applied.
func (m *Mind) applyOpinion(d Opinion, trigger string, source string, reason string, fromWords bool) Opinion {
	if d.Trust == 0 && d.Respect == 0 && d.Affection == 0 {
		return d
	}
	before := m.Opinion
	m.Opinion = Opinion{
		Trust:     before.Trust + d.Trust,
		Respect:   before.Respect + d.Respect,
		Affection: before.Affection + d.Affection,
	}.clamped()
	applied := Opinion{
		Trust:     m.Opinion.Trust - before.Trust,
		Respect:   m.Opinion.Respect - before.Respect,
		Affection: m.Opinion.Affection - before.Affection,
	}
	m.OpinionLog = append(m.OpinionLog, OpinionChange{
		Unix:      time.Now().Unix(),
		Trigger:   trigger,
		Source:    source,
		Delta:     applied,
		After:     m.Opinion,
		Reason:    reason,
		FromWords: fromWords,
	})
	if len(m.OpinionLog) > 100 {
		m.OpinionLog = append([]OpinionChange(nil), m.OpinionLog[len(m.OpinionLog)-100:]...)
	}
	return applied
}

// opinionWords renders the opinion for the prompt. Numbers never leave the
// server.
func opinionWords(o Opinion, name string) []string {
	var trust, respect, affection string
	switch {
	case o.Trust <= -50:
		trust = `You do not trust ` + name + ` at all.`
	case o.Trust <= -15:
		trust = `You are wary of ` + name + `.`
	case o.Trust < 15:
		trust = `You have not yet decided whether to trust ` + name + `.`
	case o.Trust < 50:
		trust = `You trust ` + name + ` somewhat.`
	case o.Trust < 80:
		trust = `You trust ` + name + `.`
	default:
		trust = `You would trust ` + name + ` with your life.`
	}
	switch {
	case o.Respect <= -50:
		respect = `You have no respect for ` + name + `.`
	case o.Respect <= -15:
		respect = `You think ` + name + ` is careless or foolish.`
	case o.Respect < 15:
		respect = `You have not yet taken ` + name + `'s measure.`
	case o.Respect < 50:
		respect = `You think ` + name + ` is fairly capable.`
	default:
		respect = `You respect ` + name + ` a great deal.`
	}
	switch {
	case o.Affection <= -50:
		affection = `You dislike ` + name + ` intensely.`
	case o.Affection <= -15:
		affection = `You do not much like ` + name + `.`
	case o.Affection < 15:
		affection = `You feel neither warm nor cold toward ` + name + ` yet.`
	case o.Affection < 50:
		affection = `You like ` + name + `.`
	case o.Affection < 80:
		affection = `You are fond of ` + name + `.`
	default:
		affection = `You care about ` + name + ` deeply.`
	}
	return []string{trust, respect, affection}
}

// ruleChangesSince counts rule-sourced changes with a trigger since a time.
func (m *Mind) ruleChangesSince(trigger string, unix int64) int {
	n := 0
	for _, c := range slices.Backward(m.OpinionLog) {
		if c.Unix < unix {
			break
		}
		if c.Source == `rule` && c.Trigger == trigger {
			n++
		}
	}
	return n
}

// How she is with her owner, in bands. A companion starts professional: a
// hired pair of eyes on the road, courteous and no more. Warmth has to be
// earned, and can be lost; at the far end she is openly fond, and at the
// other she can barely stand them. The band picks the manner; her profile
// decides what that manner sounds like in her own voice.

const (
	mannerDisdain      = `disdain`
	mannerCold         = `cold`
	mannerProfessional = `professional`
	mannerFriendly     = `friendly`
	mannerWarm         = `warm`
	mannerClose        = `close`
)

// mannerBand reads the relationship. Affection sets the band; trust holds
// the top band back, because closeness without trust is not closeness.
func mannerBand(o Opinion) string {
	switch {
	case o.Affection <= -60:
		return mannerDisdain
	case o.Affection <= -20:
		return mannerCold
	case o.Affection < 20:
		return mannerProfessional
	case o.Affection < 45:
		return mannerFriendly
	case o.Affection < 70 || o.Trust < 40:
		return mannerWarm
	}
	return mannerClose
}

// defaultManner is how any companion carries itself in a band, before its
// own profile colours it.
var defaultManner = map[string]string{
	mannerDisdain: `You can barely stand them. You are curt to the point of rudeness, you do not volunteer anything, ` +
		`and you make no secret of what you think. You stay because of the road, not because of them.`,
	mannerCold: `You have gone cold on them. Short answers, no warmth, no jokes. You do what is agreed and no more.`,
	mannerProfessional: `You are professional with them: courteous, useful, and private. You answer what you are asked, ` +
		`keep your own counsel, and neither seek nor refuse company. This is where every road starts.`,
	mannerFriendly: `You have decided you like them. You talk more freely, tease a little, and volunteer the odd opinion, ` +
		`but you still keep some things back.`,
	mannerWarm: `You are fond of them. You speak plainly and easily, share what you think, notice when something is ` +
		`wrong with them, and put yourself out for them without making a thing of it.`,
	mannerClose: `You care about them a great deal and you trust them. You are openly affectionate in your own way, ` +
		`you tease, you say things you would tell nobody else, and you would take a bad risk for them.`,
}

// mannerWords is what the prompt tells her about how she is with her owner
// just now: the band, her own voice for it where the profile gives one, and
// how warmth shows in her, so a dry character never turns gushing.
func mannerWords(p *Profile, o Opinion, owner string) []string {
	band := mannerBand(o)
	text := defaultManner[band]
	if own, ok := p.Manner[band]; ok && strings.TrimSpace(own) != `` {
		text = strings.TrimSpace(own)
	}
	out := []string{fmt.Sprintf(`How you are with %s just now: %s. %s`, owner, band, text)}
	if strings.TrimSpace(p.WarmthStyle) != `` {
		out = append(out, `How warmth shows in you: `+strings.TrimSpace(p.WarmthStyle))
	}
	out = append(out, `This is how you feel today, not a costume: let it change slowly, with what they do.`)
	return out
}
