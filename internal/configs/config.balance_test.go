package configs

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestBalanceConfig_DefaultPricingBaselineQty(t *testing.T) {
	cfg := &Balance{}
	cfg.Validate()
	if int(cfg.DefaultPricingBaselineQty) != 3 {
		t.Errorf("DefaultPricingBaselineQty default = %d, want 3", int(cfg.DefaultPricingBaselineQty))
	}
}

func TestBalanceConfig_CaravanDefaults(t *testing.T) {
	cfg := &Balance{}
	cfg.Validate()
	if cfg.CaravanDepotDwellRounds != 360 {
		t.Errorf("CaravanDepotDwellRounds default = %d, want 360", cfg.CaravanDepotDwellRounds)
	}
	if len(cfg.CaravanServedZones) == 0 {
		t.Error("CaravanServedZones default should not be empty")
	}
	expected := map[string]bool{"Stillwater": true, "Thornwall City": true}
	for _, z := range cfg.CaravanServedZones {
		if !expected[z] {
			t.Errorf("unexpected zone in default CaravanServedZones: %q", z)
		}
		delete(expected, z)
	}
	if len(expected) > 0 {
		t.Errorf("missing default zones: %v", expected)
	}
}

func TestBalance_BossInterruptDefaults(t *testing.T) {
	cfg := &Balance{}
	cfg.Validate()

	if !cfg.IsBossInterruptItem(30057) {
		t.Error("expected flashbang (30057) to be a configured boss-interrupt item")
	}
	if cfg.IsBossInterruptItem(99999) {
		t.Error("unexpected item id reported as a boss-interrupt item")
	}

	for _, spellId := range []string{"neural-stun", "sensory-overload", "kinetic-shove"} {
		if !cfg.IsBossInterruptSpell(spellId) {
			t.Errorf("expected %q to be a configured boss-interrupt spell", spellId)
		}
	}
	if cfg.IsBossInterruptSpell("fireball") {
		t.Error("unexpected spell id reported as a boss-interrupt spell")
	}
}

func TestBalance_BountyHunterDefaults(t *testing.T) {
	b := &Balance{}
	b.validateMisc()
	if b.BountyHunterGoldThreshold != 500 {
		t.Fatalf("BountyHunterGoldThreshold = %d, want 500", int(b.BountyHunterGoldThreshold))
	}
	if b.BountyHunterBaseStatpool != 250 {
		t.Fatalf("BountyHunterBaseStatpool = %d, want 250", int(b.BountyHunterBaseStatpool))
	}
	if b.BountyHunterStatpoolPerGold != 0.25 {
		t.Fatalf("BountyHunterStatpoolPerGold = %v, want 0.25", float64(b.BountyHunterStatpoolPerGold))
	}
	if b.BountyHunterMinStatpool != 300 || b.BountyHunterMaxStatpool != 500 {
		t.Fatalf("min/max statpool = %d/%d, want 300/500", int(b.BountyHunterMinStatpool), int(b.BountyHunterMaxStatpool))
	}
	if b.BountyHunterRedispatchCooldown != 500 {
		t.Fatalf("BountyHunterRedispatchCooldown = %d, want 500", int(b.BountyHunterRedispatchCooldown))
	}
	if b.BountyHunterGearGoldDivisor != 5 {
		t.Fatalf("BountyHunterGearGoldDivisor = %d, want 5", int(b.BountyHunterGearGoldDivisor))
	}
}

func TestBalance_MobUpgradeDefaults(t *testing.T) {
	b := &Balance{}
	b.validateMisc()
	if int(b.MobUpgradeGoldReserve) != 50 {
		t.Fatalf("MobUpgradeGoldReserve = %d, want 50", int(b.MobUpgradeGoldReserve))
	}
	if float64(b.MobUpgradeMinDelta) != 1.0 {
		t.Fatalf("MobUpgradeMinDelta = %v, want 1.0", float64(b.MobUpgradeMinDelta))
	}
}

