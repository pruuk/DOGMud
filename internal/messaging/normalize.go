package messaging

import (
	"regexp"
	"strings"
)

// normalizeSkip indicates per-Category opt-outs from individual
// normalization stages. Defaults to all-on; categories whose prose
// is hand-authored set bits here to keep their original styling.
//
// Bitmask values (powers of 2) are OR'd together. Zero = all stages
// active.
type normalizeStage uint8

const (
	stageCapitalize      normalizeStage = 1 << iota // sentence-start caps
	stageAAnAgreement                               // a/an agreement
	stageDupWordCollapse                            // duplicate word collapse
	stageEndPunct                                   // sentence-end punctuation
	stageNameCanon                                  // ANSI name canonicalization (T8 stub; future polish)
)

// skipStages returns the stage mask of stages to SKIP for cat.
func skipStages(cat Category) normalizeStage {
	switch cat {
	case CategoryMobEmote:
		// Authored mob prose keeps its casing, articles and words, but a
		// sentence still ends: 700+ shipped idle emotes were written with
		// no full stop and printed that way (#455). Player-typed emotes
		// ride CategoryEmote, which stays fully exempt below.
		return stageCapitalize | stageAAnAgreement | stageDupWordCollapse |
			stageNameCanon
	case CategoryRoomDescription, CategoryRoomEntry, CategoryRoomExit,
		CategoryWeather, CategoryTimeOfDay, CategoryLight, CategorySplash,
		CategoryNPCDialogue, CategoryDialogueHint,
		CategoryMobIdle,
		CategorySpeech, CategoryWhisper, CategoryShout, CategoryEmote,
		CategoryBroadcast,
		CategorySkillProgress: // banner has its own formatting
		// CategoryBroadcast has exactly two production senders and
		// neither wants the server rewriting its text: the MOTD box
		// (internal/usercommands/motd.go:77) owns its formatting
		// completely, down to the hand-spaced banner title, and
		// channel chat (internal/hooks/ChannelMessage_SendToAll.go:30)
		// sends text a player typed, same as the other player-typed
		// categories (CategorySpeech, CategoryWhisper, CategoryShout,
		// CategoryEmote) already exempted above.
		// These categories own their prose shape. Skip everything.
		return stageCapitalize | stageAAnAgreement | stageDupWordCollapse |
			stageEndPunct | stageNameCanon
	}
	return 0
}

var (
	// wordRunPattern finds whitespace-delimited word runs. We then
	// scan adjacent matches to collapse duplicates (Go's RE2 doesn't
	// support backreferences, so the doc-comment shorthand
	// `\b(\w+) \1\b` is implemented manually below).
	wordRunPattern = regexp.MustCompile(`\w+`)

	// aBeforeVowel finds an article "a"/"A" followed by one space, then
	// zero or more <ansi …> open tags, then a vowel. The tags are looked
	// through because names arrive wrapped ("a <ansi fg="itemname">Ivory
	// Handled Fan</ansi>"), which once hid the vowel. Only the article
	// byte is rewritten; the tags are kept byte for byte. Idempotent: an
	// "an" never matches, since the article must be followed by a space.
	//
	// Known limitation: the test is by letter, not sound, so "a useful"
	// becomes "an useful" and "a hour" stays "a hour". That predates the
	// tag look-through and is out of scope here.
	aBeforeVowel = regexp.MustCompile(`\b([aA]) (?:` + ansiOpenTagExpr + `)*[aeiouAEIOU]`)
)

// Normalize runs the five style-normalization stages on text. Stages
// individually opt-out via skipStages(cat).
//
// Idempotent: Normalize(cat, Normalize(cat, x)) == Normalize(cat, x).
//
// Pure-string transforms; safe to call on any input including
// already-tagged ANSI text. Dup-word collapse is ANSI-blind — see
// collapseDupWords for the manual-scan implementation (RE2 doesn't
// support the backreference the spec originally suggested).
func Normalize(cat Category, text string) string {
	if text == "" {
		return text
	}
	skip := skipStages(cat)

	// 1. Sentence-start capitalization.
	if skip&stageCapitalize == 0 {
		text = capitalizeStart(text)
	}

	// 2. a/an agreement.
	if skip&stageAAnAgreement == 0 {
		text = aBeforeVowel.ReplaceAllStringFunc(text, func(match string) string {
			// match is `[aA] <tags…>[aeiouAEIOU]`. Rewrite only the
			// article byte, preserve its case, keep the rest verbatim.
			article := match[:1]
			rest := match[1:]
			if article == "A" {
				return "An" + rest
			}
			return "an" + rest
		})
	}

	// 3. Duplicate-word collapse.
	if skip&stageDupWordCollapse == 0 {
		text = collapseDupWords(text)
	}

	// 4. Sentence-end punctuation auto-append.
	if skip&stageEndPunct == 0 {
		text = appendEndPunct(text)
	}

	// 5. ANSI name canonicalization is deferred to a future polish
	// pass — v1 relies on the per-package audit to tag names
	// explicitly at the call site. Hook remains here for later
	// extension.
	_ = stageNameCanon

	return text
}

