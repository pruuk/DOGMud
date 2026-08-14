package spells

import (
	"math/rand"
	"os"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"gopkg.in/yaml.v2"
)

// CastingMessages holds the varied atmospheric messages for the casting system.
type CastingMessages struct {
	AlreadyCasting []string `yaml:"already_casting"`
	CastStarted    []string `yaml:"cast_started"`
	// CastContinuing is the per-round line while folds are still being laid
	// down. It exists because the round loop used to reuse CastStarted, which
	// told the player the cast was *beginning* again on every round of a
	// multi-fold spell, and duplicated the real start line on the round right
	// after the cast was initiated.
	CastContinuing       []string `yaml:"cast_continuing"`
	ConcentrationSlipped []string `yaml:"concentration_slipped"`
}

var (
	castingMessages     *CastingMessages
	castingMessagesOnce sync.Once
)

// loadCastingMessages loads the YAML file once and caches the result.
func loadCastingMessages() *CastingMessages {
	castingMessagesOnce.Do(func() {
		path := string(configs.GetFilePathsConfig().DataFiles) + `/casting-messages.yaml`
		data, err := os.ReadFile(path)
		if err != nil {
			castingMessages = defaultCastingMessages()
			return
		}
		var cm CastingMessages
		if err := yaml.Unmarshal(data, &cm); err != nil {
			castingMessages = defaultCastingMessages()
			return
		}
		castingMessages = &cm
	})
	return castingMessages
}

// defaultCastingMessages returns hardcoded fallback messages if the YAML file is missing.
func defaultCastingMessages() *CastingMessages {
	return &CastingMessages{
		AlreadyCasting: []string{
			"You are already casting {spell}. Type cancel to release the folds.",
		},
		CastStarted: []string{
			"You gather your will and begin forming the image of {spell}...",
		},
		CastContinuing: []string{
			"You hold the shape of {spell} steady while the next fold settles...",
		},
		ConcentrationSlipped: []string{
			"You reach for the folds of {spell} but your concentration slips.",
		},
	}
}

// GetCastMessage picks a random message from the named category, substituting
// {spell} with spellName.
//
// category must be one of: "already_casting", "cast_started",
// "cast_continuing", "concentration_slipped".
//
// spellName is the player-facing DISPLAY name (spellInfo.Name), never the
// spellid. Passing the id leaks an internal identifier into player output,
// which is exactly what the round loop used to do.
func GetCastMessage(category, spellName string) string {
	cm := loadCastingMessages()

	var pool []string
	switch category {
	case "already_casting":
		pool = cm.AlreadyCasting
	case "cast_started":
		pool = cm.CastStarted
	case "cast_continuing":
		pool = cm.CastContinuing
	case "concentration_slipped":
		pool = cm.ConcentrationSlipped
	}

	if len(pool) == 0 {
		return "Something stirs with " + spellName + "."
	}

	msg := pool[rand.Intn(len(pool))]
	return strings.ReplaceAll(msg, "{spell}", spellName)
}
