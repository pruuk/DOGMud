package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Password(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// Get if already exists, otherwise create new
	cmdPrompt, _ := user.StartPrompt(`password`, rest)

	if !user.HasPlaintextPassword() {
		question := cmdPrompt.Ask(`What is your current password?`, []string{})
		if !question.Done {
			return true, nil
		}

		if !user.PasswordMatches(question.Response) {
			user.SendText(messaging.CategoryRefusal, `<ansi fg="alert-5">Sorry, your password was incorrect.</ansi>`)
			user.ClearPrompt()
			return true, nil
		}
	}

	question := cmdPrompt.Ask(`What new password would you like?`, []string{})
	if !question.Done {
		return true, nil
	}

	newPW := question.Response

	question = cmdPrompt.Ask(`Confirm the change by entered the new password one more time.`, []string{})
	if !question.Done {
		return true, nil
	}

	newPWConfirm := question.Response

	if newPW != newPWConfirm {
		user.SendText(messaging.CategoryRefusal, `<ansi fg="alert-5">Sorry, your new password and the confirmation password did not match.</ansi>`)
		user.ClearPrompt()
		return true, nil
	}

	// The length rule is the player's to fix; SetPassword applies the same
	// rule, so any error it returns after this check is a server fault.
	if users.ValidatePassword(newPW) != nil {
		v := configs.GetValidationConfig()
		user.SendText(messaging.CategoryRefusal, fmt.Sprintf(`<ansi fg="alert-5">Your new password must be between %d and %d characters long.</ansi>`, v.PasswordSizeMin, v.PasswordSizeMax))
		user.ClearPrompt()
		return true, nil
	}

	if err := user.SetPassword(newPW); err != nil {
		refusal.Failed(user, "password change", err, "userId", user.UserId)
		user.ClearPrompt()
		return true, nil
	}

	users.SaveUser(user)

	user.SendText(messaging.CategorySystem, `<ansi fg="alert-1">Your password has been changed!</ansi>`)

	return true, nil
}
