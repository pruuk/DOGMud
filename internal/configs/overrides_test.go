package configs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestOverlay tests overlaying a nested map into the Config struct.
func TestOverlay(t *testing.T) {
	// Start with a default config.
	cfg := Config{
		Validation: Validation{
			NameRejectRegex: "test",
		},
	}

	newValues := map[string]any{
		"Validation": map[string]any{
			"NameRejectRegex": "test-changed",
		},
	}

	if err := cfg.OverlayOverrides(newValues); err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}

	if cfg.Validation.NameRejectRegex != "test-changed" {
		t.Errorf("Expected NameRejectRegex to be \"test-changed\", got \"%s\"", cfg.Validation.NameRejectRegex)
	}
}

// TestOverlayDotMap tests overlaying a configuration using dot-syntax keys.
func TestOverlayDotMap(t *testing.T) {
	// Start with a default config.
	cfg := Config{
		Validation: Validation{
			NameRejectRegex: "test",
		},
	}

	dotValues := map[string]any{
		"Validation.NameRejectRegex": "test-changed",
	}

	if err := cfg.OverlayOverrides(dotValues); err != nil {
		t.Fatalf("OverlayDotMap failed: %v", err)
	}

	if cfg.Validation.NameRejectRegex != "test-changed" {
		t.Errorf("Expected LeaderboardSize to be \"test-changed\", got \"%s\"", cfg.Validation.NameRejectRegex)
	}
}

// TestOverlayDotMapMultipleFields demonstrates overlaying multiple fields using dot-syntax.
// Here, we extend the configuration to have an additional field.
func TestOverlayDotMapMultipleFields(t *testing.T) {
	// Define an extended configuration.
	type ExtendedStatistics struct {
		LeaderboardSize int    `yaml:"LeaderboardSize"`
		SomeField       string `yaml:"SomeField"`
	}

	type ExtendedConfig struct {
		Statistics ExtendedStatistics `yaml:"Statistics"`
	}

	cfg := ExtendedConfig{
		Statistics: ExtendedStatistics{
			LeaderboardSize: 5,
			SomeField:       "default",
		},
	}

	dotValues := map[string]any{
		"Statistics.LeaderboardSize": 25,
		"Statistics.SomeField":       "updated",
	}

	// Unflatten the dot-syntax map.
	nestedMap := unflattenMap(dotValues)
	// Marshal to YAML and then unmarshal into the extended config.
	b, err := yaml.Marshal(nestedMap)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if cfg.Statistics.LeaderboardSize != 25 {
		t.Errorf("Expected LeaderboardSize to be 25, got %d", cfg.Statistics.LeaderboardSize)
	}
	if cfg.Statistics.SomeField != "updated" {
		t.Errorf("Expected SomeField to be 'updated', got '%s'", cfg.Statistics.SomeField)
	}
}

// TestAddOverlayOverridesPreservesOperatorOverrides reproduces the boot
// sequence for a world whose config-overrides.yaml contains a partial
// Modules block: the operator has set some (but not all) of a module's keys,
// and the module's data-overlay config ships a superset of those keys.
// Applying the overlay must fill in the missing defaults WITHOUT clobbering
// the operator-supplied values or other modules' blocks — neither in the
// live config nor in the in-memory `overrides` union (which SetVal later
// persists to config-overrides.yaml on disk).
func TestAddOverlayOverridesPreservesOperatorOverrides(t *testing.T) {

	// Snapshot and restore package globals so this test is hermetic.
	origConfigData := configData
	origOverrides := overrides
	origKeyLookups := keyLookups
	origTypeLookups := typeLookups
	origModuleOverlayKeys := moduleOverlayKeys
	t.Cleanup(func() {
		configData = origConfigData
		overrides = origOverrides
		keyLookups = origKeyLookups
		typeLookups = origTypeLookups
		moduleOverlayKeys = origModuleOverlayKeys
	})

	configData = Config{}
	keyLookups = map[string]string{}
	typeLookups = map[string]string{}
	moduleOverlayKeys = map[string]struct{}{}

	// The operator's config-overrides.yaml: a partial Modules block for
	// "weather" (missing NewSetting), plus an unrelated module's block.
	operatorYAML := []byte(`
Modules:
  weather:
    Enabled: true
    CycleSeconds: 120
  othermod:
    Setting: keepme
`)
	loadedOverrides := map[string]any{}
	require.NoError(t, yaml.Unmarshal(operatorYAML, &loadedOverrides))
	overrides = loadedOverrides

	// ReloadConfig applies operator overrides onto the live config at boot.
	require.NoError(t, configData.OverlayOverrides(overrides))

	// The module loads and registers its data-overlay config, a superset of
	// the operator's keys. This mirrors how internal/plugins builds the map.
	err := AddOverlayOverrides(map[string]any{
		`Modules.weather.Enabled`:      false,        // module default; operator set true
		`Modules.weather.CycleSeconds`: 60,           // module default; operator set 120
		`Modules.weather.NewSetting`:   `overlayval`, // new key, absent from operator overrides
	})
	require.NoError(t, err)

	flat := Flatten(map[string]any(configData.Modules))

	// (a) Operator-supplied values must survive in the live config.
	require.Equal(t, true, flat[`weather.Enabled`], `operator override Modules.weather.Enabled was clobbered by the module overlay`)
	require.Equal(t, 120, flat[`weather.CycleSeconds`], `operator override Modules.weather.CycleSeconds was clobbered by the module overlay`)

	// (b) Keys absent from the operator overrides get the overlay defaults.
	require.Equal(t, `overlayval`, flat[`weather.NewSetting`])

	// (c) Other modules' blocks are untouched.
	require.Equal(t, `keepme`, flat[`othermod.Setting`])

	// (d) The in-memory overrides union must still carry the operator values.
	// The pre-fix code unconditionally did overrides[k] = v for every overlay
	// key, planting flat dotted duplicates of operator-set keys carrying the
	// module-default values; any later SetVal call persists that corrupted
	// union to config-overrides.yaml, destroying operator values ON DISK.
	_, dupEnabled := overrides[`Modules.weather.Enabled`]
	require.False(t, dupEnabled, `module default for operator-set Modules.weather.Enabled was added to the overrides union as a flat duplicate (SetVal would persist it to disk)`)
	_, dupCycle := overrides[`Modules.weather.CycleSeconds`]
	require.False(t, dupCycle, `module default for operator-set Modules.weather.CycleSeconds was added to the overrides union as a flat duplicate (SetVal would persist it to disk)`)

	flatUnion := Flatten(overrides)
	require.Equal(t, true, flatUnion[`Modules.weather.Enabled`], `operator override corrupted in the in-memory overrides union`)
	require.Equal(t, 120, flatUnion[`Modules.weather.CycleSeconds`], `operator override corrupted in the in-memory overrides union`)
	// The genuinely new key SHOULD be recorded in the union.
	require.Equal(t, `overlayval`, flatUnion[`Modules.weather.NewSetting`])
}

