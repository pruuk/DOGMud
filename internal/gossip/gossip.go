// Package gossip is the gossip template store: pools of lines a gossiping
// NPC says about recent world events and known facts, keyed
// "EventType-Significance[-Local|-Distant]", "fact-<id|tag|default>" and
// "fallback", loaded from DataFiles/gossip_templates.yaml.
//
// Which key and which token apply is decided by the gossiper in
// internal/hooks (buildGossipLine); this package owns the text, its
// validation and the pick.
package gossip

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"gopkg.in/yaml.v2"
)

const fileName = "gossip_templates.yaml"

// tokens a template may carry; each at most once per line.
var tokens = []string{"{desc}", "{description}"}

var templates map[string][]string

// Load reads DataFiles/gossip_templates.yaml. A world with no such file (the
// default world) gets an empty store. A read error other than a missing file,
// a parse error, or a Validate failure panics, as a bad recipe or spell does:
// before the store existed these were logged and gossip went silently empty
// for the life of the process.
func Load() {
	start := time.Now()
	path := string(configs.GetFilePathsConfig().DataFiles) + "/" + fileName

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			templates = map[string][]string{}
			mudlog.Info("gossip.Load()", "loadedKeys", 0, "note", "this world has no "+fileName)
			return
		}
		panic(fmt.Errorf("gossip: read %s: %w", path, err))
	}

	loaded := map[string][]string{}
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		panic(fmt.Errorf("gossip: parse %s: %w", path, err))
	}
	if err := Validate(loaded); err != nil {
		panic(fmt.Errorf("gossip: %s: %w", path, err))
	}

	templates = loaded
	mudlog.Info("gossip.Load()", "loadedKeys", len(templates), "Time Taken", time.Since(start))
}

// Validate refuses an empty pool, a blank line, and a line that carries the
// same token twice. The last rule is what keeps rendering byte-identical to
// the pre-store code, which replaced {desc} only once: with no repeats, once
// and every-occurrence substitution produce the same line.
func Validate(t map[string][]string) error {
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pool := t[key]
		if len(pool) == 0 {
			return fmt.Errorf("key %q has no lines", key)
		}
		for i, line := range pool {
			if strings.TrimSpace(line) == "" {
				return fmt.Errorf("key %q line %d is blank", key, i)
			}
			for _, tok := range tokens {
				if strings.Count(line, tok) > 1 {
					return fmt.Errorf("key %q line %d uses %s more than once", key, i, tok)
				}
			}
		}
	}
	return nil
}

// Pool returns the lines for key, or nil.
func Pool(key string) []string {
	return templates[key]
}

// Keys returns every loaded key, sorted.
func Keys() []string {
	out := make([]string, 0, len(templates))
	for k := range templates {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Render picks one line from pool and substitutes token with value. It draws
// exactly once, through narration.DefaultPicker (util.Rand), as the pre-store
// code did with util.Rand(len(pool)). An empty token substitutes nothing.
func Render(pool []string, token, value string) string {
	return renderWith(pool, token, value, narration.DefaultPicker)
}

// renderWith is Render with an explicit picker. A gossiping NPC is the only
// speaker, so the pool is the Actor role and nothing else is authored.
func renderWith(pool []string, token, value string, pick narration.Picker) string {
	if len(pool) == 0 {
		return ""
	}
	var subs map[string]string
	if token != "" {
		subs = map[string]string{token: value}
		// A line whose token follows a lead-in ("I heard {desc}") takes the
		// value with its leading article lowercased, so it reads "I heard a
		// caravan", not "I heard A caravan" (#430). The lines are adjusted
		// before the pick, so the draw count and the line drawn are unchanged.
		if lowered := LowerLeadingArticle(value); lowered != value {
			adjusted := make([]string, len(pool))
			for i, line := range pool {
				adjusted[i] = line
				if tokenFollowsLeadIn(line, token) {
					adjusted[i] = strings.Replace(line, token, lowered, 1)
				}
			}
			pool = adjusted
		}
	}
	return narration.Render(narration.Variants{Actor: pool}, subs, pick).Actor
}

// LowerLeadingArticle lowercases a leading "A", "An" or "The" in s, for a
// rumour dropped into the middle of a sentence. Only those three words: any
// other capital may be a proper noun ("Thornwall closed its gates"), so it
// stays.
func LowerLeadingArticle(s string) string {
	for _, article := range []string{"A ", "An ", "The "} {
		if strings.HasPrefix(s, article) {
			return strings.ToLower(s[:1]) + s[1:]
		}
	}
	return s
}

// tokenFollowsLeadIn reports whether token sits mid-sentence in line: some
// text comes before it, and that text does not end a sentence.
func tokenFollowsLeadIn(line, token string) bool {
	idx := strings.Index(line, token)
	if idx < 0 {
		return false
	}
	before := strings.TrimRight(line[:idx], " ")
	if before == "" {
		return false
	}
	switch before[len(before)-1] {
	case '.', '!', '?', ':', '"':
		return false
	}
	return true
}
