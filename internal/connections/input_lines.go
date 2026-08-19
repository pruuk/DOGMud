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
//
// inSubneg says this chunk RESUMES a subnegotiation whose IAC SE terminator did
// not arrive in the previous read; stillInSubneg reports the same state for the
// next one. Without that carry, the continuation half of a fragmented
// negotiation is scanned as ordinary text, and a payload byte that happens to
// be 0x0A or 0x0D is treated as an end-of-line and splits the sequence in two.
// NAWS encodes window dimensions as raw bytes and GMCP payloads are JSON large
// enough to cross a TCP read boundary, so both can carry one. The pre-U8 rule
// ("any chunk containing IAC passes through whole") hid this by never splitting
// such a chunk at all.
func splitInputLines(b []byte, inSubneg bool) (parts [][]byte, stillInSubneg bool) {
	if len(b) == 0 {
		return nil, inSubneg
	}

	var out [][]byte

	start := 0
	i := 0

	if inSubneg {
		end, complete := subnegotiationEnd(b, 0)
		out = append(out, b[:end])
		if !complete {
			// The whole chunk is still payload. Emit it as one opaque segment
			// and stay resumed; nothing here may be split on.
			return splitResult(out), true
		}
		i, start = end, end
	}

	for i < len(b) {
		if b[i] == telnetIAC {
			if start < i {
				out = append(out, b[start:i])
			}

			end, complete := telnetSequenceEnd(b, i)
			out = append(out, b[i:end])
			if !complete {
				return splitResult(out), true
			}
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

	return splitResult(out), false
}

// splitResult collapses a single-segment result to nil. One segment means the
// chunk was never actually divided, and callers treat nil as "leave the read
// whole" -- returning a one-element slice instead would make every caller
// re-derive that.
func splitResult(out [][]byte) [][]byte {
	if len(out) < 2 {
		return nil
	}
	return out
}

// subnegotiationEnd scans from `from` for the IAC SE that closes a
// subnegotiation, honouring IAC IAC as an escaped literal 0xFF inside the
// payload. It returns the index just past the terminator and whether one was
// found; when it was not, the caller is still inside the subnegotiation and the
// rest of the buffer is payload.
func subnegotiationEnd(b []byte, from int) (end int, complete bool) {
	for i := from; i+1 < len(b); i++ {
		if b[i] != telnetIAC {
			continue
		}
		if b[i+1] == telnetIAC {
			i++ // escaped 0xFF in the payload, not a terminator
			continue
		}
		if b[i+1] == telnetSE {
			return i + 2, true
		}
	}
	return len(b), false
}

// telnetSequenceEnd returns the first byte after the IAC sequence beginning at
// start, and whether that sequence was COMPLETE within this buffer. An
// incomplete sequence consumes the remainder of the read and leaves the caller
// resumed, so the continuation chunk is not mistaken for typed text.
func telnetSequenceEnd(b []byte, start int) (end int, complete bool) {
	if start+1 >= len(b) {
		return len(b), false
	}

	switch b[start+1] {
	case telnetIAC:
		return start + 2, true
	case telnetWILL, telnetWONT, telnetDO, telnetDONT:
		if start+3 > len(b) {
			return len(b), false
		}
		return start + 3, true
	case telnetSB:
		// Subnegotiation payloads are the only sequences long enough to be
		// split across socket reads, and the only ones whose payload can carry
		// a byte that looks like an end-of-line.
		return subnegotiationEnd(b, start+2)
	default:
		return start + 2, true
	}
}

// queueSplitInput splits a freshly read chunk and stores everything after the
// first line for later Read calls. It returns the byte length of the first
// line when a split happened, and 0 when the chunk was left whole.
//
// The queued segments are copied. They alias the caller's read buffer, which is
// reused on the very next read.
// The mid-subnegotiation flag lives on the connection, under the same mutex as
// the read queue, because it is per-connection resume state that has to survive
// from one socket read to the next.
func (cd *ConnectionDetails) queueSplitInput(b []byte) int {
	cd.readQueueMu.Lock()
	resuming := cd.midSubnegotiation
	cd.readQueueMu.Unlock()

	parts, stillInSubneg := splitInputLines(b, resuming)

	cd.readQueueMu.Lock()
	cd.midSubnegotiation = stillInSubneg
	cd.readQueueMu.Unlock()

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
