package questengine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
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

// turnInRequires reads the questRequired list of one hand-in topic from a
// shipped dialogue file (#375: the hand-in is a topic, keyed as the old tree
// node was).
func turnInRequires(t *testing.T, file string, key string) []string {
	t.Helper()
	df := loadCouleeTopics(t, file)
	tp := df.FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %s", file, key)
	require.Equal(t, dialogue.KindHandIn, tp.Kind, "%s topic %s is the quest's hand-in", file, key)
	require.NotEmpty(t, tp.QuestRequired, "%s topic %s gates on nothing", file, key)
	return tp.QuestRequired
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

// darkAside is the one sentence a step hint may carry besides its command:
// the owner-requested reminder on kill quests whose rooms go dark at night
// (2026-10-17 audit, B4). It is a precondition, not a second step, and it is
// stripped verbatim before counting so any other wording still counts. It
// names the spell by its alias, `cast glow` (owner 2026-10-21).
const darkAside = "If it is dark, type cast glow first."

// helpPointer is the other allowed extra: "For more, type help <topic>." The
// owner asked that a step explaining a setting point at its help file
// (2026-10-17), the same pattern the quest 28 tutorial hints use. Reading
// help is never the step, so it is stripped before counting.
var helpPointer = regexp.MustCompile(`(?i)\s*For more, type help [\w-]+\.`)

// newbieQuestStart matches a quest start granted in dialogue or a behaviour
// tree: `grantsQuest: "41-start"` or `quest: "29-start"`.
var newbieQuestStart = regexp.MustCompile(`(?m)^\s*(?:grantsQuest|quest):\s*"?(\d+)-start"?`)

// potholeCouleeQuests enumerates the newbie-area quests FROM DATA: every
// quest whose start token is granted by a Pothole Coulee dialogue file or
// behaviour tree, plus every quest those quests grant from their own
// triggers (quest 31's send-off grants the seven cert quests). A new coulee
// quest is covered the moment a coulee giver grants it.
func potholeCouleeQuests(t *testing.T) []*QuestDef {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	world := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")

	ids := map[int]bool{}
	for _, dir := range []string{"dialogue", "behaviors"} {
		files, err := filepath.Glob(filepath.Join(world, dir, "pothole_coulee", "*.yaml"))
		require.NoError(t, err)
		require.NotEmpty(t, files, "no %s files under pothole_coulee; the enumeration would prove nothing", dir)
		for _, f := range files {
			raw, err := os.ReadFile(f)
			require.NoError(t, err)
			for _, m := range newbieQuestStart.FindAllStringSubmatch(string(raw), -1) {
				id := 0
				for _, r := range m[1] {
					id = id*10 + int(r-'0')
				}
				ids[id] = true
			}
		}
	}

	byId := map[int]*QuestDef{}
	files, err := filepath.Glob(filepath.Join(world, "quests", "*.yaml"))
	require.NoError(t, err)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		var q QuestDef
		require.NoError(t, yaml.Unmarshal(raw, &q), "parse %s", f)
		qq := q
		byId[q.QuestId] = &qq
	}

	// Close over grants made by the coulee quests' own triggers.
	for changed := true; changed; {
		changed = false
		for id := range ids {
			q := byId[id]
			if q == nil {
				continue
			}
			for _, tr := range q.Triggers {
				for _, a := range tr.Actions {
					if !strings.HasSuffix(a.Grant, "-start") {
						continue
					}
					gid, _ := (&linearPlayer{}).split(a.Grant)
					if gid > 0 && !ids[gid] {
						ids[gid] = true
						changed = true
					}
				}
			}
		}
	}

	var out []*QuestDef
	for id := range ids {
		q := byId[id]
		require.NotNil(t, q, "a coulee giver grants %d-start but no quest file defines quest %d", id, id)
		out = append(out, q)
	}
	require.GreaterOrEqual(t, len(out), 30, "found only %d coulee quests; the enumeration is broken", len(out))
	return out
}

// walksToNextStep reports whether the trigger that finishes step i is a
// room_enter: a walk, which has no command to type.
func walksToNextStep(q *QuestDef, i int) bool {
	if i+1 >= len(q.Steps) {
		return false
	}
	next := strings.ToLower(q.Steps[i+1].Id)
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			if strings.HasSuffix(a.Grant, "-"+next) && tr.Event == "room_enter" {
				return true
			}
		}
	}
	return false
}

