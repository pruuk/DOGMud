package messaging

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// ansiOpenTagExpr matches one <ansi …> open tag with any attribute set
// (fg, bg, or combinations). The a/an stage in normalize.go uses it to look
// through a run of them; ansiTagLen is the same grammar for the wrapper.
const ansiOpenTagExpr = `<ansi[^>]*>`

const (
	ansiOpenPrefix = `<ansi`
	ansiCloser     = `</ansi>`
)

// ansiTagLen returns the byte length of the <ansi …> or </ansi> tag that
// begins s, or 0 when s does not begin with one. Anything other than a tag
// is refused on its first few bytes, and a tag is read no further than its
// own closing '>'.
func ansiTagLen(s string) int {
	if strings.HasPrefix(s, ansiCloser) {
		return len(ansiCloser)
	}
	if !strings.HasPrefix(s, ansiOpenPrefix) {
		return 0
	}
	end := strings.IndexByte(s[len(ansiOpenPrefix):], '>')
	if end < 0 {
		return 0
	}
	return len(ansiOpenPrefix) + end + 1
}

// runeColumns is the display columns one rune takes. A carriage return takes
// none: text pre-formatted elsewhere arrives with CRLF line ends, and counting
// the CR stranded a word on a line of its own (#430). Every other ASCII byte
// is one, control characters included; anything else is the width
// util.VisibleWidth would give it.
func runeColumns(r rune) int {
	if r == '\r' {
		return 0
	}
	if r < utf8.RuneSelf {
		return 1
	}
	return runewidth.RuneWidth(r)
}

// WrapAnsi wraps text at maxWidth display columns. ANSI escape
// sequences (<ansi …> / </ansi> tags) don't count toward width.
// Open tags carry across line breaks: a break inside N nested spans
// closes all N before the newline and reopens all N, outermost first,
// on the next line.
//
// Only a line wider than maxWidth is folded. A line that fits is copied byte
// for byte, its padding and indent included, so a table or a box narrower than
// the reader survives. A fold rejoins that line's words with single spaces. An
// embedded newline is always a hard break.
//
// On malformed input (orphan tags, unmatched closers), falls back to
// a byte-count wrap to avoid panicking. The visual output is uglier
// but the server stays up.
//
// A maxWidth of 0 (unset) returns the input unchanged.
func WrapAnsi(text string, maxWidth int) (wrapped string) {
	if maxWidth <= 0 || text == "" {
		return text
	}
	defer func() {
		// Last-resort: if anything in the parser panics, the caller
		// gets the original text back. This needs a NAMED result and an
		// explicit assignment: a bare recover() on an unnamed return
		// silently handed the caller "" and erased the message.
		if r := recover(); r != nil {
			wrapped = text
		}
	}()

	// tagOp is a push or a pop recorded while scanning the word that is
	// still being accumulated in curWord. Tag state for a word in
	// progress must not mutate openTags until that word actually
	// commits to line: the wrap decision for THIS word has to see the
	// stack as it stood at the end of the PREVIOUS word, not a stack
	// already advanced by a closer sitting at this word's own tail
	// (e.g. "floor</ansi>" as one token). Applying pops live during the
	// scan let a same-word closer erase the span the wrap decision was
	// about to close, which left the next line unreopened.
	type tagOp struct {
		close bool
		tag   string
	}

	// Walk the text token-by-token, tracking display column and the
	// stack of currently-open ANSI tags. When we cross maxWidth at a
	// word boundary, emit a newline; every open tag is closed before
	// the break and reopened, outermost first, on the next line.
	var (
		out            strings.Builder
		line           strings.Builder
		col            int
		openTags       []string // stack committed to `line`/`out`, outermost first
		pending        []tagOp  // tag ops seen inside the word still in curWord
		curWord        strings.Builder
		curWordW       int
		lineHasContent bool // visible content on line (not just tag re-opener)
	)

	// closeAll and reopenAll keep a wrapped line balanced. A line break
	// inside N open spans must close all N before the newline and reopen
	// all N, outermost first, on the next line. The old code tracked a
	// single tag, so a balanced inner </ansi> silently dropped the outer
	// span for the rest of the message.
	closeAll := func(b *strings.Builder) {
		for range openTags {
			b.WriteString(`</ansi>`)
		}
	}
	reopenAll := func(b *strings.Builder) {
		for _, tag := range openTags {
			b.WriteString(tag)
		}
	}

	// commitPending folds the tag ops seen so far into the stack that governs
	// future wrap decisions.
	commitPending := func() {
		for _, op := range pending {
			if op.close {
				if len(openTags) > 0 {
					openTags = openTags[:len(openTags)-1]
				}
			} else {
				openTags = append(openTags, op.tag)
			}
		}
		pending = pending[:0]
	}

	// Every tag ends in '>', so a '<' past the last one is plain text. Before
	// it, the search for an open tag's '>' always succeeds and the walk then
	// jumps over what was searched, so WrapAnsi does constant work at a stray
	// less-than sign and one linear pass overall.
	lastTagEnd := strings.LastIndexByte(text, '>')

	// nextTagLen is the length of the ansi tag at text[at:], or 0 for none. A
	// found tag is added to pending, for the caller to commit when its text
	// lands on a line.
	nextTagLen := func(at int) int {
		if text[at] != '<' || at >= lastTagEnd {
			return 0
		}
		tagLen := ansiTagLen(text[at:])
		if tagLen > 0 {
			tag := text[at : at+tagLen]
			if strings.HasPrefix(tag, `</`) {
				pending = append(pending, tagOp{close: true})
			} else {
				pending = append(pending, tagOp{close: false, tag: tag})
			}
		}
		return tagLen
	}

	// copyFittingLine copies the logical line starting at start to line, byte
	// for byte, when it fits in maxWidth, and returns where it ends (its
	// newline, or the end of the text). A line too wide for that is left for
	// the fold below and nothing is changed.
	copyFittingLine := func(start int) (end int, fits bool) {
		cols := 0
		at := start
		for at < len(text) && text[at] != '\n' {
			if tagLen := nextTagLen(at); tagLen > 0 {
				at += tagLen
				continue
			}
			r, size := utf8.DecodeRuneInString(text[at:])
			cols += runeColumns(r)
			if cols > maxWidth {
				pending = pending[:0]
				return 0, false
			}
			at += size
		}
		line.WriteString(text[start:at])
		commitPending()
		return at, true
	}

	// hardBreak ends the current line at an embedded newline.
	hardBreak := func() {
		closeAll(&line)
		out.WriteString(line.String())
		out.WriteByte('\n')
		line.Reset()
		col = 0
		lineHasContent = false
		reopenAll(&line)
	}

	flushWord := func() {
		// Add space before word if line already has content and there's room.
		// The wrap/no-wrap decision below uses openTags as committed by the
		// PREVIOUS word: pending (this word's own tag ops) is applied only
		// after the word lands in line.
		if lineHasContent && col+1+curWordW > maxWidth {
			// Wrap before the word.
			closeAll(&line)
			out.WriteString(line.String())
			out.WriteByte('\n')
			line.Reset()
			col = 0
			lineHasContent = false
			reopenAll(&line)
		} else if lineHasContent {
			line.WriteByte(' ')
			col++
		}
		line.WriteString(curWord.String())
		col += curWordW
		lineHasContent = true
		curWord.Reset()
		curWordW = 0

		// Now that the word has committed to line, its tag ops count.
		commitPending()
	}

	i := 0
	atLineStart := true
	for i < len(text) {
		if atLineStart {
			atLineStart = false
			if end, fits := copyFittingLine(i); fits {
				i = end
				if i < len(text) {
					hardBreak()
					i++
					atLineStart = true
				}
				continue
			}
		}
		// ANSI tag?
		if tagLen := nextTagLen(i); tagLen > 0 {
			curWord.WriteString(text[i : i+tagLen])
			i += tagLen
			continue
		}
		// The CR of a CRLF belongs to the line end, which the deliverer writes.
		if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
			i++
			continue
		}
		// Whitespace boundary?
		if text[i] == ' ' || text[i] == '\n' {
			if curWord.Len() > 0 {
				flushWord()
			}
			if text[i] == '\n' {
				hardBreak()
				atLineStart = true
			}
			i++
			continue
		}
		// Visible character. Advance by one RUNE and count its display
		// columns: indexing the string byte-by-byte made every multi-byte
		// character count as two or more columns, and counting runes made
		// a wide character count as one.
		r, size := utf8.DecodeRuneInString(text[i:])
		curWord.WriteString(text[i : i+size])
		curWordW += runeColumns(r)
		i += size
	}
	if curWord.Len() > 0 {
		flushWord()
	}
	out.WriteString(line.String())
	return out.String()
}

