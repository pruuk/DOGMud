package configs

import (
	"path/filepath"
	"testing"
)

// setBalanceForTest replaces the module-level balance config with
// the provided instance for the duration of the calling test. The
// prior balance is automatically restored via t.Cleanup when the
// test ends. Intended for use from _test.go files in the configs
// package.
func setBalanceForTest(t *testing.T, b *Balance) {
	t.Helper()
	original := configData.Balance
	configData.Balance = *b
	t.Cleanup(func() {
		configData.Balance = original
		// Re-validate to fill any zero'd defaults from the original.
		configData.Balance.Validate()
	})
}

// SetConfigForTest replaces the entire package-level config with the
// provided instance for the duration of the calling test. The prior config
// is automatically restored via t.Cleanup when the test ends. Exported (unlike
// its sibling setBalanceForTest) because callers outside this package — e.g.
// internal/characters, which must chdir + configs.ReloadConfig() to read the
// real _datafiles/config.yaml — need it too. Self-registering the restore
// here (rather than leaving it to the caller) is the point: a caller that
// mutates configData and forgets its own cleanup leaks that mutation into
// every later test sharing the same test binary.
func SetConfigForTest(t *testing.T, c Config) {
	t.Helper()
	configDataLock.Lock()
	original := configData
	configData = c
	configDataLock.Unlock()
	t.Cleanup(func() {
		configDataLock.Lock()
		defer configDataLock.Unlock()
		configData = original
	})
}

// ReloadConfigForTest loads the shipped _datafiles/config.yaml (ReloadConfig)
// for the calling test and puts back, at cleanup, everything the reload
// replaced: the config, the key and type lookups, the overrides union and the
// module overlay ledger. A bare ReloadConfig in a test leaked the shipped
// values (AmbientListenerBudgetRounds 5, for one) into every later test in the
// same binary, which then passed or failed by test order (#720). Tests call
// this, never ReloadConfig; the guard
// TestTestsReloadTheShippedConfigOnlyThroughTheRestoringHelper holds that. It
// reads _datafiles/config.yaml relative to the working directory, as
// ReloadConfig does.
func ReloadConfigForTest(t testing.TB) error {
	t.Helper()
	configDataLock.Lock()
	prevConfig := configData
	prevKeys, prevTypes := keyLookups, typeLookups
	prevOverrides, prevOwned := overrides, moduleOverlayKeys
	configDataLock.Unlock()
	t.Cleanup(func() {
		configDataLock.Lock()
		defer configDataLock.Unlock()
		configData = prevConfig
		keyLookups, typeLookups = prevKeys, prevTypes
		overrides, moduleOverlayKeys = prevOverrides, prevOwned
	})
	return ReloadConfig()
}

// SetConfigWithLookupsForTest installs c as SetConfigForTest does and also
// builds the key and type lookups FindFullPath reads from c, so SetVal
// resolves keys in a test binary that never ran ReloadConfig. It snapshots and
// restores the lookups, the overrides union and the module overlay ledger,
// all of which SetVal mutates, and points CONFIG_PATH at a scratch file so a
// SetVal that succeeds never writes a real config-overrides.yaml. It returns
// that scratch path. Not for parallel tests (it uses t.Setenv). The swap and
// the restore both hold configDataLock, as SetConfigForTest does.
func SetConfigWithLookupsForTest(t *testing.T, c Config) string {
	t.Helper()
	overridePath := filepath.Join(t.TempDir(), `config-overrides.yaml`)
	t.Setenv(`CONFIG_PATH`, overridePath)
	SetConfigForTest(t, c)

	// Built before locking: AllConfigData takes no lock of its own.
	newKeys, newTypes := buildKeyLookups(c)

	configDataLock.Lock()
	prevKeys, prevTypes := keyLookups, typeLookups
	prevOverrides, prevOwned := overrides, moduleOverlayKeys
	keyLookups, typeLookups = newKeys, newTypes
	overrides = map[string]any{}
	moduleOverlayKeys = map[string]struct{}{}
	configDataLock.Unlock()

	t.Cleanup(func() {
		configDataLock.Lock()
		defer configDataLock.Unlock()
		keyLookups, typeLookups = prevKeys, prevTypes
		overrides, moduleOverlayKeys = prevOverrides, prevOwned
	})
	return overridePath
}
