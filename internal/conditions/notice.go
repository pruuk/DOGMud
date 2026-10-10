package conditions

import (
	"fmt"
	"slices"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// hasFlag reports whether this spec carries the given flag.
func (b *ConditionSpec) hasFlag(f Flag) bool {
	return slices.Contains(b.Flags, f)
}

// StartUserNotice is the line the holder reads when this condition lands: the
// authored start_actee, or "<Name> takes effect." when none is authored.
// A secret condition says nothing. A silent-start condition also says nothing at start
// because whatever applies it narrates the moment itself: warcry and rally go
// through Character.AddCondition, which never queues the condition event, so no line
// could reach the holder anyway; the bloom detox drink DOES reach
// Condition_ApplyConditions on the unscaled drink path, and the flag is what keeps the
// purge's own narration from being doubled. Authored start_actee is not
// consulted for a silent-start condition. A condition with no name keeps its authored line but
// gets no generic one, rather than print " takes effect."; the root guard
// fails the build on a nameless non-secret condition.
//
// This is the one door for the player-side start line. Condition_ApplyConditions reads
// it instead of StartUserText, so a condition added without text can no longer
// land in silence.
func (b *ConditionSpec) StartUserNotice() string {
	if b.Secret {
		return ""
	}
	if b.hasFlag(Quiet) {
		return ""
	}
	if b.hasFlag(SilentStart) {
		return ""
	}
	if b.StartUserText != "" {
		return b.StartUserText
	}
	if b.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s takes effect.", b.Name)
}

// StartActorNotice is the line the CASTER reads when this condition lands on
// someone else: the authored start_actor, silenced by the same flags as
// StartUserNotice. There is no generic fallback: a condition with no caster
// line keeps the spell's own lines instead (NarratesCastStart).
func (b *ConditionSpec) StartActorNotice() string {
	if b.Secret || b.hasFlag(Quiet) || b.hasFlag(SilentStart) {
		return ""
	}
	return b.StartActorText
}

// NarratesCastStart reports whether this condition's own start lines will
// tell every audience of a spell landing it, so the spell drops its generic
// "takes effect" lines (owner ruling R11): an authored holder line and room
// line, and, when the caster is someone else, an authored caster line. The
// generic "<Name> takes effect." fallback does not count, and neither does a
// silent start. selfCast: the caster is the holder, so the holder line is the
// caster's line and no start_actor is needed. A refresh of a record already
// held narrates no start, and the caller checks that separately. It judges
// authored text only; whether a hidden holder's room line is actually told is
// the caller's call.
func (b *ConditionSpec) NarratesCastStart(selfCast bool) bool {
	if b.StartUserNotice() == "" || b.StartUserText == "" || b.StartRoomText == "" {
		return false
	}
	return selfCast || b.StartActorNotice() != ""
}

// EndUserNotice is the line the holder reads when this condition ends: the
// authored end_actee, or "<Name> has expired." when none is authored.
// A secret condition says nothing; a nameless one keeps its authored line only.
// A hidden condition (Hidden, Empathic Shroud) also says nothing at end, even
// over authored text: a hider must not learn when their cover lapsed, or
// the notice itself becomes the leak. Room text still goes out through the
// prune pass, since observers' view of the reappearance is not the secret.
// The player prune pass reads it instead of EndUserText.
func (b *ConditionSpec) EndUserNotice() string {
	if b.Secret {
		return ""
	}
	if b.hasFlag(Quiet) {
		return ""
	}
	if b.hasFlag(Hidden) {
		return ""
	}
	if b.EndUserText != "" {
		return b.EndUserText
	}
	if b.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s has expired.", b.Name)
}

// SilentNoticeConditions lists every loaded non-secret condition that relies on the
// generic line for its start or end notice, as "<id> <name> (start, end)".
// Sorted by id. The root guard keeps this empty for the shipped world; the
// boot warning reports it for any other world or a hot edit.
func SilentNoticeConditions() []string {
	ids := GetAllConditionIds()
	sort.Ints(ids)
	out := []string{}
	for _, id := range ids {
		b := GetConditionSpec(id)
		if b == nil || b.Secret {
			continue
		}
		missing := ""
		if b.StartUserText == "" && !b.hasFlag(SilentStart) && !b.hasFlag(Quiet) {
			missing = "start"
		}
		if b.EndUserText == "" && !b.hasFlag(Hidden) && !b.hasFlag(Quiet) {
			if missing != "" {
				missing += ", "
			}
			missing += "end"
		}
		if missing != "" {
			out = append(out, fmt.Sprintf("%d %s (%s)", id, b.Name, missing))
		}
	}
	return out
}

// WarnSilentNotices logs one warning per condition relying on the generic notice.
// Wired at boot after the conditions load. A warning, not a panic: the generic
// line exists so play continues; the root guard is what blocks a merge.
func WarnSilentNotices() {
	for _, entry := range SilentNoticeConditions() {
		mudlog.Warn("conditions.WarnSilentNotices", "condition", entry, "notice", "relies on the generic takes effect / has expired line; author start_actee and end_actee")
	}
}
