package main

import (
	"strings"
	"testing"
)

func TestHighlightCodePHP(t *testing.T) {
	code := `<?php function foo($x) { return $x; } ?>`
	hl := highlightCode(code, "php")
	if hl == code {
		t.Error("expected highlighted PHP to differ from input")
	}
}

func TestHighlightCodeSwift(t *testing.T) {
	code := `func foo() -> Int { return 42 }`
	hl := highlightCode(code, "swift")
	if hl == code {
		t.Error("expected highlighted Swift to differ from input")
	}
}

func TestHighlightCodeDart(t *testing.T) {
	code := `void main() { print("hello"); }`
	hl := highlightCode(code, "dart")
	if hl == code {
		t.Error("expected highlighted Dart to differ from input")
	}
}

func TestHighlightCodeLua(t *testing.T) {
	code := `function foo() return 42 end`
	hl := highlightCode(code, "lua")
	if hl == code {
		t.Error("expected highlighted Lua to differ from input")
	}
}

func TestHighlightCodeElixir(t *testing.T) {
	code := `def foo do 42 end`
	hl := highlightCode(code, "elixir")
	if hl == code {
		t.Error("expected highlighted Elixir to differ from input")
	}
}

func TestHighlightCodeZig(t *testing.T) {
	code := `fn foo() i32 { return 42; }`
	hl := highlightCode(code, "zig")
	if hl == code {
		t.Error("expected highlighted Zig to differ from input")
	}
}

func TestHighlightCodeNim(t *testing.T) {
	code := `proc foo(): int = 42`
	hl := highlightCode(code, "nim")
	if hl == code {
		t.Error("expected highlighted Nim to differ from input")
	}
}

func TestHighlightCodeTerraform(t *testing.T) {
	code := `resource "aws_instance" "web" { count = 2 }`
	hl := highlightCode(code, "terraform")
	if hl == code {
		t.Error("expected highlighted Terraform to differ from input")
	}
}

func TestHighlightCodeDockerfile(t *testing.T) {
	code := `FROM alpine:3.18\nRUN apk add curl`
	hl := highlightCode(code, "dockerfile")
	if hl == code {
		t.Error("expected highlighted Dockerfile to differ from input")
	}
}

func TestHighlightCodeJulia(t *testing.T) {
	code := `function foo()::Int return 42 end`
	hl := highlightCode(code, "julia")
	if hl == code {
		t.Error("expected highlighted Julia to differ from input")
	}
}

func TestHighlightCodeGraphQL(t *testing.T) {
	code := `type Query { hello: String }`
	hl := highlightCode(code, "graphql")
	if hl == code {
		t.Error("expected highlighted GraphQL to differ from input")
	}
}

func TestHighlightCodeKotlin(t *testing.T) {
	code := `fun foo(): Int { return 42 }`
	hl := highlightCode(code, "kotlin")
	if hl == code {
		t.Error("expected highlighted Kotlin to differ from input")
	}
}

func TestHighlightCodeScala(t *testing.T) {
	code := `def foo(): Int = 42`
	hl := highlightCode(code, "scala")
	if hl == code {
		t.Error("expected highlighted Scala to differ from input")
	}
}

func TestHighlightCodeClojure(t *testing.T) {
	code := `(defn foo [x] (+ x 1))`
	hl := highlightCode(code, "clojure")
	if hl == code {
		t.Error("expected highlighted Clojure to differ from input")
	}
}

func TestHighlightCodeR(t *testing.T) {
	code := `foo <- function(x) { return(x + 1) }`
	hl := highlightCode(code, "r")
	if hl == code {
		t.Error("expected highlighted R to differ from input")
	}
}

func TestHighlightCodeTOML(t *testing.T) {
	code := `[package]\nname = "test"\nversion = "1.0"`
	hl := highlightCode(code, "toml")
	if hl == code {
		t.Error("expected highlighted TOML to differ from input")
	}
}

