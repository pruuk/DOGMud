package configs

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// The item chatter knobs (item behaviour slice 2, spec "Config knobs").
// An absent key reads 0, and 0 is not an off switch: it reads as unset and
// takes the default, the KnockdownFrequencyScale rule
// (config.balance.misc.go), so deleting a line cannot silence every item.
// PinnacleItemsEnabled is the off switch.
func TestItemChatterKnobDefaults(t *testing.T) {
	b := Balance{}
	b.Validate()
	cases := []struct {
		name      string
		got, want int
	}{
		{"ItemChatterQuietCooldownRounds", int(b.ItemChatterQuietCooldownRounds), 40},
		{"ItemChatterQuietChancePct", int(b.ItemChatterQuietChancePct), 10},
		{"ItemChatterNormalCooldownRounds", int(b.ItemChatterNormalCooldownRounds), 20},
		{"ItemChatterNormalChancePct", int(b.ItemChatterNormalChancePct), 15},
		{"ItemChatterChattyCooldownRounds", int(b.ItemChatterChattyCooldownRounds), 10},
		{"ItemChatterChattyChancePct", int(b.ItemChatterChattyChancePct), 25},
		{"ItemChatterListenerCapRounds", int(b.ItemChatterListenerCapRounds), 10},
		{"HungerFeedingLineCooldownRounds", int(b.HungerFeedingLineCooldownRounds), 20},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}

	b = Balance{ItemChatterNormalChancePct: 101, ItemChatterChattyChancePct: -5, ItemChatterQuietCooldownRounds: -1}
	b.Validate()
	if b.ItemChatterNormalChancePct != 15 || b.ItemChatterChattyChancePct != 25 || b.ItemChatterQuietCooldownRounds != 40 {
		t.Errorf("out-of-range values must coerce to the defaults, got normal %d%%, chatty %d%%, quiet %d rounds",
			int(b.ItemChatterNormalChancePct), int(b.ItemChatterChattyChancePct), int(b.ItemChatterQuietCooldownRounds))
	}
}

// The shipped config.yaml and the Go defaults agree, or a server started
// without the block behaves differently from one started with it (the
// TestBaubleShippedConfigMatchesDefaults precedent).
func TestItemChatterShippedConfigMatchesDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	s := cfg.Balance
	var d Balance
	d.Validate()
	if s.ItemChatterQuietCooldownRounds != d.ItemChatterQuietCooldownRounds ||
		s.ItemChatterQuietChancePct != d.ItemChatterQuietChancePct ||
		s.ItemChatterNormalCooldownRounds != d.ItemChatterNormalCooldownRounds ||
		s.ItemChatterNormalChancePct != d.ItemChatterNormalChancePct ||
		s.ItemChatterChattyCooldownRounds != d.ItemChatterChattyCooldownRounds ||
		s.ItemChatterChattyChancePct != d.ItemChatterChattyChancePct ||
		s.ItemChatterListenerCapRounds != d.ItemChatterListenerCapRounds ||
		s.HungerFeedingLineCooldownRounds != d.HungerFeedingLineCooldownRounds {
		t.Errorf("shipped item chatter knobs differ from the Go defaults: shipped quiet %d/%d normal %d/%d chatty %d/%d cap %d feeding %d",
			int(s.ItemChatterQuietCooldownRounds), int(s.ItemChatterQuietChancePct),
			int(s.ItemChatterNormalCooldownRounds), int(s.ItemChatterNormalChancePct),
			int(s.ItemChatterChattyCooldownRounds), int(s.ItemChatterChattyChancePct),
			int(s.ItemChatterListenerCapRounds), int(s.HungerFeedingLineCooldownRounds))
	}
}
