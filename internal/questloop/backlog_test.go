package questloop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBacklog(t *testing.T) {
	b, err := ParseBacklog([]byte("# header\n\nhint 53 step firstkill\nend 7 step end\n"))
	require.NoError(t, err)
	require.True(t, b["hint 53 step firstkill"])
	require.Len(t, b, 2)
	_, err = ParseBacklog([]byte("hint 53 step start\nhint 53 step start\n"))
	require.Error(t, err, "a repeated line is a mistake")
}

func TestRatchet(t *testing.T) {
	fs := []Finding{
		{Rule: "hint", QuestId: 53, Where: "step firstkill"},
		{Rule: "end", QuestId: 9, Where: "step end"},
	}
	b := Backlog{"hint 53 step firstkill": true, "start 4 step start": true}
	fresh, known, stale := Ratchet(fs, b)
	require.Equal(t, []string{"end 9 step end"}, keys(fresh))
	require.Equal(t, []string{"hint 53 step firstkill"}, keys(known))
	require.Equal(t, []string{"start 4 step start"}, stale)
}

func TestFormatBacklog_RoundTrips(t *testing.T) {
	fs := []Finding{{Rule: "hint", QuestId: 53, Where: "step firstkill"}, {Rule: "hint", QuestId: 53, Where: "step firstkill"}}
	b, err := ParseBacklog(FormatBacklog(fs))
	require.NoError(t, err)
	require.Equal(t, Backlog{"hint 53 step firstkill": true}, b)
}

func keys(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Key())
	}
	return out
}
