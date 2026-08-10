package configs

import "math"

type Timing struct {
	TurnMs            ConfigInt `yaml:"TurnMs"`
	RoundSeconds      ConfigInt `yaml:"RoundSeconds"`
	RoundsPerAutoSave ConfigInt `yaml:"RoundsPerAutoSave"`

	// AutosaveWritesPerTick bounds how many prepared writes autosave commits per
	// turn (roadmap chunk 3.6b-1). Autosave prepares every dirty room and user
	// in one atomic pass, then spreads the durable writes across later turns so
	// no single turn stalls the world.
	//
	// A durable write measured 3.46ms in 3.6a, so 3 per turn is about 10ms of a
	// 50ms turn. Raising it drains a cycle sooner at the cost of a larger slice
	// of each turn; lowering it does the reverse and eventually logs a WARN that
	// a cycle did not finish before the next one began.
	AutosaveWritesPerTick ConfigInt `yaml:"AutosaveWritesPerTick"`
	RoundsPerDay          ConfigInt `yaml:"RoundsPerDay"` // How many rounds are in a day
	NightHours            ConfigInt `yaml:"NightHours"`   // How many hours of night

	// Protected values
	turnsPerRound   int     // calculated and cached when data is validated.
	turnsPerSave    int     // calculated and cached when data is validated.
	turnsPerSecond  int     // calculated and cached when data is validated.
	roundsPerMinute float64 // calculated and cached when data is validated.
}

func (e *Timing) Validate() {

	if e.TurnMs < 10 {
		e.TurnMs = 100 // default
	}

	if e.RoundSeconds < 1 {
		e.RoundSeconds = 4 // default
	}

	if e.RoundsPerAutoSave < 1 {
		e.RoundsPerAutoSave = 900 // default of 15 minutes worth of rounds
	}

	// Clamped, not defaulted, and the distinction matters. At zero the queue
	// never drains: the pending set grows without bound and NOTHING is ever
	// persisted, while every other signal says the game is healthy. That is a
	// worse failure than any pause this knob exists to smooth out, and it is one
	// typo away, so the floor is enforced here as well as in savequeue.Drain.
	if e.AutosaveWritesPerTick < 1 {
		e.AutosaveWritesPerTick = 3 // default: ~10ms of a 50ms turn
	}

	if e.RoundsPerDay < 10 {
		e.RoundsPerDay = 20 // default of 24 hours worth of rounds
	}

	if e.NightHours < 0 {
		e.NightHours = 0
	} else if e.NightHours > 24 {
		e.NightHours = 24
	}

	// Pre-calculate and cache useful values
	e.turnsPerRound = int((e.RoundSeconds * 1000) / e.TurnMs)
	e.turnsPerSave = int(e.RoundsPerAutoSave) * e.turnsPerRound
	e.turnsPerSecond = int(1000 / e.TurnMs)
	e.roundsPerMinute = 60 / float64(e.RoundSeconds)

}

func (e Timing) TurnsPerRound() int {
	return e.turnsPerRound
}

func (e Timing) TurnsPerAutoSave() int {
	return e.turnsPerSave
}

func (e Timing) TurnsPerSecond() int {
	return e.turnsPerSecond
}

func (e Timing) MinutesToRounds(minutes int) int {
	return int(math.Ceil(e.roundsPerMinute * float64(minutes)))
}

func (e Timing) SecondsToRounds(seconds int) int {
	return int(math.Ceil(float64(seconds) / float64(e.RoundSeconds)))
}

func (e Timing) MinutesToTurns(minutes int) int {
	return int(math.Ceil(float64(minutes*60*1000) / float64(e.TurnMs)))
}

func (e Timing) SecondsToTurns(seconds int) int {
	return int(math.Ceil(float64(seconds*1000) / float64(e.TurnMs)))
}

func (e Timing) RoundsToSeconds(rounds int) int {
	return int(math.Ceil(float64(rounds) * float64(e.RoundSeconds)))
}

func GetTimingConfig() Timing {
	ensureConfigValidated()

	configDataLock.RLock()
	defer configDataLock.RUnlock()
	return configData.Timing
}
