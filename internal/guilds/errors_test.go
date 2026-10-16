package guilds

import (
	"errors"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// Callers branch on these cases with errors.Is, so each must come back as its
// sentinel, not as look-alike text.
func TestSentinels_ReturnedForTheirCases(t *testing.T) {
	defer SetDataDirForTest(t.TempDir())()
	resetRegistry()

	if err := AddMember("NOPE", 2, "X"); !errors.Is(err, ErrNoSuchGuild) {
		t.Errorf("AddMember on a missing guild: err=%v want ErrNoSuchGuild", err)
	}
	if err := WithdrawGold("NOPE", 5); !errors.Is(err, ErrNoSuchGuild) {
		t.Errorf("WithdrawGold on a missing guild: err=%v want ErrNoSuchGuild", err)
	}
	if err := DonateItem("NOPE", items.Item{ItemId: 1}); !errors.Is(err, ErrNoSuchGuild) {
		t.Errorf("DonateItem on a missing guild: err=%v want ErrNoSuchGuild", err)
	}

	if _, err := Create("SN", "Sentinels", 1, "Lead"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := WithdrawGold("SN", 5); !errors.Is(err, ErrTreasuryShort) {
		t.Errorf("WithdrawGold beyond the treasury: err=%v want ErrTreasuryShort", err)
	}

	g, _ := Get("SN")
	g.Vault = make([]items.Item, int(configs.GetBalanceConfig().GuildVaultCapacity))
	if err := DonateItem("SN", items.Item{ItemId: 1}); !errors.Is(err, ErrVaultFull) {
		t.Errorf("DonateItem into a full vault: err=%v want ErrVaultFull", err)
	}
}

// A disk failure inside Create is a server fault (ErrSaveFailed); a
// validation failure is the player's and must not match it.
func TestCreate_SaveFailureIsDistinguishable(t *testing.T) {
	defer SetDataDirForTest(unwritableDir(t))()
	resetRegistry()

	_, err := Create("SF", "Savefail", 1, "Lead")
	if !errors.Is(err, ErrSaveFailed) {
		t.Errorf("Create on an unwritable store: err=%v want ErrSaveFailed", err)
	}
	if _, ok := Get("SF"); ok {
		t.Error("a guild whose save failed must not stay in the registry")
	}

	_, err = Create("X", "Badtag", 1, "Lead")
	if err == nil || errors.Is(err, ErrSaveFailed) {
		t.Errorf("Create with a bad tag: err=%v want a validation error, not ErrSaveFailed", err)
	}
}
