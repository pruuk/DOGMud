package configs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// The shipped config names every Bloom knob, at its Go default, so the live
// value is visible in config.yaml rather than silently the default. Moving them
// changed no behaviour: this compares each shipped value to validateCombat's.
func TestBloomKnobsShipAtTheirDefaults(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	shipped := cfg.Balance
	if shipped.BloomMutationAdvanceChance == 0 {
		t.Fatal("BloomMutationAdvanceChance is absent from config.yaml")
	}
	def := &Balance{}
	def.validateCombat()
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"BloomAddictionPerDose", float64(shipped.BloomAddictionPerDose), float64(def.BloomAddictionPerDose)},
		{"BloomAddictionDecayRounds", float64(shipped.BloomAddictionDecayRounds), float64(def.BloomAddictionDecayRounds)},
		{"BloomWithdrawalOnsetRounds", float64(shipped.BloomWithdrawalOnsetRounds), float64(def.BloomWithdrawalOnsetRounds)},
		{"BloomMutationAdvanceChance", float64(shipped.BloomMutationAdvanceChance), float64(def.BloomMutationAdvanceChance)},
		{"BloomNewMutationChance", float64(shipped.BloomNewMutationChance), float64(def.BloomNewMutationChance)},
		{"BloomCommunionRounds", float64(shipped.BloomCommunionRounds), float64(def.BloomCommunionRounds)},
		{"BloomCrashRoundsMult", float64(shipped.BloomCrashRoundsMult), float64(def.BloomCrashRoundsMult)},
		{"BloomDealerWafersPerDay", float64(shipped.BloomDealerWafersPerDay), float64(def.BloomDealerWafersPerDay)},
		{"BloomDealerWindowHours", float64(shipped.BloomDealerWindowHours), float64(def.BloomDealerWindowHours)},
	} {
		if c.got != c.want {
			t.Errorf("shipped %s = %v, default %v", c.name, c.got, c.want)
		}
	}
}
