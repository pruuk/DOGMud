package configs

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestSetConfigWithLookupsForTestResolvesKeys proves the helper gives
// FindFullPath real lookups (a test binary never runs ReloadConfig) and puts
// the previous lookups back when the test ends.
//
// It resolves `aicompanion.apikey`, not the bare `apikey`: once the
// APIFramework section exists the bare suffix is shared by two keys, and
// buildKeyLookups resolves a shared suffix by map iteration order.
func TestSetConfigWithLookupsForTestResolvesKeys(t *testing.T) {
	beforePath, beforeType := FindFullPath(`aicompanion.apikey`)

	t.Run(`installed`, func(t *testing.T) {
		c := GetConfig()
		c.Modules = Modules{`aicompanion`: map[string]any{`APIKey`: ``}}
		SetConfigWithLookupsForTest(t, c)

		if p, typ := FindFullPath(`seed`); p != `Server.Seed` || typ != `configs.ConfigSecret` {
			t.Errorf(`FindFullPath("seed") = (%q, %q), want ("Server.Seed", "configs.ConfigSecret")`, p, typ)
		}
		if p, typ := FindFullPath(`aicompanion.apikey`); p != `Modules.aicompanion.APIKey` || typ != `string` {
			t.Errorf(`FindFullPath("aicompanion.apikey") = (%q, %q), want ("Modules.aicompanion.APIKey", "string")`, p, typ)
		}
	})

	if p, typ := FindFullPath(`aicompanion.apikey`); p != beforePath || typ != beforeType {
		t.Errorf(`lookups not restored: FindFullPath("aicompanion.apikey") = (%q, %q), was (%q, %q)`, p, typ, beforePath, beforeType)
	}
}

// lockTestConfig is the shipped Server.Locked list plus an aicompanion block,
// so the module keys resolve through FindFullPath. Modules is a fresh map:
// never mutate a map GetConfig handed back, another test may share it.
func lockTestConfig() Config {
	c := GetConfig()
	c.Server.Locked = ConfigSliceString{`FilePaths`, `Server.CurrentVersion`, `Server.NextRoomId`, `Server.Seed`, `Server.OnLoginCommands`, `Server.BannedNames`}
	c.Modules = Modules{`aicompanion`: map[string]any{`APIKey`: ``, `Model`: ``, `RelayOrigin`: ``, `ModerateOutput`: true, `DailyTokenBudget`: 2000000}}
	return c
}

func TestSetValRefusesLockedKeys(t *testing.T) {
	overridePath := SetConfigWithLookupsForTest(t, lockTestConfig())

	// Each case carries its reason as a field, not a trailing comment, so the
	// table stays gofmt-stable when a row is added.
	cases := []struct{ key, resolved, why string }{
		{`Server.Seed`, `Server.Seed`, `Server.Locked, exact`},
		{`seed`, `Server.Seed`, `bare suffix key: the RESOLVED path is checked`},
		{`FilePaths.DataFiles`, `FilePaths.DataFiles`, `Server.Locked prefix`},
		{`Server.Locked`, `Server.Locked`, `hard list`},
		{`locked`, `Server.Locked`, `hard list through a suffix key`},
		{`FilePaths.WebDomain`, `FilePaths.WebDomain`, `hard list`},
		{`Modules.aicompanion.APIKey`, `Modules.aicompanion.APIKey`, `hard list, module key`},
		{`aicompanion.apikey`, `Modules.aicompanion.APIKey`, `hard list through a suffix key`},
		{`apikey`, `APIFramework.APIKey|Modules.aicompanion.APIKey`, `hard list through a SHARED suffix key: either resolution is locked`},
		{`Modules.aicompanion.RelayOrigin`, `Modules.aicompanion.RelayOrigin`, `hard list`},
		{`Modules.aicompanion.Model`, `Modules.aicompanion.Model`, `hard list`},
		{`Modules.aicompanion.ModerateOutput`, `Modules.aicompanion.ModerateOutput`, `hard list (ruling 13), resolves through the lookups`},
		{`moderateoutput`, `Modules.aicompanion.ModerateOutput`, `hard list (ruling 13) through a suffix key`},
		{`Modules.aicompanion.ModerationModel`, `Modules.aicompanion.ModerationModel`, `hard list (ruling 13), not in the lookups: refused as LOCKED, not as unknown`},
		{`Modules.aicompanion.BaseURL`, `Modules.aicompanion.BaseURL`, `not in the lookups: refused as LOCKED, not as unknown`},
		{`APIFramework.APIKey`, `APIFramework.APIKey`, `hard list, the shared model key`},
		{`apiframework.apikey`, `APIFramework.APIKey`, `hard list, lowercased full path`},
		{`Integrations.Discord.WebhookUrl`, `Integrations.Discord.WebhookUrl`, `hard list: names where server data is sent`},
		{`webhookurl`, `Integrations.Discord.WebhookUrl`, `hard list through a suffix key`},
	}
	for _, tc := range cases {
		err := SetVal(tc.key, `x`)
		if !errors.Is(err, ErrLockedConfig) {
			t.Errorf(`SetVal(%q) = %v, want ErrLockedConfig (%s)`, tc.key, err, tc.why)
			continue
		}
		// resolved may list alternatives separated by "|": a suffix shared by
		// two keys resolves to either (buildKeyLookups, map order).
		named := false
		for want := range strings.SplitSeq(tc.resolved, `|`) {
			if strings.Contains(err.Error(), want) {
				named = true
			}
		}
		if !named {
			t.Errorf(`SetVal(%q) error %q does not name the resolved path %q (%s)`, tc.key, err, tc.resolved, tc.why)
		}
	}

	if got := string(GetServerConfig().Seed); got == `x` {
		t.Errorf(`Server.Seed changed to %q through a refused SetVal`, got)
	}
	if _, err := os.Stat(overridePath); !os.IsNotExist(err) {
		t.Errorf(`a refused SetVal wrote %s (stat err %v)`, overridePath, err)
	}
}

