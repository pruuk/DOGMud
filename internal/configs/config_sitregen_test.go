package configs

import "testing"

// SitRegenMultiplier (#637). Owner ruling 2026-10-19: sitting raises regen
// like sleep does, but by less. Pinned as a relationship between the two
// knobs, on the shipped config.yaml and on the Go defaults.

func TestSitRegen_ShippedBelowSleep(t *testing.T) {
	src := shippedConfigSource(t)
	for _, key := range []string{"SleepRegenMultiplier:", "SitRegenMultiplier:"} {
		if !bytesContainsKey(src, key) {
			t.Fatalf("config.yaml must ship %s so its live value is visible (#637)", key)
		}
	}
	cfg, err := loadConfig(src)
	if err != nil {
		t.Fatalf("loadConfig(config.yaml): %v", err)
	}
	cfg.Validate()
	sit, sleep := float64(cfg.Balance.SitRegenMultiplier), float64(cfg.Balance.SleepRegenMultiplier)
	if sit <= 1 {
		t.Errorf("SitRegenMultiplier = %v; sitting must raise regen (owner ruling)", sit)
	}
	if sit >= sleep {
		t.Errorf("SitRegenMultiplier %v must stay below SleepRegenMultiplier %v (owner ruling)", sit, sleep)
	}
}

func TestSitRegen_AbsentKeyLoadsTheDefaultBelowSleep(t *testing.T) {
	cfg, err := loadConfig([]byte("Balance:\n  StandMinStamina: 0.15\n"))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.SitRegenMultiplier != 2.0 {
		t.Errorf("absent SitRegenMultiplier must load the default 2.0, got %v", cfg.Balance.SitRegenMultiplier)
	}
	if cfg.Balance.SitRegenMultiplier >= cfg.Balance.SleepRegenMultiplier {
		t.Errorf("default sit %v must stay below default sleep %v", cfg.Balance.SitRegenMultiplier, cfg.Balance.SleepRegenMultiplier)
	}
}
