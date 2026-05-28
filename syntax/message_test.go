package syntax

import (
	"strings"
	"testing"
)

func TestScanMsgBlocks_Extraction(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantMsgs []string
		wantErr  string
	}{
		{
			name:     "simple",
			src:      `m"hello"`,
			wantMsgs: []string{"hello"},
		},
		{
			name:     "single hash delimiter",
			src:      `m#"body with "quotes"#`,
			wantMsgs: []string{`body with "quotes`},
		},
		{
			name:     "double hash delimiter",
			src:      `m##"body with "# inner"##`,
			wantMsgs: []string{`body with "# inner`},
		},
		{
			name:     "undecorated without hash delimiter",
			src:      `m"hello world"`,
			wantMsgs: []string{"hello world"},
		},
		{
			name:     "triple hash delimiter",
			src:      `m###"nested "## inside"###`,
			wantMsgs: []string{`nested "## inside`},
		},
		{
			name:     "with target",
			src:      `m(neil)"hello neil"`,
			wantMsgs: []string{"hello neil"},
		},
		{
			name:     "target with hashes",
			src:      `m(neil)##"hello ## neil"##`,
			wantMsgs: []string{"hello ## neil"},
		},
		{
			name:     "target with hyphen and underscore",
			src:      `m(my-agent)"msg"`,
			wantMsgs: []string{"msg"},
		},
		{
			name:     "multiline body",
			src:      "m\"line 1\nline 2\"",
			wantMsgs: []string{"line 1\nline 2"},
		},
		{
			name:     "multiple blocks",
			src:      "m\"first\"\nm\"second\"",
			wantMsgs: []string{"first", "second"},
		},
		{
			name:     "escaped quote inside body",
			src:      `m"escaped \" quote"`,
			wantMsgs: []string{`escaped \" quote`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, _, err := ScanMsgBlocks([]byte(tt.src), 0)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var bodies []string
			for _, m := range blocks {
				bodies = append(bodies, m.Body)
			}
			if !stringSlicesEqual(bodies, tt.wantMsgs) {
				t.Fatalf("messages:\n  got  %q\n  want %q", bodies, tt.wantMsgs)
			}
		})
	}
}

func TestScanMsgBlocks_Rejection(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "m in echo", src: `echo m"not a block"`},
		{name: "m in comment", src: "# m\"not a block\"\necho hello"},
		{name: "m in single quotes", src: `echo 'm"not a block"'`},
		{name: "m in double quotes", src: `echo "m\"not a block\""`},
		{name: "m in backticks", src: "echo `m\"not a block\"`"},
		{name: "m after pipe", src: "cat file | m\"not at start\""},
		{name: "m in dollar-single quotes", src: `echo $'m"not a block"'`},
		{name: "m in dollar-double quotes", src: `echo $"m\"not a block\""`},
		{name: "m in heredoc", src: "cat <<EOF\nm\"not a block\"\nEOF\necho done"},
		{name: "m in heredoc with dash", src: "cat <<-EOF\n\tm\"not a block\"\nEOF\necho done"},
		{name: "m in subshell", src: "( m\"not a block\" )"},
		{name: "m in function body", src: "f() {\n  m\"not a block\"\n}"},
		{name: "m in brace block", src: "{ m\"not a block\"; }"},
		{name: "m in command substitution", src: "echo $(m\"not a block\")"},
		{name: "m in if body", src: "if true; then\n  m\"not a block\"\nfi"},
		{name: "m in for body", src: "for x in 1; do\n  m\"not a block\"\ndone"},
		{name: "m in while body", src: "while true; do\n  m\"not a block\"\ndone"},
		{name: "m in else body", src: "if false; then true; else\n  m\"not a block\"\nfi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, _, err := ScanMsgBlocks([]byte(tt.src), 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(blocks) != 0 {
				t.Fatalf("expected 0 blocks, got %d: %v", len(blocks), blocks)
			}
		})
	}
}

func TestScanMsgBlocks_ValidTopLevel(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantBody string
	}{
		{name: "after semicolon", src: "echo done; m\"valid\"", wantBody: "valid"},
		{name: "after ampersand", src: "echo done & m\"valid\"", wantBody: "valid"},
		{name: "after newline after function", src: "f() { echo x; }\nm\"valid\"", wantBody: "valid"},
		{name: "indented", src: "  m\"valid\"", wantBody: "valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, _, err := ScanMsgBlocks([]byte(tt.src), 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(blocks) != 1 {
				t.Fatalf("expected 1 block, got %d", len(blocks))
			}
			if blocks[0].Body != tt.wantBody {
				t.Fatalf("body = %q, want %q", blocks[0].Body, tt.wantBody)
			}
		})
	}
}

func TestScanMsgBlocks_Errors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{name: "unterminated", src: `m"unterminated`, wantErr: "unterminated message block"},
		{name: "invalid target char", src: `m(in valid)"body"`, wantErr: "invalid target character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ScanMsgBlocks([]byte(tt.src), 0)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEscapeMessageBlock(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"hello", `m"hello"`},
		{`has "quotes"`, `m#"has "quotes""#`},
		{`has "# both`, `m##"has "# both"##`},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			got := EscapeMessageBlock(tt.body)
			if got != tt.want {
				t.Fatalf("EscapeMessageBlock(%q) = %q, want %q", tt.body, got, tt.want)
			}
			blocks, _, err := ScanMsgBlocks([]byte(got), 0)
			if err != nil {
				t.Fatalf("round-trip parse error: %v", err)
			}
			if len(blocks) != 1 || blocks[0].Body != tt.body {
				t.Fatalf("round-trip body = %q, want %q", blocks[0].Body, tt.body)
			}
		})
	}
}

func TestMessageBlockErrorIncomplete(t *testing.T) {
	e := MessageBlockError{Message: "unterminated message block"}
	if !e.Incomplete() {
		t.Fatal("expected Incomplete() = true for unterminated")
	}
	e2 := MessageBlockError{Message: "invalid target character"}
	if e2.Incomplete() {
		t.Fatal("expected Incomplete() = false for non-unterminated")
	}
}
