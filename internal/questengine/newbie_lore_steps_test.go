package questengine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// topicHints is the lowercased hint of one topic in a Pothole Coulee
// dialogue file.
func topicHints(t *testing.T, file, key string) string {
	t.Helper()
	tp := loadCouleeTopics(t, file).FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %s", file, key)
	return strings.ToLower(tp.Hints)
}

// roomDescription is a shipped room's description.
func roomDescription(t *testing.T, zoneFolder string, roomId int) string {
	t.Helper()
	var room struct {
		Description string `yaml:"description"`
	}
	require.NoError(t, yaml.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "rooms", zoneFolder, fmt.Sprintf("%d.yaml", roomId)), &room))
	return room.Description
}

// The lore line (Pothole Coulee quests 47, 48, 49 and the repeatable 58)
// under #375 phase 7: giver to giver. Toke offers 47 and Wenna takes it;
// Wenna offers 48 and Hale takes it; Hale offers 49 and Sere takes it; Sere
// keeps the repeatable 58 and the bridge 95. One command per step; every
// quest after the first opens with the walk to the giver who takes it.

func loreStepHint(t *testing.T, q *QuestDef, step string) string {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == step {
			return s.Hint
		}
	}
	t.Fatalf("quest %d has no step %q", q.QuestId, step)
	return ""
}

// loreOnlyFinalStepReports: the report back to the giver belongs to the last
// step before end. A meeting step (start, when the next step is met) also
// teaches talk, to the giver the player is walking to, and is left out.
func loreOnlyFinalStepReports(t *testing.T, q *QuestDef, giver string) {
	t.Helper()
	report := "type talk " + giver
	meeting := len(q.Steps) > 1 && q.Steps[1].Id == "met"
	last := q.Steps[len(q.Steps)-2].Id
	for _, s := range q.Steps {
		if s.Id == "end" || (meeting && s.Id == "start") {
			continue
		}
		has := strings.Contains(strings.ToLower(s.Hint), report)
		if s.Id == last {
			require.True(t, has, "quest %d last step %q must tell the player to type %s: %q", q.QuestId, s.Id, report, s.Hint)
		} else {
			require.False(t, has, "quest %d step %q previews the report; it belongs to the last step only: %q", q.QuestId, s.Id, s.Hint)
		}
	}
}

// loreOfferOn: the quest's offer sits on this lore giver and on no other.
func loreOfferOn(t *testing.T, id int, file string) {
	t.Helper()
	for _, f := range []string{"9153.yaml", "9157.yaml", "9160.yaml"} {
		has := false
		for _, tp := range loadCouleeTopics(t, f).Topics {
			if tp.Kind == dialogue.KindOffer && tp.Quest == id {
				has = true
			}
		}
		require.Equal(t, f == file, has, "quest %d's offer belongs on %s only (on %s: %v)", id, file, f, has)
	}
}

func TestQuest47FirstWords_GiverToGiver(t *testing.T) {
	q := loadShippedQuest(t, "47-first_words.yaml", 47)
	require.Equal(t, []string{"start", "met", "help", "end"}, newLinearPlayer(q).steps,
		"start and help keep their ids so a saved player resumes; met is the meeting with Wenna")
	turnIn := turnInRequires(t, "9153.yaml", "words_turnin")
	require.Equal(t, []string{"47-help"}, turnIn, "the turn-in gates on the last step")
	loreOnlyFinalStepReports(t, q, "wenna")
	require.Equal(t, "Type help.", strings.TrimSpace(loreStepHint(t, q, "met")))
	require.Contains(t, squash(loreStepHint(t, q, "start")), "east, then north six times to the commons yard")
	require.Equal(t, 5324, foldingWalk(t, foldingHubSquare, "east", "north", "north", "north", "north", "north", "north"))

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "help", RoomId: 5200})
		require.Equal(t, "start", w.p.current, "help before Wenna's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9153, Topic: "words_lesson"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", EventDetails{Command: "say", RoomId: 5324})
		require.Equal(t, "met", w.p.current, "another command must not finish the help lesson")
		// Help works anywhere, which is the lesson (owner 2026-10-17).
		w.notify("command", EventDetails{Command: "help", RoomId: 5200})
		require.Equal(t, "help", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on help can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("help").p, turnIn))
	})

	t.Run("Wenna's greeting and answers follow the step", func(t *testing.T) {
		f := "47-first_words.yaml"
		pre := []string{"30-end", "31-end"}
		checkMeetStep(t, "wenna", "9153.yaml", f, 47, "words_lesson", "type help", pre...)
		checkTopicSteps(t, "wenna", "9153.yaml", []topicStep{
			{f, 47, "met", "type help", pre, "words_progress", false},
			{f, 47, "help", "", pre, "words_turnin", true},
		})
	})

	// #690: nothing the player did was called "the reckoning"; the reckoning
	// is Hale's carved bone.
	t.Run("Wenna never calls the help the reckoning", func(t *testing.T) {
		raw := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9153.yaml")))
		require.NotContains(t, raw, "reckoning")
	})
}

