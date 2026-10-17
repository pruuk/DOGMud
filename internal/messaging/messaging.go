// Package messaging owns the centralized player-facing-text pipeline.
//
// Every Room.SendText / Room.SendTextVisual / UserRecord.SendText call
// flows through this package's pipeline (compose → normalize →
// anonymize → color → wrap → deliver). Sites identify their text by
// Category; the pipeline resolves the color, applies style
// normalization, sight-gates visual content using the Perception FSM,
// and wraps to each recipient's LineWidth preference.
//
// See docs/superpowers/specs/completed/2026-05-19-messaging-framework-design.md
// for the design and rationale.
package messaging

import "github.com/GoMudEngine/GoMud/internal/combatvocab"

// Category enumerates every recognized text class. Used by the
// pipeline to look up a color alias and to drive per-Category
// normalization-skip behavior. Adding a new Category is a 2-line
// change (constant here + alias in ansi-aliases.yaml).
type Category int

const (
	CategoryDefault Category = iota

	// Combat — hits.
	CategoryHitMelee
	CategoryHitBlunt
	CategoryHitNaturalSharp
	CategoryHitRanged
	CategoryHitCaster
	CategoryHitUnarmed

	// Combat — defense.
	CategoryDodge
	CategoryParry
	CategoryBlock

	// Combat — grapple.
	CategoryGrappleFlow
	CategoryGrappleHigh

	// Combat — submissions / death.
	CategorySubmission
	CategoryDeath
	CategoryCombatSummary      // per-round compact tally (light verbosity)
	CategoryCombatBlindWarning // per-round "you can't see" notice (M4d PR 2, Task 4)

	// Combat — special moves.
	CategorySurpriseAttack
	CategoryKick
	CategoryTrip
	CategoryBash
	CategoryRally
	CategoryWarcry
	CategoryTauntSuccess
	CategoryTauntResist
	CategoryTauntFailure

	// Spells.
	CategorySpellFold
	CategorySpellDisruption
	CategorySpellElemental
	CategorySpellEnhancement
	CategorySpellMental
	CategorySpellVital
	CategorySpellManifestation

	// Social.
	CategorySpeech
	CategoryWhisper
	CategoryShout
	CategoryOOC
	CategoryNPCDialogue
	CategoryDialogueHint
	CategoryEmote
	CategoryMobIdle
	CategoryMobEmote

	// System / meta.
	CategoryBroadcast
	CategoryTip
	CategorySystem
	CategoryError
	CategoryWarning
	CategorySkillProgress
	CategoryLogin
	CategoryLogout

	// Environment.
	CategoryRoomDescription
	CategoryRoomEntry
	CategoryRoomExit
	CategoryWeather
	CategoryTimeOfDay
	CategorySplash

	// Other.
	CategoryLoot
	CategoryEquipment
	CategoryConditionApply
	CategoryConditionExpire
	CategoryMutation
	CategoryToxin

	// Environment, appended rather than grouped with CategoryTimeOfDay so no
	// existing Category value shifts. Lighting plan 3d's band-change notices.
	CategoryLight

	// Work, appended so no existing Category value shifts (#515). The
	// crafter's and salvager's OWN lines: begin, per-round progress,
	// success, failure, interrupt and stop. Recipe success and failure
	// lines are authored prose that often runs past 80 columns, so this
	// category wraps where CategorySystem, a mixed bucket, cannot.
	// Craft refusals ("you are missing...") and cost refusals ride
	// CategoryRefusal like every refusal (M7 slice 3, #371); CategorySystem
	// keeps reports, confirmations, help, usage and admin output until
	// slice 4.
	CategoryCraft

	// The room's lines ABOUT a crafter (owner ruling C, 2026-10-13):
	// actions.NarrateCraftPhase's seen, shapes and heard lines, for a player
	// or a mob crafter. They rode CategoryEmote (player) and CategoryMobIdle
	// (mob), and CategoryEmote never wraps, so a long name and item name ran
	// past 80. Appended so no existing value shifts; normalised and colored
	// as CategoryEmote is (alias craft-seen, same value as emote).
	CategoryCraftSeen

	// Refusals and prompts (messaging M7, #371): every line internal/refusal
	// sends, and since slice 3 every refusal anywhere in the tree (the root
	// guard TestRefusalOpenersNeverRideSystem holds it). Appended so no
	// existing value shifts. Treated exactly as
	// CategorySystem, where these lines rode (every normalize stage, never
	// wrapped, alias refusal at the same value as system), so the move
	// changes nothing a player sees until the owner recolours it.
	CategoryRefusal

	// Usage lines and blocks (messaging M7 slice 4, #371): "Usage: x <y>"
	// and the *Usage(user) blocks, staff and player alike. Appended so no
	// existing value shifts; treated exactly as CategorySystem (alias usage
	// at system's value), so the move changes nothing a player sees.
	CategoryUsage

	// Staff output (M7 slice 4, #371): everything an admin-only command or a
	// staff-only subcommand shows the staff member who typed it, its
	// refusals included (owner ruling R2). Treated exactly as CategorySystem
	// (alias admin at system's value).
	CategoryAdmin

	// Party and guild membership notices (M7 slice 4, #371, owner ruling
	// R1): joined, left, kicked, promoted, invited, loot mode. Treated
	// exactly as CategorySystem (alias group at system's value). Group chat
	// rides CategoryBroadcast, where channel chat rides.
	CategoryGroup

	// A laid-out screen a command draws whose lines are headings, legends
	// and rows rather than sentences: the map (its legend ends on a name)
	// and the achievements list (its headings are bare category names).
	// Every normalize stage is skipped, as for CategorySplash, so no stop
	// lands after "Shore" or "Combat" and no map cell is recased or
	// collapsed. Never wrapped and coloured as CategorySystem (alias screen
	// at system's value), where these lines rode. Appended so no existing
	// value shifts.
	CategoryScreen

	// categoryMax is the exclusive upper bound of the Category enum.
	// Used by tests to iterate the full range; do not consume in
	// production code.
	categoryMax
)

