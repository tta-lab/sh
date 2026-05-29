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

## Protocol scope

Lenos message blocks are an agent protocol, not a general-purpose Bash
extension. The supported shape is intentionally narrow:

- `m` must be the first non-whitespace token on a physical line.
- The line must be top-level source, not inside a heredoc body, quote,
  comment, function body, subshell, command substitution, or keyword body.
- Same-line statement separators are outside the protocol contract. Agents
  should emit a new physical line before every message block. A message-block
  form found at a same-line statement boundary is a protocol error, not a
  silently ignored shell command.

Supported:

```sh
echo "working"
m"Done."

  m"Indented top-level message."
```

Unsupported:

```sh
echo "working"; m"Done."
cat <<EOF; m"Same physical line as heredoc setup."
body
EOF
```

The scanner reports same-line message-block forms as errors so agents can
repair the response instead of accidentally losing prose.

## Top-level enforcement

Message blocks are recognized at statement start when both `depth`
(brace/paren nesting) and `kwDepth` (keyword-body depth) are zero. The protocol
contract is stricter than Bash statement-start grammar: message blocks should
start their own physical line.

**Recognized contexts (block extracted):**
- Top-level physical lines
- After closing `}` of a function
- Indented top-level lines

**Suppressed contexts (block NOT extracted):**
- Single quotes, double quotes, backticks, `$'...'`, `$"..."`, comments
- Heredoc bodies (normal `<<` and `<<-` with tab-indented delimiters)
- Command substitutions (`$(...)`, `` `...` ``)
- Subshells (`(...)`)
- Function bodies, brace blocks (`{...}`)
- If/for/while/case bodies (`then`/`do` → `fi`/`done` keyword tracking)

## Non-goals and unsupported shell corners

- Same-line message blocks after `;`, `&`, `|`, or heredoc setup are not part
  of the protocol. Use a new physical line. Message-block syntax after `;`,
  `&`, or heredoc setup is reported as an error.
- Full Bash heredoc grammar is not a goal. The scanner only needs to keep
  heredoc bodies opaque and preserve useful line/offset accounting for later
  diagnostics.
- `<<-` is supported only to avoid misreading tab-indented heredoc bodies as
  message blocks. It is not a protocol feature agents should prefer.
- Multiple pending heredocs on one command line are not a protocol target.
  Agents should use simple heredocs or separate commands when they need file
  writes.
- Message blocks embedded in command substitutions, process substitutions,
  arithmetic contexts, arrays, aliases, or dynamically generated shell are not
  recognized.
- No structured `SyntaxDiagnostic` types — those belong in the Lenos runtime,
  not the shell fork.

## Tracking upstream

Keep the fork delta narrow; all changes live in `syntax/`. Track upstream
releases for security/bug-fix backports. Upstream commits that don't touch
`syntax/` can be backported without conflict.
