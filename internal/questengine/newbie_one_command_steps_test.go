package questengine

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// Owner ruling 2026-10-17 (pre-deploy playtest): a newbie quest asks for ONE
// command per step, and each step's hint names that command plainly ("Type
// forage."). First Sign asked for track AND forage in one step, and First Shot
// asked for sneak AND a second shot in one step. These tests walk both quests
// through the real YAML, step by step, with a player whose progress behaves the
// way a real character's does.
//
// linearPlayer mirrors characters.Character's quest progress, which is ONE
// current step per quest, not a set of tokens: HasQuest(t) holds for the
// current step and every step before it (characters.HasQuest via
// quests.IsTokenAfter), and a grant only moves the step forward
// (characters.GiveQuestToken). That is why "do these two in any order" cannot
// work in this engine: doing the later one first marks the earlier one done.
type linearPlayer struct {
	*fullMockPlayer
	questId int
	steps   []string
	current string // "" before the quest is held
}

func newLinearPlayer(q *QuestDef) *linearPlayer {
	p := &linearPlayer{fullMockPlayer: newFullMockPlayer(0), questId: q.QuestId}
	for _, s := range q.Steps {
		p.steps = append(p.steps, s.Id)
	}
	return p
}

func (p *linearPlayer) stepIndex(step string) int {
	for i, s := range p.steps {
		if s == step {
			return i
		}
	}
	return -1
}

func (p *linearPlayer) split(token string) (int, string) {
	parts := strings.SplitN(token, "-", 2)
	if len(parts) != 2 {
		return 0, ""
	}
	id := 0
	for _, r := range parts[0] {
		id = id*10 + int(r-'0')
	}
	return id, parts[1]
}

func (p *linearPlayer) HasQuest(token string) bool {
	id, step := p.split(token)
	if id != p.questId {
		return p.fullMockPlayer.HasQuest(token)
	}
	if p.current == "" {
		return false
	}
	want := p.stepIndex(step)
	return want >= 0 && want <= p.stepIndex(p.current)
}

func (p *linearPlayer) grant(token string) {
	id, step := p.split(token)
	if id != p.questId {
		p.fullMockPlayer.quests[token] = true
		return
	}
	next := p.stepIndex(step)
	if next < 0 {
		return
	}
	if p.current == "" {
		if step == "start" {
			p.current = step
		}
		return
	}
	if next > p.stepIndex(p.current) {
		p.current = step
	}
}

type linearCtx struct {
	*mockActionContext
	p *linearPlayer
}

func (c *linearCtx) GrantQuest(token string) {
	c.mockActionContext.GrantQuest(token)
	c.p.grant(token)
}

func loadShippedQuest(t *testing.T, file string, wantId int) *QuestDef {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "quests", file)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "must be able to read the real quest YAML %s", file)
	var q QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, wantId, q.QuestId, "loaded the wrong quest file")
	return &q
}

// turnInRequires reads the questRequired list of one dialogue node from a
// shipped dialogue file.
func turnInRequires(t *testing.T, file string, nodeId string) []string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", file)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var df struct {
		Tree struct {
			Nodes []struct {
				Id            string   `yaml:"id"`
				QuestRequired []string `yaml:"questRequired"`
			} `yaml:"nodes"`
		} `yaml:"tree"`
	}
	require.NoError(t, yaml.Unmarshal(data, &df))
	for _, n := range df.Tree.Nodes {
		if n.Id == nodeId {
			require.NotEmpty(t, n.QuestRequired, "%s node %s gates on nothing", file, nodeId)
			return n.QuestRequired
		}
	}
	t.Fatalf("%s has no node %s", file, nodeId)
	return nil
}

func holdsAll(p *linearPlayer, tokens []string) bool {
	for _, tok := range tokens {
		if !p.HasQuest(tok) {
			return false
		}
	}
	return true
}

type questWalker struct {
	e   *Engine
	p   *linearPlayer
	ctx *linearCtx
}

func newQuestWalker(q *QuestDef) *questWalker {
	e := NewEngine()
	e.RegisterQuest(q)
	p := newLinearPlayer(q)
	return &questWalker{e: e, p: p, ctx: &linearCtx{mockActionContext: newMockActionContext(1), p: p}}
}

func (w *questWalker) at(step string) *questWalker { w.p.current = step; return w }

func (w *questWalker) notify(event string, d EventDetails) {
	d.UserId = 1
	w.p.roomId = d.RoomId
	w.e.Notify(event, d, w.p, w.ctx)
}

// typedCommand matches each command a hint tells the player to type.
var typedCommand = regexp.MustCompile(`(?i)\btype\s`)

