// Copyright (c) 2026, Neil Chen
// See LICENSE for licensing information

package syntax

import (
	"fmt"
	"strings"
)

// MessageBlock represents a Lenos top-level message block.
//
// Message blocks are recognized only at the top level of a shell file,
// outside strings, comments, heredocs, command substitutions, and
// control-flow bodies.
//
// Valid forms:
//
//	m"Done."
//	m#"body with "quotes"#"
//	m##"body with "# delimiter"##
//	m(neil)"addressed message"
//	m(neil)#"addressed with hashes"#
type MessageBlock struct {
	Mpos  Pos
	Target string
	Hashes int
	Quote  Pos
	Body   string
	Rquote Pos
	EndPos Pos
}

func (m *MessageBlock) Pos() Pos { return m.Mpos }
func (m *MessageBlock) End() Pos {
	if m.EndPos.IsValid() {
		return m.EndPos
	}
	return posAddCol(m.Rquote, 1+m.Hashes)
}

type MessageBlockError struct {
	Pos     Pos
	Message string
}

func (e MessageBlockError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Pos.Line(), e.Pos.Col(), e.Message)
}

type msgCtxState int

const (
	msgTop msgCtxState = iota
	msgSglQuote
	msgDblQuote
	msgComment
	msgBckQuote
	msgDollarSingle
	msgDollarDouble
)

// scanMsgBlocks extracts Lenos message blocks from source.
func scanMsgBlocks(src []byte) (blocks []*MessageBlock, clean []byte, err error) {
	if len(src) == 0 {
		return nil, src, nil
	}
	clean = make([]byte, len(src))
	copy(clean, src)

	var ctx msgCtxState
	var line, col uint = 1, 1
	startOk := true

	for i := 0; i < len(src); {
		b := src[i]
		if b == '\n' {
			line++
			col = 1
			startOk = true
		} else {
			col++
		}

		switch ctx {
		case msgTop:
			switch b {
			case '\'':
				ctx = msgSglQuote
				startOk = false
			case '"':
				ctx = msgDblQuote
				startOk = false
			case '`':
				ctx = msgBckQuote
				startOk = false
			case '#':
				ctx = msgComment
				startOk = false
			case '$':
				if i+1 < len(src) {
					switch src[i+1] {
					case '\'':
						ctx = msgDollarSingle
						i += 2
						continue
					case '"':
						ctx = msgDollarDouble
						i += 2
						continue
					default:
						startOk = false
					}
				} else {
					startOk = false
				}
			case ';', '&', '|', '\n':
				startOk = true
			default:
				if b == 'm' && startOk {
					block, consumed, mErr := tryParseMsgBlock(src[i:], line, col-1)
					if mErr != nil {
						return blocks, clean, mErr
					}
					if block != nil {
						blocks = append(blocks, block)
						for j := 0; j < consumed; j++ {
							if src[i+j] != '\n' {
								clean[i+j] = ' '
							}
						}
						for j := 0; j < consumed; j++ {
							if src[i+j] == '\n' {
								line++
								col = 1
							} else {
								col++
							}
						}
						i += consumed
						startOk = false
						continue
					}
				}
				startOk = false
			}

		case msgSglQuote:
			if b == '\'' {
				ctx = msgTop
			}

		case msgDblQuote:
			switch b {
			case '"':
				ctx = msgTop
			case '\\':
				i++
			}

		case msgComment:
			if b == '\n' {
				ctx = msgTop
				startOk = true
			}

		case msgBckQuote:
			switch b {
			case '`':
				ctx = msgTop
			case '\\':
				i++
			}

		case msgDollarSingle:
			switch b {
			case '\'':
				ctx = msgTop
			case '\\':
				i++
			}

		case msgDollarDouble:
			switch b {
			case '"':
				ctx = msgTop
			case '\\':
				i++
			}
		}
		i++
	}
	return blocks, clean, nil
}

// tryParseMsgBlock attempts to parse a message block at src[0].
func tryParseMsgBlock(src []byte, line, col uint) (*MessageBlock, int, error) {
	if len(src) == 0 || src[0] != 'm' {
		return nil, 0, nil
	}
	i := 1

	target := ""
	if i < len(src) && src[i] == '(' {
		i++
		targetStart := i
		for i < len(src) && src[i] != ')' && src[i] != '\n' {
			b := src[i]
			if !isTargetChar(b) {
				return nil, 0, MessageBlockError{
					Pos:     NewPos(0, line, col+uint(i)),
					Message: fmt.Sprintf("invalid target character %q in message block", rune(b)),
				}
			}
			i++
		}
		if i >= len(src) || src[i] == '\n' {
			return nil, 0, nil
		}
		if i == targetStart {
			return nil, 0, nil
		}
		target = string(src[targetStart:i])
		i++
	}

	hashes := 0
	for i < len(src) && src[i] == '#' {
		hashes++
		i++
	}

	if i >= len(src) || src[i] != '"' {
		return nil, 0, nil
	}
	quotePos := NewPos(0, line, col+uint(i))
	i++

	bodyStart := i
	for {
		if i >= len(src) {
			return nil, 0, MessageBlockError{
				Pos:     NewPos(uint(len(src)), line, col),
				Message: "unterminated message block",
			}
		}
		if hashes == 0 {
			if src[i] == '"' {
				break
			}
			if src[i] == '\\' && i+1 < len(src) {
				i += 2
				continue
			}
		} else {
			if src[i] == '"' {
				j := 1
				for j <= hashes && i+j < len(src) {
					if src[i+j] != '#' {
						break
					}
					j++
				}
				if j == hashes+1 {
					if i+j >= len(src) || src[i+j] != '#' {
						break
					}
				}
			}
		}
		i++
	}

	body := string(src[bodyStart:i])
	rquotePos := NewPos(0, line, col+uint(i))
	i++

	for h := 0; h < hashes; h++ {
		if i >= len(src) || src[i] != '#' {
			return nil, 0, MessageBlockError{
				Pos:     NewPos(0, line, col+uint(i)),
				Message: fmt.Sprintf("mismatched hash delimiter: expected %d '#' after closing quote", hashes),
			}
		}
		i++
	}

	endPos := NewPos(0, line, col+uint(i))
	return &MessageBlock{
		Mpos:   NewPos(uint(len(src)), line, col),
		Target: target,
		Hashes: hashes,
		Quote:  quotePos,
		Body:   body,
		Rquote: rquotePos,
		EndPos: endPos,
	}, i, nil
}

func isTargetChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

// EscapeMessageBlock returns the Lenos message block syntax for the given body.
func EscapeMessageBlock(body string) string {
	if !strings.Contains(body, `"`) {
		return "m\"" + body + "\""
	}
	for n := 1; ; n++ {
		delim := "\"" + strings.Repeat("#", n)
		if !strings.Contains(body, delim) {
			return "m" + strings.Repeat("#", n) + "\"" + body + "\"" + strings.Repeat("#", n)
		}
	}
}