func TestQuest48StandingStones_GiverToGiver(t *testing.T) {
	const green, circle, garrow, ferd = 5335, 5333, 9158, 9159
	f := "48-the_standing_stones.yaml"
	q := loadShippedQuest(t, f, 48)
	require.Equal(t, []string{"start", "met", "taunt", "end"}, newLinearPlayer(q).steps,
		"start and taunt keep their ids so a saved player resumes; met is the meeting with Hale")
	turnIn := turnInRequires(t, "9157.yaml", "stones_turnin")
	require.Equal(t, []string{"48-taunt"}, turnIn)
	loreOnlyFinalStepReports(t, q, "hale")
	loreOfferOn(t, 48, "9153.yaml")
	require.Contains(t, squash(loreStepHint(t, q, "met")), "type taunt garrow")
	require.Contains(t, squash(loreStepHint(t, q, "start")),
		"north, west, north, north and north to the outer ring, then west into the stone circle")
	require.Equal(t, circle, foldingWalk(t, 5324, "north", "west", "north", "north", "north", "west"))

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "taunt", RoomId: green, MobId: garrow})
		require.Equal(t, "start", w.p.current, "a taunt before Hale's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9157, Topic: "stones_meet"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", EventDetails{Command: "taunt", RoomId: circle, MobId: garrow})
		require.Equal(t, "met", w.p.current, "a taunt away from the green must not count")
		w.notify("command", EventDetails{Command: "taunt", RoomId: green, MobId: ferd})
		require.Equal(t, "met", w.p.current, "a taunt at Ferd, not Garrow, must not count")
		w.notify("command", EventDetails{Command: "taunt", RoomId: green})
		require.Equal(t, "met", w.p.current, "a taunt at no one must not count")
		w.notify("command", EventDetails{Command: "taunt", RoomId: green, MobId: garrow})
		require.Equal(t, "taunt", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("Hale's greeting and answers follow the step", func(t *testing.T) {
		checkMeetStep(t, "hale", "9157.yaml", f, 48, "stones_meet", "type taunt garrow", "30-end", "47-end")
		checkTopicSteps(t, "hale", "9157.yaml", []topicStep{
			{f, 48, "met", "type taunt garrow", []string{"47-end"}, "stones_progress", false},
			{f, 48, "taunt", "", []string{"47-end"}, "stones_turnin", true},
		})
	})

	t.Run("Hale teaches rally and warcry", func(t *testing.T) {
		hale := loadCouleeTopics(t, "9157.yaml")
		for key, cmd := range map[string]string{"rally": "type rally", "warcry": "type warcry"} {
			tp := hale.FindTopic(key)
			require.NotNil(t, tp, "Hale has no %s topic", key)
			require.Equal(t, dialogue.KindInfo, tp.Kind)
			require.Contains(t, squash(tp.Hints), cmd)
		}
	})
}

// searchIn is the search command in a room: since #375 phase 7 quests 49 and
// 58 listen for the command itself, any search in the Reliquary.
func searchIn(roomId int) EventDetails {
	return EventDetails{Command: "search", RoomId: roomId}
}

