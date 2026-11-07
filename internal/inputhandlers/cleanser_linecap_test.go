package inputhandlers

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/connections"
)

func TestCleanser_LineStopsGrowingAtTheCap(t *testing.T) {
	ci := &connections.ClientInput{}
	chunk := bytes.Repeat([]byte{'a'}, connections.ReadBufferSize)

	for sent := 0; sent < connections.MaxInputLineBytes*2; sent += len(chunk) {
		ci.DataIn = append(ci.DataIn[:0], chunk...)
		CleanserInputHandler(ci, nil)
	}

	if len(ci.Buffer) != connections.MaxInputLineBytes {
		t.Fatalf("line is %d bytes, want the cap, %d", len(ci.Buffer), connections.MaxInputLineBytes)
	}
	if len(ci.DataIn) != 0 {
		t.Errorf("%d bytes past the cap stayed in DataIn, and the echo handler would print them", len(ci.DataIn))
	}
}

func TestCleanser_CapNeverSplitsACharacter(t *testing.T) {
	ci := &connections.ClientInput{Buffer: bytes.Repeat([]byte{'a'}, connections.MaxInputLineBytes-1)}
	ci.DataIn = []byte("€€") // 3 bytes each, and only 1 byte of room

	CleanserInputHandler(ci, nil)

	if !utf8.Valid(ci.Buffer) {
		t.Fatal("the cap cut a character in half")
	}
	if len(ci.Buffer) != connections.MaxInputLineBytes-1 {
		t.Errorf("line is %d bytes, want %d: a character with no room is dropped whole", len(ci.Buffer), connections.MaxInputLineBytes-1)
	}
}

func TestCleanser_EnterStillSubmitsACappedLine(t *testing.T) {
	ci := &connections.ClientInput{Buffer: bytes.Repeat([]byte{'a'}, connections.MaxInputLineBytes)}
	ci.DataIn = []byte("more\r")

	CleanserInputHandler(ci, nil)

	if !ci.EnterPressed {
		t.Error("Enter at the cap was lost, so the player could never send or clear the line")
	}
	if len(ci.Buffer) != connections.MaxInputLineBytes {
		t.Errorf("line is %d bytes, want the cap", len(ci.Buffer))
	}
}

func TestCleanser_BackspaceWorksAtTheCap(t *testing.T) {
	ci := &connections.ClientInput{Buffer: bytes.Repeat([]byte{'a'}, connections.MaxInputLineBytes)}
	ci.DataIn = []byte{0x7f}

	CleanserInputHandler(ci, nil)

	if len(ci.Buffer) != connections.MaxInputLineBytes-1 {
		t.Errorf("line is %d bytes after a backspace, want %d", len(ci.Buffer), connections.MaxInputLineBytes-1)
	}
}

func TestCleanser_AnOrdinaryLineIsUntouched(t *testing.T) {
	ci := &connections.ClientInput{}
	ci.DataIn = []byte("say hello there")

	CleanserInputHandler(ci, nil)

	if got := string(ci.Buffer); got != "say hello there" {
		t.Errorf("Buffer = %q", got)
	}
}
