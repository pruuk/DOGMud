package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 1: scan passes over a secret exit the scanner has not
// found, as look does, and scans through it once found (task 17 playtest:
// scan skipped every secret exit, found or not).
func TestScan_ASecretExitIsScannedOnceFound(t *testing.T) {
	actor := scanLockScene(t, 90, false, false, true)
	here := rooms.LoadRoom(scanLockHereId)
	north := here.Exits["north"]
	north.Secret = true
	here.Exits["north"] = north

	require.Empty(t, Scan(actor, ScanOptions{}).Sightings, "an unfound secret exit is not scanned")

	actor.char.LearnSecretExit(scanLockHereId, "north")
	result := Scan(actor, ScanOptions{})
	require.Len(t, result.Sightings, 1, "a found secret exit is scanned")
	require.Equal(t, "north", result.Sightings[0].ExitName)
}

// Owner ruling 2026-11-26 (#781 task 17): a scanner who sees nothing in its
// own room scans nothing and names no exit, as look names none: a player reads
// look's own too-dark line, and a mob, under the same rule, gets no sightings.
func TestScan_InTotalDarknessNamesNoExit(t *testing.T) {
	player := scanLockScene(t, 0, false, false, true)
	result := Scan(player, ScanOptions{})
	require.Empty(t, result.Sightings, "no exit is scanned in total darkness")
	sent := strings.Join(player.sent, "\n")
	require.Contains(t, sent, "It is too dark to see. You need light, or eyes that do not need it.")
	require.NotContains(t, sent, "north")
	require.NotContains(t, sent, "Midroad Scout")
	look, _ := NoSightLine(ResolveLook(player, "").Kind)
	require.Contains(t, sent, look, "scan says what look says")

	mob := scanLockScene(t, 0, false, false, false)
	require.Empty(t, Scan(mob, ScanOptions{HostileOnly: true}).Sightings, "a mob in total darkness scans nothing")
}