func TestQuest49OldShrine_GiverToGiver(t *testing.T) {
	const reliquary, nave, shrine, circle = 5343, 5342, 5341, 5333
	f := "49-the_old_shrine.yaml"
	q := loadShippedQuest(t, f, 49)
	require.Equal(t, []string{"start", "met", "find", "end"}, newLinearPlayer(q).steps,
		"start and find keep their ids so a saved player resumes; met is the meeting with Sere")
	turnIn := turnInRequires(t, "9160.yaml", "shrine_turnin")
	require.Equal(t, []string{"49-find"}, turnIn)
	loreOnlyFinalStepReports(t, q, "sere")
	loreOfferOn(t, 49, "9157.yaml")

	// #689: the way from the Stone Circle is north twice, west, north twice.
	require.Contains(t, squash(loreStepHint(t, q, "start")),
		"north twice to the tall stone, then west, then north twice to the old shrine")
	require.Equal(t, shrine, foldingWalk(t, circle, "north", "north", "west", "north", "north"))

	met := strings.ToLower(loreStepHint(t, q, "met"))
	require.True(t, strings.HasPrefix(met, "from the old shrine"),
		"hint can be typed anywhere, so the directions start from where Sere gives the lesson")
	require.Contains(t, met, "type search floor")
	require.Contains(t, met, "type help search", "the lesson points at its help page")
	require.NotContains(t, met, "just search", "plain search counts as fully, so the hint no longer ranks the two")
	require.NotContains(t, met, "look at", "looking at the find is not a step")
	require.NotContains(t, met, "go up", "the Reliquary is west then north of the shrine, not up")
	require.Equal(t, reliquary, foldingWalk(t, shrine, "west", "north"))
	require.Equal(t, shrine, foldingWalk(t, reliquary, "south", "east"), "the way back to Sere")

	t.Run("the Reliquary hints at the floor, never instructs, and keeps no disc", func(t *testing.T) {
		desc := strings.ToLower(roomDescription(t, "pothole_coulee", reliquary))
		require.Contains(t, desc, "flagstone floor")
		require.NotContains(t, desc, "search", "a room hints at its secret; the step hint teaches the command (ruling 7)")
		require.NotContains(t, desc, "lying under", "no line promises a find the room no longer holds")
		room := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "rooms", "pothole_coulee", "5343.yaml")))
		require.NotContains(t, room, "disc", "the disc hidden noun moved out with quest 76 (#625)")
		require.NotContains(t, room, "admit a finger", "no line promises a find the room no longer holds")
	})

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", searchIn(reliquary))
		require.Equal(t, "start", w.p.current, "a search before Sere's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9160, Topic: "shrine_meet"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", searchIn(nave))
		require.Equal(t, "met", w.p.current, "a search outside the Reliquary must not count")
		w.notify("room_interact", EventDetails{RoomId: reliquary, Verb: "look", Noun: "floor", RoomNoun: "floor"})
		require.Equal(t, "met", w.p.current, "looking at the floor is not searching it")
		w.notify("command", searchIn(reliquary))
		require.Equal(t, "find", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
		require.Empty(t, w.ctx.discovered, "there is no hidden noun to discover any more")
	})

	t.Run("Sere's greeting and answers follow the step", func(t *testing.T) {
		checkMeetStep(t, "sere", "9160.yaml", f, 49, "shrine_meet", "type search floor", "30-end", "47-end", "48-end")
		checkTopicSteps(t, "sere", "9160.yaml", []topicStep{
			{f, 49, "met", "type search floor", []string{"47-end", "48-end"}, "shrine_progress", false},
			{f, 49, "find", "", []string{"47-end", "48-end"}, "shrine_turnin", true},
		})
	})

	// A character saved before the meeting step may hold 49-start and 49-find
	// with no 49-met. Sere must not greet it as a stranger or teach the lesson
	// again, and Hale must not send it back to her.
	t.Run("a player saved on find without the meeting is not sent back", func(t *testing.T) {
		// Tokens are held outright: playerOn(q, ...) derives quest 49's own
		// tokens from the step, so it would grant 49-met too. A player on the
		// bridge's quest keeps quest 49's tokens as plain held tokens.
		bridge := loadShippedQuest(t, "95-report_to_toke__lore_.yaml", 95)
		old := func() *linearPlayer {
			return playerOn(bridge, "", "47-end", "48-end", "49-start", "49-find")
		}
		require.False(t, old().HasQuest("49-met"), "the fixture holds no meeting token")
		sere := newNpcTalker(loadCouleeTopics(t, "9160.yaml"), old())
		_, hints := sere.opener()
		require.Contains(t, squash(hints), "choose the old shrine", "Sere asks for the report")
		require.Nil(t, sere.eligible("shrine_meet"), "the lesson is behind this player")
		require.NotNil(t, sere.eligible("shrine_turnin"), "the hand-in is open")
		hale := newNpcTalker(loadCouleeTopics(t, "9157.yaml"), old())
		_, hints = hale.opener()
		require.NotContains(t, squash(hints), "type talk sere", "Hale does not point at Sere again")
	})

	t.Run("Hale stops sending a player who has met Sere", func(t *testing.T) {
		hale := newNpcTalker(loadCouleeTopics(t, "9157.yaml"), playerOn(q, "met", "47-end", "48-end"))
		_, hints := hale.opener()
		require.NotContains(t, squash(hints), "type talk sere", "the meeting is done; Hale no longer sends the player to Sere")
		_, hints = newNpcTalker(loadCouleeTopics(t, "9157.yaml"), playerOn(q, "start", "47-end", "48-end")).opener()
		require.Contains(t, squash(hints), "type talk sere", "before the meeting Hale still points at Sere")
	})
}

func TestQuest58ReadingTheStones_ReportsToSere(t *testing.T) {
	const reliquary = 5343
	f := "58-reading_the_stones.yaml"
	q := loadShippedQuest(t, f, 58)
	require.True(t, q.Repeatable)
	require.Equal(t, []string{"start", "read", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9160.yaml", "stones_loop_turnin")
	require.Equal(t, []string{"58-read"}, turnIn)
	require.Contains(t, squash(loreStepHint(t, q, "start")), "type search floor")
	require.NotContains(t, strings.ToLower(loreStepHint(t, q, "start")), "just search")
	require.Contains(t, squash(loreStepHint(t, q, "read")), "type talk sere")

	w := newQuestWalker(q).at("start")
	w.notify("command", searchIn(5342))
	require.Equal(t, "start", w.p.current, "a search outside the Reliquary must not count")
	w.notify("command", searchIn(reliquary))
	require.Equal(t, "read", w.p.current)
	require.True(t, holdsAll(w.p, turnIn))

	// A real character on 49-end holds every earlier 48 and 49 token too.
	pre := []string{"47-end", "48-start", "48-met", "48-taunt", "48-end", "49-start", "49-met", "49-find", "49-end"}
	checkTopicSteps(t, "sere", "9160.yaml", []topicStep{
		{f, 58, "start", "type search floor", pre, "stones_loop_progress", false},
		{f, 58, "read", "", pre, "stones_loop_turnin", true},
	})
}
