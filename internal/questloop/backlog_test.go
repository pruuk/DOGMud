package questloop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBacklog(t *testing.T) {
	b, err := ParseBacklog([]byte("# header\n\nhint 53 step 1 (firstkill)\nend 7 step 3 (end)\n"))
	require.NoError(t, err)
	require.True(t, b["hint 53 step 1 (firstkill)"])
	require.Len(t, b, 2)
	_, err = ParseBacklog([]byte("hint 53 step 1\nhint 53 step 1\n"))
	require.Error(t, err, "a repeated line is a mistake")
}

func TestRatchet(t *testing.T) {
	fs := []Finding{
		{Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)"},
		{Rule: "end", QuestId: 9, Where: "step 2 (end)"},
	}
	b := Backlog{"hint 53 step 1 (firstkill)": true, "start 4 step 0 (start)": true}
	fresh, known, stale := Ratchet(fs, b)
	require.Equal(t, []string{"end 9 step 2 (end)"}, keys(fresh))
	require.Equal(t, []string{"hint 53 step 1 (firstkill)"}, keys(known))
	require.Equal(t, []string{"start 4 step 0 (start)"}, stale)
}

func TestFormatBacklog_RoundTrips(t *testing.T) {
	fs := []Finding{{Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)"}, {Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)"}}
	b, err := ParseBacklog(FormatBacklog(fs))
	require.NoError(t, err)
	require.Equal(t, Backlog{"hint 53 step 1 (firstkill)": true}, b)
}

func keys(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Key())
	}
	return out
}