func TestBalance_RangedDefaults(t *testing.T) {
	b := &Balance{}
	b.validateCombat()
	if b.RangedShotScale != 1.0 {
		t.Errorf("RangedShotScale default = %v, want 1.0", float64(b.RangedShotScale))
	}
	if b.RangedShieldDefenseBonus != 15 {
		t.Errorf("RangedShieldDefenseBonus default = %d, want 15", int(b.RangedShieldDefenseBonus))
	}
}

func TestBalance_ActionCostBaseDefaultsRejectMissingAndInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		bad  ConfigFloat
	}{
		{"missing", 0},
		{"negative", -1},
		{"nan", ConfigFloat(math.NaN())},
		{"positive infinity", ConfigFloat(math.Inf(1))},
		{"negative infinity", ConfigFloat(math.Inf(-1))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Balance{
				ShootBaseStaminaCost:             tt.bad,
				ReloadBaseStaminaCost:            tt.bad,
				SpecialMoveBaseStaminaCost:       tt.bad,
				SneakBaseStaminaCost:             tt.bad,
				RhetoricActionBaseConvictionCost: tt.bad,
				GrappleStaminaCostPerRound:       tt.bad,
			}
			b.validateCombat()

			got := []ConfigFloat{
				b.ShootBaseStaminaCost,
				b.ReloadBaseStaminaCost,
				b.SpecialMoveBaseStaminaCost,
				b.SneakBaseStaminaCost,
				b.RhetoricActionBaseConvictionCost,
				b.GrappleStaminaCostPerRound,
			}
			want := []ConfigFloat{2, 1, 4, 2.5, 4, 2}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("base %d = %v, want %v", i, float64(got[i]), float64(want[i]))
				}
			}
		})
	}
}

func TestBalance_ShippedActionCostBasesMatchModelEvidence(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}

	got := []ConfigFloat{
		cfg.Balance.ShootBaseStaminaCost,
		cfg.Balance.ReloadBaseStaminaCost,
		cfg.Balance.SpecialMoveBaseStaminaCost,
		cfg.Balance.SneakBaseStaminaCost,
		cfg.Balance.RhetoricActionBaseConvictionCost,
		cfg.Balance.GrappleStaminaCostPerRound,
	}
	want := []ConfigFloat{2, 1, 4, 2.5, 4, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("shipped base %d = %v, want %v", i, float64(got[i]), float64(want[i]))
		}
	}
}

func TestBalance_SneakFailCooldownInvalidValueDefaultsToEffectiveZero(t *testing.T) {
	b := &Balance{SneakFailCooldown: -1}
	b.validateCombat()
	if b.SneakFailCooldown != 0 {
		t.Fatalf("SneakFailCooldown fallback = %d, want effective shipped value 0", int(b.SneakFailCooldown))
	}
}

func TestBalanceConfig_ForagerDefaults(t *testing.T) {
	cfg := &Balance{}
	cfg.Validate()

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"FernwayPickupDwellRounds", cfg.FernwayPickupDwellRounds, ConfigInt(6)},
		{"ForagerCarryThresholdPct", cfg.ForagerCarryThresholdPct, ConfigFloat(0.75)},
		{"ForagerHPRecallThresholdPct", cfg.ForagerHPRecallThresholdPct, ConfigFloat(0.50)},
		{"ForagerHealPotionThresholdPct", cfg.ForagerHealPotionThresholdPct, ConfigFloat(0.75)},
		{"ForagerWaitTimeoutRounds", cfg.ForagerWaitTimeoutRounds, ConfigInt(150)},
		{"ForagerRestCarryThreshold", cfg.ForagerRestCarryThreshold, ConfigFloat(0.5)},
		{"ForagerLockboxCapacity", cfg.ForagerLockboxCapacity, ConfigInt(500)},
		{"ChestBackpressureResumePct", cfg.ChestBackpressureResumePct, ConfigFloat(0.9)},
		{"ForagerStuckThresholdRounds", cfg.ForagerStuckThresholdRounds, ConfigInt(600)},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s default = %v, want %v", c.name, c.got, c.want)
		}
	}
}
