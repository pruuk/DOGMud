package itemlight

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/uuid"
)

func id(b byte) uuid.UUID { return uuid.UUID{b} }

func TestSetRecordsAndReportsChange(t *testing.T) {
	t.Cleanup(ResetForTest())
	if !Set(7, id(1), Light, 52) {
		t.Fatal("first Set reported no change")
	}
	if Set(7, id(1), Light, 52) {
		t.Error("the same output again reported a change: the tick would rewrite every round")
	}
	if !Set(7, id(1), Light, 30) {
		t.Error("a new value reported no change")
	}
	if v, ok := Get(7, id(1)); !ok || v != 30 {
		t.Errorf("Get = %v, %v; want 30, true", v, ok)
	}
}

func TestUnlitIsKnownButAddsNoTerm(t *testing.T) {
	t.Cleanup(ResetForTest())
	Set(7, id(1), Light, math.Inf(-1))
	Set(7, id(2), Light, -3) // negative reads as unlit
	if v, ok := Get(7, id(1)); !ok || !math.IsInf(v, -1) {
		t.Errorf("unlit Get = %v, %v; want -Inf, true", v, ok)
	}
	if Lit(7, id(1)) || Lit(7, id(2)) {
		t.Error("an unlit fixture reads lit")
	}
	if light, dark := Terms(7); len(light) != 0 || len(dark) != 0 {
		t.Errorf("Terms = %v, %v; want none", light, dark)
	}
}

func TestTermsSplitByKindInUUIDOrder(t *testing.T) {
	t.Cleanup(ResetForTest())
	Set(7, id(3), Light, 20)
	Set(7, id(1), Light, 52)
	Set(7, id(2), Darkness, 40)
	Set(8, id(4), Light, 99) // another room
	light, dark := Terms(7)
	if len(light) != 2 || light[0] != 52 || light[1] != 20 {
		t.Errorf("light = %v, want [52 20] (UUID order)", light)
	}
	if len(dark) != 1 || dark[0] != 40 {
		t.Errorf("dark = %v, want [40]", dark)
	}
}

func TestClearRetainAndClearRoom(t *testing.T) {
	t.Cleanup(ResetForTest())
	Set(7, id(1), Light, 52)
	Set(7, id(2), Light, 30)
	Set(7, id(3), Light, 20)
	Clear(7, id(1))
	if _, ok := Get(7, id(1)); ok {
		t.Error("Clear left the output")
	}
	Retain(7, map[uuid.UUID]bool{id(2): true})
	if _, ok := Get(7, id(3)); ok {
		t.Error("Retain kept an output not in keep")
	}
	if _, ok := Get(7, id(2)); !ok {
		t.Error("Retain dropped an output in keep")
	}
	ClearRoom(7)
	if light, _ := Terms(7); len(light) != 0 {
		t.Errorf("ClearRoom left %v", light)
	}
}
