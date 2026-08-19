package actions

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test item constructors for reload
// ---------------------------------------------------------------------------

func reloadRangedWeapon(id int, ammoTag string) items.Item {
	return items.Item{
		ItemId: id,
		Spec: &items.ItemSpec{
			ItemId:  id,
			Name:    "longbow",
			Type:    items.Weapon,
			Subtype: items.Shooting,
			AmmoTag: ammoTag,
			Hands:   2,
		},
	}
}

func reloadMeleeWeapon(id int) items.Item {
	return items.Item{
		ItemId: id,
		Spec: &items.ItemSpec{
			ItemId:  id,
			Name:    "sword",
			Type:    items.Weapon,
			Subtype: items.Slashing,
			Hands:   1,
		},
	}
}

func reloadAmmoBundle(id int, ammoTag string, uses int) items.Item {
	return items.Item{
		ItemId: id,
		Uses:   uses,
		Spec: &items.ItemSpec{
			ItemId:  id,
			Name:    "arrows",
			Type:    items.Ammo,
			AmmoTag: ammoTag,
		},
	}
}

type staleRangedSecondaryActor struct {
	Actor
	characterCalls int
	onAdmission    func(*characters.Character)
}

func (a *staleRangedSecondaryActor) GetCharacter() *characters.Character {
	a.characterCalls++
	char := a.Actor.GetCharacter()
	if a.characterCalls == 3 && a.onAdmission != nil {
		a.onAdmission(char)
	}
	return char
}

// specialMoveCooldownPeriod is the cooldown string ExecuteReload uses; tests
// reuse it to probe whether the cooldown was consumed.
func specialMoveCooldownPeriod() string {
	return fmt.Sprintf("%d rounds", configs.GetBalanceConfig().SpecialMoveCooldown)
}

// ---------------------------------------------------------------------------
// 1. No ranged weapon equipped → NoWeapon
// ---------------------------------------------------------------------------

func TestReload_NoRangedWeapon(t *testing.T) {
	char := characters.New()
	char.Equipment.Weapon = reloadMeleeWeapon(2)
	actor := newStubActor(char, newTestRoom())

	res := ExecuteReload(actor)

	assert.True(t, res.NoWeapon, "expected NoWeapon when no ranged weapon equipped")
	assert.False(t, res.Loaded)
}

// ---------------------------------------------------------------------------
// 2. Ranged weapon already loaded → AlreadyLoaded
// ---------------------------------------------------------------------------

func TestReload_AlreadyLoaded(t *testing.T) {
	char := characters.New()
	w := reloadRangedWeapon(1, "arrows")
	w.Loaded = true
	char.Equipment.Weapon = w
	char.Items = append(char.Items, reloadAmmoBundle(3, "arrows", 20))
	actor := newStubActor(char, newTestRoom())

	res := ExecuteReload(actor)

	assert.True(t, res.AlreadyLoaded, "expected AlreadyLoaded")
	assert.False(t, res.Loaded)
	// Ammo must not be consumed.
	assert.Equal(t, 20, char.Items[0].Uses, "ammo should be untouched")
}

// ---------------------------------------------------------------------------
// 3. No matching ammo bundle → NoAmmo, cooldown NOT consumed
// ---------------------------------------------------------------------------

func TestReload_NoAmmo_DoesNotBurnCooldown(t *testing.T) {
	char := characters.New()
	char.Equipment.Weapon = reloadRangedWeapon(1, "arrows")
	// Wrong tag bundle present; should not match.
	char.Items = append(char.Items, reloadAmmoBundle(4, "bolts", 20))
	actor := newStubActor(char, newTestRoom())

	res := ExecuteReload(actor)

	assert.True(t, res.NoAmmo, "expected NoAmmo")
	assert.Equal(t, "arrows", res.AmmoTag, "NoAmmo result should report the needed tag")
	assert.False(t, res.Loaded)
	assert.False(t, char.Equipment.Weapon.Loaded, "weapon must stay unloaded")

	// Proof the cooldown was never consumed: a fresh Try succeeds.
	assert.True(t, char.Cooldowns.Try("special-move", specialMoveCooldownPeriod()),
		"special-move cooldown must not have been consumed on a no-ammo reload")
}

// ---------------------------------------------------------------------------
// 4. Cooldown busy → OnCooldown, ammo NOT consumed, weapon stays unloaded
// ---------------------------------------------------------------------------

func TestReload_OnCooldown(t *testing.T) {
	char := characters.New()
	char.Equipment.Weapon = reloadRangedWeapon(1, "arrows")
	char.Items = append(char.Items, reloadAmmoBundle(3, "arrows", 20))

	// Pre-occupy the shared special-move cooldown slot.
	assert.True(t, char.Cooldowns.Try("special-move", specialMoveCooldownPeriod()))

	actor := newStubActor(char, newTestRoom())
	res := ExecuteReload(actor)

	assert.True(t, res.OnCooldown, "expected OnCooldown when special-move is busy")
	assert.False(t, res.Loaded)
	assert.False(t, char.Equipment.Weapon.Loaded, "weapon must stay unloaded")
	assert.Equal(t, 20, char.Items[0].Uses, "ammo must not be consumed on cooldown")
}

