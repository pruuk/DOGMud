package inputhandlers

import (
	// ... other imports

	"fmt"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/moderation"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// FinalizeLoginOrCreate is called after all prompts are successfully answered.
func FinalizeLoginOrCreate(results map[string]string, sharedState map[string]any, clientInput *connections.ClientInput) bool {

	// IP ban: reject (and block re-registration) before any account work. Skip
	// local/loopback connections so an admin can never lock themselves out.
	//
	// ClientIP(), not RemoteAddr(): for a websocket the socket peer is the TCP
	// peer of the HTTP upgrade, which behind a reverse proxy is the proxy. That
	// reported 127.0.0.1 for every /webclient player, so IsLocal() was true and
	// this whole block was skipped — a banned player only had to switch from
	// telnet to the web client. Telnet is unaffected: nothing sets clientIP on
	// a telnet connection, so ClientIP() is still the socket peer there.
	if connDetails := connections.Get(clientInput.ConnectionId); connDetails != nil && !connDetails.IsLocal() {
		host := connDetails.ClientIP()
		if reason, banned := moderation.IsIPBanned(host); banned {
			sendLoginText(clientInput.ConnectionId, "Your connection has been banned. Reason: "+reason)
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false
		}
	}

	username := results["username"]
	password := results["password"]

	if username != `new` {
		userExists := users.Exists(username)

		if userExists {

			if results["kickuser"] == "y" {

				connDetails := connections.Get(clientInput.ConnectionId)

				// Disconnect/kick the user currently connected
				userid := users.FindUserId(results["username"])
				user := users.GetByUserId(userid)

				// The session may have disappeared since the kick prompt (e.g. link-dead
				// cleanup), in which case there's nothing to kick. This also covers
				// FindUserId returning 0, since GetByUserId(0) returns nil.
				if user != nil {

					existingConnectionId := user.ConnectionId()

					// Send a goodbye message to the currently connected user.
					connections.SendTo([]byte(goodbyeFor(existingConnectionId)), existingConnectionId)

					users.SetZombieUser(userid)
					// Error deliberately discarded, and made explicit because
					// the lint gate is only-new-issues and re-indenting this
					// line into the nil guard above made errcheck treat it as
					// new. Kick fails only when the connection is already gone,
					// which is precisely the outcome being asked for -- and the
					// user is a zombie either way. Every other Kick call site
					// (systemcommands, ban, boot) discards it the same way.
					_ = connections.Kick(existingConnectionId, fmt.Sprintf(`Duplicate login (ip: %s)`, connDetails.RemoteAddr()))

				}

			}

			// Existing User Login Logic (No changes needed)
			tmpUser, err := users.LoadUser(username)
			if err != nil {
				mudlog.Error("Failed to load existing user during login", "username", username, "error", err)
				sendLoginText(clientInput.ConnectionId, language.T("Error.LoginFailedGeneric"))
				connections.SendTo(term.CRLF, clientInput.ConnectionId)
				connections.Remove(clientInput.ConnectionId)
				return false // Indicate failure, connection removed
			}

			if !tmpUser.PasswordMatches(password) {
				sendLoginText(clientInput.ConnectionId, `Nope. Bye!`)
				connections.SendTo(term.CRLF, clientInput.ConnectionId)
				connections.Remove(clientInput.ConnectionId)
				return false // Indicate failure, connection removed
			}

			// Account ban: reject a banned account after the password is verified
			// (so it can't be used to probe which accounts exist) and before the
			// character enters the world.
			if reason, banned := moderation.IsAccountBanned(username); banned {
				sendLoginText(clientInput.ConnectionId, "This account has been banned. Reason: "+reason)
				connections.SendTo(term.CRLF, clientInput.ConnectionId)
				connections.Remove(clientInput.ConnectionId)
				return false
			}

			// The connection's screen reader report turns the account's setting
			// on (never off): before the user is published, and again on the
			// link-dead session a reconnect continues (loginWithScreenReader).
			loggedInUser, msg, srLines, err := loginWithScreenReader(tmpUser,
				connections.GetClientSettings(clientInput.ConnectionId).ScreenReader,
				func(u *users.UserRecord) (*users.UserRecord, string, error) {
					return users.LoginUser(u, clientInput.ConnectionId)
				})
			if err != nil {
				sendLoginText(clientInput.ConnectionId, msg)
				connections.SendTo(term.CRLF, clientInput.ConnectionId)
				connections.Remove(clientInput.ConnectionId)
				return false // Indicate failure, connection removed
			}

			sharedState["UserObject"] = loggedInUser // For main loop

			sendLoginLines(clientInput.ConnectionId, srLines)

			if len(msg) > 0 {
				sendLoginText(clientInput.ConnectionId, msg)
				connections.SendTo(term.CRLF, clientInput.ConnectionId)
			}
			mudlog.Info("User logged in", "username", username, "connectionId", clientInput.ConnectionId)
			return true // Indicate success, handler can be removed

		} else {
			sendLoginText(clientInput.ConnectionId, `Invalid login.`)
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false // Indicate failure, connection removed
		}
	} else {
		/*
			username-new
			password-new
			password-new-verify
			email-new
			screen-reader-new y/n
			confirm_create y/n
		*/

		confirmCreate, exists := results["confirm_create"] // Assumes step ID "confirm_create"
		if !exists || confirmCreate != "y" {
			sendLoginText(clientInput.ConnectionId, `Okay, bye!`) // Use language key
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false // Indicate failure, connection removed
		}

		username := results["username-new"]
		password := results["password-new"]

		if users.Exists(results["username-new"]) {
			sendLoginText(clientInput.ConnectionId, `I'm sorry, that user already exists!`) // Use language key
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false
		}

		newUser := users.NewUserRecord(0, clientInput.ConnectionId)
		newUser.EmailAddress = results["email-new"]
		srCreationLines := creationScreenReaderLines(newUser, results, connections.GetClientSettings(clientInput.ConnectionId).ScreenReader)

		// Error handling for SetUsername/SetPassword might be redundant if validation passed, but good practice
		if err := newUser.SetUsername(username); err != nil {
			mudlog.Error("Internal error setting username post-validation", "username", username, "error", err)
			sendLoginText(clientInput.ConnectionId, language.T("Error.UserCreationFailed")) // Generic creation error
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false
		}
		if err := newUser.SetPassword(password); err != nil {
			mudlog.Error("Internal error setting password post-validation", "username", username, "error", err)
			sendLoginText(clientInput.ConnectionId, language.T("Error.UserCreationFailed")) // Generic creation error
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false
		}

		// Character name is the same as the username
		if err := newUser.SetCharacterName(username); err != nil {
			mudlog.Error("Internal error setting character name from username", "name", username, "error", err)
			sendLoginText(clientInput.ConnectionId, language.T("Error.UserCreationFailed"))
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false
		}

		// All players are human in Delusions of Grandeur
		if humanSpecies, ok := species.FindSpecies("human"); ok {
			newUser.Character.SpeciesId = humanSpecies.Id()
			newUser.Character.Validate()
			// Wire intrinsic mutations for the species (no-op for humans today;
			// future-proofs non-human playable species).
			sp := species.GetSpecies(humanSpecies.Id())
			newUser.Character.ApplyIntrinsicMutations(sp)
		}

		if err := users.CreateUser(newUser); err != nil {
			mudlog.Error("Could not create user", "username", username, "error", err)
			// Try to give specific feedback if possible, otherwise generic
			sendLoginText(clientInput.ConnectionId, err.Error())
			connections.SendTo(term.CRLF, clientInput.ConnectionId)
			connections.Remove(clientInput.ConnectionId)
			return false // Indicate failure, connection removed
		}

		sharedState["UserObject"] = newUser // For main loop

		sendLoginLines(clientInput.ConnectionId, srCreationLines)

		mudlog.Info("New user created", "username", username, "connectionId", clientInput.ConnectionId)

		return true // Indicate success, handler can be removed
	}
}

// loginSteps are the login and creation prompts in order.
func loginSteps() []*PromptStep {
	return []*PromptStep{
		{
			ID:             "username",
			PromptTemplate: "login/username.prompt",
			MaskInput:      false,
			Validator:      ValidateNewEntry,
			// Plaintext MSSP: crawlers may send "mssp-request" instead of a
			// name — answer with the status block and close.
			Intercept: MSSPTextRequestIntercept,
		},
		//////////////////////////////////////////////////
		// If NOT a new user signup (Just a login)
		//////////////////////////////////////////////////
		{
			ID:             "password",
			PromptTemplate: "login/password.prompt",
			MaskInput:      true,
			MaskTemplate:   "login/password.mask", // Optional: specify if different from "*"
			Validator:      ValidatePassword,
			Condition:      func(results map[string]string) bool { return results["username"] != `new` }, // Only run if username was not "new"
		},
		{
			ID:             "kickuser",
			PromptTemplate: "generic/prompt.yn",
			GetDataFunc: func(results map[string]string) map[string]any {
				// Dynamically generate the data for the generic y/n prompt
				return map[string]any{
					"prompt":  "User is already connected. Kick them?",
					"options": []string{"y", "n"},
					"default": "n", // Default shown in the prompt, actual default on empty input handled by validator
				}
			},
			MaskInput: false,
			Validator: ValidateYesNo,
			Condition: func(results map[string]string) bool {
				if results["username"] == `new` {
					return false
				}

				userid := users.FindUserId(results["username"])

				user := users.GetByUserId(userid)

				return user != nil && user.PasswordMatches(results["password"])
			}, // Only run if username was not "new", password matches, and user is currently online.
		},
		//////////////////////////////////////////////////
		// End If NOT a new user signup (Just a login)
		//////////////////////////////////////////////////
		//////////////////////////////////////////////////
		// If a new user signup
		//////////////////////////////////////////////////
		{
			ID:             "username-new",
			PromptTemplate: "login/username-new.prompt",
			MaskInput:      false,
			Validator:      ValidateUsername,
			Condition:      func(results map[string]string) bool { return results["username"] == `new` }, // Only run if username was "new"
		},
		{
			ID:             "password-new",
			PromptTemplate: "login/password-new.prompt",
			MaskInput:      true,
			MaskTemplate:   "login/password.mask", // Optional: specify if different from "*"
			Validator:      ValidatePassword,
			Condition:      func(results map[string]string) bool { return results["username"] == `new` }, // Only run if username was "new"
		},
		{
			ID:             "password-new-verify",
			PromptTemplate: "login/password-new-verify.prompt",
			MaskInput:      true,
			MaskTemplate:   "login/password.mask", // Optional: specify if different from "*"
			Validator:      ValidatePassword2,
			Condition:      func(results map[string]string) bool { return results["username"] == `new` }, // Only run if username was "new"
		},
		{
			ID:             "email-new",
			PromptTemplate: "login/email-new.prompt",
			GetDataFunc: func(results map[string]string) map[string]any {
				// Dynamically generate the data for the generic y/n prompt
				return map[string]any{
					"emailIsOptional": configs.GetValidationConfig().EmailOnJoin != `required`,
				}
			},
			MaskInput: false,
			Validator: ValidateEmail,
			Condition: func(results map[string]string) bool {
				return results["username"] == `new` && configs.GetValidationConfig().EmailOnJoin != `none` // Only run if username was "new" and email is enabled
			},
		},
		{
			ID:             "screen-reader-new",
			PromptTemplate: "generic/prompt.yn",
			GetDataFunc: func(results map[string]string) map[string]any {
				// Dynamically generate the data for the generic y/n prompt
				return map[string]any{
					"prompt":  "Are you using a screen reader?",
					"options": []string{"y", "n"},
					"default": "n", // Default shown in the prompt, actual default on empty input handled by validator
				}
			},
			MaskInput: false,
			Validator: ValidateYesNo,
			// Asked only of a new player on a telnet client that did not answer
			// MTTS: a web client's answer is "screen reader on", and any MTTS
			// reply is an answer (spec 2026-11-17, ruling 4).
			Condition: func(results map[string]string) bool {
				return results["username"] == `new` && askScreenReader(results)
			},
		},
		{
			ID:             "confirm_create",
			PromptTemplate: "generic/prompt.yn", // Use the generic yes/no template
			GetDataFunc: func(results map[string]string) map[string]any {
				// Dynamically generate the data for the generic y/n prompt
				return map[string]any{
					"prompt": language.T("Login.CreateUser", map[any]any{ // Use language.T for the prompt text
						"Username": results["username-new"], // Inject username
					}),
					"options": []string{"y", "n"},
					"default": "n", // Default shown in the prompt, actual default on empty input handled by validator
				}
			},
			MaskInput: false,
			Validator: ValidateYesNo,
			Condition: func(results map[string]string) bool { return results["username"] == `new` }, // Only run if username was "new"
		},
		//////////////////////////////////////////////////
		// End If a new user signup
		//////////////////////////////////////////////////
	}
}

// GetLoginPromptHandler returns a new login and creation prompt sequence
// for one connection.
func GetLoginPromptHandler() connections.InputHandler {
	// Create and return the handler using the generic factory function
	return CreatePromptHandler(loginSteps(), FinalizeLoginOrCreate)
}

// askScreenReader reports whether the creation question is asked: never on
// a web client, and never when a telnet client answered MTTS.
func askScreenReader(results map[string]string) bool {
	return results[connFactWebsocket] != `y` && results[connFactMTTS] != `y`
}

// newUserScreenReader is a new account's screen reader setting: the answer
// when the question was asked, else what the connection reported.
func newUserScreenReader(results map[string]string, reported bool) bool {
	if answer, asked := results["screen-reader-new"]; asked {
		return answer == `y`
	}
	return reported
}

// reportedScreenReaderLines turns u's setting on when its connection
// reported a screen reader, and returns the lines that tell the player so
// (none when nothing changed). A connection never turns the setting off.
// Only for a record no other goroutine can see yet.
func reportedScreenReaderLines(u *users.UserRecord, reported bool) []string {
	if !reported {
		return nil
	}
	return u.TurnOnReportedScreenReader()
}

// loginWithScreenReader logs tmpUser in with login (users.LoginUser in
// production) and applies the connection's screen reader report. On an
// ordinary login it turns tmpUser on before login publishes it, so
// MainWorker never reads the fields mid-write, and returns the lines to
// tell the player. On a reconnect to a link-dead session login returns the
// user already in the world (users.go:240-242), which MainWorker reads every
// round, so this goroutine never writes it: it queues ScreenReaderReported
// and the hook on MainWorker turns it on and tells the player
// (hooks.TurnOnReportedScreenReader). tmpUser's change is then discarded.
func loginWithScreenReader(tmpUser *users.UserRecord, reported bool, login func(*users.UserRecord) (*users.UserRecord, string, error)) (*users.UserRecord, string, []string, error) {
	lines := reportedScreenReaderLines(tmpUser, reported)
	loggedIn, msg, err := login(tmpUser)
	if err != nil {
		return loggedIn, msg, nil, err
	}
	if loggedIn != tmpUser {
		if reported {
			events.AddToQueue(events.ScreenReaderReported{UserId: loggedIn.UserId})
		}
		return loggedIn, msg, nil, nil
	}
	return loggedIn, msg, lines, nil
}

// creationScreenReaderLines sets a new account's screen reader setting and
// returns the lines to tell the player. When the question was asked, the
// answer decides and only the combat lines follow (the player just said
// yes). When it was skipped, the connection's report decides and the player
// also hears that the mode is on and how to turn it off.
func creationScreenReaderLines(u *users.UserRecord, results map[string]string, reported bool) []string {
	if !newUserScreenReader(results, reported) {
		return nil
	}
	lines := []string{}
	if _, asked := results["screen-reader-new"]; !asked {
		lines = append(lines, users.ScreenReaderOnNotices...)
	}
	if u.TurnOnScreenReader() {
		lines = append(lines, users.ScreenReaderCombatLines...)
	}
	return lines
}

// sendLoginLines sends each line as login text, one per line.
func sendLoginLines(id connections.ConnectionId, lines []string) {
	for _, line := range lines {
		sendLoginText(id, line)
		connections.SendTo(term.CRLF, id)
	}
}
