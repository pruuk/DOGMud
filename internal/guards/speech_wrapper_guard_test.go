package guards

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Sight gates slice 5b moved every speech and emote rule into shared bodies
// in internal/actions (Say, Shout, SendHeard, SendSeen). Before that, the
// player and mob wrappers each sent their own room line, and they had forked:
// player speech named the speaker in any light, mob speech was two-tier with a
// lit-room shortcut that named the speaker to a blinded listener, and mob
// shouts neither revealed the shouter nor carried their words next door.
// These tests fail if a wrapper sends a room line, hides a name, judges sight,
// reveals, walks the neighbours or wakes sleepers itself again.

var speechWrapperFiles = []string{
	"internal/usercommands/say.go", "internal/mobcommands/say.go",
	"internal/usercommands/shout.go", "internal/mobcommands/shout.go",
	"internal/usercommands/rally.go", "internal/mobcommands/rally.go",
	"internal/usercommands/warcry.go", "internal/mobcommands/warcry.go",
	"internal/usercommands/emote.go", "internal/mobcommands/emote.go",
}

var speechWrapperForbidden = regexp.MustCompile(`SendTextCommunication|sendAudioRoomText|HideNames|HideSpeakerNames|ParticipantSight|TransitionToRevealing|ForEachAdjacentRoom|OnSleeperWoken|HidingNames|room\.SendText\(|room\.SendTextVisual\(`)

// speechWrapperRequired is the shared body each wrapper must call, and how
// many times: one call per room line the wrapper sends. A file-level match
// alone would stay green if one branch (emote's free-form line, say) stopped
// sending while a sibling branch still called the body.
var speechWrapperRequired = map[string]struct {
	call  *regexp.Regexp
	count int
}{
	"internal/usercommands/say.go":    {regexp.MustCompile(`actions\.Say\(`), 1},
	"internal/mobcommands/say.go":     {regexp.MustCompile(`actions\.Say\(`), 1},
	"internal/usercommands/shout.go":  {regexp.MustCompile(`actions\.Shout\(`), 1},
	"internal/mobcommands/shout.go":   {regexp.MustCompile(`actions\.Shout\(`), 1},
	"internal/usercommands/rally.go":  {regexp.MustCompile(`actions\.SendHeard\(`), 2},
	"internal/mobcommands/rally.go":   {regexp.MustCompile(`actions\.SendHeard\(`), 1},
	"internal/usercommands/warcry.go": {regexp.MustCompile(`actions\.SendHeard\(`), 2},
	"internal/mobcommands/warcry.go":  {regexp.MustCompile(`actions\.SendHeard\(`), 1},
	"internal/usercommands/emote.go":  {regexp.MustCompile(`actions\.SendSeen\(`), 3},
	"internal/mobcommands/emote.go":   {regexp.MustCompile(`actions\.SendSeen\(`), 2},
}

// speechGuardCode is path's Go source with every comment removed: parsed
// without ParseComments and printed back, so a comment that names a
// forbidden word cannot fail the guard and one that names a required call
// cannot pass it.
func speechGuardCode(path string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestSpeechWrappersDoNotReFork(t *testing.T) {
	for _, path := range speechWrapperFiles {
		code, err := speechGuardCode(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if loc := speechWrapperForbidden.FindStringIndex(code); loc != nil {
			t.Errorf("%s handles speech sight or delivery itself (%q); call the shared body in internal/actions instead",
				path, code[loc[0]:loc[1]])
		}
		want := speechWrapperRequired[path]
		if got := len(want.call.FindAllStringIndex(code, -1)); got != want.count {
			t.Errorf("%s calls %s %d times, want %d: one per room line it sends",
				path, want.call, got, want.count)
		}
	}
}

// speechGuardWalk calls fn with the comment-free code of every production Go
// file under internal/ and modules/.
func speechGuardWalk(t *testing.T, fn func(rel, code string)) {
	t.Helper()
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			code, perr := speechGuardCode(path)
			if perr != nil {
				// A syntax error is the compiler's to report, and another
				// test may create and remove a scratch file mid-walk.
				return nil
			}
			fn(filepath.ToSlash(path), code)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}

// The two deafen-marked senders carry player chatter only. A caller outside
// the rooms package that defines them and the actions package that owns the
// shared bodies could mark NPC speech as chatter, which ruling 6 forbids.
func TestOnlyTheSharedBodiesSendPlayerChatter(t *testing.T) {
	pattern := regexp.MustCompile(`SendCommunicationHidingNames|SendVisualCommunicationHidingNames`)
	seenInActions := false
	speechGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if strings.HasPrefix(rel, "internal/actions/") {
			seenInActions = true
			return
		}
		if !strings.HasPrefix(rel, "internal/rooms/") {
			t.Errorf("%s calls a deafen-marked sender; only internal/actions may", rel)
		}
	})
	if !seenInActions {
		t.Fatal("no call found in internal/actions: the pattern cannot match, so this guard proves nothing")
	}
}

// actions.Say owns the say room line. A second formatter call is a second
// room line: hooks/justice_wiring.go, actions/sell.go and
// usercommands/offer.go each sent one until this slice.
func TestSayRoomLineHasOneFormatter(t *testing.T) {
	pattern := regexp.MustCompile(`FormatSayText\(`)
	seen := false
	speechGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if rel == "internal/actions/say.go" {
			seen = true
			return
		}
		t.Errorf("%s formats a say line itself; call actions.Say", rel)
	})
	if !seen {
		t.Fatal("FormatSayText not found in internal/actions/say.go: the pattern cannot match")
	}
}
