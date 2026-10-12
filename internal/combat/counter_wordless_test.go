package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/require"
)

const (
	counterWordlessSpeakerId = 9611
	counterWordlessBeastId   = 9612
)

// markedCounterPool is a counter pool whose every line carries marker, so a
// test can tell which pool a retort read from.
func markedCounterPool(pool items.DefencePool, marker string) *items.DefenseMessageGroup {
	mk := func(band string) items.DefenseOptions {
		messages := func(audience string) items.MessageOptions {
			result := make(items.MessageOptions, 6)
			for i := range result {
				result[i] = items.ItemMessage(marker + "-" + band + "-" + audience + " {actee} answers {actor}")
			}
			return result
		}
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: messages("defender"),
			ToAttacker: messages("attacker"),
			ToRoom:     messages("room"),
		}}
	}
	return &items.DefenseMessageGroup{OptionId: pool, Options: items.DefenseIntensity{
		items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
	}}
}

func seedCounterWordlessSpecies(t *testing.T) {
	t.Helper()
	no := false
	species.SetSpeciesForTest(t, map[int]*species.Species{
		counterWordlessSpeakerId: {SpeciesId: counterWordlessSpeakerId, Name: "speaker"},
		counterWordlessBeastId:   {SpeciesId: counterWordlessBeastId, Name: "beast", Speech: &no},
	})
}

// M6 slice 3: a speechless counterer's retort reads the counter-defy-wordless
// pool and the wordless damage text; a speaking counterer keeps counter-defy
// and the spoken text. 20 of 200 conviction is the band where the two tables
// differ ("a stinging insult" against "a jolt of fear").
func TestBuildCounterTauntMessagesSpeechlessReadsWordless(t *testing.T) {
	seedCounterWordlessSpecies(t)
	restore := items.SeedDefenseMessagesForTest(map[items.DefencePool]*items.DefenseMessageGroup{
		items.CounterPoolDefy:         markedCounterPool(items.CounterPoolDefy, "SPOKEN"),
		items.CounterPoolDefyWordless: markedCounterPool(items.CounterPoolDefyWordless, "WORDLESS"),
	})
	defer restore()

	cases := []struct {
		name                string
		speciesId           int
		wantPool, notPool   string
		wantDesc, wantDesc2 string
		notDesc             string
	}{
		{"speechless counterer", counterWordlessBeastId, "WORDLESS", "SPOKEN",
			GetWordlessConvictionDamageDescription(20, 200), GetWordlessConvictionDamageDescriptionToTarget(20, 200),
			GetConvictionDamageDescription(20, 200)},
		{"speaking counterer", counterWordlessSpeakerId, "SPOKEN", "WORDLESS",
			GetConvictionDamageDescription(20, 200), GetConvictionDamageDescriptionToTarget(20, 200),
			GetWordlessConvictionDamageDescription(20, 200)},
	}
	require.NotEqual(t, cases[0].wantDesc, cases[1].wantDesc, "fixture error: the two damage tables must differ at this band")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			counterer, countered := counterTauntFixtureChars("Selka", "Rurik")
			counterer.SpeciesId = tc.speciesId
			countererMsg, taunterMsg, roomMsg := BuildCounterTauntMessages(counterer, countered, false, 20, 200)
			for audience, line := range map[string]string{"counterer": countererMsg, "taunter": taunterMsg, "room": roomMsg} {
				require.Contains(t, line, tc.wantPool, "%s line read the wrong pool", audience)
				require.NotContains(t, line, tc.notPool, "%s line read the wrong pool", audience)
			}
			require.Contains(t, countererMsg, tc.wantDesc)
			require.Contains(t, taunterMsg, tc.wantDesc2)
			require.NotContains(t, countererMsg, tc.notDesc)
			require.NotContains(t, taunterMsg, tc.notDesc)
		})
	}
}

// Without the wordless pool a speechless counterer falls back to counter-defy
// rather than going silent, and still reads the wordless damage text.
func TestBuildCounterTauntMessagesSpeechlessFallsBackToCounterDefy(t *testing.T) {
	seedCounterWordlessSpecies(t)
	restore := items.SeedDefenseMessagesForTest(map[items.DefencePool]*items.DefenseMessageGroup{
		items.CounterPoolDefy: markedCounterPool(items.CounterPoolDefy, "SPOKEN"),
	})
	defer restore()

	counterer, countered := counterTauntFixtureChars("Selka", "Rurik")
	counterer.SpeciesId = counterWordlessBeastId
	countererMsg, _, roomMsg := BuildCounterTauntMessages(counterer, countered, false, 20, 200)
	require.Contains(t, roomMsg, "SPOKEN")
	require.Contains(t, countererMsg, GetWordlessConvictionDamageDescription(20, 200))
}

// With no pool loaded at all, a speechless counterer's generic retort names
// no words.
func TestBuildCounterTauntMessagesSpeechlessGenericNamesNoWords(t *testing.T) {
	seedCounterWordlessSpecies(t)
	restore := items.SeedDefenseMessagesForTest(nil)
	defer restore()

	for _, damage := range []int{0, 20} {
		counterer, countered := counterTauntFixtureChars("Selka", "Rurik")
		counterer.SpeciesId = counterWordlessBeastId
		countererMsg, taunterMsg, roomMsg := BuildCounterTauntMessages(counterer, countered, false, damage, 200)
		for audience, line := range map[string]string{"counterer": countererMsg, "taunter": taunterMsg, "room": roomMsg} {
			require.Contains(t, line, "RETORT!", "%s line must carry the retort prefix", audience)
			require.False(t, strings.Contains(strings.ToLower(line), "word"),
				"a speechless %s retort names words: %q", audience, line)
		}
	}
}
