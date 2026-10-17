package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// The 2026-10-17 owner ruling added lessons to Waking to Gaius: the skills
// and spells commands and a first cast of Chrysalis Glow in room 6259, and
// whispered lines from Dewey during the effigy fight. These tests walk the
// real quest 28 YAML with a player whose progress behaves the way
// characters.HasQuest does: one current step per quest, and every step
// before it counts as held. That is what lets a player saved mid-tutorial
// under the old step list carry on under the new one.

// repoFile reads a file from the repository root.
func repoFile(t *testing.T, rel ...string) []byte {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	parts := append([]string{filepath.Dir(here), "..", ".."}, rel...)
	data, err := os.ReadFile(filepath.Join(parts...))
	require.NoError(t, err)
	return data
}

func loadQuest28(t *testing.T) QuestDef {
	t.Helper()
	var q QuestDef
	require.NoError(t, yaml.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "quests", "28-waking_to_gaius.yaml"), &q))
	require.Equal(t, 28, q.QuestId)
	return q
}

// orderedPlayer holds one current step for quest 28, as a saved character
// does, and answers HasQuest by step order.
type orderedPlayer struct {
	steps   []string
	current string
	roomId  int
}

func (p *orderedPlayer) index(token string) int {
	parts := strings.SplitN(token, "-", 2)
	if len(parts) != 2 || parts[0] != "28" {
		return -1
	}
	for i, s := range p.steps {
		if s == parts[1] {
			return i
		}
	}
	return -1
}

func (p *orderedPlayer) HasQuest(token string) bool {
	if p.current == "" {
		return false
	}
	want := p.index(token)
	return want >= 0 && want <= p.index("28-"+p.current)
}
func (p *orderedPlayer) HasItem(int) bool           { return false }
func (p *orderedPlayer) GetRoomId() int             { return p.roomId }
func (p *orderedPlayer) GetQuestFlag(string) string { return "" }
func (p *orderedPlayer) GetGold() int               { return 0 }
func (p *orderedPlayer) HasOwnMasterwork(int) bool  { return false }
func (p *orderedPlayer) step() string               { return p.current }

type orderedCtx struct {
	*mockActionContext
	player *orderedPlayer
}

// GrantQuest moves the player forward only, as GiveQuestToken does.
func (c *orderedCtx) GrantQuest(token string) {
	c.mockActionContext.GrantQuest(token)
	if c.player.current == "" || c.player.index(token) > c.player.index("28-"+c.player.current) {
		c.player.current = strings.SplitN(token, "-", 2)[1]
	}
}

func newQuest28Walk(t *testing.T, savedStep string, roomId int) (*Engine, *orderedPlayer, *orderedCtx) {
	t.Helper()
	q := loadQuest28(t)
	e := NewEngine()
	e.RegisterQuest(&q)
	steps := make([]string, 0, len(q.Steps))
	for _, s := range q.Steps {
		steps = append(steps, s.Id)
	}
	p := &orderedPlayer{steps: steps, current: savedStep, roomId: roomId}
	return e, p, &orderedCtx{mockActionContext: newMockActionContext(1), player: p}
}

// TestQuest28_NewLessonsAdvanceOnSuccessOnly walks room 6259 from the status
// lesson to the north exit. Each new step must not move on a command that
// only looks like it (a typed cast that may yet be refused, or the previous
// lesson's command again) and must move on the real thing.
func TestQuest28_NewLessonsAdvanceOnSuccessOnly(t *testing.T) {
	e, p, ctx := newQuest28Walk(t, "status_prompt", 6259)

	notify := func(event string, d EventDetails) {
		d.UserId = 1
		e.Notify(event, d, p, ctx)
	}

	notify("command_issued", EventDetails{RoomId: 6259, Command: "status"})
	require.Equal(t, "statused", p.step())

	// skills
	notify("command_issued", EventDetails{RoomId: 6259, Command: "status"})
	notify("command_issued", EventDetails{RoomId: 6259, Command: "spells"})
	require.Equal(t, "statused", p.step(), "only skills teaches skills")
	notify("command_issued", EventDetails{RoomId: 6259, Command: "skills"})
	require.Equal(t, "skilled", p.step())

	// spells
	notify("command_issued", EventDetails{RoomId: 6259, Command: "skills"})
	require.Equal(t, "skilled", p.step(), "only spells teaches spells")
	notify("command_issued", EventDetails{RoomId: 6259, Command: "spells"})
	require.Equal(t, "spelled", p.step())

	// cast glow: typing it is not enough (a cast can be refused), and a
	// different spell does not count.
	notify("command_issued", EventDetails{RoomId: 6259, Command: "cast", Noun: "glow"})
	require.Equal(t, "spelled", p.step(), "a typed cast that may be refused must not count")
	notify("command", EventDetails{RoomId: 6259, Command: "cast", Noun: "conviction-spike"})
	require.Equal(t, "spelled", p.step(), "only Chrysalis Glow counts")
	notify("command", EventDetails{RoomId: 6259, Command: "cast", Noun: "chrysalis-glow"})
	require.Equal(t, "glowed", p.step())

	// The next room opens only now.
	notify("room_enter", EventDetails{RoomId: 6260})
	require.Equal(t, "carry_prompt", p.step())
}

