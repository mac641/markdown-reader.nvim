package ipc

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func reader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestSequence(t *testing.T) {
	r := reader("CONTENT 7\n# hi\nyo\n" +
		"FILE 10\n/a b/ü.md\n" +
		"SCROLL 12 40\r\n" +
		"\n" +
		"SCROLL 3\n" +
		"QUIT\n")

	want := []Message{
		{Kind: Content, Text: []byte("# hi\nyo")},
		{Kind: File, Path: "/a b/ü.md"},
		{Kind: Scroll, Line: 12, Percent: 40},
		{Kind: Scroll, Line: 3},
		{Kind: Quit},
	}
	for i, w := range want {
		m, err := Read(r)
		if err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
		if m.Kind != w.Kind || string(m.Text) != string(w.Text) || m.Path != w.Path || m.Line != w.Line || m.Percent != w.Percent {
			t.Errorf("message %d: got %+v, want %+v", i, m, w)
		}
	}
	if _, err := Read(r); !errors.Is(err, io.EOF) {
		t.Errorf("at end: got %v, want io.EOF", err)
	}
}

func TestContentMayContainAnything(t *testing.T) {
	r := reader("CONTENT 0\nCONTENT 5\nQUIT\n")
	if m, err := Read(r); err != nil || m.Kind != Content || len(m.Text) != 0 {
		t.Errorf("empty content: %+v, %v", m, err)
	}
	if m, err := Read(r); err != nil || m.Kind != Content || string(m.Text) != "QUIT\n" {
		t.Errorf("content QUIT: %+v, %v", m, err)
	}
}

func TestErrors(t *testing.T) {
	for _, in := range []string{
		"BOGUS\n",
		"CONTENT\n",
		"CONTENT 10\nshort",
		"CONTENT -1\n",
		"CONTENT +1\n",
		"CONTENT 99999999999999999999\n",
		"CONTENT 5x\n",
		"FILE 0\n",
		"FILE 3\na\x00b",
		"FILE 4097\n",
		"FILE 5\n/a",
		"SCROLL\n",
		"SCROLL 1 101\n",
		"SCROLL 1 2 3\n",
		"SCROLL 1.5\n",
		"QUIT now\n",
		"QUIT", // unterminated
		"CONTENT " + strings.Repeat("1", 70) + "\n",
	} {
		_, err := Read(reader(in))
		var pe *ProtocolError
		if !errors.As(err, &pe) {
			t.Errorf("%q: got %v, want a protocol error", in, err)
		}
	}
}