// TestSetValStillWritesAnUnlockedKey is the positive control: without it the
// refusals above could come from a SetVal that refuses everything.
func TestSetValStillWritesAnUnlockedKey(t *testing.T) {
	overridePath := SetConfigWithLookupsForTest(t, lockTestConfig())

	if err := SetVal(`motd`, `hello from the lock test`); err != nil {
		t.Fatalf(`SetVal("motd") = %v, want nil`, err)
	}
	if got := string(GetServerConfig().Motd); got != `hello from the lock test` {
		t.Errorf(`Server.Motd = %q after SetVal`, got)
	}
	written, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf(`read %s: %v`, overridePath, err)
	}
	if !strings.Contains(string(written), `hello from the lock test`) {
		t.Errorf(`override file does not carry the new value:\n%s`, written)
	}
}

func TestSetEngineValSkipsServerLockedButNotTheHardList(t *testing.T) {
	SetConfigWithLookupsForTest(t, lockTestConfig())

	if err := SetVal(`Server.NextRoomId`, `4321`); !errors.Is(err, ErrLockedConfig) {
		t.Fatalf(`operator SetVal("Server.NextRoomId") = %v, want ErrLockedConfig`, err)
	}
	if err := SetEngineVal(`Server.NextRoomId`, `4321`); err != nil {
		t.Fatalf(`SetEngineVal("Server.NextRoomId") = %v, want nil`, err)
	}
	if got := int(GetServerConfig().NextRoomId); got != 4321 {
		t.Errorf(`Server.NextRoomId = %d after SetEngineVal, want 4321`, got)
	}
	for _, key := range []string{`Server.Locked`, `Modules.aicompanion.APIKey`, `Modules.aicompanion.ModerateOutput`, `FilePaths.WebDomain`} {
		if err := SetEngineVal(key, `x`); !errors.Is(err, ErrLockedConfig) {
			t.Errorf(`SetEngineVal(%q) = %v, want ErrLockedConfig`, key, err)
		}
	}
}

func TestIsLocked(t *testing.T) {
	SetConfigWithLookupsForTest(t, lockTestConfig())

	for _, p := range []string{`filepaths`, `FilePaths.DataFiles`, `server.seed`, `Server.Locked`,
		`modules.aicompanion.apikey`, `MODULES.AICOMPANION.PLAYERKEYS`, `APIFramework.BaseURL`,
		`modules.aicompanion.moderateoutput`, `Modules.aicompanion.ModerationModel`,
		`Integrations.Discord.WebhookUrl`} {
		if !IsLocked(p) {
			t.Errorf(`IsLocked(%q) = false, want true`, p)
		}
	}
	// A partial path must stay open: the server config menu browses through
	// modules.aicompanion to reach its unlocked budget knobs.
	for _, p := range []string{`Server.Motd`, `Modules.aicompanion.DailyTokenBudget`, `modules.aicompanion`, `Network.HttpPort`} {
		if IsLocked(p) {
			t.Errorf(`IsLocked(%q) = true, want false`, p)
		}
	}
}