// collapseDupWords is Go-RE2's stand-in for `\b(\w+) \1\b`. It walks
// the word-run match list and drops any run that exactly equals the
// previous run when separated by a single ASCII space. Repeats until
// no more collapses occur so triples like "the the the" become "the".
// ANSI-blind by construction — `\w` excludes `<` and `>`, so a run
// can't straddle a tag.
func collapseDupWords(text string) string {
	for {
		matches := wordRunPattern.FindAllStringIndex(text, -1)
		if len(matches) < 2 {
			return text
		}
		collapsed := false
		var b strings.Builder
		b.Grow(len(text))
		// Emit text up to and including the first word.
		b.WriteString(text[:matches[0][1]])
		prevEnd := matches[0][1]
		prevWord := text[matches[0][0]:matches[0][1]]
		for i := 1; i < len(matches); i++ {
			start, end := matches[i][0], matches[i][1]
			word := text[start:end]
			gap := text[prevEnd:start]
			if gap == " " && word == prevWord {
				// Drop this word + its preceding space. Advance
				// prevEnd past the skipped word so the next gap is
				// computed from the right offset; keep prevWord
				// unchanged so triples ("the the the") still collapse
				// to a single "the" in subsequent iterations.
				collapsed = true
				prevEnd = end
				continue
			}
			b.WriteString(gap)
			b.WriteString(word)
			prevEnd = end
			prevWord = word
		}
		// Emit any trailing tail after the last word.
		b.WriteString(text[prevEnd:])
		text = b.String()
		if !collapsed {
			return text
		}
	}
}

// capitalizeStart uppercases the first non-ansi-tag character of text.
// Idempotent — if the first letter is already uppercase, no-op.
func capitalizeStart(text string) string {
	// Skip past any opening <ansi …> tag(s) before capitalizing.
	i := 0
	for i < len(text) && text[i] == '<' {
		j := strings.IndexByte(text[i:], '>')
		if j < 0 {
			return text // malformed; leave alone
		}
		i += j + 1
	}
	if i >= len(text) {
		return text
	}
	c := text[i]
	if c >= 'a' && c <= 'z' {
		return text[:i] + string(c-32) + text[i+1:]
	}
	return text
}

// appendEndPunct adds a `.` if the last non-tag non-space char isn't
// already a sentence terminator. Skips banner lines (start with `━`),
// `***`-decorated lines (crit/death banners), pure exclamations, and
// pure-tag wrappers. Trailing newlines are looked *through* for the
// check and preserved on output — previously a message ending "...\n"
// had the `.` appended after the newline, printing a stray lone-period
// line (seen after `search`, `help`, and `inventory` output).
func appendEndPunct(text string) string {
	trimmed := strings.TrimRight(text, " \t\r\n")
	if trimmed == "" {
		return text
	}
	// Banner skip.
	if strings.HasPrefix(strings.TrimSpace(text), "━") {
		return text
	}
	// Trailing whitespace/newlines to reattach after the period.
	trailing := text[len(trimmed):]
	// Strip a trailing </ansi> for the check; reattach.
	suffix := ""
	for strings.HasSuffix(trimmed, "</ansi>") {
		suffix = "</ansi>" + suffix
		trimmed = trimmed[:len(trimmed)-len("</ansi>")]
	}
	if trimmed == "" {
		return text
	}
	last := trimmed[len(trimmed)-1]
	switch last {
	case '.', '!', '?', ',', ')', ']', '"', '\'', '*':
		// `*` covers `***` banner decorations (crits, deaths,
		// achievements) — never append punctuation after them.
		// `)` and `]` cover trailing parenthesized/bracketed tags
		// ("(a soul-shattering tirade)", "[CRIT!]") — a line ending
		// in one is already terminated.
		return text
	}
	return trimmed + "." + suffix + trailing
}
