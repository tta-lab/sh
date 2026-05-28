package syntax

import (
	"testing"
)

func TestHeredocLineAccounting(t *testing.T) {
	// Issue 1: m"ok" after heredoc should be at correct line
	src := "cat <<EOF\nEOF\nm\"ok\""
	blocks, _, err := ScanMsgBlocks([]byte(src), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Mpos.Line() != 3 {
		t.Fatalf("expected m\"ok\" at line 3, got line %d", blocks[0].Mpos.Line())
	}

	// Issue 1b: multiline heredoc body
	src2 := "cat <<EOF\nm\"bad\"\nEOF\nm\"ok\""
	blocks2, _, err := ScanMsgBlocks([]byte(src2), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks2) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks2))
	}
	if blocks2[0].Mpos.Line() != 4 {
		t.Fatalf("expected m\"ok\" at line 4, got line %d", blocks2[0].Mpos.Line())
	}

	// Issue 2: m"same line" before heredoc body should be extracted
	src3 := "cat <<EOF; m\"same line\"\nbody\nEOF"
	blocks3, _, err := ScanMsgBlocks([]byte(src3), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks3) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks3))
	}
	if blocks3[0].Body != "same line" {
		t.Fatalf("body = %q, want %q", blocks3[0].Body, "same line")
	}
}