// ---------------------------------------------------------------------------
// 5. Success: bundle decremented, weapon Loaded persisted, cooldown consumed
// ---------------------------------------------------------------------------

func TestReload_Success(t *testing.T) {
	char := characters.New()
	char.Equipment.Weapon = reloadRangedWeapon(1, "arrows")
	char.Items = append(char.Items, reloadAmmoBundle(3, "arrows", 20))
	actor := newStubActor(char, newTestRoom())

	res := ExecuteReload(actor)

	assert.True(t, res.Loaded, "expected success")
	assert.False(t, res.BundleEmptied, "bundle had plenty left")
	assert.Equal(t, "arrows", res.AmmoTag)
	assert.Equal(t, 19, char.Items[0].Uses, "bundle Uses should drop by one")
	// Writeback proof: the equipment slot itself is now loaded.
	assert.True(t, char.Equipment.Weapon.Loaded, "weapon Loaded must persist on the slot")

	// Cooldown consumed: an immediate second Try fails.
	assert.False(t, char.Cooldowns.Try("special-move", specialMoveCooldownPeriod()),
		"special-move cooldown should have been consumed by the reload")
}

// TestReload_RefusedCostIsAtomic catches cost admission being omitted or
// moved after bundle, loaded-state, or cooldown mutation.
func TestReload_RefusedCostIsAtomic(t *testing.T) {
	char := characters.New()
	char.Stamina = 0
	char.StaminaMax.Value = 100
	char.Equipment.Weapon = reloadRangedWeapon(1, "arrows")
	char.Items = append(char.Items, reloadAmmoBundle(3, "arrows", 20))
	char.Cooldowns = characters.Cooldowns{"special-move": -2, "other": 7}
	cooldownsBefore := maps.Clone(char.Cooldowns)

	res := ExecuteReload(newStubActor(char, newTestRoom()))

	require.Equal(t, characters.CostRefused, res.Cost.Status)
	assert.False(t, res.Loaded)
	assert.Equal(t, 0, char.Stamina)
	assert.False(t, char.Equipment.Weapon.Loaded)
	require.Len(t, char.Items, 1)
	assert.Equal(t, 20, char.Items[0].Uses)
	assert.Equal(t, cooldownsBefore, char.Cooldowns)
}

// TestReload_SuccessPaysOnceAndMutatesOnce catches duplicate charging,
// consuming the wrong bundle, skipping Loaded writeback, or omitting cooldown.
func TestReload_SuccessPaysOnceAndMutatesOnce(t *testing.T) {
	char := characters.New()
	char.Stats.Strength.ValueAdj = 100
	char.Skills["ranged combat"] = 25
	char.Stamina = 10
	char.StaminaMax.Value = 100
	char.Equipment.Weapon = reloadRangedWeapon(1, "arrows")
	char.Items = []items.Item{
		reloadAmmoBundle(4, "bolts", 7),
		reloadAmmoBundle(3, "arrows", 20),
	}

	res := ExecuteReload(newStubActor(char, newTestRoom()))

	require.True(t, res.Loaded)
	require.Equal(t, characters.CostPaid, res.Cost.Status)
	assert.Equal(t, 1, res.Cost.Charged)
	assert.Equal(t, 9, char.Stamina)
	assert.Equal(t, 7, char.Items[0].Uses, "incompatible bundle must be untouched")
	assert.Equal(t, 19, char.Items[1].Uses, "exactly one compatible projectile must be consumed")
	assert.True(t, char.Equipment.Weapon.Loaded)
	assert.Greater(t, char.Cooldowns["special-move"], 0)
}

