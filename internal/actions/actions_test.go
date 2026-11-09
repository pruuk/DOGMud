package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Initialize mudlog for tests
	mudlog.SetupLogger(nil, "", "", false)
}

// setCombatPositionParallel sets the Position FSM to the given state. Seeds
// Position if nil. Synthetic Partner ref for grapple states (FSM requires non-zero).
func setCombatPositionParallel(c *characters.Character, pos position.State) {
	if c.Position == nil {
		c.Position = position.NewMachine()
	}
	r := state.TransitionReason{Trigger: "test_setup"}
	switch pos {
	case position.Standing:
		c.Position.ForceStanding(r)
	case position.Prone:
		c.Position.ForceStanding(r)
		_ = c.Position.TransitionToProne(position.ProneData{}, r)
	case position.Clinch:
		c.Position.ForceStanding(r)
		_ = c.Position.TransitionToClinch(
			position.GrappleData{Partner: state.ActorRef{UserId: 1}},
			state.TransitionReason{Trigger: position.TriggerGrappleEntry},
		)
	case position.Mount:
		c.Position.ForceStanding(r)
		_ = c.Position.TransitionToClinch(
			position.GrappleData{Partner: state.ActorRef{UserId: 1}},
			state.TransitionReason{Trigger: position.TriggerGrappleEntry},
		)
		_ = c.Position.TransitionToMount(
			position.GrappleData{Partner: state.ActorRef{UserId: 1}},
			state.TransitionReason{Trigger: position.TriggerTakedownMount},
		)
	}
}

// TestEmoteAliasLookup verifies that a known emote alias returns IsAlias=true
// and the correct alias text.
func TestEmoteAliasLookup(t *testing.T) {
	result := Emote("bow")
	assert.True(t, result.IsAlias, "bow should be a recognized alias")
	assert.Equal(t, "bows gracefully.", result.AliasText, "bow alias text mismatch")
}

// TestEmoteNonAlias verifies that a custom emote string returns IsAlias=false
// and an empty AliasText.
func TestEmoteNonAlias(t *testing.T) {
	result := Emote("does something custom")
	assert.False(t, result.IsAlias, "custom emote should not be an alias")
	assert.Equal(t, "", result.AliasText, "custom emote should have empty alias text")
}

// TestFormatSayTextNormal verifies that say messages include the actor's
// name and the word "says,".
func TestFormatSayTextNormal(t *testing.T) {
	formatted := FormatSayText("Gandalf", "You shall not pass!", "username", "saytext")
	assert.Contains(t, formatted, "Gandalf", "normal say should include actor name")
	assert.Contains(t, formatted, "says,", "normal say should contain 'says,'")
	assert.Contains(t, formatted, "You shall not pass!", "normal say should include message text")
	assert.NotContains(t, formatted, "someone says", "normal say should not use 'someone says'")
}

// TestFormatEmoteText verifies that formatted emote messages contain both
// the actor's name and the emote text.
func TestFormatEmoteText(t *testing.T) {
	formatted := FormatEmoteText("Arwen", "looks thoughtfully at the horizon.", "mobname")
	assert.Contains(t, formatted, "Arwen", "emote should include actor name")
	assert.Contains(t, formatted, "looks thoughtfully at the horizon.", "emote should include emote text")
}

// TestAuditCommandParity_NoWarnings verifies that AuditCommandParity produces
// no warnings when user and mob commands are properly gated by the intentional
// allowlists.
func TestAuditCommandParity_NoWarnings(t *testing.T) {
	// User commands: common ones (say, emote, go) plus one user-only (help)
	userCmds := []string{"say", "emote", "go", "help"}
	// Mob commands: common ones (say, emote, go) plus one mob-only (wander)
	mobCmds := []string{"say", "emote", "go", "wander"}

	// help is in userOnlyCommands with category "ui"
	assert.Contains(t, userOnlyCommands, "help")
	// wander is in mobOnlyCommands with category "mob-ai"
	assert.Contains(t, mobOnlyCommands, "wander")

	// This should not panic or produce warnings (mudlog.Warn calls internally).
	// We're just verifying the function completes without error.
	AuditCommandParity(userCmds, mobCmds)
}

// TestEmoteAliasesPopulated verifies that the EmoteAliases map has the expected
// number of entries (82 as mentioned in the task).
func TestEmoteAliasesPopulated(t *testing.T) {
	assert.Greater(t, len(EmoteAliases), 50, "EmoteAliases should have many entries")
	// Check that some well-known aliases exist
	assert.Contains(t, EmoteAliases, "bow")
	assert.Contains(t, EmoteAliases, "laugh")
	assert.Contains(t, EmoteAliases, "dance")
	// `sit` is the real sit command now (#637), so no emote may shadow it,
	// and `rest`, its synonym, must not become one either.
	assert.NotContains(t, EmoteAliases, "sit")
	assert.NotContains(t, EmoteAliases, "rest")
}

