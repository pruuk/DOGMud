package characters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestHeard_MarkKeepsSortedUnique(t *testing.T) {
	c := &Character{}
	require.False(t, c.HasHeard("94.b"))
	c.MarkHeard("94.b")
	c.MarkHeard("12.a")
	c.MarkHeard("94.b")
	require.Equal(t, []string{"12.a", "94.b"}, c.HeardTopics)
	require.True(t, c.HasHeard("12.a"))
}

func TestHeard_RenameAppliesAndDedupes(t *testing.T) {
	c := &Character{HeardTopics: []string{"12.a", "12.old", "94.b"}}
	c.RenameHeardTopics(map[string]string{"12.old": "12.a", "94.b": "94.c"})
	require.Equal(t, []string{"12.a", "94.c"}, c.HeardTopics)
}

func TestHeard_RenameFollowsChains(t *testing.T) {
	c := &Character{HeardTopics: []string{"a"}}
	c.RenameHeardTopics(map[string]string{"a": "b", "b": "c"})
	require.Equal(t, []string{"c"}, c.HeardTopics)
}

func TestHeard_RenameCycleTerminates(t *testing.T) {
	c := &Character{HeardTopics: []string{"a"}}
	c.RenameHeardTopics(map[string]string{"a": "b", "b": "a"})
	require.Len(t, c.HeardTopics, 1)
}

func TestHeard_RenameRepairsUnsortedList(t *testing.T) {
	c := &Character{HeardTopics: []string{"b", "a", "a"}}
	c.RenameHeardTopics(nil)
	require.Equal(t, []string{"a", "b"}, c.HeardTopics)
}

func TestHeard_SurvivesYAMLRoundTrip(t *testing.T) {
	c := &Character{Name: "Mira"}
	c.MarkHeard("94.where_to_look")
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	require.Contains(t, string(out), "heardtopics:")
	var back Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.True(t, back.HasHeard("94.where_to_look"))
}

func TestHeard_EmptyIsOmitted(t *testing.T) {
	out, err := yaml.Marshal(&Character{Name: "Mira"})
	require.NoError(t, err)
	require.NotContains(t, string(out), "heardtopics")
}
