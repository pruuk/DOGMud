package actions

import (
	"fmt"
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// Verdict is who comes out ahead in a read fight, and how cleanly (#301).
// It never speaks of pace; Pace does.
type Verdict int

const (
	VerdictUntouched  Verdict = iota // they won't lay a finger on you
	VerdictScratches                 // a few scratches at most
	VerdictHurt                      // you'll win, but you'll feel it
	VerdictBarely                    // you'd win, barely, if at all
	VerdictLikelyLose                // you'd likely lose
	VerdictLoseBadly                 // you'd lose badly
	VerdictHopeless                  // zero chance
	VerdictStalemate                 // neither of you is ending this soon
)

// Lethal reports a verdict that predicts the reader loses.
func (v Verdict) Lethal() bool {
	return v == VerdictLikelyLose || v == VerdictLoseBadly || v == VerdictHopeless
}

// Danger orders verdicts from safest to deadliest; a stalemate sits with
// barely.
func (v Verdict) Danger() int {
	switch v {
	case VerdictUntouched:
		return 0
	case VerdictScratches:
		return 1
	case VerdictHurt:
		return 2
	case VerdictBarely, VerdictStalemate:
		return 3
	case VerdictLikelyLose:
		return 4
	case VerdictLoseBadly:
		return 5
	}
	return 6
}

// Pace is how long one side needs to bring the other down.
type Pace int

const (
	PaceMoments Pace = iota
	PaceShort
	PaceLong
	PaceAge
	PaceNever
)

// ReadQuality is how clearly a player read the fight: their consider roll.
// ReadNone is a read with no roll (a mob, the AI companion).
type ReadQuality int

const (
	ReadNone ReadQuality = iota
	ReadClear
	ReadFair
	ReadMuddy
)

// FightRead is a classified prediction: the verdict and both paces.
type FightRead struct {
	Verdict    Verdict
	YourPace   Pace // how long you need to bring them down
	TheirPace  Pace // how long they need to bring you down
	Prediction combat.FightPrediction
}

// ReadFight is you's read of them with no roll: the prediction, classified.
// Mobs, the AI companion and the pinned tests use it; a player's consider
// adds the roll on top.
func ReadFight(you, them *characters.Character) FightRead {
	return classifyFight(combat.PredictFight(you, them), configs.GetBalanceConfig())
}

func paceFor(rounds float64, bal configs.Balance) Pace {
	switch {
	case math.IsInf(rounds, 1):
		return PaceNever
	case rounds <= float64(bal.ConsiderPaceMoments):
		return PaceMoments
	case rounds <= float64(bal.ConsiderPaceShort):
		return PaceShort
	case rounds <= float64(bal.ConsiderPaceLong):
		return PaceLong
	}
	return PaceAge
}

// classifyFight applies the spec's band order: untouched first (a target that
// cannot hurt you is never hopeless), stalemate before hopeless (a slow,
// pillow-hitting target is not zero chance), then the cost ladder.
func classifyFight(p combat.FightPrediction, bal configs.Balance) FightRead {
	r := FightRead{
		YourPace:   paceFor(p.RoundsToKillThem, bal),
		TheirPace:  paceFor(p.RoundsToKillYou, bal),
		Prediction: p,
	}
	switch {
	case p.TheirDamagePerRound <= 0 || p.Cost < float64(bal.ConsiderBandUntouched):
		r.Verdict = VerdictUntouched
	case r.YourPace == PaceAge && r.TheirPace == PaceAge:
		r.Verdict = VerdictStalemate
	case p.Dent < float64(bal.ConsiderBandHopelessDent):
		r.Verdict = VerdictHopeless
	case p.Cost < float64(bal.ConsiderBandScratches):
		r.Verdict = VerdictScratches
	case p.Cost < float64(bal.ConsiderBandHurt):
		r.Verdict = VerdictHurt
	case p.Cost < float64(bal.ConsiderBandBarely):
		r.Verdict = VerdictBarely
	case p.Cost < float64(bal.ConsiderBandLethal):
		r.Verdict = VerdictLikelyLose
	default:
		r.Verdict = VerdictLoseBadly
	}
	return r
}

// muddySlide moves a muddy read's verdict one step toward barely. Untouched,
// barely, hopeless and stalemate never slide (owner ruling 2026-10-13: no
// extra slide on a critical failure either).
func muddySlide(v Verdict) Verdict {
	switch v {
	case VerdictScratches:
		return VerdictHurt
	case VerdictHurt, VerdictLikelyLose:
		return VerdictBarely
	case VerdictLoseBadly:
		return VerdictLikelyLose
	}
	return v
}

func readQualityFor(res contest.Result, readScore float64, bal configs.Balance) ReadQuality {
	if !res.Success {
		return ReadMuddy
	}
	if sd := dice.StdDevFor(readScore); sd > 0 && res.Margin/sd >= float64(bal.ConsiderClearReadZ) {
		return ReadClear
	}
	return ReadFair
}

// Copy (player-copy rules: no numbers, 80 columns, plain words).

var verdictText = map[Verdict]string{
	VerdictUntouched:  `They won't lay a finger on you.`,
	VerdictScratches:  `A few scratches at most.`,
	VerdictHurt:       `You'll win, but you'll feel it.`,
	VerdictBarely:     `You'd win, barely, if at all.`,
	VerdictLikelyLose: `You'd likely lose.`,
	VerdictLoseBadly:  `You'd lose badly.`,
	VerdictHopeless:   `What could you possibly be thinking? You have zero chance.`,
	VerdictStalemate:  `Neither of you is ending this soon.`,
}

var verdictColor = map[Verdict]string{
	VerdictUntouched: `blue-bold`, VerdictScratches: `green`, VerdictHurt: `green`,
	VerdictBarely: `yellow`, VerdictStalemate: `yellow`,
	VerdictLikelyLose: `red-bold`, VerdictLoseBadly: `red-bold`, VerdictHopeless: `red-bold`,
}

var yourPaceText = map[Pace]string{
	PaceMoments: `you'd beat them in moments`,
	PaceShort:   `you'd beat them in a short fight`,
	PaceLong:    `beating them would be a long fight`,
	PaceAge:     `beating them would take an age`,
	PaceNever:   `you could never beat them`,
}

var theirPaceText = map[Pace]string{
	PaceMoments: `they could beat you in moments`,
	PaceShort:   `they could beat you in a short fight`,
	PaceLong:    `they'd need a long fight to beat you`,
	PaceAge:     `beating you would take them an age`,
	PaceNever:   `they could never beat you`,
}

// openerText is the read's first line; the muddy opener is the hedge.
func openerText(q ReadQuality, targetName string) string {
	switch q {
	case ReadClear:
		return fmt.Sprintf(`You size up %s with a practised eye.`, targetName)
	case ReadMuddy:
		return fmt.Sprintf(`You study %s, unsure of what you see.`, targetName)
	}
	return fmt.Sprintf(`You take a fresh look at %s, weighing your chances.`, targetName)
}

// paceText joins the two paces into one sentence.
func paceText(r FightRead) string {
	s := yourPaceText[r.YourPace] + `; ` + theirPaceText[r.TheirPace] + `.`
	return strings.ToUpper(s[:1]) + s[1:]
}

// verdictLine is the verdict in its colour, for a player.
func verdictLine(r FightRead) string {
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, verdictColor[r.Verdict], verdictText[r.Verdict])
}

// FightReadPlain is the read as untagged sentences, verdict then pace: the AI
// companion's words for it.
func FightReadPlain(r FightRead) string {
	return verdictText[r.Verdict] + ` ` + paceText(r)
}
