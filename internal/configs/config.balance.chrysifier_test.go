package configs

import "testing"

func TestChrysifierDefaults(t *testing.T) {
	b := GetBalanceConfig()
	if float64(b.HomunculusCraftScale) != 4.0 {
		t.Fatalf("HomunculusCraftScale = %v, want 4.0", float64(b.HomunculusCraftScale))
	}
	if int(b.HomunculusConvictionReserve) != 300 {
		t.Fatalf("HomunculusConvictionReserve = %v, want 300", int(b.HomunculusConvictionReserve))
	}
}
