package users

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// ValidateActorOpts tunes which checks ValidateActorName performs.
type ValidateActorOpts struct {
	SkipMobCheck    bool // skip mob-template-name collision check
	SkipBannedCheck bool // skip banned-pattern check
	ExcludeUserId   int  // ignore collisions on this user (self-rename)
}

// ValidateActorName is the single source of truth for any name a player
// or companion will be referred to by in-world. Pet/companion/character
// creation and self-rename all go through this. Returns nil on success
// or a player-displayable error explaining the rejection.
func ValidateActorName(name string, opts ValidateActorOpts) error {
	validation := configs.GetValidationConfig()

	if len(name) < int(validation.NameSizeMin) || len(name) > int(validation.NameSizeMax) {
		return fmt.Errorf("Names must be between %d and %d characters long.",
			validation.NameSizeMin, validation.NameSizeMax)
	}

	if validation.NameRejectRegex != `` {
		if !regexp.MustCompile(validation.NameRejectRegex.String()).MatchString(name) {
			return errors.New(validation.NameRejectReason.String())
		}
	}

	if !opts.SkipBannedCheck {
		if _, ok := configs.GetConfig().IsBannedName(name); ok {
			return errors.New("That name is not allowed.")
		}
	}

	if !opts.SkipMobCheck {
		for _, mobName := range mobs.GetAllMobNames() {
			if strings.EqualFold(mobName, name) {
				return errors.New("That name is already in use.")
			}
		}
	}

	if foundUserId, _ := CharacterNameSearch(name); foundUserId > 0 && foundUserId != opts.ExcludeUserId {
		return errors.New("That name is already in use.")
	}

	userManager.mu.RLock()
	existingId, exists := userManager.Usernames[strings.ToLower(name)]
	userManager.mu.RUnlock()
	if exists && existingId != opts.ExcludeUserId {
		return errors.New("That name is already in use.")
	}

	if CompanionNameExists(name) {
		return errors.New("That name is already in use.")
	}

	return nil
}
