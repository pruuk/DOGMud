package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// SpecialMoveCooldownTag is the one place this key is spelled.
//
// It was typed by hand at 56 non-test call sites before this existed, which is
// how internal/actions/combat_helpers.go came to run a hardcoded "1 rounds"
// against every other verb's configured value with nothing able to notice.
const SpecialMoveCooldownTag = "special-move"

// SpecialMoveReady reports whether the shared special-move cooldown is free.
//
// ⚠️ Use this ONLY to decide what to OFFER or DISPLAY -- an AI weighing a move,
// a readiness listing, a refusal message written before any cost is admitted.
// To actually take the cooldown, call ClaimSpecialMove and branch on its
// return.
//
// Asking here and claiming later is the split that hid the ranged bug: reload
// asked at combat_reload.go:86 and took at :133, and in the 47 lines between
// them sat the ambush that the claim would go on to deny.
func SpecialMoveReady(char *characters.Character) bool {
	return char.CooldownReady(SpecialMoveCooldownTag)
}

// ClaimSpecialMove takes the shared special-move cooldown, returning true when
// it was free and is now held, false when it was already running. Asking and
// taking are one call on purpose.
//
// The duration always comes from Balance.SpecialMoveCooldown. Do not pass a
// literal: a hardcoded duration in one path is exactly the drift this function
// exists to make impossible.
func ClaimSpecialMove(char *characters.Character) bool {
	cfg := configs.GetBalanceConfig()
	return char.TryCooldown(SpecialMoveCooldownTag,
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown))
}

// ReleaseSpecialMove returns the cooldown to free.
//
// It exists for actions that claim and then abort, so a player is not charged
// for a move that did not happen. It is NOT a way to dodge the cooldown, and it
// is safe on a character that has never claimed one (deleting an absent key
// from a nil map is a no-op).
func ReleaseSpecialMove(char *characters.Character) {
	delete(char.Cooldowns, SpecialMoveCooldownTag)
}
