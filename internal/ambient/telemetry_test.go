package ambient

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func pinTelemetry(t *testing.T, on bool, minutes int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientTelemetry = configs.ConfigBool(on)
	cfg.Balance.AmbientTelemetryReportMinutes = configs.ConfigInt(minutes)
	configs.SetConfigForTest(t, cfg)
}

func captureEmit(t *testing.T) *[]map[string]any {
	t.Helper()
	var got []map[string]any
	orig := emit
	emit = func(args ...any) {
		m := map[string]any{}
		for i := 0; i+1 < len(args); i += 2 {
			m[args[i].(string)] = args[i+1]
		}
		got = append(got, m)
	}
	t.Cleanup(func() { emit = orig })
	return &got
}

func TestTelemetry_OffKeepsNothing(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	pinTelemetry(t, false, 1)
	lines := captureEmit(t)
	Plan(Line{Room: room, Source: SourceMobChatter})
	CountMove(ambRoom, MoveIdle)
	telMu.Lock()
	n := len(telRooms)
	telMu.Unlock()
	if n != 0 {
		t.Fatalf("telemetry off must count nothing, %d rooms counted", n)
	}
	for r := ambNow + 1; r <= ambNow+40; r++ {
		RoundTick(r)
	}
	if len(*lines) != 0 {
		t.Fatalf("telemetry off must write nothing, wrote %d lines", len(*lines))
	}
}

func TestTelemetry_OnCountsAndWritesOneLinePerRoom(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	pinTelemetry(t, true, 1) // 1 minute = 15 rounds at 4 seconds
	lines := captureEmit(t)

	ChargeForTest(listenA, ambNow+5)
	Plan(Line{Room: room, Source: SourceMobChatter}) // A suppressed, B delivered
	for i := 0; i < 3; i++ {
		CountMove(ambRoom, MoveIdle)
	}
	CountMove(ambRoom, MovePath)

	for r := ambNow + 1; r <= ambNow+16; r++ {
		RoundTick(r)
	}
	if len(*lines) != 1 {
		t.Fatalf("one active room, one window: want 1 line, got %d: %v", len(*lines), *lines)
	}
	l := (*lines)[0]
	want := map[string]any{
		"window": 1, "room": ambRoom, "zone": ambZone, "round_seconds": 4, "window_rounds": 15,
		"listener_rounds":       32, // two players for 16 ticks
		"mob_chatter_offered":   2,
		"mob_chatter_delivered": 1, "mob_chatter_suppressed": 1,
		"room_idle_offered": 0, "move_idle": 3, "move_track": 0, "move_path": 1,
	}
	for k, v := range want {
		if l[k] != v {
			t.Errorf("%s = %v, want %v (line %v)", k, l[k], v, l)
		}
	}

	// The window reset its counters: the next line has listeners only.
	for r := ambNow + 17; r <= ambNow+31; r++ {
		RoundTick(r)
	}
	if len(*lines) != 2 || (*lines)[1]["mob_chatter_offered"] != 0 || (*lines)[1]["move_idle"] != 0 {
		t.Fatalf("second window must start from zero: %v", *lines)
	}
}

// An ambient say's echo (Task 8A) is planned against the neighbouring room,
// so it is counted there under say_echo, apart from the speaker's room.
func TestTelemetry_SayEchoCountsInTheReceivingRoom(t *testing.T) {
	neighbour := seedAmbientRoom(t, 0, nil, listenA, listenB)
	pinTelemetry(t, true, 1)
	lines := captureEmit(t)
	Plan(Line{Room: neighbour, Source: SourceSayEcho})
	for r := ambNow + 1; r <= ambNow+16; r++ {
		RoundTick(r)
	}
	if len(*lines) != 1 || (*lines)[0]["room"] != ambRoom || (*lines)[0]["say_echo_delivered"] != 2 || (*lines)[0]["mob_chatter_offered"] != 0 {
		t.Fatalf("the echo must be counted as say_echo in the receiving room: %v", *lines)
	}
}
