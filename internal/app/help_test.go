package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var expectedLeafTopics = []string{
	"init",
	"status",
	"doctor",
	"discover",
	"workspace init",
	"workspace status",
	"workspace available",
	"workspace attach",
	"task start",
	"task status",
	"task resume",
	"checkpoint",
	"handoff",
	"sync init",
	"sync status",
	"sync push",
	"sync pull",
	"sync upgrade",
	"man",
}

func TestTopLevelHelpListsAllCommands(t *testing.T) {
	out := runCLI(t, t.TempDir(), t.TempDir(), "--help")
	if !strings.Contains(out, "Context Baggage") || !strings.Contains(out, "Usage:") || !strings.Contains(out, "Commands:") {
		t.Fatalf("help missing intro/usage/commands:\n%s", out)
	}
	for _, topic := range expectedLeafTopics {
		doc, ok := findCommandDoc(topic)
		if !ok {
			t.Fatalf("expected doc for %q not found", topic)
		}
		if !strings.Contains(out, doc.Usage) {
			t.Fatalf("help missing usage for %s:\n%s", topic, out)
		}
		if !strings.Contains(out, doc.Summary) {
			t.Fatalf("help missing summary for %s:\n%s", topic, out)
		}
	}
	if !strings.Contains(out, "ctx-bag man") {
		t.Fatalf("help missing manual pointer:\n%s", out)
	}
}

func TestManualIndex(t *testing.T) {
	out := runCLI(t, t.TempDir(), t.TempDir(), "man")
	if !strings.Contains(out, "Context Baggage Manual") {
		t.Fatalf("missing manual header:\n%s", out)
	}
	for _, topic := range []string{"workspace", "task", "sync", "man", "workspace attach"} {
		if !strings.Contains(out, topic) {
			t.Fatalf("manual index missing %q:\n%s", topic, out)
		}
	}
}

func TestManualNestedTopics(t *testing.T) {
	out := runCLI(t, t.TempDir(), t.TempDir(), "man", "workspace", "attach")
	if !strings.Contains(out, "ctx-bag workspace attach <workspace-id>") {
		t.Fatalf("missing exact usage:\n%s", out)
	}
	if !strings.Contains(out, "does not pull automatically") {
		t.Fatalf("missing explicit-attachment semantics:\n%s", out)
	}
	if !strings.Contains(out, "sync pull") {
		t.Fatalf("missing next-step hint:\n%s", out)
	}

	upgrade := runCLI(t, t.TempDir(), t.TempDir(), "man", "sync", "upgrade")
	if !strings.Contains(upgrade, "legacy") || !strings.Contains(upgrade, "v2") {
		t.Fatalf("missing legacy/v2 semantics:\n%s", upgrade)
	}
}

func TestManualDoctor(t *testing.T) {
	out := runCLI(t, t.TempDir(), t.TempDir(), "man", "doctor")
	if !strings.Contains(out, "warnings") || !strings.Contains(out, "never auto-repair") {
		t.Fatalf("doctor manual missing semantics:\n%s", out)
	}
}

func TestManualUnknownTopic(t *testing.T) {
	for _, args := range [][]string{{"man", "nope"}, {"man", "workspace", "nope"}, {"man", "workspace", "attach", "extra"}} {
		_, err := runCLIErr(t, t.TempDir(), t.TempDir(), args...)
		if err == nil || !strings.Contains(err.Error(), "manual topic not found") {
			t.Fatalf("expected unknown-topic error for %v, got %v", args, err)
		}
	}
}

func TestHelpAliases(t *testing.T) {
	for _, alias := range []string{"-h"} {
		out := runCLI(t, t.TempDir(), t.TempDir(), alias)
		if !strings.Contains(out, "Commands:") {
			t.Fatalf("alias %s did not print help:\n%s", alias, out)
		}
	}
}

func TestDocumentationCoverageAndUniqueTopics(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range commandDocs {
		if d.Topic == "" || d.Usage == "" || d.Summary == "" {
			t.Fatalf("doc %q has empty Topic/Usage/Summary", d.Topic)
		}
		if seen[d.Topic] {
			t.Fatalf("duplicate topic %q", d.Topic)
		}
		seen[d.Topic] = true
	}
	for _, topic := range expectedLeafTopics {
		d, ok := findCommandDoc(topic)
		if !ok {
			t.Fatalf("leaf %q missing documentation entry", topic)
		}
		if !d.VisibleInHelp {
			t.Fatalf("leaf %q must be VisibleInHelp", topic)
		}
		if d.Details == "" {
			t.Fatalf("leaf %q missing manual Details", topic)
		}
	}
	for _, group := range []string{"workspace", "task", "sync"} {
		d, ok := findCommandDoc(group)
		if !ok {
			t.Fatalf("group %q missing documentation entry", group)
		}
		if d.VisibleInHelp {
			t.Fatalf("group %q must be hidden from help", group)
		}
	}
}

func TestHelpAndManWorkWithoutInit(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing-home")
	for _, args := range [][]string{{"--help"}, {"man"}, {"man", "workspace", "attach"}} {
		if _, err := runCLIErr(t, home, t.TempDir(), args...); err != nil {
			t.Fatalf("%v failed without init: %v", args, err)
		}
	}
}

func TestHelpAndManNoSideEffects(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing-home")
	for _, args := range [][]string{{"--help"}, {"man"}, {"man", "sync", "push"}} {
		runCLI(t, home, t.TempDir(), args...)
		if _, err := os.Stat(home); !os.IsNotExist(err) {
			t.Fatalf("%v created home: %v", args, err)
		}
	}
}
