package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// ReloadResult holds the outcome of a reload attempt.
type ReloadResult struct {
	WeaponName    string // display name of the weapon involved
	AmmoTag       string // ammo type involved (set for NoAmmo messaging too)
	AmmoName      string // display name of the bundle consumed from
	BundleEmptied bool   // the bundle's last Use was consumed
	Cost          characters.CostCommitResult

	Loaded        bool // success
	NoWeapon      bool // no ranged weapon equipped
	AlreadyLoaded bool
	NoAmmo        bool
	OnCooldown    bool
	Crafting      bool
}

// findRangedWeaponSlot returns a pointer to the equipped ranged weapon
// (main hand first, then offhand) so Loaded can be written back, or nil.
// The returned pointer addresses an Equipment struct field — it is valid
// only for immediate in-place mutation. Do not retain it across any
// equip change (Wear/Remove overwrite the slot value, invalidating the
// pointer).
func findRangedWeaponSlot(actor Actor) *items.Item {
	char := actor.GetCharacter()
	if char.Equipment.Weapon.IsRangedWeapon() {
		return &char.Equipment.Weapon
	}
	if char.Equipment.Offhand.IsRangedWeapon() {
		return &char.Equipment.Offhand
	}
	return nil
}

// ExecuteReload chambers/nocks a projectile into the actor's equipped
// ranged weapon (main hand first, then offhand): consumes the shared
// special-move cooldown and one Use from a matching ammo bundle. The
// cooldown is only consumed once every other precondition has passed —
// a reload that fails for no-ammo never burns it.
// Callers handle all messaging and progression events.
func ExecuteReload(actor Actor) ReloadResult {
	char := actor.GetCharacter()

	// Don't interrupt any active activity (cast/craft/salvage) to reload.
	if char.IsActing() {
		return ReloadResult{Crafting: true}
	}

	weapon := findRangedWeaponSlot(actor)
	if weapon == nil {
		return ReloadResult{NoWeapon: true}
	}
	if weapon.Loaded {
		return ReloadResult{WeaponName: weapon.DisplayName(), AlreadyLoaded: true}
	}
	weaponSnapshot := *weapon

	ammoTag := weapon.GetSpec().AmmoTag

	// Find a matching ammo bundle in the backpack.
	bundleIdx := -1
	for idx := range char.Items {
		spec := char.Items[idx].GetSpec()
		if char.Items[idx].Uses > 0 && spec.Type == items.Ammo && spec.AmmoTag == ammoTag {
			bundleIdx = idx
			break
		}
	}
	if bundleIdx < 0 {
		return ReloadResult{WeaponName: weapon.DisplayName(), AmmoTag: ammoTag, NoAmmo: true}
	}
	bundleSnapshot := char.Items[bundleIdx]

	// Shared special-move cooldown availability is the last read-only gate.
	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return ReloadResult{WeaponName: weapon.DisplayName(), OnCooldown: true}
	}

	result := ReloadResult{
		WeaponName: weapon.DisplayName(),
		AmmoTag:    ammoTag,
		AmmoName:   char.Items[bundleIdx].DisplayName(),
	}
	result.Cost = admitFullCost(actor, costs.ActionReload, characters.PoolStamina,
		float64(cfg.ReloadBaseStaminaCost))
	if result.Cost.Status == characters.CostRefused {
		return result
	}

	// Admission calls through the actor seam, so a test double (or a future
	// synchronous hook) can invalidate the already-checked equipment/inventory
	// state. Follow the admitted item identities rather than a stale slice index;
	// if either disappeared, retain the one paid admission and do nothing else.
	if !weapon.Equals(weaponSnapshot) || weapon.Loaded || weapon.GetSpec().AmmoTag != ammoTag {
		return result
	}
	bundleIdx = -1
	for idx := range char.Items {
		spec := char.Items[idx].GetSpec()
		if char.Items[idx].Equals(bundleSnapshot) && char.Items[idx].Uses > 0 &&
			spec.Type == items.Ammo && spec.AmmoTag == ammoTag {
			bundleIdx = idx
			break
		}
	}
	if bundleIdx < 0 {
		return result
	}
	bundleBefore := char.Items[bundleIdx]

	// Consume one Use; remove the bundle when emptied. RemoveItem matches by
	// ItemId+UUID (Item.Equals), so the post-decrement value still matches.
	char.Items[bundleIdx].Uses--
	bundleRemoved := false
	if char.Items[bundleIdx].Uses <= 0 {
		result.BundleEmptied = true
		char.RemoveItem(char.Items[bundleIdx])
		bundleRemoved = true
	}

	weapon.Loaded = true
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		weapon.Loaded = false
		result.BundleEmptied = false
		if bundleRemoved {
			char.Items = append(char.Items, items.Item{})
			copy(char.Items[bundleIdx+1:], char.Items[bundleIdx:])
			char.Items[bundleIdx] = bundleBefore
		} else {
			char.Items[bundleIdx] = bundleBefore
		}
		return result
	}
	result.Loaded = true
	return result
}