// TestReload_StaleSecondaryStateKeepsSingleAdmission catches a post-gate item
// reorder consuming the old index's wrong bundle, and a post-gate cooldown
// collision leaking ammo/load mutation. Both paths retain one paid admission.
func TestReload_StaleSecondaryStateKeepsSingleAdmission(t *testing.T) {
	newActor := func() (*staleRangedSecondaryActor, *characters.Character) {
		char := characters.New()
		char.Stats.Strength.ValueAdj = 100
		char.Skills["ranged-combat"] = 25
		char.Stamina = 10
		char.StaminaMax.Value = 100
		char.Equipment.Weapon = reloadRangedWeapon(1, "arrows")
		char.Items = []items.Item{
			reloadAmmoBundle(4, "bolts", 7),
			reloadAmmoBundle(3, "arrows", 20),
		}
		return &staleRangedSecondaryActor{Actor: newStubActor(char, newTestRoom())}, char
	}

	t.Run("reordered inventory follows the admitted bundle identity", func(t *testing.T) {
		actor, char := newActor()
		actor.onAdmission = func(c *characters.Character) {
			c.Items[0], c.Items[1] = c.Items[1], c.Items[0]
		}

		res := ExecuteReload(actor)

		require.Equal(t, characters.CostPaid, res.Cost.Status)
		assert.Equal(t, 1, res.Cost.Charged)
		assert.Equal(t, 9, char.Stamina)
		require.True(t, res.Loaded)
		assert.Equal(t, 3, char.Items[0].ItemId)
		assert.Equal(t, 19, char.Items[0].Uses)
		assert.Equal(t, 4, char.Items[1].ItemId)
		assert.Equal(t, 7, char.Items[1].Uses)
	})

	t.Run("stale cooldown rolls back ammo and load without another quote", func(t *testing.T) {
		actor, char := newActor()
		actor.onAdmission = func(c *characters.Character) {
			c.Cooldowns["special-move"] = 3
		}

		res := ExecuteReload(actor)

		require.Equal(t, characters.CostPaid, res.Cost.Status)
		assert.Equal(t, 1, res.Cost.Charged)
		assert.Equal(t, 9, char.Stamina)
		assert.False(t, res.Loaded)
		assert.False(t, char.Equipment.Weapon.Loaded)
		assert.Equal(t, 7, char.Items[0].Uses)
		assert.Equal(t, 20, char.Items[1].Uses)
		assert.Equal(t, 3, char.Cooldowns["special-move"])
	})
}

// TestReloadAdmissionOrdering guards the distinct reload sequence: read-only
// cooldown readiness, one exact admission, then projectile/load/cooldown
// mutation. AST nodes make this an executable structure check, not source grep.
func TestReloadAdmissionOrdering(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(thisFile), "combat_reload.go"), nil, 0)
	require.NoError(t, err)

	var body *ast.BlockStmt
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "ExecuteReload" {
			body = fn.Body
			break
		}
	}
	require.NotNil(t, body)

	ready := exactCallPositions(t, fset, body, `char.CooldownReady("special-move")`, false)
	admit := admissionCallPositions(t, fset, body, "costs.ActionReload", "ReloadBaseStaminaCost")
	consumeCooldown := exactCallPositions(t, fset, body,
		`char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown))`, false)
	require.Len(t, ready, 1)
	require.Len(t, admit, 1)
	require.Len(t, consumeCooldown, 1)

	projectiles := []token.Pos{}
	var loaded token.Pos
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.IncDecStmt:
			if n.Tok == token.DEC && formattedASTNode(t, fset, n.X) == "char.Items[bundleIdx].Uses" {
				projectiles = append(projectiles, n.Pos())
			}
		case *ast.AssignStmt:
			if len(n.Lhs) == 1 && len(n.Rhs) == 1 &&
				formattedASTNode(t, fset, n.Lhs[0]) == "weapon.Loaded" &&
				formattedASTNode(t, fset, n.Rhs[0]) == "true" {
				loaded = n.Pos()
			}
		}
		return true
	})
	require.Len(t, projectiles, 1)
	require.NotEqual(t, token.NoPos, loaded)
	assert.Less(t, int(ready[0]), int(admit[0]))
	assert.Less(t, int(admit[0]), int(projectiles[0]))
	assert.Less(t, int(projectiles[0]), int(loaded))
	assert.Less(t, int(loaded), int(consumeCooldown[0]))
}

// ---------------------------------------------------------------------------
// 6. Bundle at Uses==1 → success + bundle removed from backpack
// ---------------------------------------------------------------------------

func TestReload_LastUse_RemovesBundle(t *testing.T) {
	char := characters.New()
	char.Equipment.Weapon = reloadRangedWeapon(1, "arrows")
	char.Items = append(char.Items, reloadAmmoBundle(3, "arrows", 1))
	actor := newStubActor(char, newTestRoom())

	res := ExecuteReload(actor)

	assert.True(t, res.Loaded, "expected success")
	assert.True(t, res.BundleEmptied, "expected BundleEmptied on last use")
	assert.True(t, char.Equipment.Weapon.Loaded)
	assert.Empty(t, char.Items, "emptied bundle should be removed from backpack")
}

// ---------------------------------------------------------------------------
// 7. Offhand ranged + main melee → reload targets the offhand
// ---------------------------------------------------------------------------

func TestReload_OffhandRanged(t *testing.T) {
	char := characters.New()
	char.Equipment.Weapon = reloadMeleeWeapon(2)
	char.Equipment.Offhand = reloadRangedWeapon(1, "arrows")
	char.Items = append(char.Items, reloadAmmoBundle(3, "arrows", 20))
	actor := newStubActor(char, newTestRoom())

	res := ExecuteReload(actor)

	assert.True(t, res.Loaded, "expected success reloading the offhand ranged weapon")
	assert.True(t, char.Equipment.Offhand.Loaded, "offhand should be loaded")
	assert.False(t, char.Equipment.Weapon.Loaded, "main-hand melee should be untouched")
	assert.Equal(t, 19, char.Items[0].Uses)
}
