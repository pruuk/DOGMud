package species

import "testing"

// SetSpeciesForTest replaces the package-level species roster with the provided
// map for the duration of the calling test, restoring the previous roster via
// t.Cleanup when the test ends.
//
// LoadDataFiles is called once from main.go, so a test binary starts with an
// empty roster. A test that simply calls LoadDataFiles() to fill it leaves it
// filled for every later test in the same binary, which is not inert: several
// packages have fixtures built as bare structs whose behaviour changes once a
// species record exists to hydrate from. internal/characters' Wear tests are a
// live example.
//
// Modelled on configs.SetConfigForTest, and exported for the same reason: the
// callers that need it are in other packages.
func SetSpeciesForTest(t *testing.T, s map[int]*Species) {
	t.Helper()
	original := allSpecies
	allSpecies = s
	t.Cleanup(func() { allSpecies = original })
}

// LoadForTest fills the roster from the real data files for the duration of the
// calling test and then restores whatever was there before.
//
// It reads configs.GetFilePathsConfig().DataFiles relative to the working
// directory, so the caller must already be at the repo root.
func LoadForTest(t *testing.T) {
	t.Helper()
	original := allSpecies
	t.Cleanup(func() { allSpecies = original })
	LoadDataFiles()
}
