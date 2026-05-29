# AGENTS.md — lensh

**lensh** — shell parser for Lenos. Fork of `mvdan.cc/sh/v3`.

**Why we fork:** Lenos needs the `syntax` package to parse bash and add `m"..."` / `m#"..."#` / `m##"..."##` message block support. We don't use `interp` (bash execution happens in Temenos sandbox), `shfmt`, `gosh`, `expand`, `pattern`, `shell`, or `moreinterp`. Those exist upstream and may be removed from this fork later.

## Commands

```sh
go test ./...                              # all tests (still needed until unused packages are removed)
cd moreinterp && go test ./...             # moreinterp has its own go.mod
go test -run=- -fuzz=ParsePrint ./syntax   # fuzz the parser
go vet ./...                               # static analysis
gofmt -s -d .                              # format check (CI asserts clean diff)
```

## What we use: `syntax/`

The parser and AST. This is the only package Lenos touches.

**Parse flow:**
1. `syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(io.Reader, name)` → `*syntax.File` (AST)
2. Walk/inspect the AST via `syntax.Walk(node, visitor)` or type-switch on `syntax.Command`
3. Message blocks (Lenos extension, TASKS1) will be top-level nodes extracted before runtime

**Key AST types** (`syntax/nodes.go`):
- `Node` — root interface, has `Pos()` and `End()`
- `*File` → `[]*Stmt` → each `Stmt` has a `Cmd` (one of `Command` interface types)
- `Command` interface: `*CallExpr`, `*FuncDecl`, `*IfClause`, `*ForClause`, `*CaseClause`, `*BinaryCmd`, `*Subshell`, `*Block`, `*DeclClause`, `*LetClause`, `*TimeClause`, `*CoprocClause`, `*TestDecl`
- `*Assign`, `*Redirect`, `*Word`, `*Lit`, `*DblQuoted`, `*SglQuoted`, `*ParamExp`, `*ArithmExp`, `*CmdSubst` — nested structures

**Source spans** (`syntax/nodes.go:Pos`):
- `Pos` packs offset, line, and column into two `uint32` fields
- `node.Pos()` / `node.End()` give source positions — use for span-based removal of message blocks
- Invalid positions use reserved high values (`offsetRecovered`, `offsetMax`). Don't construct `Pos` manually.

**Walk** (`syntax/walk.go`):
- `Walk(node, func(Node) bool)` — depth-first, visitor returns `true` to recurse into children
- Type-switch on the node to handle each kind

**Code generation** (`syntax/`):
- `cd syntax && go generate` runs `stringer` for `token` and `gen_token_parse.go` → `tokens_parse.go`
- `gen_token_parse.go` has `//go:build ignore` — it's not compiled into the module

## What we don't use (may remove later)

| Package | Purpose | Why unused |
|---------|---------|------------|
| `interp` | Shell interpreter | Bash runs in Temenos sandbox |
| `expand` | Variable/parameter expansion | Only needed by interp |
| `pattern` | Glob/extglob matching | Only needed by expand/interp |
| `shell` | High-level shell helpers | Thin wrapper over expand |
| `cmd/shfmt` | Shell formatter CLI | Not needed |
| `cmd/gosh` | Proof-of-concept shell | Not needed |
| `moreinterp/` | Coreutils middleware for interp | Not needed (separate go.mod) |
| `syntax/typedjson/` | JSON serialization of ASTs | Not needed |
| `fileutil` | Shebang/extension detection | Not needed |

Non-Bash `LangVariant`s (`LangPOSIX`, `LangMirBSDKorn`, `LangBats`, `LangZsh`) exist in the parser but are unused. `LangAuto` panics if passed to `Variant()`.

## Upstream tracking

Module path: `mvdan.cc/sh/v3`. Upstream: `github.com/mvdan/sh`. Keep the fork delta narrow — our changes are in `syntax/` for message block support. Track upstream releases for security/bugfix backports.

See `docs/lensh-delta.md` for a detailed record of the fork changes.

## Gotchas

- **`.gitattributes` disables text normalization** (`* -text`). Don't add CRLF line endings.
- **`$((` and `((` ambiguity is unsupported.** Parser can't backtrack.
- **`export`, `let`, `declare` are keywords**, not simple commands — dedicated AST nodes (`DeclClause`, `LetClause`).
- **Fuzz targets** in `syntax/fuzz_test.go` (e.g. `FuzzParsePrint`) — run before major parser changes.