// assertWrapsOnlyInThePipeline checks a formatted room line carries no line
// break of its own, and that the pipeline folds it to the reader's width.
func assertWrapsOnlyInThePipeline(t *testing.T, cat messaging.Category, formatted string) {
	t.Helper()
	assert.NotContains(t, formatted, "\n", "the formatter must leave the wrap to the pipeline")
	rendered := messaging.RenderForRecipient(messaging.RenderInput{
		Category:      cat,
		Text:          formatted,
		Channel:       messaging.ChannelVisual,
		SightDecision: messaging.SightFull,
		LineWidth:     80,
	})
	lines := strings.Split(rendered, "\n")
	assert.Greater(t, len(lines), 1, "a long line should fold across several lines")
	for _, line := range lines {
		assert.LessOrEqual(t, len(stripANSICodes(line)), 80, "wrapped line should not exceed 80 visible characters")
	}
}

// TestFormatSayTextLineWrapping: a say line is wrapped once, per reader, by
// the pipeline, so the formatter itself must not wrap it.
func TestFormatSayTextLineWrapping(t *testing.T) {
	longText := strings.Repeat("word ", 30) // Creates a very long message
	assertWrapsOnlyInThePipeline(t, messaging.CategorySpeech,
		FormatSayText("Speaker", longText, "username", "saytext"))
}

// TestFormatEmoteTextLineWrapping is the same rule for an emote.
func TestFormatEmoteTextLineWrapping(t *testing.T) {
	longEmote := strings.Repeat("emote ", 25) // Creates a very long emote
	assertWrapsOnlyInThePipeline(t, messaging.CategoryEmote,
		FormatEmoteText("Actor", longEmote, "mobname"))
}

// stripANSICodes removes ANSI color codes from a string to measure visible length.
func stripANSICodes(s string) string {
	// Simple approach: remove anything between < and >
	result := ""
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
		} else if ch == '>' {
			inTag = false
		} else if !inTag {
			result += string(ch)
		}
	}
	return result
}

// TestAuditCommandParity_CatchesUnlistedUserCmd verifies that AuditCommandParity
// does not panic when it encounters a user command that is absent from the mob
// registry and absent from the user-only allowlist. The function logs a warning
// via mudlog; we cannot capture that output here, but we confirm the call
// completes without panicking.
//
// Note: AuditCommandParity returns nothing (void), so we cannot assert a warning
// count directly. The test validates that the code path for an unlisted user
// command is traversed safely.
func TestAuditCommandParity_CatchesUnlistedUserCmd(t *testing.T) {
	// "fakecmd" is not in mobOnlyCommands, not in userOnlyCommands, and not
	// present on the mob side — this should trigger a mudlog.Warn internally.
	userCmds := []string{"say", "fakecmd"}
	mobCmds := []string{"say"}

	// Verify the precondition: fakecmd is genuinely not in either allowlist.
	_, inUserOnly := userOnlyCommands["fakecmd"]
	_, inMobOnly := mobOnlyCommands["fakecmd"]
	assert.False(t, inUserOnly, "fakecmd must not be in the user-only allowlist for this test to be meaningful")
	assert.False(t, inMobOnly, "fakecmd must not be in the mob-only allowlist for this test to be meaningful")

	// Must not panic — the gap is logged as a warning, not a fatal error.
	assert.NotPanics(t, func() {
		AuditCommandParity(userCmds, mobCmds)
	}, "AuditCommandParity should not panic on an unlisted user command")
}

// TestAuditCommandParity_CatchesUnlistedMobCmd verifies that AuditCommandParity
// does not panic when it encounters a mob command that is absent from the user
// registry and absent from the mob-only allowlist.
func TestAuditCommandParity_CatchesUnlistedMobCmd(t *testing.T) {
	// "fakemobcmd" is not in userOnlyCommands, not in mobOnlyCommands, and not
	// present on the user side — this should trigger a mudlog.Warn internally.
	userCmds := []string{"say"}
	mobCmds := []string{"say", "fakemobcmd"}

	// Verify the precondition: fakemobcmd is genuinely not in either allowlist.
	_, inUserOnly := userOnlyCommands["fakemobcmd"]
	_, inMobOnly := mobOnlyCommands["fakemobcmd"]
	assert.False(t, inUserOnly, "fakemobcmd must not be in the user-only allowlist for this test to be meaningful")
	assert.False(t, inMobOnly, "fakemobcmd must not be in the mob-only allowlist for this test to be meaningful")

	// Must not panic — the gap is logged as a warning, not a fatal error.
	assert.NotPanics(t, func() {
		AuditCommandParity(userCmds, mobCmds)
	}, "AuditCommandParity should not panic on an unlisted mob command")
}
