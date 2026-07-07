package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const sampleDiff = `diff --git a/foo.go b/foo.go
index 1234567..abcdefg 100644
--- a/foo.go
+++ b/foo.go
@@ -1,5 +1,7 @@
 package main
 
-import "fmt"
+import (
+	"fmt"
+	"os"
+)
 
 func main() {
-	fmt.Println("hello")
+	fmt.Println("hello world")
+	os.Exit(0)
 }
`

func TestParseUnifiedDiff(t *testing.T) {
	files := parseUnifiedDiff(sampleDiff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	fc := files[0]
	if fc.newPath != "foo.go" {
		t.Errorf("expected newPath=foo.go, got %s", fc.newPath)
	}
	if fc.oldPath != "foo.go" {
		t.Errorf("expected oldPath=foo.go, got %s", fc.oldPath)
	}
	if len(fc.hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(fc.hunks))
	}
	h := fc.hunks[0]
	if h.oldStart != 1 || h.oldLines != 5 {
		t.Errorf("expected oldStart=1 oldLines=5, got %d %d", h.oldStart, h.oldLines)
	}
	if h.newStart != 1 || h.newLines != 7 {
		t.Errorf("expected newStart=1 newLines=7, got %d %d", h.newStart, h.newLines)
	}

	adds := 0
	dels := 0
	ctxs := 0
	for _, line := range h.lines {
		switch line.kind {
		case diffAdd:
			adds++
		case diffDelete:
			dels++
		case diffContext:
			ctxs++
		}
	}
	if adds != 6 {
		t.Errorf("expected 6 add lines, got %d", adds)
	}
	if dels != 2 {
		t.Errorf("expected 2 delete lines, got %d", dels)
	}
	if ctxs != 5 {
		t.Errorf("expected 5 context lines, got %d", ctxs)
	}
}

func TestRenderDiff(t *testing.T) {
	files := parseUnifiedDiff(sampleDiff)
	rendered := renderDiff(files, 80)
	if rendered == "" {
		t.Fatal("expected non-empty rendered diff")
	}
	if !strings.Contains(rendered, "foo.go") {
		t.Error("expected rendered diff to contain filename")
	}
	if !strings.Contains(rendered, "@@ -1,5 +1,7 @@") {
		t.Error("expected rendered diff to contain hunk header")
	}
	fmt.Printf("Rendered diff (%d chars):\n%s\n", len(rendered), rendered)
}

func TestHighlightCode(t *testing.T) {
	code := `func main() { fmt.Println("hello") }`
	hl := highlightCode(code, "go")
	if hl == code {
		t.Error("expected highlighted code to differ from input")
	}
	plain := highlightCode(code, "unknownlang")
	if plain != code {
		t.Error("expected unknown language to return plain code")
	}
}

func TestParseMultipleHunks(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,3 +1,3 @@
 line1
-old
+new
 line3
@@ -10,3 +10,3 @@
 line10
-old2
+new2
line12
`
	files := parseUnifiedDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].hunks) != 2 {
		t.Errorf("expected 2 hunks, got %d", len(files[0].hunks))
	}
}

func TestParseMultipleFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,1 @@
-a
+b
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,1 +1,1 @@
-c
+d
`
	files := parseUnifiedDiff(diff)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].newPath != "a.go" {
		t.Errorf("expected first file a.go, got %s", files[0].newPath)
	}
	if files[1].newPath != "b.go" {
		t.Errorf("expected second file b.go, got %s", files[1].newPath)
	}
}

func TestFormatDate(t *testing.T) {
	// zero time should return empty
	if formatDate(time.Time{}) != "" {
		t.Error("expected empty string for zero time")
	}
}
