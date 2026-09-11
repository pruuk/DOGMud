package rooms

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const sightTestInfraredBuffId = 7401

var sightTestTag = regexp.MustCompile(`<[^>]*>`)

func sightTestPlain(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(sightTestTag.ReplaceAllString(l, "")))
	}
	return out
}

// sightTestRoom seeds three players in one room of the given biome: 7411
// Aliceia and 7412 Bobrick are the parties, 7413 Ordel watches.
func sightTestRoom(t *testing.T, biome string) *Room {
	t.Helper()
	t.Cleanup(SeedBiomesForTest(map[string]*BiomeInfo{
		"cave":    {BiomeId: "cave", DarkArea: true},
		"city":    {BiomeId: "city", LitArea: true},
		"default": {BiomeId: "default", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		sightTestInfraredBuffId: {BuffId: sightTestInfraredBuffId, Name: "Test Heat Eyes", Flags: []buffs.Flag{buffs.InfraredVision}},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7411: users.NewTestUser(7411, "aliceia", "Aliceia", 97411),
		7412: users.NewTestUser(7412, "bobrick", "Bobrick", 97412),
		7413: users.NewTestUser(7413, "ordel", "Ordel", 97413),
	}))
	r := &Room{RoomId: 7410, Biome: biome}
	for _, id := range []int{7411, 7412, 7413} {
		r.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	return r
}

func TestSendTextVisualHidingNames_InfraredObserverReadsFigures(t *testing.T) {
	r := sightTestRoom(t, "cave")
	if !users.GetByUserId(7413).Character.Buffs.AddBuff(sightTestInfraredBuffId, true) {
		t.Fatal("precondition: the observer should now carry infrared")
	}
	r.SendTextVisualHidingNames(messaging.CategoryKick, "Aliceia kicks Bobrick!",
		[]string{"Aliceia", "Bobrick"}, 7411, 7412)

	got := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
	if len(got) != 1 || got[0] != "A figure kicks a figure!" {
		t.Fatalf("infrared observer read %q, want one line %q", got, "A figure kicks a figure!")
	}
}

func TestSendTextVisualHidingNames_UnsightedObserverGetsNothing(t *testing.T) {
	r := sightTestRoom(t, "cave")
	r.SendTextVisualHidingNames(messaging.CategoryKick, "Aliceia kicks Bobrick!",
		[]string{"Aliceia", "Bobrick"}, 7411, 7412)

	if got := events.DrainQueuedMessagesForTest(7413); len(got) != 0 {
		t.Fatalf("an observer who cannot see got %q", got)
	}
}

func TestSendTextVisualHidingNames_LitObserverReadsTheNames(t *testing.T) {
	r := sightTestRoom(t, "city")
	r.SendTextVisualHidingNames(messaging.CategoryKick, "Aliceia kicks Bobrick!",
		[]string{"Aliceia", "Bobrick"}, 7411, 7412)

	got := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
	if len(got) != 1 || got[0] != "Aliceia kicks Bobrick!" {
		t.Fatalf("lit observer read %q, want the names", got)
	}
}

func TestRoomParticipantSight_JudgesTheUserInThisRoom(t *testing.T) {
	r := sightTestRoom(t, "cave")
	users.GetByUserId(7412).Character.Buffs.AddBuff(sightTestInfraredBuffId, true)

	if d := r.ParticipantSight(7411); d != messaging.SightNone {
		t.Errorf("no vision in a cave = %v, want SightNone", d)
	}
	if d := r.ParticipantSight(7412); d != messaging.SightShapes {
		t.Errorf("infrared in a cave = %v, want SightShapes", d)
	}
	if d := r.ParticipantSight(99999); d != messaging.SightFull {
		t.Errorf("unknown user = %v, want SightFull", d)
	}
}
