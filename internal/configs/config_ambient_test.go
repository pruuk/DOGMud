package configs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// #317 ambient knobs. Each source's 0 is its off switch, so an absent key is
// told apart by a -1 sentinel seeded in newUnloadedConfig (the
// ProneAutoStandRounds idiom). The Go defaults are today's behaviour; the
// retune lives in config.yaml. These run the real load path.

func loadAndValidate(t *testing.T, doc string) Balance {
	t.Helper()
	cfg, err := loadConfig([]byte(doc))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	return cfg.Balance
}

func TestAmbientKnobs_AbsentKeysKeepTodaysBehaviour(t *testing.T) {
	b := loadAndValidate(t, "Balance:\n  StandMinStamina: 0.15\n")
	if b.MobIdleChatterScale != 1.0 || b.MobIdleMovementScale != 1.0 {
		t.Errorf("idle scales absent must load 1.0/1.0, got %v/%v", b.MobIdleChatterScale, b.MobIdleMovementScale)
	}
	if b.RoomIdleChancePct != 5 {
		t.Errorf("RoomIdleChancePct absent must load today's 5, got %v", b.RoomIdleChancePct)
	}
	if b.TipIntervalRounds != 75 {
		t.Errorf("TipIntervalRounds absent must load today's 75, got %v", b.TipIntervalRounds)
	}
	if b.AmbientListenerBudgetRounds != 0 {
		t.Errorf("AmbientListenerBudgetRounds absent must load 0 (no budget, as today), got %v", b.AmbientListenerBudgetRounds)
	}
	if bool(b.AmbientTelemetry) || b.AmbientTelemetryReportMinutes != 5 {
		t.Errorf("telemetry absent must load off with a 5 minute window, got %v/%v", b.AmbientTelemetry, b.AmbientTelemetryReportMinutes)
	}
	if b.ConversationBaseChancePct != 1.0 || b.ConversationPlayerArrivalBoostPct != 25 || b.ConversationCooldownRounds != 50 {
		t.Errorf("conversation knobs absent must load 1.0/25/50, got %v/%v/%v",
			b.ConversationBaseChancePct, b.ConversationPlayerArrivalBoostPct, b.ConversationCooldownRounds)
	}
	if b.GossipIntervalRounds != 75 {
		t.Errorf("GossipIntervalRounds absent must load 75, got %v", b.GossipIntervalRounds)
	}
}

func TestAmbientKnobs_ExplicitZeroIsOff(t *testing.T) {
	b := loadAndValidate(t, "Balance:\n"+
		"  MobIdleChatterScale: 0\n  MobIdleMovementScale: 0\n  RoomIdleChancePct: 0\n"+
		"  TipIntervalRounds: 0\n  AmbientListenerBudgetRounds: 0\n"+
		"  ConversationBaseChancePct: 0\n  ConversationPlayerArrivalBoostPct: 0\n  ConversationCooldownRounds: 0\n"+
		"  GossipIntervalRounds: 0\n  AmbientTelemetryReportMinutes: 0\n")
	zeros := map[string]float64{
		"MobIdleChatterScale":               float64(b.MobIdleChatterScale),
		"MobIdleMovementScale":              float64(b.MobIdleMovementScale),
		"RoomIdleChancePct":                 float64(b.RoomIdleChancePct),
		"TipIntervalRounds":                 float64(b.TipIntervalRounds),
		"AmbientListenerBudgetRounds":       float64(b.AmbientListenerBudgetRounds),
		"ConversationBaseChancePct":         float64(b.ConversationBaseChancePct),
		"ConversationPlayerArrivalBoostPct": float64(b.ConversationPlayerArrivalBoostPct),
		"ConversationCooldownRounds":        float64(b.ConversationCooldownRounds),
		"GossipIntervalRounds":              float64(b.GossipIntervalRounds),
	}
	for name, got := range zeros {
		if got != 0 {
			t.Errorf("%s: an explicit 0 is the off switch and must survive validation, got %v", name, got)
		}
	}
	if b.AmbientTelemetryReportMinutes != 5 {
		t.Errorf("AmbientTelemetryReportMinutes 0 has no off meaning and must take 5, got %v", b.AmbientTelemetryReportMinutes)
	}
}

func TestAmbientKnobs_Floors(t *testing.T) {
	b := loadAndValidate(t, "Balance:\n  GossipIntervalRounds: 10\n  RoomIdleChancePct: 140\n")
	if b.GossipIntervalRounds != 75 {
		t.Errorf("GossipIntervalRounds 1 to 19 keeps the old floor and reads as 75, got %v", b.GossipIntervalRounds)
	}
	if b.RoomIdleChancePct != 100 {
		t.Errorf("RoomIdleChancePct is a percent and caps at 100, got %v", b.RoomIdleChancePct)
	}
}

func TestAmbientKnobs_ShippedValues(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	cfg, err := loadConfig(raw)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	b := cfg.Balance
	if b.MobIdleChatterScale != 0.5 || b.MobIdleMovementScale != 1.0 || b.RoomIdleChancePct != 3 ||
		b.TipIntervalRounds != 75 || b.AmbientListenerBudgetRounds != 5 || bool(b.AmbientTelemetry) ||
		b.AmbientTelemetryReportMinutes != 5 {
		t.Errorf("shipped ambient knobs: chatter %v movement %v room %v tips %v budget %v telemetry %v/%v; want 0.5 1.0 3 75 5 false/5",
			b.MobIdleChatterScale, b.MobIdleMovementScale, b.RoomIdleChancePct, b.TipIntervalRounds,
			b.AmbientListenerBudgetRounds, b.AmbientTelemetry, b.AmbientTelemetryReportMinutes)
	}
	if b.ConversationBaseChancePct != 1.0 || b.ConversationPlayerArrivalBoostPct != 25 || b.ConversationCooldownRounds != 50 || b.GossipIntervalRounds != 75 {
		t.Errorf("shipped conversation/gossip knobs %v/%v/%v/%v, want today's 1.0/25/50/75",
			b.ConversationBaseChancePct, b.ConversationPlayerArrivalBoostPct, b.ConversationCooldownRounds, b.GossipIntervalRounds)
	}
}

func TestNewUnloadedConfig_SeedsAmbientSentinels(t *testing.T) {
	b := newUnloadedConfig().Balance
	if b.MobIdleChatterScale >= 0 || b.MobIdleMovementScale >= 0 || b.RoomIdleChancePct >= 0 || b.TipIntervalRounds >= 0 ||
		b.ConversationBaseChancePct >= 0 || b.ConversationPlayerArrivalBoostPct >= 0 || b.ConversationCooldownRounds >= 0 ||
		b.GossipIntervalRounds >= 0 {
		t.Fatalf("every ambient knob whose 0 is off and whose default is not 0 needs a negative sentinel: %+v", b)
	}
}
