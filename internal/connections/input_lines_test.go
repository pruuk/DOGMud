package connections

import (
	"bytes"
	"testing"
)

// splitInputLines is what stops a client that puts several commands in one TCP
// segment from having them merged into one nonsense command. See the comment on
// the function itself for the failure it prevents.

func TestSplitInputLines_SingleLineIsNotSplit(t *testing.T) {
	for _, in := range [][]byte{
		[]byte("east\r\n"),
		[]byte("east\n"),
		[]byte("east"), // no terminator yet: a partial line still being typed
		[]byte(""),
	} {
		if got, _ := splitInputLines(in, false); got != nil {
			t.Errorf("splitInputLines(%q) = %q, want nil (no split needed)", in, got)
		}
	}
}

func TestSplitInputLines_SplitsMultipleCommands(t *testing.T) {
	// The exact payload that produced "Eastwesteast not recognized" in the
	// 2026-08-14 playtest.
	got, _ := splitInputLines([]byte("east\r\nwest\r\neast\r\n"), false)

	want := [][]byte{
		[]byte("east\r\n"),
		[]byte("west\r\n"),
		[]byte("east\r\n"),
	}

	if len(got) != len(want) {
		t.Fatalf("got %d segments %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("segment %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitInputLines_TerminatorVariants(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want [][]byte
	}{
		{
			name: "bare LF, unix-style client",
			in:   []byte("north\nsouth\n"),
			want: [][]byte{[]byte("north\n"), []byte("south\n")},
		},
		{
			name: "bare CR, classic mac / some telnet clients",
			in:   []byte("north\rsouth\r"),
			want: [][]byte{[]byte("north\r"), []byte("south\r")},
		},
		{
			name: "CRLF is one terminator, not two",
			in:   []byte("north\r\nsouth\r\n"),
			want: [][]byte{[]byte("north\r\n"), []byte("south\r\n")},
		},
		{
			name: "trailing partial line is kept as its own segment",
			in:   []byte("north\r\nsou"),
			want: [][]byte{[]byte("north\r\n"), []byte("sou")},
		},
		{
			name: "blank line is a real command (bare enter), not dropped",
			in:   []byte("north\r\n\r\nsouth\r\n"),
			want: [][]byte{[]byte("north\r\n"), []byte("\r\n"), []byte("south\r\n")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := splitInputLines(tc.in, false)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d segments %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if !bytes.Equal(got[i], tc.want[i]) {
					t.Errorf("segment %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// A standalone telnet negotiation payload is binary and may legitimately
// contain 0x0A or 0x0D. It must remain untouched.
func TestSplitInputLines_PreservesStandaloneIACChunk(t *testing.T) {
	const iac = 0xFF

	naws := []byte{iac, 0xFA, 0x1F, 0x00, 0x0A, 0x00, 0x18, iac, 0xF0}
	if got, _ := splitInputLines(naws, false); got != nil {
		t.Errorf("split a NAWS negotiation with a 0x0A height byte: %q", got)
	}
}

func TestSplitInputLines_SeparatesTextAfterIACNegotiation(t *testing.T) {
	const iac = 0xFF

	gmcp := []byte{iac, 0xFA, 0xC9, 'C', 'o', 'r', 'e', '.', 'H', 'e', 'l', 'l', 'o', iac, 0xF0}
	glued := append(append([]byte{}, gmcp...), []byte("pt_early_abc123\r\n")...)

	got, _ := splitInputLines(glued, false)
	want := [][]byte{gmcp, []byte("pt_early_abc123\r\n")}
	if len(got) != len(want) {
		t.Fatalf("splitInputLines(glued negotiation + username) = %q, want %q", got, want)
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("segment %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestConnectionDetails_ReadQueueDrainsOneLinePerCall(t *testing.T) {
	cd := &ConnectionDetails{}
	cd.queueSplitInput([]byte("east\r\nwest\r\neast\r\n"))

	buf := make([]byte, 64)

	for _, want := range []string{"west\r\n", "east\r\n"} {
		n, ok := cd.nextQueuedInput(buf)
		if !ok {
			t.Fatalf("queue drained early, wanted %q", want)
		}
		if got := string(buf[:n]); got != want {
			t.Errorf("queued read = %q, want %q", got, want)
		}
	}

	if _, ok := cd.nextQueuedInput(buf); ok {
		t.Error("queue still reports data after being drained")
	}
}

// A subnegotiation split across two socket reads must not be split on a payload
// byte that happens to look like an end-of-line.
//
// Before the resume flag existed, the first chunk was emitted whole (correct),
// but the SECOND chunk did not begin with IAC, so it was scanned as ordinary
// typed text -- and the 0x0A inside it was treated as a line terminator,
// cutting the negotiation in half and delivering the pieces as two separate
// "commands". NAWS puts raw dimension bytes on the wire and GMCP payloads are
// long enough to cross a read boundary, so both can carry one.
func TestSplitInputLines_ResumesFragmentedSubnegotiation(t *testing.T) {
	const iac = 0xFF

	// IAC SB NAWS, then the payload is cut mid-flight.
	head := []byte{iac, 0xFA, 0x1F, 0x00}
	got, stillInSubneg := splitInputLines(head, false)
	if got != nil {
		t.Fatalf("incomplete subnegotiation head = %q, want nil (nothing to split)", got)
	}
	if !stillInSubneg {
		t.Fatal("an unterminated subnegotiation must leave the connection resumed")
	}

	// The continuation carries a 0x0A height byte and then closes.
	tail := []byte{0x0A, 0x00, 0x18, iac, 0xF0}
	got, stillInSubneg = splitInputLines(tail, true)
	if stillInSubneg {
		t.Error("IAC SE arrived, so the connection must no longer be resumed")
	}
	if got != nil {
		t.Errorf("split the continuation of a subnegotiation on its 0x0A payload byte: %q", got)
	}

	// Once closed, text after the terminator resumes normal line splitting.
	tailThenText := append(append([]byte{}, tail...), []byte("north\r\n")...)
	got, stillInSubneg = splitInputLines(tailThenText, true)
	if stillInSubneg {
		t.Error("terminator present, so the connection must not stay resumed")
	}
	want := [][]byte{tail, []byte("north\r\n")}
	if len(got) != len(want) {
		t.Fatalf("resumed split = %q, want %q", got, want)
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("segment %d = %q, want %q", i, got[i], want[i])
		}
	}
}
