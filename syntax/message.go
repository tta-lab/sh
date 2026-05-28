// Copyright (c) 2026, Neil Chen
// See LICENSE for licensing information

package syntax

import (
	"fmt"
	"strings"
)

type MessageBlock struct {
	Mpos   Pos
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

// Incomplete reports whether the error was caused by an unterminated
// message block (e.g. missing closing quote or delimiter).
func (e MessageBlockError) Incomplete() bool {
	return strings.Contains(e.Message, "unterminated")
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
	msgHeredoc
)

// ScanMsgBlocks extracts Lenos message blocks from shell source.
// baseOffset is the byte offset of src in the original source (0 if src is
// the complete source).
//
// It returns extracted message blocks and a cleaned copy of source where
// message block spans are replaced with whitespace (preserving newlines).
// Callers should parse the cleaned bash with syntax.NewParser().Parse.
func ScanMsgBlocks(src []byte, baseOffset uint) (blocks []*MessageBlock, clean []byte, err error) {
	if len(src) == 0 {
		return nil, src, nil
	}
	clean = make([]byte, len(src))
	copy(clean, src)

	var ctx msgCtxState
	var depth int
	var kwDepth int // keyword-body depth (then/do → ++, fi/done → --)
	heredocDelim := ""
	heredocStripTabs := false
	var line, col uint = 1, 1
	startOk := true

	for i := 0; i < len(src); {
		b := src[i]

		if b == '\n' {
			line++
			col = 1
			if heredocDelim != "" && matchHeredocDelim(src, i+1, heredocDelim, heredocStripTabs) {
				delimLen := len(heredocDelim)
				heredocDelim = ""
				heredocStripTabs = false
				ctx = msgTop
				i += 1 + delimLen + 1 // newline + delimiter + trailing newline
				startOk = true
				continue
			}
			startOk = true
			i++
			continue
		} else {
			col++
		}

		if ctx == msgHeredoc {
			i++
			continue
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
			case '(':
				depth++
				startOk = false
			case ')':
				if depth > 0 {
					depth--
				}
				startOk = false
			case '{':
				depth++
				startOk = false
			case '}':
				if depth > 0 {
					depth--
				}
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
			case '\n':
				startOk = true
			case ';', '&':
				startOk = true // end of statement
			case '<':
				if i+1 < len(src) && src[i+1] == '<' {
					j := i + 2
					stripTabs := false
					if j < len(src) && src[j] == '-' {
						stripTabs = true
						j++
					}
					delimStart := j
					for j < len(src) && src[j] != '\n' && src[j] != ' ' && src[j] != '\t' {
						j++
					}
					if j > delimStart {
						d := string(src[delimStart:j])
						if len(d) >= 2 && (d[0] == '"' || d[0] == '\'') && d[len(d)-1] == d[0] {
							d = d[1 : len(d)-1]
						}
						heredocDelim = d
						heredocStripTabs = stripTabs
						ctx = msgHeredoc
						i = j
						startOk = false
						continue
					}
				}
				startOk = false
			case ' ', '\t', '\r':
			default:
				// Track keyword-delimited bodies (if/for/while).
				if startOk && kwDepth >= 0 {
					switch {
					case matchWordAt(src, i, "then"):
						kwDepth++
					case matchWordAt(src, i, "do"):
						if !matchWordAt(src, i, "done") {
							kwDepth++
						}
					case matchWordAt(src, i, "done"):
						if kwDepth > 0 {
							kwDepth--
						}
					case matchWordAt(src, i, "fi"):
						if kwDepth > 0 {
							kwDepth--
						}
					case matchWordAt(src, i, "elif"):
						if kwDepth > 0 {
							kwDepth--
						}
					}
				}
				if b == 'm' && startOk && depth == 0 && kwDepth == 0 {
					offset := baseOffset + uint(i)
					block, consumed, mErr := TryParseMsgBlock(src[i:], offset, line, col-1)
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

// wordEnd reports whether src[pos] is a word boundary (non-alphanumeric,
// non-underscore, or EOF).
func wordEnd(src []byte, pos int) bool {
	if pos >= len(src) {
		return true
	}
	b := src[pos]
	return !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_')
}

// matchWordAt reports whether the word s appears at src[pos:] with a word
// boundary before and after.
func matchWordAt(src []byte, pos int, s string) bool {
	end := pos + len(s)
	if end > len(src) {
		return false
	}
	if string(src[pos:end]) != s {
		return false
	}
	return wordEnd(src, end)
}

func matchHeredocDelim(src []byte, pos int, delim string, stripTabs bool) bool {
	p := pos
	if stripTabs {
		for p < len(src) && src[p] == '\t' {
			p++
		}
	}
	end := p + len(delim)
	if end > len(src) {
		return false
	}
	if string(src[p:end]) != delim {
		return false
	}
	return end >= len(src) || src[end] == '\n'
}

// TryParseMsgBlock attempts to parse a message block at src[0].
// src[0] must be 'm'. offset is the absolute byte offset of src[0] in the
// original source. Returns (block, bytesConsumed, nil) on success,
// (nil, 0, nil) if src doesn't look like a message block,
// (nil, 0, error) on malformed input.
func TryParseMsgBlock(src []byte, offset uint, line, col uint) (*MessageBlock, int, error) {
	if len(src) == 0 || src[0] != 'm' {
		return nil, 0, nil
	}
	mpos := NewPos(offset, line, col)
	i := 1
	curLine, curCol := line, col+1
	curOff := offset + 1

	target := ""
	if i < len(src) && src[i] == '(' {
		i++
		curOff++
		curCol++
		targetStart := i
		for i < len(src) && src[i] != ')' && src[i] != '\n' {
			b := src[i]
			if !isTargetChar(b) {
				return nil, 0, MessageBlockError{
					Pos:     NewPos(curOff, curLine, curCol),
					Message: fmt.Sprintf("invalid target character %q in message block", rune(b)),
				}
			}
			i++
			curOff++
			curCol++
		}
		if i >= len(src) || src[i] == '\n' {
			return nil, 0, nil
		}
		if i == targetStart {
			return nil, 0, nil
		}
		target = string(src[targetStart:i])
		i++
		curOff++
		curCol++
	}

	hashes := 0
	for i < len(src) && src[i] == '#' {
		hashes++
		i++
		curOff++
		curCol++
	}

	if i >= len(src) || src[i] != '"' {
		return nil, 0, nil
	}
	quotePos := NewPos(curOff, curLine, curCol)
	i++
	curOff++
	curCol++

	bodyStart := i
	for {
		if i >= len(src) {
			return nil, 0, MessageBlockError{
				Pos:     NewPos(curOff, curLine, curCol),
				Message: "unterminated message block",
			}
		}
		if hashes == 0 {
			if src[i] == '"' {
				break
			}
			if src[i] == '\\' && i+1 < len(src) {
				i += 2
				curOff += 2
				curCol += 2
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
		if src[i] == '\n' {
			curLine++
			curCol = 1
		} else {
			curCol++
		}
		curOff++
		i++
	}

	body := string(src[bodyStart:i])
	rquotePos := NewPos(curOff, curLine, curCol)
	i++
	curOff++
	curCol++

	for h := 0; h < hashes; h++ {
		if i >= len(src) || src[i] != '#' {
			return nil, 0, MessageBlockError{
				Pos:     NewPos(curOff, curLine, curCol),
				Message: fmt.Sprintf("mismatched hash delimiter: expected %d '#' after closing quote", hashes),
			}
		}
		i++
		curOff++
		curCol++
	}

	endPos := NewPos(curOff, curLine, curCol)
	return &MessageBlock{
		Mpos:   mpos,
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