// TestAddOverlayOverridesAllowsModuleReRegistration pins a deliberate DOGMud
// divergence from upstream GoMud: a key whose current value came from a
// previous AddOverlayOverrides call (not from the operator) may be
// overwritten by a later AddOverlayOverrides call. Much of the DOGMud test
// suite uses AddOverlayOverrides as an in-memory set/restore seam and relies
// on this, and it also lets a module legitimately update its own default
// within a process lifetime. Operator values (file load / SetVal) remain
// protected — see TestAddOverlayOverridesPreservesOperatorOverrides.
func TestAddOverlayOverridesAllowsModuleReRegistration(t *testing.T) {

	origConfigData := configData
	origOverrides := overrides
	origKeyLookups := keyLookups
	origTypeLookups := typeLookups
	origModuleOverlayKeys := moduleOverlayKeys
	t.Cleanup(func() {
		configData = origConfigData
		overrides = origOverrides
		keyLookups = origKeyLookups
		typeLookups = origTypeLookups
		moduleOverlayKeys = origModuleOverlayKeys
	})

	configData = Config{}
	overrides = map[string]any{}
	keyLookups = map[string]string{}
	typeLookups = map[string]string{}
	moduleOverlayKeys = map[string]struct{}{}

	require.NoError(t, AddOverlayOverrides(map[string]any{
		`Modules.weather.CycleSeconds`: 60,
	}))
	require.NoError(t, AddOverlayOverrides(map[string]any{
		`Modules.weather.CycleSeconds`: 90,
	}))

	flat := Flatten(map[string]any(configData.Modules))
	require.Equal(t, 90, flat[`weather.CycleSeconds`], `module re-registration of its own default was ignored`)
	require.Equal(t, 90, Flatten(overrides)[`Modules.weather.CycleSeconds`])
}

// TestAddOverlayOverrides_OperatorValueSurvives_PublicAPI is the same guarantee
// as TestAddOverlayOverridesPreservesOperatorOverrides, expressed WITHOUT
// touching the moduleOverlayKeys ledger.
//
// That distinction is the point. The other two tests reset the private ledger,
// so against the unfixed code they fail to COMPILE rather than fail an
// assertion -- which proves the fix is absent but never demonstrates the bug.
// This one touches no private ledger, so it compiles fine against the old code
// and fails on the ASSERTION, showing the actual defect: a module default
// overwriting a value the operator set in config-overrides.yaml.
//
// Verified RED by reverting only configs.go to the pre-fix version (which did
// an unconditional `overrides[k] = v`): CycleSeconds came back 60, not 120.
func TestAddOverlayOverrides_OperatorValueSurvives_PublicAPI(t *testing.T) {
	origConfigData := configData
	origOverrides := overrides
	origKeyLookups := keyLookups
	origTypeLookups := typeLookups
	t.Cleanup(func() {
		configData = origConfigData
		overrides = origOverrides
		keyLookups = origKeyLookups
		typeLookups = origTypeLookups
	})

	configData = Config{}
	keyLookups = map[string]string{}
	typeLookups = map[string]string{}

	// The operator pinned CycleSeconds to 120 in config-overrides.yaml, applied
	// onto the live config the way ReloadConfig does at boot.
	operatorYAML := []byte("Modules:\n  weather:\n    CycleSeconds: 120\n")
	loaded := map[string]any{}
	require.NoError(t, yaml.Unmarshal(operatorYAML, &loaded))
	overrides = loaded
	require.NoError(t, configData.OverlayOverrides(overrides))

	// The module then registers its shipped default of 60 for the same key.
	if err := AddOverlayOverrides(map[string]any{
		"Modules.weather.CycleSeconds": 60,
	}); err != nil {
		t.Fatalf("AddOverlayOverrides: %v", err)
	}

	got := Flatten(overrides)["Modules.weather.CycleSeconds"]
	if fmt.Sprintf("%v", got) != "120" {
		t.Errorf("the operator's CycleSeconds=120 was overwritten by the module default; got %v. A module may fill in keys the operator did not set, never replace ones they did", got)
	}
}
