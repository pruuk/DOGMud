package gmcp

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// questBuilderJS is the quest editor; its EVENT_FILTERS lists are the only
// trigger fields it reads back on save.
const questBuilderJS = "../../_datafiles/html/public/static/js/quests.js"

// builderEventFilters parses EVENT_FILTERS out of the quest editor.
func builderEventFilters(t *testing.T) map[string][]string {
	t.Helper()
	src, err := os.ReadFile(questBuilderJS)
	if err != nil {
		t.Fatalf("read %s: %v", questBuilderJS, err)
	}
	block := regexp.MustCompile(`(?s)var EVENT_FILTERS = \{(.*?)\};`).FindSubmatch(src)
	if block == nil {
		t.Fatalf("EVENT_FILTERS not found in %s", questBuilderJS)
	}
	out := map[string][]string{}
	for _, m := range regexp.MustCompile(`(\w+): \[([^\]]*)\]`).FindAllSubmatch(block[1], -1) {
		var keys []string
		for _, k := range regexp.MustCompile(`"(\w+)"`).FindAllSubmatch(m[2], -1) {
			keys = append(keys, string(k[1]))
		}
		out[string(m[1])] = keys
	}
	return out
}

// The editor keeps every slice 4 field it may write (#375): area and zone on
// every event, count and label wherever count is allowed, distinct and
// explored_line on room_enter, and noun on command. Each field has a kind,
// and the builder's event descriptions name the same fields.
func TestQuestBuilderFiltersMatchTheVocab(t *testing.T) {
	filters := builderEventFilters(t)
	src, _ := os.ReadFile(questBuilderJS)
	if len(filters) != len(questEventVocab) {
		t.Fatalf("EVENT_FILTERS has %d events, the vocab %d", len(filters), len(questEventVocab))
	}
	for _, v := range questEventVocab {
		fs, ok := filters[v.Key]
		if !ok {
			t.Errorf("%s: no EVENT_FILTERS entry", v.Key)
			continue
		}
		want := []string{"area", "zone"}
		if quests.CountAllowed(v.Key) {
			want = append(want, "count", "label")
		} else if slices.Contains(fs, "count") || slices.Contains(fs, "label") {
			t.Errorf("%s offers count or label, which the loader refuses there", v.Key)
		}
		if v.Key == "room_enter" {
			want = append(want, "distinct", "explored_line")
		}
		if v.Key == "command" {
			want = append(want, "noun")
		}
		for _, w := range want {
			if !slices.Contains(fs, w) {
				t.Errorf("%s: EVENT_FILTERS lacks %s, so a save drops it", v.Key, w)
			}
		}
		for _, f := range fs {
			if !regexp.MustCompile(`\b` + f + `\b`).MatchString(v.Description) {
				t.Errorf("%s: vocab description does not name %s: %q", v.Key, f, v.Description)
			}
			if !strings.Contains(string(src), f+`: "`) {
				t.Errorf("%s: FILTER_KINDS has no kind for %s", v.Key, f)
			}
		}
	}
}