func TestHighlightCodeProto(t *testing.T) {
	code := `syntax = "proto3"; message Foo { string bar = 1; }`
	hl := highlightCode(code, "protobuf")
	if hl == code {
		t.Error("expected highlighted Protobuf to differ from input")
	}
}

func TestHighlightCodeVue(t *testing.T) {
	code := `<template><div>hello</div></template>`
	hl := highlightCode(code, "vue")
	if hl == code {
		t.Error("expected highlighted Vue to differ from input")
	}
}

func TestNormalizeLangPHP(t *testing.T) {
	if normalizeLang("php") != "php" {
		t.Error("expected php to normalize to php")
	}
}

func TestNormalizeLangTerraform(t *testing.T) {
	if normalizeLang("tf") != "terraform" {
		t.Error("expected tf to normalize to terraform")
	}
}

func TestNormalizeLangHCL(t *testing.T) {
	if normalizeLang("hcl") != "terraform" {
		t.Error("expected hcl to normalize to terraform")
	}
}

func TestNormalizeLangProto(t *testing.T) {
	if normalizeLang("proto") != "protobuf" {
		t.Error("expected proto to normalize to protobuf")
	}
}

func TestParseDiffNewFile(t *testing.T) {
	diff := `diff --git a/new.txt b/new.txt
new file mode 100644
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,1 @@
+hello
`
	files := parseUnifiedDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].isNew {
		t.Error("expected isNew=true")
	}
	if files[0].newPath != "new.txt" {
		t.Errorf("expected newPath=new.txt, got %s", files[0].newPath)
	}
}

func TestParseDiffDeletedFile(t *testing.T) {
	diff := `diff --git a/old.txt b/old.txt
deleted file mode 100644
--- a/old.txt
+++ /dev/null
@@ -1,1 +0,0 @@
-hello
`
	files := parseUnifiedDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].isDeleted {
		t.Error("expected isDeleted=true")
	}
}

func TestParseDiffRenamedFile(t *testing.T) {
	diff := `diff --git a/old.txt b/new.txt
rename from old.txt
rename to new.txt
`
	files := parseUnifiedDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].isRenamed {
		t.Error("expected isRenamed=true")
	}
	if files[0].oldPath != "old.txt" {
		t.Errorf("expected oldPath=old.txt, got %s", files[0].oldPath)
	}
	if files[0].newPath != "new.txt" {
		t.Errorf("expected newPath=new.txt, got %s", files[0].newPath)
	}
}

func TestParseDiffBinaryFile(t *testing.T) {
	diff := `diff --git a/binary.png b/binary.png
Binary files differ
`
	files := parseUnifiedDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].isBinary {
		t.Error("expected isBinary=true")
	}
}

func TestRenderDiffNewFileIndicator(t *testing.T) {
	diff := `diff --git a/new.go b/new.go
new file mode 100644
--- /dev/null
+++ b/new.go
@@ -0,0 +1,1 @@
+package main
`
	files := parseUnifiedDiff(diff)
	rendered := renderDiff(files, 80)
	if !strings.Contains(rendered, "new file") {
		t.Error("expected 'new file' indicator in rendered diff")
	}
}

func TestRenderDiffDeletedFileIndicator(t *testing.T) {
	diff := `diff --git a/old.go b/old.go
deleted file mode 100644
--- a/old.go
+++ /dev/null
@@ -1,1 +0,0 @@
-package main
`
	files := parseUnifiedDiff(diff)
	rendered := renderDiff(files, 80)
	if !strings.Contains(rendered, "deleted") {
		t.Error("expected 'deleted' indicator in rendered diff")
	}
}

func TestRenderDiffBinaryFile(t *testing.T) {
	diff := `diff --git a/binary.png b/binary.png
Binary files differ
`
	files := parseUnifiedDiff(diff)
	rendered := renderDiff(files, 80)
	if !strings.Contains(rendered, "Binary") {
		t.Error("expected 'Binary' in rendered diff")
	}
}
