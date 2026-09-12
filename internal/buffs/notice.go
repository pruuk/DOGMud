package buffs

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// StartUserNotice is the line the holder reads when this buff lands: the
// authored start_user_text, or "<Name> takes effect." when none is authored.
// A secret buff says nothing. A buff with no name says nothing either, rather
// than print " takes effect."; the root guard fails the build on that case.
//
// This is the one door for the player-side start line. Buff_ApplyBuffs reads
// it instead of StartUserText, so a buff added without text can no longer
// land in silence.
func (b *BuffSpec) StartUserNotice() string {
	if b.Secret || b.Name == "" {
		return ""
	}
	if b.StartUserText != "" {
		return b.StartUserText
	}
	return fmt.Sprintf("%s takes effect.", b.Name)
}

// EndUserNotice is the line the holder reads when this buff ends: the
// authored end_user_text, or "<Name> has expired." when none is authored.
// A secret or nameless buff says nothing. The player prune pass reads it
// instead of EndUserText.
func (b *BuffSpec) EndUserNotice() string {
	if b.Secret || b.Name == "" {
		return ""
	}
	if b.EndUserText != "" {
		return b.EndUserText
	}
	return fmt.Sprintf("%s has expired.", b.Name)
}

// SilentNoticeBuffs lists every loaded non-secret buff that relies on the
// generic line for its start or end notice, as "<id> <name> (start, end)".
// Sorted by id. The root guard keeps this empty for the shipped world; the
// boot warning reports it for any other world or a hot edit.
func SilentNoticeBuffs() []string {
	ids := GetAllBuffIds()
	sort.Ints(ids)
	out := []string{}
	for _, id := range ids {
		b := GetBuffSpec(id)
		if b == nil || b.Secret {
			continue
		}
		missing := ""
		if b.StartUserText == "" {
			missing = "start"
		}
		if b.EndUserText == "" {
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

// WarnSilentNotices logs one warning per buff relying on the generic notice.
// Wired at boot after the buffs load. A warning, not a panic: the generic
// line exists so play continues; the root guard is what blocks a merge.
func WarnSilentNotices() {
	for _, entry := range SilentNoticeBuffs() {
		mudlog.Warn("buffs.WarnSilentNotices", "buff", entry, "notice", "relies on the generic takes effect / has expired line; author start_user_text and end_user_text")
	}
}
