package hooks

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// screenCapture is a telnet connection whose client end records every byte
// the server writes.
type screenCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *screenCapture) bytes() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func captureConnection(t *testing.T) (uint64, *screenCapture) {
	t.Helper()
	server, client := net.Pipe()
	cd := connections.Add(server, nil)
	sc := &screenCapture{}
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := client.Read(chunk)
			if n > 0 {
				sc.mu.Lock()
				sc.buf.Write(chunk[:n])
				sc.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = connections.Remove(cd.ConnectionId())
		_ = client.Close()
	})
	return cd.ConnectionId(), sc
}

// visibleLastLine is what a telnet screen shows on its last line: the text
// after the final newline, with everything before the last
// cursor-to-column-1 + erase-line pair wiped.
func visibleLastLine(screen string) string {
	if i := strings.LastIndex(screen, "\n"); i >= 0 {
		screen = screen[i+1:]
	}
	if i := strings.LastIndex(screen, "\x1b[2K"); i >= 0 {
		screen = screen[i+len("\x1b[2K"):]
	}
	return screen
}

// TestRedraw_ANewPromptQuestionIsTheLastThingOnScreen replays the stake
// prompt the playtest lost (#375 R14, seat C). A command opens a prompt and
// asks a question while its own lines (the player's say, the NPC's answer,
// the guidance) are still queued. Each queued line erases the prompt line
// before it prints, so the question must be redrawn after them. It was not
// when the same question had been drawn before (an earlier stake): the redraw
// dedup compared against that old draw and skipped it, leaving the question
// wiped and the player typing an answer to nothing.
func TestRedraw_ANewPromptQuestionIsTheLastThingOnScreen(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	events.DrainAllQueuedEventsForTest()

	connId, screen := captureConnection(t)
	u := users.NewTestUser(1, "alice", "Aliceia", connId)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: u})
	defer cleanupUsers()

	const question = `How much gold will you stake?`
	ask := func() {
		p, _ := u.StartPrompt(`portalstake`, ``)
		p.Ask(question, []string{})
	}
	speak := func(lines ...string) {
		for _, l := range lines {
			Message_SendMessage(events.Message{UserId: 1, Text: l + "\n"})
		}
		RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1})
	}

	for stake := 1; stake <= 2; stake++ {
		ask()
		speak(`You say, "Open a rift to the Arena."`, `Sable says, "The Arena, then."`, `The more gold you stake, the harder the Arena fights.`)
		require.Eventually(t, func() bool {
			return strings.Contains(visibleLastLine(screen.bytes()), question)
		}, time.Second, 10*time.Millisecond,
			"stake %d: the question is not the last line on screen; last line %q", stake, visibleLastLine(screen.bytes()))
		// The player answers; the prompt closes.
		u.ClearPrompt()
		speak(`Sable says, "That barely covers the runes."`)
	}
}
