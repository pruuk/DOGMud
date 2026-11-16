package aicompanion

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Goals (F12.1 to F12.11). Three time scales: long-term ambitions from the
// profile, medium-term objectives (restocking, getting something, learning
// something), and the immediate step, which is simply the next action the
// model picks. Goals that can be checked against the game are checked by
// code every few rounds, never by the model's say-so; goals that cannot be
// checked (an ambition, a promise to visit someone) are settled by the model.

// Goal is one thing the companion is working toward.
type Goal struct {
	Id       int       `yaml:"id"`
	Level    string    `yaml:"level"`  // long, medium
	Kind     string    `yaml:"kind"`   // ambition, restock, acquire, explore, request, other
	Origin   string    `yaml:"origin"` // profile, need, self, owner
	Text     string    `yaml:"text"`
	Priority int       `yaml:"priority"` // 1 low .. 5 high
	Status   string    `yaml:"status"`   // active, done, paused, dropped
	Created  int64     `yaml:"created"`
	Updated  int64     `yaml:"updated"`
	Check    GoalCheck `yaml:"check,omitempty"`
}

// GoalCheck is a condition code can verify from live game state.
type GoalCheck struct {
	Kind     string `yaml:"kind,omitempty"` // carry, gold, skill, visit
	Supply   Supply `yaml:"supply,omitempty"`
	Count    int    `yaml:"count,omitempty"`
	Skill    string `yaml:"skill,omitempty"`
	Rank     string `yaml:"rank,omitempty"`
	RoomId   int    `yaml:"room,omitempty"`
	Verified bool   `yaml:"verified,omitempty"`
}

const (
	maxActiveLong   = 5
	maxActiveMedium = 6
	goalStaleDays   = 7
)

var skillRanks = []string{`unknown`, `novice`, `apprentice`, `journeyman`, `adept`, `expert`, `master`, `grandmaster`}

func rankIndex(rank string) int {
	for i, r := range skillRanks {
		if r == rank {
			return i
		}
	}
	return -1
}

// addGoal stores a goal unless an active goal with the same text exists or
// the level is full. Returns its id, or 0.
func (m *Mind) addGoal(g Goal, nowUnix int64) int {
	g.Text = strings.TrimSpace(g.Text)
	if g.Text == `` {
		return 0
	}
	if g.Level != `long` {
		g.Level = `medium`
	}
	active := 0
	for _, old := range m.Goals {
		if old.Status != `active` {
			continue
		}
		if strings.EqualFold(old.Text, g.Text) {
			return 0
		}
		if old.Level == g.Level {
			active++
		}
	}
	limit := maxActiveMedium
	if g.Level == `long` {
		limit = maxActiveLong
	}
	if active >= limit {
		return 0
	}
	m.NextGoalId++
	g.Id = m.NextGoalId
	g.Status = `active`
	g.Priority = clampInt(g.Priority, 1, 5)
	g.Created, g.Updated = nowUnix, nowUnix
	m.Goals = append(m.Goals, g)
	m.pruneGoals()
	return g.Id
}

// pruneGoals keeps every active goal and the 20 most recent finished ones.
func (m *Mind) pruneGoals() {
	finished := 0
	for i, v := range slices.Backward(m.Goals) {
		if v.Status == `active` || v.Status == `paused` {
			continue
		}
		finished++
		if finished > 20 {
			m.Goals = append(m.Goals[:i], m.Goals[i+1:]...)
		}
	}
}

func (m *Mind) goalById(id int) *Goal {
	for i := range m.Goals {
		if m.Goals[i].Id == id {
			return &m.Goals[i]
		}
	}
	return nil
}

// activeGoals returns active goals of a level, highest priority first.
func (m *Mind) activeGoals(level string) []Goal {
	var out []Goal
	for _, g := range m.Goals {
		if g.Status == `active` && (level == `` || g.Level == level) {
			out = append(out, g)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority > out[j].Priority })
	return out
}

// seedAmbitions gives a new mind its profile ambitions as long-term goals.
func (m *Mind) seedAmbitions(p *Profile, nowUnix int64) {
	if m.AmbitionsSeeded {
		return
	}
	m.AmbitionsSeeded = true
	for _, a := range p.Ambitions {
		m.addGoal(Goal{Level: `long`, Kind: `ambition`, Origin: `profile`, Text: a, Priority: 3}, nowUnix)
	}
	// What she sets out wanting on the day she takes up with someone: a
	// present want, not a life's ambition, and high on her list.
	for _, g := range p.StartingGoals {
		m.addGoal(Goal{Level: `medium`, Kind: `acquire`, Origin: `profile`, Text: g, Priority: 5}, nowUnix)
	}
}

