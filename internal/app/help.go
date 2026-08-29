package app

import (
	"fmt"
	"io"
	"strings"
)

// commandDoc is the single source of truth for a command's usage, concise
// summary, and detailed built-in manual content. Usage and Summary are used by
// both top-level --help and the manual page rendering, so they never drift
// apart. Details holds only additional topic-specific manual content.
type commandDoc struct {
	Topic         string
	Usage         string
	Summary       string
	VisibleInHelp bool
	Details       string
}

// commandDocs is the ordered documentation catalog. Order is deliberate so help
// and manual output are deterministic.
var commandDocs = []commandDoc{
	{Topic: "init", Usage: "ctx-bag init", Summary: "Initialize Context Baggage on this machine.", VisibleInHelp: true, Details: docInit},
	{Topic: "status", Usage: "ctx-bag status", Summary: "Show current Context Baggage state.", VisibleInHelp: true, Details: docStatus},
	{Topic: "doctor", Usage: "ctx-bag doctor", Summary: "Check configuration, workspace identity, and sync health.", VisibleInHelp: true, Details: docDoctor},
	{Topic: "discover", Usage: "ctx-bag discover", Summary: "Discover supported coding-agent configuration.", VisibleInHelp: true, Details: docDiscover},

	{Topic: "workspace", Usage: "ctx-bag workspace <command>", Summary: "Manage the canonical workspace for the current directory.", VisibleInHelp: false, Details: docWorkspace},
	{Topic: "workspace init", Usage: "ctx-bag workspace init [--sync|--no-sync]", Summary: "Initialize the current directory as a workspace.", VisibleInHelp: true, Details: docWorkspaceInit},
	{Topic: "workspace status", Usage: "ctx-bag workspace status", Summary: "Show the current workspace identity and local state.", VisibleInHelp: true, Details: docWorkspaceStatus},
	{Topic: "workspace available", Usage: "ctx-bag workspace available", Summary: "List portable workspaces available in shared state.", VisibleInHelp: true, Details: docWorkspaceAvailable},
	{Topic: "workspace attach", Usage: "ctx-bag workspace attach <workspace-id>", Summary: "Attach the current directory to an existing portable workspace.", VisibleInHelp: true, Details: docWorkspaceAttach},

	{Topic: "task", Usage: "ctx-bag task <command>", Summary: "Manage tasks, checkpoints, and handoffs.", VisibleInHelp: false, Details: docTask},
	{Topic: "task start", Usage: "ctx-bag task start <name>", Summary: "Start a task in the current workspace.", VisibleInHelp: true, Details: docTaskStart},
	{Topic: "task status", Usage: "ctx-bag task status", Summary: "Show the active task.", VisibleInHelp: true, Details: docTaskStatus},
	{Topic: "task resume", Usage: "ctx-bag task resume <name>", Summary: "Resume an existing task.", VisibleInHelp: true, Details: docTaskResume},

	{Topic: "checkpoint", Usage: "ctx-bag checkpoint -m <message>", Summary: "Save a continuity checkpoint for the active task.", VisibleInHelp: true, Details: docCheckpoint},
	{Topic: "handoff", Usage: "ctx-bag handoff", Summary: "Show or create the current task handoff.", VisibleInHelp: true, Details: docHandoff},

	{Topic: "sync", Usage: "ctx-bag sync <command>", Summary: "Manage portable state synced through a shared folder.", VisibleInHelp: false, Details: docSync},
	{Topic: "sync init", Usage: "ctx-bag sync init <folder>", Summary: "Configure the shared filesystem sync folder.", VisibleInHelp: true, Details: docSyncInit},
	{Topic: "sync status", Usage: "ctx-bag sync status", Summary: "Show sync configuration and shared-state status.", VisibleInHelp: true, Details: docSyncStatus},
	{Topic: "sync push", Usage: "ctx-bag sync push", Summary: "Push portable state to the shared folder.", VisibleInHelp: true, Details: docSyncPush},
	{Topic: "sync pull", Usage: "ctx-bag sync pull", Summary: "Pull portable state from the shared folder.", VisibleInHelp: true, Details: docSyncPull},
	{Topic: "sync upgrade", Usage: "ctx-bag sync upgrade", Summary: "Convert legacy shared state to sync format v2.", VisibleInHelp: true, Details: docSyncUpgrade},

	{Topic: "man", Usage: "ctx-bag man [topic...]", Summary: "Show the detailed built-in manual.", VisibleInHelp: true, Details: docMan},
}

// printHelp renders the concise top-level help surface.
func printHelp(out io.Writer) error {
	if err := writeOutput(out, "Context Baggage — portable continuity for coding-agent work.\n\nUsage:\n  ctx-bag <command>\n\nCommands:\n"); err != nil {
		return err
	}
	for _, d := range commandDocs {
		if !d.VisibleInHelp {
			continue
		}
		if err := writeOutput(out, "  %s\n      %s\n", d.Usage, d.Summary); err != nil {
			return err
		}
	}
	return writeOutput(out, "\nFor detailed documentation:\n  ctx-bag man\n  ctx-bag man <command>\n")
}

// runMan prints the built-in manual index or a requested topic page. It is a
// state-independent, read-only operation: it must work before Context Baggage
// initialization and must not create or mutate any state.
func runMan(args []string, out io.Writer) error {
	if len(args) == 0 {
		return printManualIndex(out)
	}
	topic := strings.Join(args, " ")
	doc, found := findCommandDoc(topic)
	if !found {
		return fmt.Errorf("manual topic not found: %s\nrun: ctx-bag man", topic)
	}
	return renderCommand(out, doc)
}

// printManualIndex lists all available manual topics with intended usage.
func printManualIndex(out io.Writer) error {
	if err := writeOutput(out, "Context Baggage Manual\n\nUsage:\n  ctx-bag man <topic>\n\nTopics:\n"); err != nil {
		return err
	}
	for _, d := range commandDocs {
		if err := writeOutput(out, "  %s\n", d.Topic); err != nil {
			return err
		}
	}
	return writeOutput(out, "\nExamples:\n  ctx-bag man workspace attach\n  ctx-bag man sync pull\n")
}

// findCommandDoc returns the documentation entry for an exact topic.
func findCommandDoc(topic string) (commandDoc, bool) {
	for _, d := range commandDocs {
		if d.Topic == topic {
			return d, true
		}
	}
	return commandDoc{}, false
}

// renderCommand writes a single manual page using the authoritative Usage,
// Summary, and the topic-specific Details.
func renderCommand(out io.Writer, d commandDoc) error {
	if err := writeOutput(out, "%s\n\nUsage:\n  %s\n\nPurpose:\n  %s\n", strings.ToUpper(d.Topic), d.Usage, d.Summary); err != nil {
		return err
	}
	if d.Details == "" {
		return nil
	}
	return writeOutput(out, "%s", d.Details)
}
