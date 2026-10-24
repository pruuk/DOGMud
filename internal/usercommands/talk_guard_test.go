package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The talk guards (#375 phase 1): ask only redirects to talk, and neither
// talk, ask nor internal/conversation reaches the LLM client.

var askTopicHandling = regexp.MustCompile(`dialogue\.|TreeAdvance|\.Match\(|player_ask`)

// askHasTopicHandling reports topic handling in code; a // comment that
// names a retired symbol is not code (stripLineComments, menu_guard_test.go).
func askHasTopicHandling(src string) bool {
	return askTopicHandling.MatchString(stripLineComments(src))
}

func usesLLM(src string) bool {
	return strings.Contains(src, `"github.com/GoMudEngine/GoMud/internal/llm"`)
}

func TestGuard_AskHasNoTopicHandling(t *testing.T) {
	src, err := os.ReadFile("ask.go")
	if err != nil {
		t.Fatal(err)
	}
	if askHasTopicHandling(string(src)) {
		t.Fatal("ask.go handles topics again; ask only redirects to talk (#375 requirements rule 7)")
	}
}

func TestGuard_AskGuardCanFail(t *testing.T) {
	if !askHasTopicHandling("df := dialogue.Load(1, z)") {
		t.Fatal("the ask guard cannot see topic handling")
	}
	if askHasTopicHandling("// dialogue.Load is gone") {
		t.Fatal("the ask guard trips on a comment")
	}
}

func TestGuard_TalkAndAskMakeNoLLMCalls(t *testing.T) {
	files := []string{"talk.go", "ask.go"}
	more, err := filepath.Glob(filepath.Join("..", "conversation", "*.go"))
	if err != nil || len(more) == 0 {
		t.Fatalf("found no internal/conversation files (err %v): the guard cannot run", err)
	}
	files = append(files, more...)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if usesLLM(string(src)) {
			t.Errorf("%s imports internal/llm; LLM dialogue is out of this arc (R4, #643)", f)
		}
	}
}

func TestGuard_LLMGuardCanFail(t *testing.T) {
	if !usesLLM("import \"github.com/GoMudEngine/GoMud/internal/llm\"") {
		t.Fatal("the LLM guard cannot see the import")
	}
}
