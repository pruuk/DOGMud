package messaging

import "testing"

func TestNormalizeCapitalizesSentenceStart(t *testing.T) {
	got := Normalize(CategoryHitMelee, "in control, you press forward")
	if got[:1] != "I" {
		t.Errorf("expected capitalized start, got %q", got)
	}
}

func TestNormalizeAAnAgreement(t *testing.T) {
	got := Normalize(CategoryHitMelee, "with a aggressive posture")
	if got != "With an aggressive posture." {
		t.Errorf("a/an: got %q", got)
	}
}

func TestNormalizeCollapsesDuplicateWord(t *testing.T) {
	got := Normalize(CategoryHitMelee, "negligible damage damage on")
	if got != "Negligible damage on." {
		t.Errorf("dup-word: got %q", got)
	}
}

func TestNormalizeAppendsEndPunctuation(t *testing.T) {
	got := Normalize(CategoryHitMelee, "you strike deeply")
	if got != "You strike deeply." {
		t.Errorf("end-punct: got %q", got)
	}
}

func TestNormalizeSkipsForRoomDescription(t *testing.T) {
	// Room descriptions manage their own capitalization and prose
	// shape; normalization is disabled for them.
	in := "the road winds west"
	got := Normalize(CategoryRoomDescription, in)
	if got != in {
		t.Errorf("CategoryRoomDescription must skip normalization, got %q", got)
	}
}

func TestNormalizeSkipsBannersWithBoxRule(t *testing.T) {
	in := "━━━ banner ━━━"
	got := Normalize(CategoryHitMelee, in)
	if got != in {
		t.Errorf("banner line must skip end-punct, got %q", in)
	}
}

// TestNormalizeSkipsBoxAndTableBottoms is #460: a stop was appended after a
// box or table's closing edge. Every drawn border is exempt; prose is not.
func TestNormalizeSkipsBoxAndTableBottoms(t *testing.T) {
	for _, in := range []string{
		" ┌─ Cooldowns ─┐\n  bash   soon\n └─────────────┘",
		"│ a │ b │\n└───┴───┘</ansi>\n",
		"╚═════╝",
		"+------+",
		"| Toxicity: clear |",
		"----------",
		"HP: ▓▓▓▒▒░",
	} {
		if got := Normalize(CategorySystem, in); got != in {
			t.Errorf("border line gained punctuation: %q -> %q", in, got)
		}
	}
	if got := Normalize(CategorySystem, "you strike deeply"); got != "You strike deeply." {
		t.Errorf("prose must still end with a stop, got %q", got)
	}
}

func TestNormalizeDoesNotDoublePunctuate(t *testing.T) {
	in := "You strike deeply."
	got := Normalize(CategoryHitMelee, in)
	if got != "You strike deeply." {
		t.Errorf("must not double-punctuate, got %q", got)
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	once := Normalize(CategoryHitMelee, "with a aggressive posture")
	twice := Normalize(CategoryHitMelee, once)
	if once != twice {
		t.Errorf("normalize must be idempotent: %q vs %q", once, twice)
	}
}

func TestNormalizeSkipsCritBannerAsterisks(t *testing.T) {
	// Crit/death/achievement banners end with `***` (often inside an
	// ansi close tag). No period may be appended after the decoration.
	in := `<ansi fg="crit-text">***</ansi> You land a PERFECT TEAR! <ansi fg="crit-text">***</ansi>`
	got := Normalize(CategoryHitMelee, in)
	if got != in {
		t.Errorf("*** banner must not gain punctuation, got %q", got)
	}
}

func TestNormalizeSkipsTrailingBracketTag(t *testing.T) {
	// A line ending in a bracketed tag like `[CRIT!]` is already
	// terminated — no period after the closing bracket.
	in := `You unleash a devastating verbal onslaught on Practice Butt! <ansi fg="crit-text">[CRIT!]</ansi>`
	got := Normalize(CategoryTauntSuccess, in)
	if got != in {
		t.Errorf("trailing ] must not gain punctuation, got %q", got)
	}
}

func TestNormalizeSkipsTrailingParenTag(t *testing.T) {
	// Same for a trailing parenthesized aside.
	in := `You mock them mercilessly! (a soul-shattering tirade)`
	got := Normalize(CategoryTauntSuccess, in)
	if got != in {
		t.Errorf("trailing ) must not gain punctuation, got %q", got)
	}
}

func TestNormalizeNoLonePeriodAfterTrailingNewline(t *testing.T) {
	// A message ending in punctuation + newline must pass through
	// unchanged — previously the newline hid the `...` from the
	// terminator check and a stray `.` line was appended.
	in := "You snoop around for a bit...\n"
	got := Normalize(CategorySystem, in)
	if got != in {
		t.Errorf("trailing newline must not cause a lone period, got %q", got)
	}
}

func TestNormalizeAppendKeepsTrailingNewline(t *testing.T) {
	got := Normalize(CategoryHitMelee, "you strike deeply\n")
	if got != "You strike deeply.\n" {
		t.Errorf("period must go before the trailing newline, got %q", got)
	}
}

func TestNormalizeSkipsMOTDBannerTitle(t *testing.T) {
	// The MOTD box (internal/usercommands/motd.go:50) hand-spaces its
	// title with single letters separated by single spaces. Dup-word
	// collapse sees the two adjacent "S" runs as a repeated word and
	// drops one, and end-punct then appends a stray period. The MOTD
	// owns its own formatting and must pass through untouched.
	in := `.:  M E S S A G E   O F   T H E   D A Y`
	got := Normalize(CategoryBroadcast, in)
	if got != in {
		t.Errorf("CategoryBroadcast must skip normalization, got %q, want %q", got, in)
	}
}

func TestNormalizeSkipsPlayerChatCasing(t *testing.T) {
	// Broadcast carries player-typed channel chat too
	// (internal/hooks/ChannelMessage_SendToAll.go:30, msg.Text). It
	// must be left alone exactly like the other player-typed
	// categories (CategorySpeech, CategoryWhisper, CategoryShout,
	// CategoryEmote) already are: no forced capitalization, no
	// appended period.
	in := "hey, anyone up for a dungeon run"
	got := Normalize(CategoryBroadcast, in)
	if got != in {
		t.Errorf("CategoryBroadcast must not rewrite player chat, got %q, want %q", got, in)
	}
}
