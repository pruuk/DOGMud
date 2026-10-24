package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"gopkg.in/yaml.v3"
)

// M6 slice 3: the wordless pools are read by a taunter or counterer with no
// voice for words (and the howl pool by any howler), so no line may speak,
// and each band of each role carries enough variety to read as more than one
// fixed sound.
var wordlessPoolSpeechWord = regexp.MustCompile(`(?i)\b(word|words|insult|mock|mocks|taunt|taunts|say|says|speak|shout|jeer|sneer)\b`)

const wordlessPoolMinVariants = 6

var wordlessPoolFiles = []string{
	"taunt-messages/wordless.yaml",
	"taunt-messages/howl.yaml",
	"defense-messages/counter-defy-wordless.yaml",
}

// wordlessPoolProblems walks a parsed pool and reports every string list
// shorter than the minimum and every line that names speech. lists counts the
// string lists it saw, so a caller can prove the walk reached the variants.
func wordlessPoolProblems(node any, path string, lists *int) []string {
	var problems []string
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			problems = append(problems, wordlessPoolProblems(v, path+"/"+k, lists)...)
		}
	case map[any]any:
		for k, v := range n {
			problems = append(problems, wordlessPoolProblems(v, path+"/"+toString(k), lists)...)
		}
	case []any:
		allStrings := len(n) > 0
		for i, v := range n {
			s, ok := v.(string)
			if !ok {
				allStrings = false
				problems = append(problems, wordlessPoolProblems(v, path, lists)...)
				continue
			}
			if m := wordlessPoolSpeechWord.FindString(s); m != "" {
				problems = append(problems, path+": names speech ("+m+") in variant "+toString(i)+": "+s)
			}
		}
		if allStrings {
			*lists++
			if len(n) < wordlessPoolMinVariants {
				problems = append(problems, path+": only "+toString(len(n))+" variants")
			}
		}
	}
	return problems
}

func toString(v any) string {
	return fmt.Sprint(v)
}

func TestWordlessPoolsSpeakNoWordsAndCarryVariety(t *testing.T) {
	// The checker must be able to fail: a short list and a speaking line are
	// both caught, and a clean six-line list is not.
	var probeLists int
	bad := map[any]any{"hit": map[any]any{
		"actor": []any{"You snarl.", "You say something."},
	}}
	if got := wordlessPoolProblems(bad, "probe", &probeLists); len(got) != 2 || probeLists != 1 {
		t.Fatalf("checker cannot tell a bad pool (got %v, %d lists); the guard would prove nothing", got, probeLists)
	}
	clean := map[any]any{"hit": map[any]any{
		"actor": []any{"a", "b", "c", "d", "e", "f"},
	}}
	if got := wordlessPoolProblems(clean, "probe", &probeLists); len(got) != 0 {
		t.Fatalf("checker flags a clean pool: %v", got)
	}

	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")
	for _, rel := range wordlessPoolFiles {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		var doc any
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		lists := 0
		for _, p := range wordlessPoolProblems(doc, rel, &lists) {
			t.Error(p)
		}
		// Taunt pools hold four bands of three roles; counter pools three.
		if lists < 9 {
			t.Errorf("%s: walked only %d variant lists; the guard checked too little", rel, lists)
		}
	}
}
