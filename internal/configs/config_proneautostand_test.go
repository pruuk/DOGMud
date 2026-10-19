package configs

import "testing"

// ProneAutoStandRounds (#398) uses the ProgressionFailureFraction idiom: 0 is
// a legal off-switch, so an absent key is told apart by the -1 sentinel
// seeded in newUnloadedConfig. These run the real load path.

func TestProneAutoStandRounds_AbsentKeyLoadsTheDefault(t *testing.T) {
	cfg, err := loadConfig([]byte("Balance:\n  StandMinStamina: 0.15\n"))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.ProneAutoStandRounds != 5 {
		t.Fatalf("a config omitting ProneAutoStandRounds must load the default 5, got %v", cfg.Balance.ProneAutoStandRounds)
	}
}

func TestProneAutoStandRounds_ExplicitZeroLoadsAsZero(t *testing.T) {
	cfg, err := loadConfig([]byte("Balance:\n  ProneAutoStandRounds: 0\n"))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.ProneAutoStandRounds != 0 {
		t.Fatalf("an explicit 0 is the off-switch and must survive validation, got %v", cfg.Balance.ProneAutoStandRounds)
	}
}

func TestProneAutoStandRounds_ExplicitValueLoadsAsAuthored(t *testing.T) {
	cfg, err := loadConfig([]byte("Balance:\n  ProneAutoStandRounds: 8\n"))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.ProneAutoStandRounds != 8 {
		t.Fatalf("an authored ProneAutoStandRounds must load unchanged, got %v", cfg.Balance.ProneAutoStandRounds)
	}
}

func TestNewUnloadedConfig_SeedsProneAutoStandSentinel(t *testing.T) {
	if got := newUnloadedConfig().Balance.ProneAutoStandRounds; got >= 0 {
		t.Fatalf("newUnloadedConfig must pre-seed a negative absence sentinel, got %v", got)
	}
}
