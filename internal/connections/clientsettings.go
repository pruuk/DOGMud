package connections

const (
	// DefaultScreenWidth is the default width of the screen
	DefaultScreenWidth = 80
	// DefaultScreenHeight is the default height of the screen
	DefaultScreenHeight = 24
)

// ClientSettings are what a connection's client has told the server about
// itself. They belong to the connection, not the account.
type ClientSettings struct {
	Display           DisplaySettings
	MSPEnabled        bool // Do they accept sound in their client?
	SendTelnetGoAhead bool // Defaults false, should we send a IAC GA after prompts?
	AsciiMode         bool // Convert UTF-8 decorative chars to ASCII equivalents?
	// ScreenReader is true when the client said its player uses a screen
	// reader: the web client's "screen reader on" (Client.Display) or a
	// telnet client's MTTS bit 64.
	ScreenReader bool
	// MTTSReported is true once a telnet client answered MTTS, with the
	// screen reader bit set or not. Either way it answers the creation
	// question, so the question is not asked.
	MTTSReported bool
	// TermTypeReplies counts the client's terminal type replies, and
	// TermTypeLast is the latest one; the MTTS cycle stops on a repeat.
	TermTypeReplies int
	TermTypeLast    string
	// NAWSReported is true once a telnet client sent its window size by
	// NAWS. NAWS then owns the screen size: the one-off ANSI cursor report
	// asked for at connect fills it only when NAWS has not (#915).
	NAWSReported bool
}

func (c ClientSettings) IsMsp() bool {
	return c.MSPEnabled
}

type DisplaySettings struct {
	ScreenWidth  uint32
	ScreenHeight uint32
}

func (c DisplaySettings) GetScreenWidth() int {
	if c.ScreenWidth == 0 {
		return DefaultScreenWidth
	}

	return int(c.ScreenWidth)
}

func (c DisplaySettings) GetScreenHeight() int {
	if c.ScreenHeight == 0 {
		return DefaultScreenHeight
	}
	return int(c.ScreenHeight)
}
