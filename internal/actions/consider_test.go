package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// fakeActor is a minimal Actor for unit tests. It records SendText lines and
// OnStatUse/OnSkillUse calls.
type fakeActor struct {
	awardRecorder // records Actor.AwardResolved calls
	char          *characters.Character
	room          *rooms.Room // nil unless a test stands the actor somewhere
	name          string
	isPlayer      bool
	sent          []string
	statUses      map[string]int
}

// newFakeActor builds a validated character (the fight prediction reads its
// Position, pools and plan) with every stat's ValueAdj at statAdj and pools
// at healthMax. Setting ValueAdj after Validate skips the Training pipeline,
// as character_test.go does for combat math.
func newFakeActor(name string, statAdj, healthMax int, isPlayer bool) *fakeActor {
	c := characters.New()
	c.Name = name
	if err := c.Validate(); err != nil {
		panic(err)
	}
	c.Stats.Strength.ValueAdj = statAdj
	c.Stats.Dexterity.ValueAdj = statAdj
	c.Stats.Perception.ValueAdj = statAdj
	c.Stats.Vitality.ValueAdj = statAdj
	c.Stats.Willpower.ValueAdj = statAdj
	c.Stats.Charisma.ValueAdj = statAdj
	c.HealthMax.Value, c.Health = healthMax, healthMax
	c.StaminaMax.Value, c.Stamina = healthMax, healthMax
	c.ConvictionMax.Value, c.Conviction = healthMax/2, healthMax/2
	return &fakeActor{char: c, name: name, isPlayer: isPlayer, statUses: map[string]int{}}
}

func (a *fakeActor) GetCharacter() *characters.Character         { return a.char }
func (a *fakeActor) GetRoom() *rooms.Room                        { return a.room }
func (a *fakeActor) SendText(_ messaging.Category, msg string)   { a.sent = append(a.sent, msg) }
func (a *fakeActor) SendBlankLine(_ messaging.Category)          { a.sent = append(a.sent, "") }
func (a *fakeActor) GetName() string                             { return a.name }
func (a *fakeActor) IsPlayer() bool                              { return a.isPlayer }
func (a *fakeActor) GetUserId() int                              { return 0 }
func (a *fakeActor) GetMobInstanceId() int                       { return 0 }
func (a *fakeActor) AddCondition(conditionId int, source string) {}
func (a *fakeActor) OnSkillUse(skillName string) bool {
	a.statUses[skillName]++
	return false
}
func (a *fakeActor) OnStatUse(statName string) bool {
	a.statUses[statName]++
	return false
}

// fixRoll pins the read roll for one test.
func fixRoll(t *testing.T, res contest.Result) {
	t.Helper()
	old := considerContest
	considerContest = func(float64, []contest.Entry) contest.Result { return res }
	t.Cleanup(func() { considerContest = old })
}

func TestConsider_PlayerReadSendsThreeLinesAndTrainsSearch(t *testing.T) {
	fixRoll(t, contest.Result{Success: true, Margin: 1})
	self := newFakeActor("hero", 100, 100, true)
	target := newFakeActor("orc", 100, 100, false)

	r := Consider(self, target)
	if len(self.sent) != 3 {
		t.Fatalf("got %d lines, want opener, verdict and pace: %q", len(self.sent), self.sent)
	}
	if !strings.Contains(self.sent[0], "orc") {
		t.Errorf("opener should name the target: %q", self.sent[0])
	}
	if r.Quality != ReadFair {
		t.Errorf("quality %v, want fair for a narrow win", r.Quality)
	}
	if _, n := self.awardedCandidate("search"); n != 1 || len(self.awards) != 1 || !self.awards[0].won {
		t.Errorf("awards %+v, want one won award offering search", self.awards)
	}
	if len(self.statUses) != 0 {
		t.Errorf("fired raw progression %v; consider trains only through AwardResolved", self.statUses)
	}
}

// A lost read still trains, at the loss weight: the resolved-contest
// convention (#301).
func TestConsider_LostReadTrainsAsALoss(t *testing.T) {
	fixRoll(t, contest.Result{Success: false})
	self := newFakeActor("hero", 100, 100, true)
	Consider(self, newFakeActor("orc", 100, 100, false))
	if _, n := self.awardedCandidate("search"); n != 1 || len(self.awards) != 1 || self.awards[0].won {
		t.Errorf("awards %+v, want one lost award offering search", self.awards)
	}
}

