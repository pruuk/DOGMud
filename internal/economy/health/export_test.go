package health

// ResetForagersSeenForTest forgets every forager seen live, as at boot, so a
// test's "(despawned)" versus "(not yet spawned)" does not depend on which
// tests ran before it.
func ResetForagersSeenForTest() {
	foragersSeenMu.Lock()
	defer foragersSeenMu.Unlock()
	foragersSeen = map[int]bool{}
}
