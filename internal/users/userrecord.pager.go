package users

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/pager"
)

// pagerState is one player's reply being built and the pages still to show
// (#842). held collects the text of the messages marked as the reply
// (events.Message.Reply) until the line's InputDone; pages and next are the
// cut reply while the player reads it. Guarded by sessionMu.
type pagerState struct {
	held  []string
	pages [][]string
	next  int // index of the next page to show
}

// HoldReply keeps one message of the reply being built. text is what
// Message_SendMessage would have written, without its line-erase prefix.
func (u *UserRecord) HoldReply(text string) {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	u.pager.held = append(u.pager.held, text)
}

// EndReply ends the reply being built and returns the text to write now:
// the whole reply when it fits a page (counting after blank lines at page
// edges are dropped, so there is never a "page 1 of 1"), when paging is off,
// while the player is in combat (owner ruling 2026-11-18), while a prompt
// question is open (the question is the prompt and takes the typed line, so a
// stop line would hide it), or on a connection to the AI port (an agent's
// next command is rarely Enter, so a stop would drop the rest); otherwise the
// first page, with the rest kept for RoutePagerInput. "" when nothing was
// held, which leaves any waiting pages alone. A reply with text replaces
// waiting pages. A page is measured in screen rows at the reader's width
// (PagerWidth), so a line the client wraps counts for each row it fills, and
// the stop line's own rows are reserved (owner ruling 6), as it is drawn:
// word wrapped at WrapWidth like every prompt (CommandPromptBody).
func (u *UserRecord) EndReply() string {
	height := u.ClientSettings().Display.GetScreenHeight()
	width := u.PagerWidth()
	wrapWidth := u.WrapWidth()
	off := u.PageSize == pager.SettingOff || u.onAIPort()
	fighting := u.Character != nil && u.Character.IsInCombat()
	asking := u.questionOpen()

	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	text := strings.Join(u.pager.held, ``)
	u.pager.held = nil
	if text == `` {
		return ``
	}
	u.pager.pages, u.pager.next = nil, 0
	if off || fighting || asking {
		return text
	}
	pages := pager.Pages(pager.Lines(text), width, wrapWidth, height, u.PageSize, u.ScreenReader)
	if len(pages) <= 1 {
		return text
	}
	u.pager.pages = pages
	u.pager.next = 1
	return pager.Join(pages[0])
}

// PagerWidth is the width in columns the reader's screen shows a line at,
// which the pager counts rows against (owner ruling 6): a web client's
// reported columns, a telnet client's reported width (NAWS, or the cursor
// position report, which sets Display.ScreenWidth without NAWSReported), and
// otherwise WrapWidth, the width the server wraps to. A connection starts at
// connections.DefaultScreenWidth, so a telnet width equal to it is taken as
// not reported.
func (u *UserRecord) PagerWidth() int {
	if cols, ok := u.reportedScreenWidth(); ok {
		return cols
	}
	return u.WrapWidth()
}

// reportedScreenWidth is the width in columns the reader's client reported
// and whether it reported one (connections.ReportedScreenWidth, the one rule
// the pager, the wrap width and the prompt record share).
func (u *UserRecord) reportedScreenWidth() (int, bool) {
	return connections.ReportedScreenWidth(u.connectionId)
}

// PageRows is the page size in screen rows the pager uses for this reader
// now, for `set pagesize` to report. 0 when paging is off.
func (u *UserRecord) PageRows() int {
	n, _ := pager.Size(u.ClientSettings().Display.GetScreenHeight(), u.PagerWidth(), u.WrapWidth(), u.PageSize, u.ScreenReader)
	return n
}

// onAIPort reports whether the player's connection came in on the AI port
// (connections.ConnAI), as AI players and the playtest harness do. The
// connection decides, not the account's IsAI flag, which only warns at login
// when the two disagree.
func (u *UserRecord) onAIPort() bool {
	cd := connections.Get(u.ConnectionId())
	return cd != nil && cd.ConnType() == connections.ConnAI
}

// questionOpen reports whether a prompt has a question waiting for an answer.
func (u *UserRecord) questionOpen() bool {
	u.sessionMu.Lock()
	p := u.activePrompt
	u.sessionMu.Unlock()
	return p != nil && p.GetNextQuestion() != nil
}

// PagesWaiting reports whether a reply has pages still to show.
func (u *UserRecord) PagesWaiting() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return len(u.pager.pages) > 0
}

// pagerStopLine is the prompt while pages wait, or "".
func (u *UserRecord) pagerStopLine() string {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if len(u.pager.pages) == 0 {
		return ``
	}
	return pager.StopLine(u.pager.next, len(u.pager.pages))
}

// RoutePagerInput reads a typed line while pages wait (owner ruling
// 2026-11-18): Enter shows the next page (out) and q drops the rest; both are
// consumed. Any other line drops the rest and is not consumed, so it reaches
// the open menu or runs as a command. With no pages waiting nothing is
// consumed. While a prompt question is open nothing is consumed and the pages
// are left as they are, so the line answers the question.
func (u *UserRecord) RoutePagerInput(line string) (out string, consumed bool) {
	if u.questionOpen() {
		return ``, false
	}
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if len(u.pager.pages) == 0 {
		return ``, false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == `` {
		page := u.pager.pages[u.pager.next]
		u.pager.next++
		if u.pager.next >= len(u.pager.pages) {
			u.pager.pages, u.pager.next = nil, 0
		}
		return pager.Join(page), true
	}
	u.pager.pages, u.pager.next = nil, 0
	return ``, strings.EqualFold(trimmed, `q`)
}

// FlushPagesInCombat returns every waiting page as one text and clears them
// when the player is in combat, so a fight never sits behind a stop (owner
// ruling 2026-11-18). "" otherwise.
func (u *UserRecord) FlushPagesInCombat() string {
	if u.Character == nil || !u.Character.IsInCombat() {
		return ``
	}
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	if len(u.pager.pages) == 0 {
		return ``
	}
	var rest []string
	for _, p := range u.pager.pages[u.pager.next:] {
		rest = append(rest, p...)
	}
	u.pager.pages, u.pager.next = nil, 0
	return pager.Join(rest)
}
