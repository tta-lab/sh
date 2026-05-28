# AGENTS.md — mvdan.cc/sh

Shell parser, formatter, and interpreter written in pure Go. Supports POSIX Shell, Bash, Zsh, and mksh. Module path: `mvdan.cc/sh/v3`.

## Commands

```sh
go test ./...                         # all tests
cd moreinterp && go test ./...        # moreinterp sub-module (has its own go.mod)
go test -race ./...                   # race detector
go test -run=- -fuzz=ParsePrint ./syntax  # fuzz the parser
go vet ./...                          # static analysis
gofmt -s -d .                         # format check (CI asserts clean diff)
```

No Makefile. No build step — it's a library. The two CLIs are `cmd/shfmt` and `cmd/gosh`.

## Architecture

Six public packages, each with a focused API surface:

| Package | Role |
|---------|------|
| `syntax` | Parse → AST (`*File`) → Print. The core. |
| `expand` | Shell variable/parameter expansion. Used by `interp`. |
| `pattern` | Glob/extglob matching. Used by `expand` and `interp`. |
| `interp` | Interpret/execute parsed ASTs. Depends on `expand`, `pattern`, `syntax`. |
| `shell` | High-level helpers (e.g. expand a string as if in a shell). Thin wrapper. |
| `fileutil` | Detect shell files via shebang/extension. Used by `shfmt`. |

`internal/` holds shared helpers (braces, environ, param expansion) used across packages.

`moreinterp/` is a **separate Go module** (`mvdan.cc/sh/moreinterp`) that provides optional `interp.ExecHandlerFunc` middlewares. It has its own `go.mod`/`go.sum` and must be tested separately.

### Data flow

1. `syntax.NewParser().Parse(io.Reader, name)` → `*syntax.File` (AST)
2. `syntax.Walk(node, visitor)` — depth-first traversal, visitor returns `true` to recurse
3. `syntax.NewPrinter().Print(io.Writer, node)` — serialize AST back to shell
4. `interp.New(opts...).Run(ctx, *syntax.File)` — execute

### Key AST types (`syntax/nodes.go`)

`Node` is the root interface (Pos/End). Concrete types: `*File`, `*Stmt`, `*CallExpr`, `*FuncDecl`, `*IfClause`, `*ForClause`, `*CaseClause`, `*BinaryCmd`, `*Redirect`, `*Assign`, `*Word`, `*Lit`, `*DblQuoted`, `*SglQuoted`, `*ParamExp`, `*ArithmExp`, `*CmdSubst`, etc.

### Interpreter design (`interp/`)

- `Runner` is the interpreter. Created via `interp.New(opts...)`, reused via `r.Reset()`.
- Options: `Env`, `Dir`, `Params`, `StdIO`, `ExecHandlers`, `OpenHandler`, `CallHandler`, etc.
- `ExecHandlers` are middlewares — chain from first to last, each can call `next` or short-circuit.
- `HandlerCtx(ctx)` gives access to `HandlerContext` (Env, Dir, Stdin/Stdout/Stderr, Pos) inside handlers.
- Subshells: `r.subshell()` copies the runner; background processes get synthetic PIDs with a `g` prefix.
- `ExitStatus` is both an error type and exit code carrier — use `errors.As` to extract.

### Parser variants

`syntax.LangVariant` is a bitmask: `LangBash`, `LangPOSIX`, `LangMirBSDKorn`, `LangBats`, `LangZsh`, `LangAuto`. Set via `syntax.Variant(v)(parser)`.

## Code generation

```sh
cd syntax && go generate
```

This runs two generators:
- `stringer` for `token` type
- `gen_token_parse.go` (build tag `//go:build ignore`) which emits `tokens_parse.go` with `encoding.TextUnmarshaler` implementations per operator type declared in `tokens.go`

## Testing patterns

Tests use table-driven style with helper functions (`lit()`, `word()`, `litWord()`, `call()`, `stmt()`, etc.) defined in `filetests_test.go` to construct AST nodes concisely.

`interp_test.go` has `runTests` — a table of `{src string, want string}` pairs that parse shell code and compare interpreter output. Timeout is `runnerRunTimeout = 5s`.

The CI matrix tests Go 1.25.x and 1.26.x on Linux, macOS, and Windows. On Ubuntu it also runs `TestRunnerRunConfirm` against real Bash 5.2 via Docker (`dockexec`).

Fuzz targets live in `syntax/fuzz_test.go` (e.g. `FuzzParsePrint`).

## Gotchas

- **`moreinterp` is a separate module.** `cd moreinterp && go test ./...` — don't assume `go test ./...` covers it.
- **`.gitattributes` disables text normalization** (`* -text`). Don't add CRLF line endings.
- **Subshells use goroutines, not fork.** Go can't fork, so `(*Runner).subshell()` uses a goroutine. This means real PIDs and file descriptors can't be used directly — background process PIDs are synthetic (`g`-prefixed).
- **`$((` and `((` ambiguity is unsupported.** The parser can't backtrack. Use `$((` explicitly or space operands as POSIX recommends.
- **`export`, `let`, `declare` are parsed as keywords**, not simple commands — they have dedicated AST nodes (`DeclClause`, `LetClause`).
- **`Pos` packs line, col, and offset into two `uint32` fields.** Invalid positions use reserved high values (`offsetRecovered`, `offsetMax`). Don't construct `Pos` manually.
- **`KeepPadding` printer option is deprecated** — it's flawed and buggy.
- **`ExecHandler` (singular) is deprecated** — use `ExecHandlers` (plural, middleware pattern).
- **`ReadDirHandler` is deprecated** — use `ReadDirHandler2` (takes `fs.DirEntry`).
- **`NewExitStatus` and `IsExitStatus` are deprecated** — use `ExitStatus` directly with `errors.As`.
- **`shfmt` reads EditorConfig by default.** Explicit parser/printer flags disable EditorConfig integration. Use `--apply-ignore` to force ignore rules.
- **`shfmt -ln=auto`** (default) detects language from filename extension, then shebang, then falls back to Bash.
