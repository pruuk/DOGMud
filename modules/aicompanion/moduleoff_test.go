package aicompanion

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// logTee keeps every line the logger writes, so a test can read what an
// operator would have seen in the server log.
type logTee struct {
	mu    sync.Mutex
	lines []string
}

func (l *logTee) Println(level string, v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, level+` `+fmt.Sprint(v...))
}

func (l *logTee) contains(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		if strings.Contains(line, s) {
			return true
		}
	}
	return false
}

// The engine raises Emote, Healed and GoldGiven for this module alone. Switched off,
// it used to register nothing, so every emote and every heal cast at a
// creature was logged as an event nobody handled.
func TestSwitchedOffStillHearsItsOwnEvents(t *testing.T) {
	events.ClearListeners()
	t.Cleanup(events.ClearListeners)
	tee := &logTee{}
	mudlog.SetupLogger(tee, "", "", false)
	t.Cleanup(func() { mudlog.SetupLogger(nil, "", "", false) })

	// The test binary reads no config.yaml, so the module loads switched
	// off, exactly as a server that has not asked for it.
	m := &AICompanionModule{plug: module.plug, ctrls: map[int]*controller{}}
	m.onLoad()
	if m.cfg.Enabled {
		t.Fatal("fixture: the module must load switched off")
	}

	// The engine samples the complaint once per NoListenerSampleSize.
	for range events.NoListenerSampleSize {
		events.DoListeners(events.Emote{UserId: 1, RoomId: 1, Text: `waves`})
		events.DoListeners(events.Healed{HealerUserId: 1, MobInstanceId: 42})
		events.DoListeners(events.GoldGiven{UserId: 1, MobInstanceId: 42, Amount: 5})
	}
	if tee.contains(`no listener for event`) {
		t.Fatal("an emote, a heal or gold given with the module off must not be logged as unhandled")
	}

	// And the null probe: an event nobody listens to is still reported, so
	// the silence above is the module's doing.
	for range events.NoListenerSampleSize {
		events.DoListeners(events.GiftAccepted{})
	}
	if !tee.contains(`no listener for event`) {
		t.Fatal("probe: an unheard event must still be logged, or this test proves nothing")
	}
}
