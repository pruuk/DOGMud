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
		if got := splitInputLines(in); got != nil {
			t.Errorf("splitInputLines(%q) = %q, want nil (no split needed)", in, got)
		}
	}
}

func TestSplitInputLines_SplitsMultipleCommands(t *testing.T) {
	// The exact payload that produced "Eastwesteast not recognized" in the
	// 2026-08-14 playtest.
	got := splitInputLines([]byte("east\r\nwest\r\neast\r\n"))

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
			got := splitInputLines(tc.in)
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

// A telnet negotiation payload is binary and may legitimately contain 0x0A or
// 0x0D — NAWS reports window dimensions as raw bytes, so a terminal 10 columns
// wide embeds a literal LF. Splitting such a chunk would corrupt the
// negotiation, so chunks carrying IAC are passed through untouched.
func TestSplitInputLines_NeverSplitsChunksCarryingIAC(t *testing.T) {
	const iac = 0xFF

	naws := []byte{iac, 0xFA, 0x1F, 0x00, 0x0A, 0x00, 0x18, iac, 0xF0}
	if got := splitInputLines(naws); got != nil {
		t.Errorf("split a NAWS negotiation with a 0x0A height byte: %q", got)
	}

	glued := append(append([]byte{}, naws...), []byte("east\r\nwest\r\n")...)
	if got := splitInputLines(glued); got != nil {
		t.Errorf("split a chunk containing IAC: %q; negotiation bytes must pass through whole", got)
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
