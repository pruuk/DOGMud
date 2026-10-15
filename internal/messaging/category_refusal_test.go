package messaging

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestRefusalAliasMatchesSystem is the data test the promise above relies on:
// the dogmud world colours refusal exactly as it colours system.
func TestRefusalAliasMatchesSystem(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "ansi-aliases.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc struct {
		Colors map[string]any `yaml:"colors"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))
	require.Contains(t, doc.Colors, "system")
	require.Contains(t, doc.Colors, "refusal", "dogmud ansi-aliases.yaml must define refusal")
	require.Equal(t, doc.Colors["system"], doc.Colors["refusal"], "refusal must keep system's colour value")
}

// TestCategoryRefusalKeepsSystemTreatment pins M7's promise (#371): moving a
// refusal off CategorySystem changes nothing a player sees. Same alias
// value (checked in the data test below), same normalisation, never wrapped,
// never suppressed.
func TestCategoryRefusalKeepsSystemTreatment(t *testing.T) {
	if CategoryRefusal.String() != "refusal" {
		t.Fatalf("CategoryRefusal.String() = %q, want refusal", CategoryRefusal.String())
	}
	if skipStages(CategoryRefusal) != skipStages(CategorySystem) {
		t.Error("CategoryRefusal must normalise exactly as CategorySystem")
	}
	if shouldWrap(CategoryRefusal) {
		t.Error("CategoryRefusal must not wrap: CategorySystem never did")
	}
	for _, v := range []Verbosity{VerbosityFull, VerbosityMedium, VerbosityLight} {
		if v.Suppresses(CategoryRefusal) {
			t.Errorf("verbosity %v suppresses CategoryRefusal; no tier may", v)
		}
	}
}

// TestCategoryRefusalRendersAsSystem renders one refusal on both categories
// and compares them with the colour tag's name taken out.
func TestCategoryRefusalRendersAsSystem(t *testing.T) {
	text := "You aren't carrying 'rope'."
	render := func(c Category) string {
		return RenderForRecipient(RenderInput{Category: c, Text: text, Channel: ChannelAudio, LineWidth: 80})
	}
	sys, ref := render(CategorySystem), render(CategoryRefusal)
	if want := `fg="refusal"`; !strings.Contains(ref, want) {
		t.Fatalf("refusal render %q lacks %s", ref, want)
	}
	if strings.ReplaceAll(ref, `fg="refusal"`, `fg="system"`) != sys {
		t.Errorf("render differs beyond the alias name:\nsystem  %q\nrefusal %q", sys, ref)
	}
}