// The cooldown is the gate: a refused read neither rolls nor trains (#301).
func TestConsider_CooldownRefusalNeitherRollsNorTrains(t *testing.T) {
	rolled := false
	old := considerContest
	considerContest = func(float64, []contest.Entry) contest.Result { rolled = true; return contest.Result{} }
	t.Cleanup(func() { considerContest = old })

	self := newFakeActor("hero", 100, 100, true)
	if !ClaimSpecialMove(self.char) {
		t.Fatal("fixture could not claim the cooldown")
	}
	r := Consider(self, newFakeActor("orc", 100, 100, false))
	if !r.OnCooldown || rolled || len(self.statUses) != 0 || len(self.awards) != 0 {
		t.Fatalf("refusal: OnCooldown %v rolled %v trained %v awards %+v", r.OnCooldown, rolled, self.statUses, self.awards)
	}
	if len(self.sent) != 1 || !strings.Contains(self.sent[0], "before you can consider anyone") {
		t.Fatalf("refusal text: %q", self.sent)
	}
}

func TestConsider_MuddyReadSlidesTheVerdict(t *testing.T) {
	fixRoll(t, contest.Result{Success: false})
	self := newFakeActor("hero", 100, 100, true)
	target := newFakeActor("orc", 120, 100, false) // reads lose-badly clean
	clean := ReadFight(self.char, target.char).Verdict
	switch clean {
	case VerdictScratches, VerdictHurt, VerdictLikelyLose, VerdictLoseBadly:
	default:
		t.Fatalf("fixture premise: clean verdict %v must be one that slides", clean)
	}

	r := Consider(self, target)
	if r.Quality != ReadMuddy || r.Read.Verdict == clean || r.Read.Verdict != muddySlide(clean) {
		t.Fatalf("muddy read: quality %v verdict %v (clean %v), want muddy %v",
			r.Quality, r.Read.Verdict, clean, muddySlide(clean))
	}

	// The pace line is the hedge, never an exact pace that could contradict
	// the slid verdict (owner ruling 2026-10-13).
	if len(self.sent) != 3 || self.sent[2] != muddyPaceText {
		t.Fatalf("muddy pace line: sent %q, want third line %q", self.sent, muddyPaceText)
	}
	for _, w := range []string{"moments", "short fight", "long fight", "never"} {
		if strings.Contains(self.sent[2], w) {
			t.Errorf("muddy pace line %q still says %q", self.sent[2], w)
		}
	}
}

// A fair or clear read keeps the exact pace line.
func TestConsider_NonMuddyReadKeepsTheExactPace(t *testing.T) {
	fixRoll(t, contest.Result{Success: true, Margin: 1})
	self := newFakeActor("hero", 100, 100, true)
	r := Consider(self, newFakeActor("orc", 100, 100, false))
	if r.Quality == ReadMuddy {
		t.Fatalf("fixture premise: a won roll read muddy")
	}
	if len(self.sent) != 3 || self.sent[2] != paceText(r.Read) {
		t.Fatalf("pace line: sent %q, want third line %q", self.sent, paceText(r.Read))
	}
}

// The reader pays the sight ramp (plan 5b): the score handed to the roll is
// Perception + Search x SkillWeight in a comfortable room, and strictly less
// in a dark one (#301).
func TestConsider_ReadScorePaysTheSightRamp(t *testing.T) {
	var got float64
	old := considerContest
	considerContest = func(score float64, _ []contest.Entry) contest.Result {
		got = score
		return contest.Result{Success: true, Margin: 1}
	}
	t.Cleanup(func() { considerContest = old })

	scoreIn := func(lamp int) float64 {
		zero := 0.0
		room := &rooms.Room{RoomId: 9740, SkyLight: &zero}
		if lamp > 0 {
			room.Lamp = rooms.LampPtr(lamp)
		}
		self := newFakeActor("hero", 100, 100, true)
		self.room = room
		self.char.SetSkill(string(skills.Search), 3)
		Consider(self, newFakeActor("orc", 100, 100, false))
		return got
	}

	want := 100 + 3*float64(configs.GetBalanceConfig().SkillWeight)

	lit := scoreIn(60)
	if lit != want {
		t.Errorf("lit read score %v, want Perception + Search x SkillWeight = %v", lit, want)
	}
	if dark := scoreIn(0); dark >= lit {
		t.Errorf("dark read score %v, want strictly less than lit %v", dark, lit)
	}
}

// A mob's consider (and the AI companion's) has no roll, no cooldown, no
// training and no text.
func TestConsider_MobReadIsSilentAndFree(t *testing.T) {
	self := newFakeActor("goblin", 100, 100, false)
	r := Consider(self, newFakeActor("hero", 100, 100, true))
	if r.Quality != ReadNone || r.OnCooldown || len(self.statUses) != 0 || len(self.awards) != 0 || len(self.sent) != 0 {
		t.Fatalf("mob read: %+v trained %v awards %+v sent %q", r, self.statUses, self.awards, self.sent)
	}
	if !SpecialMoveReady(self.char) {
		t.Fatal("a mob's consider must not take the special-move cooldown")
	}
}