// TestQuest28_NextRoomWaitsForTheGlow: arriving at 6260 before the 6259
// lessons are done must not start the 6260 lesson.
func TestQuest28_NextRoomWaitsForTheGlow(t *testing.T) {
	for _, saved := range []string{"statused", "skilled", "spelled"} {
		e, p, ctx := newQuest28Walk(t, saved, 6260)
		e.Notify("room_enter", EventDetails{UserId: 1, RoomId: 6260}, p, ctx)
		require.Equal(t, saved, p.step(), "saved at %s", saved)
	}
}

// TestQuest28_SavedPlayersResume: a player saved under the old step list
// resumes. One past room 6259 already counts the new lessons as done; one
// standing on statused is taught skills next.
func TestQuest28_SavedPlayersResume(t *testing.T) {
	e, p, ctx := newQuest28Walk(t, "carry_prompt", 6260)
	require.True(t, p.HasQuest("28-glowed"), "a player past 6259 holds the new 6259 steps")
	e.Notify("command_issued", EventDetails{UserId: 1, RoomId: 6260, Command: "get", Noun: "token"}, p, ctx)
	require.Equal(t, "got", p.step())

	e, p, ctx = newQuest28Walk(t, "statused", 6259)
	e.Notify("command_issued", EventDetails{UserId: 1, RoomId: 6259, Command: "skills"}, p, ctx)
	require.Equal(t, "skilled", p.step())
}

// TestQuest28_StepOrder pins where the new steps sit, so the 6259 exit gate
// and the 6260 arrival trigger agree on the last 6259 step.
func TestQuest28_StepOrder(t *testing.T) {
	q := loadQuest28(t)
	var ids []string
	for _, s := range q.Steps {
		ids = append(ids, s.Id)
	}
	joined := strings.Join(ids, ",")
	require.Contains(t, joined, "status_prompt,statused,skilled,spelled,glowed,carry_prompt")
}

// TestQuest28_6259GateWaitsForTheGlow: the room tree that holds the player in
// 6259 must name the last 6259 step, or the player can walk out mid-lesson.
func TestQuest28_6259GateWaitsForTheGlow(t *testing.T) {
	data := string(repoFile(t, "_datafiles", "world", "dogmud", "behaviors", "rooms", "newcomer_antechamber", "6259.yaml"))
	require.Contains(t, data, `quest: "28-glowed"`)
	require.NotContains(t, data, `quest: "28-statused"`)
}

// TestQuest28_FightLinesAreWhispered: Dewey's lines while the effigy fight
// runs are whispered to the player, so they stand out from the combat text
// (owner ruling 2026-10-17). Each one is sent once, as a whisper and not
// also as a say.
func TestQuest28_FightLinesAreWhispered(t *testing.T) {
	q := loadQuest28(t)
	fightSteps := map[string]bool{"28-attacked": true, "28-cast_spike": true, "28-tripped": true}
	seen := 0
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			if !fightSteps[a.Grant] {
				continue
			}
			for _, b := range tr.Actions {
				if b.NpcSay == nil {
					continue
				}
				for _, line := range b.NpcSay.Lines {
					seen++
					require.True(t, line.Whisper, "%s line %q must be whispered", a.Grant, line.Text)
				}
			}
		}
	}
	require.Equal(t, 6, seen, "attacked 1 + cast_spike 2 + tripped 3 lines")
}

// TestQuest28_LookLessonIsAHighlightedNoun: the room-noun lesson (owner
// ruling 2026-10-17) asks for `look guide` and Dewey points at the words
// shown in a different color, so "guide" must be a noun of room 6258 that
// its description names (rooms.highlightNouns colors only a noun the
// visible text contains, case-sensitive).
func TestQuest28_LookLessonIsAHighlightedNoun(t *testing.T) {
	var room struct {
		Description string            `yaml:"description"`
		Nouns       map[string]string `yaml:"nouns"`
	}
	require.NoError(t, yaml.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "rooms", "newcomer_antechamber", "6258.yaml"), &room))
	require.Contains(t, room.Nouns, "guide")
	require.Regexp(t, `\bguide\b`, room.Description, "the description must show the noun it teaches")

	q := loadQuest28(t)
	found := false
	for _, tr := range q.Triggers {
		if tr.Event == "command_issued" && tr.Command == "look" && tr.Noun == "guide" {
			found = true
		}
	}
	require.True(t, found, "the look lesson listens for look guide")
}

// TestNpcCommand_WhisperAddressesThePlayer: a whispered line becomes the mob
// whisper command aimed at the triggering player by id.
func TestNpcCommand_WhisperAddressesThePlayer(t *testing.T) {
	require.Equal(t, "whisper @7 hold steady", npcCommand(SayLineDef{Text: "hold steady", Whisper: true}, 7))
	require.Equal(t, "say hold steady", npcCommand(SayLineDef{Text: "hold steady"}, 7))
	require.Equal(t, "emote nods", npcCommand(SayLineDef{Text: "nods", Emote: true}, 7))
}