// ensureRestockGoals turns supply shortfalls into checkable restock goals
// (F12.5), without a model call.
func (m *Mind) ensureRestockGoals(mob *mobs.Mob, p *Profile, nowUnix int64) []string {
	var added []string
	for _, n := range supplyNeeds(mob, p.Supplies) {
		text := `Restock ` + n.Supply.Name
		exists := false
		for _, g := range m.Goals {
			if g.Status == `active` && g.Kind == `restock` && strings.EqualFold(g.Text, text) {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		// The goal is done when the shortage is over, not when the pack is
		// full: otherwise "restock arrows" never closes while she is one
		// quiver short of her ideal, and it sits in every prompt for ever.
		if m.addGoal(Goal{Level: `medium`, Kind: `restock`, Origin: `need`, Text: text, Priority: 4,
			Check: GoalCheck{Kind: `carry`, Supply: n.Supply, Count: n.Supply.Min}}, nowUnix) > 0 {
			added = append(added, text)
		}
	}
	return added
}

// checkGoals verifies checkable goals against the game (F12.4) and pauses
// stale ones. It returns the goals that were just completed.
func (m *Mind) checkGoals(mob *mobs.Mob, nowUnix int64) []Goal {
	var done []Goal
	for i := range m.Goals {
		g := &m.Goals[i]
		if g.Status != `active` {
			continue
		}
		met := false
		switch g.Check.Kind {
		case `carry`:
			met = supplyCount(mob, g.Check.Supply) >= g.Check.Count
		case `gold`:
			met = mob.Character.Gold >= g.Check.Count
		case `skill`:
			rank := skills.GetSkillRankDescription(mob.Character.GetSkillLevel(skills.SkillTag(g.Check.Skill)))
			met = rankIndex(rank) >= rankIndex(g.Check.Rank) && rankIndex(g.Check.Rank) > 0
		case `visit`:
			_, met = m.Map[g.Check.RoomId]
		}
		if met {
			g.Status, g.Updated, g.Check.Verified = `done`, nowUnix, true
			done = append(done, *g)
			continue
		}
		if g.Level == `medium` && g.Kind != `restock` && nowUnix-g.Updated > goalStaleDays*86400 {
			g.Status, g.Updated = `paused`, nowUnix
		}
	}
	return done
}

// pickAgenda chooses up to three medium goals to pursue this session
// (F12.6).
func (m *Mind) pickAgenda() []int {
	var ids []int
	for _, g := range m.activeGoals(`medium`) {
		if len(ids) >= 3 {
			break
		}
		ids = append(ids, g.Id)
	}
	return ids
}

// goalLines renders goals for the prompt, with ids the model can refer to.
func goalLines(mind *Mind, agenda []int) []string {
	today := map[int]bool{}
	for _, id := range agenda {
		today[id] = true
	}
	var out []string
	for _, g := range mind.activeGoals(`long`) {
		out = append(out, fmt.Sprintf(`[g%d] (ambition) %s`, g.Id, g.Text))
	}
	for _, g := range mind.activeGoals(`medium`) {
		mark := ``
		if today[g.Id] {
			mark = ` (hoping to see to this today)`
		}
		checked := ``
		if g.Check.Kind != `` {
			checked = ` (you will know when it is done)`
		}
		out = append(out, fmt.Sprintf(`[g%d] %s%s%s`, g.Id, g.Text, mark, checked))
	}
	return out
}

// agendaText is the agenda in a few words, for idle moments.
func agendaText(mind *Mind, agenda []int) string {
	var parts []string
	for _, id := range agenda {
		if g := mind.goalById(id); g != nil && g.Status == `active` {
			parts = append(parts, g.Text)
		}
	}
	return strings.Join(parts, `; `)
}

// GoalProposal is the model's change to its goals.
type GoalProposal struct {
	Action string `json:"action"` // none, add, done, drop
	Text   string `json:"text"`
	Level  string `json:"level"` // medium, long
	Ref    string `json:"ref"`   // g12, for done or drop
}

var goalActions = []string{`none`, `add`, `done`, `drop`}

// applyGoalProposal applies the model's goal change. Checkable goals can
// only be completed by code; the model may drop them.
func (m *Mind) applyGoalProposal(p GoalProposal, fromOwner bool, nowUnix int64) string {
	switch p.Action {
	case `add`:
		origin := `self`
		if fromOwner {
			origin = `owner`
		}
		priority := 3
		if fromOwner {
			priority = 4
		}
		if id := m.addGoal(Goal{Level: p.Level, Kind: `other`, Origin: origin, Text: p.Text, Priority: priority}, nowUnix); id > 0 {
			return fmt.Sprintf(`added g%d`, id)
		}
		return `not added`
	case `done`, `drop`:
		id, _ := strconv.Atoi(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(p.Ref)), `g`))
		g := m.goalById(id)
		if g == nil || g.Status != `active` {
			return `no such goal`
		}
		if p.Action == `done` {
			if g.Check.Kind != `` {
				return `checked goals are completed by what actually happens`
			}
			g.Status = `done`
		} else {
			g.Status = `dropped`
		}
		g.Updated = nowUnix
		return p.Action + ` g` + strconv.Itoa(id)
	}
	return ``
}

// goalDoneMemory is how a completed goal is remembered.
func goalDoneMemory(g Goal) Memory {
	importance := 5
	if g.Level == `long` {
		importance = 9
	}
	return Memory{Unix: time.Now().Unix(), Kind: `event`, Text: `I achieved something I had set out to do: ` + g.Text,
		Importance: importance, Emotion: `pride`}
}
