package connections

// telnetIAC is the telnet "Interpret As Command" marker that opens every
// negotiation sequence. Defined locally so this file stays free of any import
// that the connection layer would not otherwise need.
const telnetIAC = 0xFF

const (
	telnetSE   = 0xF0
	telnetSB   = 0xFA
	telnetWILL = 0xFB
	telnetWONT = 0xFC
	telnetDO   = 0xFD
	telnetDONT = 0xFE
)

// splitInputLines splits one socket read into its individual input lines.
//
// The defect this exists to prevent: CleanserInputHandler strips every
// non-printing rune from the read buffer -- interior CR and LF included -- and
// decides "enter was pressed" from the last byte alone. One socket read
// therefore became exactly one command no matter how many lines it carried, so
// any client that put several commands in a single TCP segment had them
// silently merged into one nonsense command. `east`, `west`, `east` arrived as
// `eastwesteast`, and the player saw "Eastwesteast not recognized."
//
// A human typing cannot produce that, which is why it survived for so long. A
// speedwalk, an alias that fires several commands, a trigger, or a pasted block
// does it every time, so it hit exactly the classic client families -- TinTin++,
// MUSHclient, the zMUD/CMUD line -- hardest.
//
// Returns nil when there is nothing to split (zero lines, one line, or a
// still-incomplete line), so callers keep their existing single-read path.
//
// Telnet negotiations are emitted as distinct segments. Their payloads are
// binary and may legitimately contain a literal 0x0A or 0x0D: NAWS encodes the
// window dimensions as raw bytes, so a terminal ten columns wide embeds an LF.
// Delimiters are therefore recognized only outside IAC sequences. Isolating
// the negotiation also preserves a printable line coalesced into the same
// socket read, such as an AI client's GMCP handshake followed by its username.
func splitInputLines(b []byte) [][]byte {
	if len(b) == 0 {
		return nil
	}

	var out [][]byte

	start := 0
	for i := 0; i < len(b); {
		if b[i] == telnetIAC {
			if start < i {
				out = append(out, b[start:i])
			}

			end := telnetSequenceEnd(b, i)
			out = append(out, b[i:end])
			i = end
			start = end
			continue
		}

		if b[i] != '\r' && b[i] != '\n' {
			i++
			continue
		}

		end := i + 1
		// CRLF is one terminator, not two, so it must not yield a phantom
		// empty command between the CR and the LF.
		if b[i] == '\r' && end < len(b) && b[end] == '\n' {
			end++
		}

		out = append(out, b[start:end])
		start = end
		i = end
	}

	// Whatever trails the last terminator is a line the client has not
	// finished sending. It stays a segment of its own so the caller can keep
	// accumulating it.
	if start < len(b) {
		out = append(out, b[start:])
	}

	if len(out) < 2 {
		return nil
	}
	return out
}

// telnetSequenceEnd returns the first byte after the IAC sequence beginning at
// start. Incomplete sequences consume the remainder of the current socket read
// and are left intact for the existing telnet handler.
func telnetSequenceEnd(b []byte, start int) int {
	if start+1 >= len(b) {
		return len(b)
	}

	switch b[start+1] {
	case telnetIAC:
		return start + 2
	case telnetWILL, telnetWONT, telnetDO, telnetDONT:
		if start+3 > len(b) {
			return len(b)
		}
		return start + 3
	case telnetSB:
		for i := start + 2; i+1 < len(b); i++ {
			if b[i] != telnetIAC {
				continue
			}
			if b[i+1] == telnetIAC {
				i++
				continue
			}
			if b[i+1] == telnetSE {
				return i + 2
			}
		}
		return len(b)
	default:
		return start + 2
	}
}

// queueSplitInput splits a freshly read chunk and stores everything after the
// first line for later Read calls. It returns the byte length of the first
// line when a split happened, and 0 when the chunk was left whole.
//
// The queued segments are copied. They alias the caller's read buffer, which is
// reused on the very next read.
func (cd *ConnectionDetails) queueSplitInput(b []byte) int {
	parts := splitInputLines(b)
	if parts == nil {
		return 0
	}

	cd.readQueueMu.Lock()
	for _, part := range parts[1:] {
		queued := make([]byte, len(part))
		copy(queued, part)
		cd.readQueue = append(cd.readQueue, queued)
	}
	cd.readQueueMu.Unlock()

	return len(parts[0])
}

// nextQueuedInput copies the next queued line into p, reporting false once the
// queue is empty and the socket must be read again.
func (cd *ConnectionDetails) nextQueuedInput(p []byte) (int, bool) {
	cd.readQueueMu.Lock()
	defer cd.readQueueMu.Unlock()

	if len(cd.readQueue) == 0 {
		return 0, false
	}

	chunk := cd.readQueue[0]
	cd.readQueue = cd.readQueue[1:]

	return copy(p, chunk), true
}