// TestNewbieCertHintsNameOneCommand pins the ruling as a principle: in these
// cert quests every step but the end names exactly one command to type. The
// list is the cert quests that follow the ruling today; a newbie quest brought
// under the ruling joins it.
func TestNewbieCertHintsNameOneCommand(t *testing.T) {
	for _, tc := range []struct {
		file string
		id   int
	}{
		{"41-first_sign.yaml", 41},
		{"44-first_casting.yaml", 44},
		{"47-first_words.yaml", 47},
		{"50-first_shot.yaml", 50},
	} {
		q := loadShippedQuest(t, tc.file, tc.id)
		for _, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			n := len(typedCommand.FindAllString(s.Hint, -1))
			require.Equal(t, 1, n, "quest %d step %q hint must name exactly one command, got %d: %q",
				tc.id, s.Id, n, s.Hint)
		}
	}
}

// TestNewbieCertTriggersListenForSuccess: a refused command must not finish a
// step, so no trigger in these quests listens on command_issued.
func TestNewbieCertTriggersListenForSuccess(t *testing.T) {
	for _, tc := range []struct {
		file string
		id   int
	}{
		{"41-first_sign.yaml", 41},
		{"50-first_shot.yaml", 50},
	} {
		q := loadShippedQuest(t, tc.file, tc.id)
		require.NotEmpty(t, q.Triggers)
		for _, tr := range q.Triggers {
			require.NotEqual(t, "command_issued", tr.Event,
				"quest %d trigger on %q listens on command_issued, which a refusal also fires", tc.id, tr.Command)
		}
	}
}

func TestQuest41FirstSign_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	require.Equal(t, []string{"start", "track", "forage", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9136.yaml", "sign_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		// Foraging before tracking must not skip the track step.
		w.notify("command", EventDetails{Command: "forage", RoomId: 5284})
		require.Equal(t, "start", w.p.current, "forage out of order must not skip the track step")
		require.False(t, holdsAll(w.p, turnIn), "turn-in must not open from start")

		w.notify("command", EventDetails{Command: "track", RoomId: 5284})
		require.Equal(t, "track", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: 5285})
		require.Equal(t, "forage", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("player saved on track resumes", func(t *testing.T) {
		w := newQuestWalker(q).at("track")
		w.notify("command", EventDetails{Command: "forage", RoomId: 5284})
		require.Equal(t, "forage", w.p.current)
	})

	t.Run("player saved on forage can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("forage")
		require.True(t, holdsAll(w.p, turnIn))
	})
}

func TestQuest50FirstShot_OneCommandPerStep(t *testing.T) {
	const terrace = 5351
	q := loadShippedQuest(t, "50-first_shot.yaml", 50)
	steps := newLinearPlayer(q).steps
	// start, shoot and ambush are the ids a saved character may hold today.
	for _, old := range []string{"start", "shoot", "ambush", "end"} {
		require.Contains(t, steps, old, "step %q must survive so a player mid-quest resumes", old)
	}
	turnIn := turnInRequires(t, "9161.yaml", "shoot_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("command", EventDetails{Command: "equip", RoomId: 5350})
		require.Equal(t, "equip", w.p.current)

		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "shoot", w.p.current)

		// A second shot in the open is not the shot from cover.
		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "shoot", w.p.current, "a shot without sneaking first must not count as the cover shot")
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("command", EventDetails{Command: "sneak", RoomId: terrace})
		require.Equal(t, "ambush", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the shot from cover")

		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "cover_shot", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the shot from cover")
	})

	// Getting shot is not a step of its own: the quest hands a pouch over when
	// it starts, unless the player already carries one (Crier Toke's path in
	// quest 31 gives the pouch alongside the grant).
	t.Run("the grant hands over shot once", func(t *testing.T) {
		const shotPouch = 30064
		w := newQuestWalker(q).at("start")
		w.notify("quest_granted", EventDetails{QuestToken: "50-start", RoomId: 5350})
		require.Equal(t, []int{shotPouch}, w.ctx.givenItems, "a player with no shot gets a pouch")

		w = newQuestWalker(q).at("start")
		w.p.items[shotPouch] = true
		w.notify("quest_granted", EventDetails{QuestToken: "50-start", RoomId: 5350})
		require.Empty(t, w.ctx.givenItems, "a player who already carries shot gets no second pouch")
	})

	t.Run("player saved on start who already wields the sling can still fire", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "shoot", w.p.current)
	})

	t.Run("player saved on shoot resumes with sneak", func(t *testing.T) {
		w := newQuestWalker(q).at("shoot")
		w.notify("command", EventDetails{Command: "sneak", RoomId: terrace})
		require.Equal(t, "ambush", w.p.current)
	})

	t.Run("player saved on ambush resumes with the shot", func(t *testing.T) {
		w := newQuestWalker(q).at("ambush")
		require.False(t, holdsAll(w.p, turnIn))
		w.notify("command", EventDetails{Command: "fire", RoomId: terrace})
		require.Equal(t, "cover_shot", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})
}
