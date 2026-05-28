package syntax

import (
	"strings"
	"testing"
)

func TestScanMsgBlocks(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantMsgs []string
		wantBash string
		wantErr  string
	}{
		{
			name:     "simple",
			src:      `m"hello"`,
			wantMsgs: []string{"hello"},
			wantBash: `        `,
		},
		{
			name:     "single hash delimiter",
			src:      `m#"body with "quotes"#`,
			wantMsgs: []string{`body with "quotes`},
			wantBash: `                      `,
		},
		{
			name:     "double hash delimiter",
			src:      `m##"body with "# inner"##`,
			wantMsgs: []string{`body with "# inner`},
			wantBash: `                         `,
		},
		{
			name:     "with target",
			src:      `m(neil)"hello neil"`,
			wantMsgs: []string{"hello neil"},
			wantBash: `                   `,
		},
		{
			name:     "target with hashes",
			src:      `m(neil)##"hello ## neil"##`,
			wantMsgs: []string{"hello ## neil"},
			wantBash: `                          `,
		},
		{
			name:     "multiline body",
			src:      "m\"line 1\nline 2\"",
			wantMsgs: []string{"line 1\nline 2"},
			wantBash: "        \n       ",
		},
		{
			name:     "multiple blocks",
			src:      "m\"first\"\nm\"second\"",
			wantMsgs: []string{"first", "second"},
			wantBash: "        \n         ",
		},
		{
			name:     "bash with message blocks",
			src:      "m\"done\"\necho hello",
			wantMsgs: []string{"done"},
			wantBash: "       \necho hello",
		},
		{
			name:     "bash only",
			src:      "echo hello",
			wantMsgs: nil,
			wantBash: "echo hello",
		},
		{
			name:     "m in echo not a block",
			src:      `echo m"not a block"`,
			wantMsgs: nil,
			wantBash: `echo m"not a block"`,
		},
		{
			name:     "m in comment not a block",
			src:      "# m\"not a block\"\necho hello",
			wantMsgs: nil,
			wantBash: "# m\"not a block\"\necho hello",
		},
		{
			name:     "m in single quotes not a block",
			src:      `echo 'm"not a block"'`,
			wantMsgs: nil,
			wantBash: `echo 'm"not a block"'`,
		},
		{
			name:     "m in double quotes not a block",
			src:      `echo "m\"not a block\""`,
			wantMsgs: nil,
			wantBash: `echo "m\"not a block\""`,
		},
		{
			name:     "m in backticks not a block",
			src:      "echo `m\"not a block\"`",
			wantMsgs: nil,
			wantBash: "echo `m\"not a block\"`",
		},
		{
			name:     "m after pipe not start",
			src:      "cat file | m\"not at start\"",
			wantMsgs: nil,
			wantBash: "cat file | m\"not at start\"",
		},
		{
			name:    "unterminated block",
			src:     `m"unterminated`,
			wantErr: "unterminated message block",
		},
		{
			name:    "invalid target char",
			src:     `m(in valid)"body"`,
			wantErr: "invalid target character",
		},
		{
			name:     "empty",
			src:      "",
			wantMsgs: nil,
			wantBash: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, clean, err := ScanMsgBlocks([]byte(tt.src), 0)
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
			if string(clean) != tt.wantBash {
				t.Fatalf("cleaned bash:\n  got  %q\n  want %q", string(clean), tt.wantBash)
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