// TestNewbieQuestHintsNameOneCommand pins the owner ruling (2026-10-17) as a
// principle over every Pothole Coulee quest: each step but the end names
// exactly one command to type. A step finished by walking into a room may
// name none.
func TestNewbieQuestHintsNameOneCommand(t *testing.T) {
	for _, q := range potholeCouleeQuests(t) {
		for i, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			hint := helpPointer.ReplaceAllString(strings.ReplaceAll(s.Hint, darkAside, ""), "")
			n := len(typedCommand.FindAllString(hint, -1))
			if n == 0 && walksToNextStep(q, i) {
				continue
			}
			if n != 1 {
				t.Errorf("quest %d (%s) step %q hint must name exactly one command, got %d: %q",
					q.QuestId, q.Name, s.Id, n, s.Hint)
			}
		}
	}
}

// TestNewbieQuestTriggersListenForSuccess: a refused command must not finish
// a step, so no trigger in any Pothole Coulee quest listens on command_issued.
func TestNewbieQuestTriggersListenForSuccess(t *testing.T) {
	for _, q := range potholeCouleeQuests(t) {
		for _, tr := range q.Triggers {
			if tr.Event == "command_issued" {
				t.Errorf("quest %d (%s) trigger on %q listens on command_issued, which a refusal also fires",
					q.QuestId, q.Name, tr.Command)
			}
		}
	}
}

// TestNewbieQuestTriggersGateOnThePreviousStep: steps run in order. A trigger
// that grants step i must require a token from an earlier step (at least
// -start), and a trigger that grants a step past the second must not gate on
// -start alone unless the quest documents why (lenientGrants), because a
// later objective done first would mark the earlier ones done.
func TestNewbieQuestTriggersGateOnThePreviousStep(t *testing.T) {
	for _, q := range potholeCouleeQuests(t) {
		idx := map[string]int{}
		for i, s := range q.Steps {
			idx[strings.ToLower(s.Id)] = i
		}
		for _, tr := range q.Triggers {
			for _, a := range tr.Actions {
				id, step := (&linearPlayer{}).split(a.Grant)
				if id != q.QuestId {
					continue
				}
				if retiredSteps[strings.ToLower(a.Grant)] {
					t.Errorf("quest %d (%s): %s is retired (kept for saved players) but a trigger still grants it",
						q.QuestId, q.Name, a.Grant)
					continue
				}
				gi, ok := idx[strings.ToLower(step)]
				if !ok || gi <= 1 {
					continue // start, or the first step after it
				}
				if lenientGrants[a.Grant] {
					continue
				}
				pi := gi - 1
				for pi > 0 && retiredSteps[fmt.Sprintf("%d-%s", q.QuestId, strings.ToLower(q.Steps[pi].Id))] {
					pi--
				}
				prev := strings.ToLower(q.Steps[pi].Id)
				want := fmt.Sprintf("%d-%s", q.QuestId, prev)
				has := false
				for _, h := range tr.Conditions.Has {
					if strings.EqualFold(h, want) {
						has = true
					}
				}
				if !has {
					t.Errorf("quest %d (%s): the trigger granting %s must gate on %s, the step before it (has: %v)",
						q.QuestId, q.Name, a.Grant, want, tr.Conditions.Has)
				}
			}
		}
	}
}

// lenientGrants are the grants that gate on an EARLIER step than the one
// before them, on purpose: the command cannot succeed unless the skipped
// steps' result is already in hand (a shot needs a readied weapon), so a
// player saved before a step was inserted is never held on a step whose
// lesson they already learned.
var lenientGrants = map[string]bool{
	"50-shoot": true, // fire needs the sling readied (the equip step)
}

// retiredSteps are steps no trigger grants any more. Each stays in its
// quest's step list only so a character saved on it resumes, so the step
// gate skips over it when it looks for the step before a grant. None today:
// quest 32's old consider step was renamed to special in step-ids.txt
// (#375 phase 7), which moves a saved player on instead of keeping the step.
var retiredSteps = map[string]bool{}

func TestQuest41FirstSign_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	require.Equal(t, []string{"start", "met", "track", "forage", "end"}, newLinearPlayer(q).steps,
		"start, track and forage are what a saved character stores; met is inserted after start")
	turnIn := turnInRequires(t, "9136.yaml", "sign_turnin")
	heard := EventDetails{Command: "", MobId: 9136, Topic: "sign_arrive", RoomId: 5283}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "track", RoomId: 5284})
		require.Equal(t, "start", w.p.current, "tracking before Tarn has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: 5284})
		require.Equal(t, "met", w.p.current, "forage out of order must not skip the track step")
		require.False(t, holdsAll(w.p, turnIn), "turn-in must not open early")

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

		// The equip event names its item (2026-10-17 newbie-duo playtest:
		// `wield cudgel` printed "The sling settles in your hand").
		w.notify("command", EventDetails{Command: "equip", RoomId: 5350, ItemId: 10043})
		require.Equal(t, "start", w.p.current, "wielding the cudgel must not finish the sling step")
		w.notify("command", EventDetails{Command: "equip", RoomId: 5350, ItemId: 10038})
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
