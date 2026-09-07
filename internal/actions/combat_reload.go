package actions

import (
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

// chamberNextRound readies the next projectile in the actor's equipped ranged
// weapon (main hand first, then offhand), consuming one Use from a matching
// ammo bundle.
//
// It is an internal STEP OF FIRING, not an action a player can take: `fire`
// resolves the shot and then calls this, so the two together are one action.
//
// ⚠️ IT MUST NOT TOUCH THE SPECIAL-MOVE COOLDOWN. It used to both gate on that
// timer and claim it, which produced a collision in BOTH directions: a recent
// reload denied the ambush that needed the timer, and a successful ambush then
// left the weapon unreloadable for the cooldown's length. ExecuteFire owns the
// single claim now. Re-adding a claim here re-creates both bugs.
//
// The bundle is deliberately re-found by identity (Equals) after cost admission
// rather than by the index found before it: admission calls through the actor
// seam, which a test double or a future synchronous hook can use to invalidate
// the inventory state underneath us.
func chamberNextRound(actor Actor) ReloadResult {
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
	// Consume one Use; remove the bundle when emptied. RemoveItem matches by
	// ItemId+UUID (Item.Equals), so the post-decrement value still matches.
	//
	// There is no rollback path any more. The snapshot vars this used to keep
	// (bundleBefore/bundleRemoved) existed only to undo the consume when the
	// cooldown claim failed, and chambering no longer claims anything.
	char.Items[bundleIdx].Uses--
	if char.Items[bundleIdx].Uses <= 0 {
		result.BundleEmptied = true
		char.RemoveItem(char.Items[bundleIdx])
	}

	weapon.Loaded = true
	result.Loaded = true
	return result
}
