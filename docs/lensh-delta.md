# lensh — Delta from upstream mvdan/sh

Upstream: https://github.com/mvdan/sh (module `mvdan.cc/sh/v3`)  
Fork: `/home/neil/code/projects/tta-lab/sh`

## Purpose

Add Lenos message block support (`m"..."`) to the shell parser so Lenos agents
can mix natural-language output with bash in a single emit.

## Files changed

| File | Change |
|------|--------|
| `syntax/nodes.go` | `File.Messages` field, `MessageBlock` type with `Mpos`, `Target`, `Hashes`, `Quote`, `Body`, `Rquote`, `EndPos` |
| `syntax/message.go` | `ScanMsgBlocks`, `TryParseMsgBlock`, `MessageBlockError`, `EscapeMessageBlock` |
| `syntax/message_test.go` | 38 test cases across 6 test functions |
| `syntax/parser.go` | Doc comment referencing `ScanMsgBlocks` for TASKS2 integration |

## Public API

```go
func ScanMsgBlocks(src []byte, baseOffset uint) ([]*MessageBlock, []byte, error)
func TryParseMsgBlock(src []byte, offset uint, line, col uint) (*MessageBlock, int, error)
func EscapeMessageBlock(body string) string
```

`MessageBlockError` implements `error` with `Line()`, `Col()`, and `Incomplete()`.

## Message block syntax

| Form | Example | Body extracted |
|------|---------|---------------|
| Plain | `m"hello"` | `hello` |
| Single hash | `m#"has "quotes""#` | `has "quotes` |
| Double hash | `m##"nested "# inner"##` | `nested "# inner` |
| Target | `m(neil)"hello"` | `hello` |
| Target + hash | `m(neil)#"hello"#` | `hello` |
| Multiline | `m"line 1\nline 2"` | `line 1\nline 2` |

Only `m` variant; `mc` is not supported.

## Top-level enforcement

Message blocks are only recognized at statement start (after newline, `;`, `&`)
when both `depth` (brace/paren nesting) and `kwDepth` (keyword-body depth) are zero.

**Recognized contexts (block extracted):**
- Top-level after `;`, `&`, newline
- After closing `}` of a function
- Indented top-level lines

**Suppressed contexts (block NOT extracted):**
- Single quotes, double quotes, backticks, `$'...'`, `$"..."`, comments
- Heredoc bodies (normal `<<` and `<<-` with tab-indented delimiters)
- Command substitutions (`$(...)`, `` `...` ``)
- Subshells (`(...)`)
- Function bodies, brace blocks (`{...}`)
- If/for/while/case bodies (`then`/`do` → `fi`/`done` keyword tracking)

## Known limitations

- A message block on the same line as a `<<HEREDOC` delimiter is not extracted.
  The scanner defers heredoc entry to the first newline; the rest of the line
  is consumed as part of the heredoc command.
- No structured `SyntaxDiagnostic` types — those belong in the Lenos runtime,
  not the shell fork.

## Tracking upstream

Keep the fork delta narrow; all changes live in `syntax/`. Track upstream
releases for security/bug-fix backports. Upstream commits that don't touch
`syntax/` can be backported without conflict.
