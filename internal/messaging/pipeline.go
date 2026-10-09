package messaging

// Channel discriminates the broadcast path. Audio bypasses the sight
// gate and the anonymizer; visual runs the full per-recipient
// pipeline. Both channels run style normalization, color, and wrap.
type Channel int

const (
	ChannelAudio Channel = iota
	ChannelVisual
)

// RenderInput bundles the parameters for one recipient's pipeline
// pass. The caller (Room/UserRecord Send helpers) constructs one of
// these per recipient and invokes RenderForRecipient.
//
// SightDecision is computed by the caller using the predicates in
// predicates.go BEFORE entering the pipeline; the pipeline trusts it.
// CanSeeClearly + ChannelVisual → full visual text.
// CanSeeShapes only + ChannelVisual → anonymized text.
// Neither + ChannelVisual → empty return ("don't deliver").
// ChannelAudio ignores SightDecision entirely.
type RenderInput struct {
	Category      Category
	Text          string
	Channel       Channel
	SightDecision SightDecision
	LineWidth     int
}

// SightDecision is the precomputed visibility verdict for one recipient.
type SightDecision int

const (
	SightFull   SightDecision = iota // CanSeeClearly
	SightShapes                      // !CanSeeClearly && CanSeeShapes (infrared in dark)
	SightNone                        // can't see at all
)

// RenderForRecipient runs the pipeline for one recipient and returns
// the final delivery string. Empty return means "don't deliver".
//
// Stage order:
//  1. Compose (caller-provided in.Text)
//  2. Normalize (normalize.go — wired in T8)
//  3. Sight gate (visual channel only)
//  4. Anonymize (infrared-only path; anonymize.go — wired in T6)
//  5. Color (color.go — wired in T2 alongside this stub)
//  6. Wrap (wrap.go — wired in T5)
//  7. Deliver (caller does this; pipeline returns the string)
//
// Each stub stage is a no-op until its task lands. Order is locked.
func RenderForRecipient(in RenderInput) string {
	text := in.Text

	// Stage 2: normalize (stubbed; T8 lands the implementation).
	text = normalize(in.Category, text)

	// Stage 3: sight gate (visual channel only).
	if in.Channel == ChannelVisual {
		switch in.SightDecision {
		case SightNone:
			return ""
		case SightShapes:
			// Stage 4: anonymize (stubbed; T6 lands the implementation).
			text = anonymize(text)
			// A shapes-only spectator of a fight sees no weapon's name
			// (ruling R8). Every spectator combat line reaches here: the
			// spectator drain and the Trio observer seats both render
			// through this stage.
			if isCombatNarration(in.Category) {
				text = HideWeapons(text, SightShapes, nil)
			}
		}
	}

	// Stage 5: color (stubbed; T2 lands a no-op, T4 wires data).
	text = applyCategoryColor(in.Category, text)

	// Stage 6: wrap, for the narration categories shouldWrap admits.
	// Pre-formatted output (tables, banners, ASCII art, side-by-side
	// templates) is excluded by category; see shouldWrap for the full
	// list and the reason for each exclusion. WrapAnsi also remains
	// callable directly by sites that wrap themselves, such as motd.go's
	// box-bordered banner.
	if shouldWrap(in.Category) {
		text = wrap(text, in.LineWidth)
	}

	return text
}

// shouldWrap controls whether the pipeline's wrap stage fires, by
// Category. It is an explicit ALLOWLIST and the default is false, so a
// category nobody has classified keeps today's behavior. That is the
// fail-safe direction: a narration category missing from this list ships
// slightly ugly, where a table category wrongly added to it ships
// mangled.
//
// Shape follows skipStages in normalize.go, this package's established
// idiom for per-Category policy.
//
// DELIBERATELY ABSENT, and each one has a reason:
//
//   - CategorySystem and CategoryBroadcast are MIXED BUCKETS. System
//     carries one-line refusals alongside the score sheet, inventory,
//     who, help, the admin DynamicList tables and the ASCII map.
//     Broadcast carries free channel chat alongside the hand-drawn MOTD
//     box. Neither can be folded without shattering the other half.
//   - CategoryRoomDescription renders a side-by-side block, prose left
//     and minimap right. TestRoomDescriptionSkipsWrap pins it.
//   - CategorySplash is rendered ASCII art.
//   - CategorySkillProgress is a banner with its own formatting, and
//     skipStages already excludes it for the same reason.
//   - Speech, Whisper, Shout and Emote are ALREADY wrapped, at a
//     hardcoded 80 that ignores the reader's LineWidth (say.go,
//     shout.go, reply.go, whisper.go). Adding them here would wrap the
//     same text twice at two different widths.
//   - Error and Warning are Group A system output, which M7 owns.
//   - GrappleHigh, Login, OOC and Toxin have zero production senders, so
//     any policy here would be unreachable.
func shouldWrap(cat Category) bool {
	switch cat {
	// Combat — hits.
	case CategoryHitMelee, CategoryHitBlunt, CategoryHitNaturalSharp,
		CategoryHitRanged, CategoryHitCaster, CategoryHitUnarmed,
		// Combat — defense.
		CategoryDodge, CategoryParry, CategoryBlock,
		// Combat — grapple.
		CategoryGrappleFlow,
		// Combat — outcome.
		CategorySubmission, CategoryDeath, CategoryCombatSummary,
		CategoryCombatBlindWarning,
		// Combat — special moves.
		CategorySurpriseAttack, CategoryKick, CategoryTrip, CategoryBash,
		CategoryRally, CategoryWarcry, CategoryTauntSuccess,
		CategoryTauntResist, CategoryTauntFailure,
		// Spells.
		CategorySpellFold, CategorySpellDisruption, CategorySpellElemental,
		CategorySpellEnhancement, CategorySpellMental, CategorySpellVital,
		CategorySpellManifestation,
		// NPC and ambient prose.
		CategoryNPCDialogue, CategoryDialogueHint, CategoryMobIdle,
		CategoryMobEmote, CategoryRoomEntry, CategoryRoomExit,
		CategoryWeather, CategoryTimeOfDay, CategoryLight,
		// Other narration plus tips.
		CategoryLoot, CategoryEquipment, CategoryConditionApply,
		CategoryConditionExpire, CategoryMutation, CategoryTip:
		return true
	}
	return false
}

// Stub implementations — each task replaces its stub.
// Keeping them in pipeline.go for now; T5/T6/T8 move them to their
// own files.

func normalize(cat Category, text string) string {
	return Normalize(cat, text)
}
func anonymize(text string) string {
	return Anonymize(text)
}
func applyCategoryColor(cat Category, text string) string {
	if cat == CategoryDefault || text == "" {
		return text
	}
	return `<ansi fg="` + cat.String() + `">` + text + `</ansi>`
}
func wrap(text string, maxWidth int) string {
	return WrapAnsi(text, maxWidth)
}
