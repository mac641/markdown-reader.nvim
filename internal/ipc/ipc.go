// Package ipc reads the messages the editor sends to mdreader --stdin.
//
// Each message starts with a header line:
//
//	CONTENT <nbytes>\n<nbytes of Markdown>   replace the document
//	SCROLL <line> [<percent>]\n              show source line <line> (1-based)
//	                                         <percent> of the window height
//	                                         from the top
//	QUIT\n                                   close the viewer
//
// Numbers are decimal integers. Blank lines are ignored, and \r\n line
// endings are accepted. End of input closes the viewer too.
package ipc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// MaxContent is the largest document accepted, in bytes.
const MaxContent = 256 << 20

// maxHeader bounds a header line, including its newline, so that garbage
// on stdin is reported instead of buffered.
const maxHeader = 64

type Kind int

const (
	Content Kind = iota
	Scroll
	Quit
)

type Message struct {
	Kind    Kind
	Text    []byte // Content: the Markdown
	Line    int    // Scroll
	Percent int    // Scroll, 0-100
}

// ProtocolError reports input that doesn't follow the protocol.
type ProtocolError struct{ Msg string }

func (e *ProtocolError) Error() string { return e.Msg }

func protocolErr(format string, args ...any) error {
	return &ProtocolError{fmt.Sprintf(format, args...)}
}

// Read blocks until the next message has been read. It returns io.EOF at
// the end of the input, a *ProtocolError for malformed input, or the
// underlying read error.
func Read(r *bufio.Reader) (Message, error) {
	var header string
	for header == "" {
		var err error
		if header, err = readHeader(r); err != nil {
			return Message{}, err
		}
	}

	cmd, args, _ := strings.Cut(header, " ")
	switch cmd {
	case "CONTENT":
		return readContent(r, args)
	case "SCROLL":
		return parseScroll(args)
	case "QUIT":
		if args != "" {
			break
		}
		return Message{Kind: Quit}, nil
	}
	return Message{}, protocolErr("unknown message: %.40q", header)
}

// readHeader returns the next line without its line ending.
func readHeader(r *bufio.Reader) (string, error) {
	var line []byte
	for {
		c, err := r.ReadByte()
		if err == io.EOF && len(line) > 0 {
			return "", protocolErr("unterminated header line")
		}
		if err != nil {
			return "", err
		}
		if c == '\n' {
			return string(bytes.TrimSuffix(line, []byte("\r"))), nil
		}
		if len(line) == maxHeader-1 {
			return "", protocolErr("header line too long")
		}
		line = append(line, c)
	}
}

func readContent(r *bufio.Reader, args string) (Message, error) {
	n, err := parseUint(args, MaxContent)
	if err != nil {
		return Message{}, protocolErr("bad CONTENT size")
	}
	text := make([]byte, n)
	if _, err := io.ReadFull(r, text); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Message{}, protocolErr("CONTENT truncated")
		}
		return Message{}, err
	}
	return Message{Kind: Content, Text: text}, nil
}

func parseScroll(args string) (Message, error) {
	fields := strings.Fields(args)
	bad := protocolErr("bad SCROLL arguments")
	if len(fields) < 1 || len(fields) > 2 {
		return Message{}, bad
	}
	line, err := parseUint(fields[0], 1<<32-1)
	if err != nil {
		return Message{}, bad
	}
	percent := 0
	if len(fields) == 2 {
		if percent, err = parseUint(fields[1], 100); err != nil {
			return Message{}, bad
		}
	}
	return Message{Kind: Scroll, Line: line, Percent: percent}, nil
}

// parseUint accepts plain decimal digits only: no sign, no spaces.
func parseUint(s string, max uint64) (int, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v > max {
		return 0, errors.New("out of range")
	}
	return int(v), nil
}