// WrapPrompt wraps text that ends where the reader types, such as a login
// prompt ending in ": ", to maxWidth as WrapAnsi does, and keeps its trailing
// spaces on its last line, so the cursor stays after the ": " instead of
// dropping to a line of its own (WrapAnsi folds a line's trailing spaces
// away). The run of spaces and </ansi> closers the text ends with is set
// aside, the rest is wrapped, and the run is put back. When the last line
// then has no room for the spaces, the rest is wrapped narrower by their
// count. A text that already fits is returned byte for byte.
func WrapPrompt(text string, maxWidth int) string {
	if maxWidth <= 0 || text == "" {
		return text
	}
	cut := len(text)
	for cut > 0 {
		if text[cut-1] == ' ' {
			cut--
			continue
		}
		if strings.HasSuffix(text[:cut], ansiCloser) {
			cut -= len(ansiCloser)
			continue
		}
		break
	}
	body, tail := text[:cut], text[cut:]
	spaces := strings.Count(tail, " ")
	wrapped := WrapAnsi(body, maxWidth)
	if spaces > 0 && spaces < maxWidth && lastLineColumns(wrapped)+spaces > maxWidth {
		wrapped = WrapAnsi(body, maxWidth-spaces)
	}
	return wrapped + tail
}

// lastLineColumns is the display columns of the text after its last newline,
// colour tags not counted.
func lastLineColumns(text string) int {
	last := text[strings.LastIndexByte(text, '\n')+1:]
	cols := 0
	for i := 0; i < len(last); {
		if tagLen := ansiTagLen(last[i:]); tagLen > 0 {
			i += tagLen
			continue
		}
		r, size := utf8.DecodeRuneInString(last[i:])
		cols += runeColumns(r)
		i += size
	}
	return cols
}
