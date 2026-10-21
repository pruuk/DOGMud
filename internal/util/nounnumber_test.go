package util

import (
	"slices"
	"testing"
)

func TestNounNumberVariants(t *testing.T) {
	has := func(word, want string) {
		t.Helper()
		if !slices.Contains(NounNumberVariants(word), want) {
			t.Errorf("NounNumberVariants(%q) = %v, missing %q", word, NounNumberVariants(word), want)
		}
	}
	lacks := func(word, bad string) {
		t.Helper()
		if slices.Contains(NounNumberVariants(word), bad) {
			t.Errorf("NounNumberVariants(%q) = %v, must not contain %q", word, NounNumberVariants(word), bad)
		}
	}
	has("wardstone", "wardstones")
	has("wardstones", "wardstone")
	has("slabs", "slab")
	has("slab", "slabs")
	has("boxes", "box")
	has("box", "boxes")
	has("ponies", "pony")
	has("pony", "ponies")
	has("berry", "berries")
	has("standing stone", "standing stones")
	has("standing stones", "standing stone")
	lacks("mos", "moss")
	lacks("moss", "mos")
}

func TestNounNumberVariants_Order(t *testing.T) {
	// "es" strip/add first, then "ies" to "y", then plain "s", then "y" to "ies".
	cases := map[string][]string{
		"ponies": {"poni", "pony", "ponie"},
		"pony":   {"ponyes", "ponys", "ponies"},
		"moss":   {"mosses"},
	}
	for word, want := range cases {
		if got := NounNumberVariants(word); !slices.Equal(got, want) {
			t.Errorf("NounNumberVariants(%q) = %v, want %v", word, got, want)
		}
	}
}

func TestNounsDifferOnlyByNumber(t *testing.T) {
	yes := [][2]string{
		{"pillar", "pillars"}, {"pillars", "pillar"}, {"box", "boxes"},
		{"pony", "ponies"}, {"standing stone", "standing stones"},
	}
	for _, p := range yes {
		if !NounsDifferOnlyByNumber(p[0], p[1]) {
			t.Errorf("NounsDifferOnlyByNumber(%q, %q) = false, want true", p[0], p[1])
		}
	}
	no := [][2]string{
		{"pillar", "pillar"}, {"", "pillars"}, {"mos", "moss"},
		{"altar", "altar stone"}, {"pillar", "column"},
	}
	for _, p := range no {
		if NounsDifferOnlyByNumber(p[0], p[1]) {
			t.Errorf("NounsDifferOnlyByNumber(%q, %q) = true, want false", p[0], p[1])
		}
	}
}
