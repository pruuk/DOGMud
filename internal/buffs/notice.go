package buffs

import (
	"fmt"
	"slices"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// hasFlag reports whether this spec carries the given flag.
func (b *BuffSpec) hasFlag(f Flag) bool {
	return slices.Contains(b.Flags, f)
}

// StartUserNotice is the line the holder reads when this buff lands: the
// authored start_user_text, or "<Name> takes effect." when none is authored.
// A secret buff says nothing. A silent-start buff (warcry, rally, the bloom
// detox drink) also says nothing at start: it is applied outside the buff
// event so Buff_ApplyBuffs never runs for it, and whatever applies it already
// narrates the start, so authored start_user_text would never be reachable
// and is not even checked. A buff with no name keeps its authored line but
// gets no generic one, rather than print " takes effect."; the root guard
// fails the build on a nameless non-secret buff.
//
// This is the one door for the player-side start line. Buff_ApplyBuffs reads
// it instead of StartUserText, so a buff added without text can no longer
// land in silence.
func (b *BuffSpec) StartUserNotice() string {
	if b.Secret {
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

// EndUserNotice is the line the holder reads when this buff ends: the
// authored end_user_text, or "<Name> has expired." when none is authored.
// A secret buff says nothing; a nameless one keeps its authored line only.
// A hidden buff (Hidden, Empathic Shroud) also says nothing at end, even
// over authored text: a hider must not learn when their cover lapsed, or
// the notice itself becomes the leak. Room text still goes out through the
// prune pass, since observers' view of the reappearance is not the secret.
// The player prune pass reads it instead of EndUserText.
func (b *BuffSpec) EndUserNotice() string {
	if b.Secret {
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
		if b.StartUserText == "" && !b.hasFlag(SilentStart) {
			missing = "start"
		}
		if b.EndUserText == "" && !b.hasFlag(Hidden) {
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
