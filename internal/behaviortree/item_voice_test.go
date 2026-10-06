package behaviortree

import (
	"strings"
	"testing"
)

// An item tree carries its voice: line pools by name and a chatter level
// (item behaviour slice 2, Rule 16).
func TestItemTreeCarriesItsVoice(t *testing.T) {
	LoadItemTreeForTest(t, "voice_probe", `
chatter: chatty
speech:
  on_idle:
    - "Hm."
    - "Hmm."
tree:
  type: condition
  check: worn
`)
	v := GetEngine().GetItemVoice("voice_probe")
	if v == nil {
		t.Fatal("GetItemVoice(voice_probe) = nil, want the tree's voice")
	}
	if v.Chatter != ChatterChatty {
		t.Errorf("Chatter = %q, want %q", v.Chatter, ChatterChatty)
	}
	if got := v.Speech["on_idle"]; len(got) != 2 || got[1] != "Hmm." {
		t.Errorf("Speech[on_idle] = %q, want the two authored lines", got)
	}

	LoadItemTreeForTest(t, "voice_default", "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: condition\n  check: worn\n")
	if v := GetEngine().GetItemVoice("voice_default"); v == nil || v.Chatter != ChatterNormal {
		t.Errorf("a tree with no chatter: level must default to %q, got %+v", ChatterNormal, v)
	}

	LoadItemTreeForTest(t, "voice_none", "tree:\n  type: condition\n  check: worn\n")
	if v := GetEngine().GetItemVoice("voice_none"); v != nil {
		t.Errorf("a tree with no speech has no voice, got %+v", v)
	}
}

// A malformed voice refuses at load, and only an item tree may carry one.
func TestItemVoiceRefusesAtLoad(t *testing.T) {
	bad := map[string]string{
		"unknown chatter": "chatter: loud\nspeech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: condition\n  check: worn\n",
		"empty pool":      "speech:\n  on_idle: []\ntree:\n  type: condition\n  check: worn\n",
		"blank line":      "speech:\n  on_idle: [\"  \"]\ntree:\n  type: condition\n  check: worn\n",
		"chatter alone":   "chatter: quiet\ntree:\n  type: condition\n  check: worn\n",
	}
	for name, src := range bad {
		if _, _, err := loadItemTreeDef([]byte(src)); err == nil {
			t.Errorf("%s: loaded, want a refusal", name)
		}
	}

	mob := "speech:\n  on_idle: [\"Hm.\"]\ntree:\n  type: condition\n  check: random_chance\n  percent: 50\n"
	if _, err := LoadTreeFromBytes([]byte(mob)); err == nil || !strings.Contains(err.Error(), "item tree") {
		t.Errorf("a mob or room tree with speech: err = %v, want a refusal naming item trees", err)
	}
}