// String returns a stable identifier for the category — used in
// logging and to key the color alias lookup in color.go.
func (c Category) String() string {
	switch c {
	case CategoryDefault:
		return "default"
	case CategoryHitMelee:
		return "hit-melee"
	case CategoryHitBlunt:
		return "hit-blunt"
	case CategoryHitNaturalSharp:
		return "hit-natural-sharp"
	case CategoryHitRanged:
		return "hit-ranged"
	case CategoryHitCaster:
		return "hit-caster"
	case CategoryHitUnarmed:
		return "hit-unarmed"
	case CategoryDodge:
		return string(combatvocab.DefenceDodge)
	case CategoryParry:
		return string(combatvocab.DefenceParry)
	case CategoryBlock:
		return string(combatvocab.DefenceBlock)
	case CategoryGrappleFlow:
		return "grapple-flow"
	case CategoryGrappleHigh:
		return "grapple-high"
	case CategorySubmission:
		return "submission"
	case CategoryDeath:
		return "death"
	case CategoryCombatSummary:
		return "combat-summary"
	case CategoryCombatBlindWarning:
		return "combat-blind-warning"
	case CategorySurpriseAttack:
		return "surprise"
	case CategoryKick:
		return "kick"
	case CategoryTrip:
		return "trip"
	case CategoryBash:
		return "bash"
	case CategoryRally:
		return "rally"
	case CategoryWarcry:
		return "warcry"
	case CategoryTauntSuccess:
		return "taunt-success"
	case CategoryTauntResist:
		return "taunt-resist"
	case CategoryTauntFailure:
		return "taunt-failure"
	case CategorySpellFold:
		return "spell-fold"
	case CategorySpellDisruption:
		return "spell-disruption"
	case CategorySpellElemental:
		return "spell-elemental"
	case CategorySpellEnhancement:
		return "spell-enhancement"
	case CategorySpellMental:
		return "spell-mental"
	case CategorySpellVital:
		return "spell-vital"
	case CategorySpellManifestation:
		return "spell-manifestation"
	case CategorySpeech:
		return "speech"
	case CategoryWhisper:
		return "whisper"
	case CategoryShout:
		return "shout"
	case CategoryOOC:
		return "ooc"
	case CategoryNPCDialogue:
		return "npc-dialogue"
	case CategoryDialogueHint:
		return "dialogue-hint"
	case CategoryEmote:
		return "emote"
	case CategoryMobIdle:
		return "mob-idle"
	case CategoryMobEmote:
		return "mob-emote"
	case CategoryBroadcast:
		return "broadcast"
	case CategoryTip:
		return "tip"
	case CategorySystem:
		return "system"
	case CategoryError:
		return "error"
	case CategoryWarning:
		return "warning"
	case CategorySkillProgress:
		return "skill-progress"
	case CategoryLogin:
		return "login"
	case CategoryLogout:
		return "logout"
	case CategoryRoomDescription:
		return "room-description"
	case CategoryRoomEntry:
		return "room-entry"
	case CategoryRoomExit:
		return "room-exit"
	case CategoryWeather:
		return "weather"
	case CategoryTimeOfDay:
		return "time-of-day"
	case CategorySplash:
		return "splash"
	case CategoryLoot:
		return "loot"
	case CategoryEquipment:
		return "equipment"
	case CategoryConditionApply:
		return "condition-apply"
	case CategoryConditionExpire:
		return "condition-expire"
	case CategoryMutation:
		return "mutation"
	case CategoryToxin:
		return "toxin"
	case CategoryLight:
		return "light"
	case CategoryCraft:
		return "craft"
	case CategoryCraftSeen:
		return "craft-seen"
	case CategoryRefusal:
		return "refusal"
	case CategoryUsage:
		return "usage"
	case CategoryAdmin:
		return "admin"
	case CategoryGroup:
		return "group"
	case CategoryScreen:
		return "screen"
	}
	return "Unknown"
}

// BlankLineAllowed reports whether a deliberate blank line may ride cat
// (#610). A blank line is screen spacing: it separates the blocks of a laid
// out screen (look, set, craft, the admin screens, scan). It belongs on the
// categories those screens ride and nowhere else, so a blank line never
// lands in a combat, ambient, speech or refusal stream. UserRecord.SendBlankLine
// refuses any other category, and the root guard TestBlankLineSendsAreRecorded
// holds every call site to a literal category from this set.
func BlankLineAllowed(cat Category) bool {
	switch cat {
	case CategorySystem, CategoryScreen, CategoryAdmin, CategoryRoomDescription:
		return true
	}
	return false
}

// EmoteCategoryFor is the category of a line told to a player about what
// another creature does: CategoryEmote when that creature is a player,
// CategoryMobEmote when it is a mob, the split M5 made for emotes. Neither
// category runs the normalizer's capitalize stage, so a sender whose line
// opens with the creature's name builds it through CapitalizeStart.
func EmoteCategoryFor(isPlayer bool) Category {
	if isPlayer {
		return CategoryEmote
	}
	return CategoryMobEmote
}
