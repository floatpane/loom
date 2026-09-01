package main

import (
	"strings"
	"testing"
)

func TestParseRebaseExec(t *testing.T) {
	content := "exec echo hello\npick abc1234 first commit\n"
	tmpFile := writeTempFile(t, content)
	defer removeTempFile(t, tmpFile)

	m := newRebaseModel(tmpFile)
	if len(m.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(m.items))
	}
	if m.items[0].action != "exec" {
		t.Errorf("expected action=exec, got %s", m.items[0].action)
	}
	if m.items[0].msg != "echo hello" {
		t.Errorf("expected msg='echo hello', got %s", m.items[0].msg)
	}
}

func TestParseRebreakBreak(t *testing.T) {
	content := "pick abc1234 first\nbreak\npick def5678 second\n"
	tmpFile := writeTempFile(t, content)
	defer removeTempFile(t, tmpFile)

	m := newRebaseModel(tmpFile)
	if len(m.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(m.items))
	}
	if m.items[1].action != "break" {
		t.Errorf("expected action=break, got %s", m.items[1].action)
	}
}

func TestParseRebaseLabelReset(t *testing.T) {
	content := "pick abc1234 first\nlabel mylabel\nreset mylabel\n"
	tmpFile := writeTempFile(t, content)
	defer removeTempFile(t, tmpFile)

	m := newRebaseModel(tmpFile)
	if len(m.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(m.items))
	}
	if m.items[1].action != "label" || m.items[1].msg != "mylabel" {
		t.Errorf("expected label mylabel, got %s %s", m.items[1].action, m.items[1].msg)
	}
	if m.items[2].action != "reset" || m.items[2].msg != "mylabel" {
		t.Errorf("expected reset mylabel, got %s %s", m.items[2].action, m.items[2].msg)
	}
}

func TestParseRebaseMerge(t *testing.T) {
	content := "pick abc1234 first\nmerge def5678 # merge message\n"
	tmpFile := writeTempFile(t, content)
	defer removeTempFile(t, tmpFile)

	m := newRebaseModel(tmpFile)
	if len(m.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(m.items))
	}
	if m.items[1].action != "merge" {
		t.Errorf("expected action=merge, got %s", m.items[1].action)
	}
}

func TestParseRebaseShortActions(t *testing.T) {
	content := "p abc1234 first\nr def5678 second\ns 1111111 third\nf 2222222 fourth\n"
	tmpFile := writeTempFile(t, content)
	defer removeTempFile(t, tmpFile)

	m := newRebaseModel(tmpFile)
	if len(m.items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(m.items))
	}
	if m.items[0].action != "pick" {
		t.Errorf("expected pick, got %s", m.items[0].action)
	}
	if m.items[1].action != "reword" {
		t.Errorf("expected reword, got %s", m.items[1].action)
	}
	if m.items[2].action != "squash" {
		t.Errorf("expected squash, got %s", m.items[2].action)
	}
	if m.items[3].action != "fixup" {
		t.Errorf("expected fixup, got %s", m.items[3].action)
	}
}

func TestRebaseWriteExec(t *testing.T) {
	m := &rebaseModel{items: []rebaseItem{
		{action: "pick", hash: "abc1234", msg: "first"},
		{action: "exec", msg: "echo done"},
		{action: "break"},
		{action: "label", msg: "mylabel"},
	}}
	content, err := m.writeToString()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "pick abc1234 first") {
		t.Error("expected pick line in output")
	}
	if !strings.Contains(content, "exec echo done") {
		t.Error("expected exec line in output")
	}
	if !strings.Contains(content, "break") {
		t.Error("expected break line in output")
	}
	if !strings.Contains(content, "label mylabel") {
		t.Error("expected label line in output")
	}
}

func TestCycleAction(t *testing.T) {
	if cycleAction("pick") != "reword" {
		t.Error("expected pick→reword")
	}
	if cycleAction("reword") != "edit" {
		t.Error("expected reword→edit")
	}
	if cycleAction("edit") != "squash" {
		t.Error("expected edit→squash")
	}
	if cycleAction("squash") != "fixup" {
		t.Error("expected squash→fixup")
	}
	if cycleAction("fixup") != "drop" {
		t.Error("expected fixup→drop")
	}
	if cycleAction("drop") != "pick" {
		t.Error("expected drop→pick")
	}
	if cycleAction("unknown") != "pick" {
		t.Error("expected unknown→pick")
	}
}

func TestSquashAllUp(t *testing.T) {
	m := &rebaseModel{
		items: []rebaseItem{
			{action: "pick", hash: "aaa", msg: "first"},
			{action: "pick", hash: "bbb", msg: "second"},
			{action: "pick", hash: "ccc", msg: "third"},
			{action: "pick", hash: "ddd", msg: "fourth"},
		},
		cursor: 3,
	}
	m.squashAllUp()
	if m.items[1].action != "squash" {
		t.Errorf("expected second to be squash, got %s", m.items[1].action)
	}
	if m.items[2].action != "squash" {
		t.Errorf("expected third to be squash, got %s", m.items[2].action)
	}
	if m.items[3].action != "squash" {
		t.Errorf("expected fourth to be squash, got %s", m.items[3].action)
	}
	if m.items[0].action != "pick" {
		t.Errorf("expected first to remain pick, got %s", m.items[0].action)
	}
}

func TestActionStyleExec(t *testing.T) {
	style := actionStyle("exec")
	_ = style // just verify it doesn't panic
}

func TestActionStyleBreak(t *testing.T) {
	style := actionStyle("break")
	_ = style // just verify it doesn't panic
}

func TestActionStyleLabel(t *testing.T) {
	style := actionStyle("label")
	_ = style // just verify it doesn't panic
}

func TestDetectEditorModeMerge(t *testing.T) {
	if mode := detectEditorMode("/path/to/.git/MERGE_MSG"); mode != "merge" {
		t.Errorf("expected 'merge', got '%s'", mode)
	}
}

func TestDetectEditorModeTag(t *testing.T) {
	if mode := detectEditorMode("/path/to/.git/TAG_EDITMSG"); mode != "tag" {
		t.Errorf("expected 'tag', got '%s'", mode)
	}
}

func TestDetectEditorModeCommit(t *testing.T) {
	if mode := detectEditorMode("/path/to/.git/COMMIT_EDITMSG"); mode != "commit" {
		t.Errorf("expected 'commit', got '%s'", mode)
	}
}

func TestDetectEditorModeDefault(t *testing.T) {
	if mode := detectEditorMode("/tmp/some-file.txt"); mode != "commit" {
		t.Errorf("expected 'commit' (default), got '%s'", mode)
	}
}
