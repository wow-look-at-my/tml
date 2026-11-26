// groups_test.go checks how the commands are laid out in help.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandGroup is the help section every command belongs to. A command missing from this table fails the test, so a
// new one cannot land ungrouped.
var commandGroup = map[string]string{
	"check":    groupDocument,
	"inspect":  groupDocument,
	"render":   groupDocument,
	"at":       groupLive,
	"elements": groupLive,
	"frame":    groupLive,
	"ids":      groupLive,
	"input":    groupLive,
	"list":     groupLive,
	"query":    groupLive,
	"restyle":  groupLive,
	"serve":    groupLive,
	"capture":  groupEither,
	"tree":     groupEither,
}

// helpSections is the order the sections must appear in.
var helpSections = []struct {
	id    string
	title string
}{
	{groupDocument, "Document commands:"},
	{groupLive, "Live program commands:"},
	{groupEither, "Document or live program:"},
}

// helpText prints the root command's help through the same path a user runs.
func helpText(t *testing.T) string {
	t.Helper()
	t.Serial()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})
	t.Cleanup(func() {
		root.SetOut(nil)
		root.SetErr(nil)
		root.SetArgs(nil)
	})
	require.NoError(t, root.Execute())
	return out.String()
}

// A command with no group would fall into cobra's Additional Commands, away from the section its siblings share.
func TestEveryCommandHasOneHelpGroup(t *testing.T) {
	helpText(t)
	listed := map[string]bool{}
	for _, cmd := range root.Commands() {
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		listed[cmd.Name()] = true
		want, ok := commandGroup[cmd.Name()]
		require.True(t, ok, "command %q is missing from commandGroup", cmd.Name())
		assert.Equal(t, want, cmd.GroupID, "command %q", cmd.Name())
	}
	assert.Len(t, listed, len(commandGroup), "every command in commandGroup should be registered")
}

func TestHelpRendersTheGroupSectionsInOrder(t *testing.T) {
	text := helpText(t)
	starts := make([]int, len(helpSections))
	for i, section := range helpSections {
		starts[i] = strings.Index(text, "\n"+section.title+"\n")
		require.NotEqual(t, -1, starts[i], "help is missing the %q section:\n%s", section.title, text)
	}
	for i := 1; i < len(starts); i++ {
		assert.Less(t, starts[i-1], starts[i], "help prints %q before %q", helpSections[i-1].title, helpSections[i].title)
	}

	end := len(text)
	final := len(helpSections) - 1
	if additional := strings.Index(text, "\nAdditional Commands:\n"); additional != -1 {
		require.Greater(t, additional, starts[final], "Additional Commands should follow the grouped sections")
		end = additional
	}
	for name := range commandGroup {
		line := "\n  " + name + " "
		assert.Equal(t, 1, strings.Count(text, line), "%q should be listed once:\n%s", name, text)
		at := strings.Index(text, line)
		require.NotEqual(t, -1, at, "help does not list %q", name)
		section := -1
		for i, candidate := range helpSections {
			if commandGroup[name] == candidate.id {
				section = i
			}
		}
		require.NotEqual(t, -1, section, "%q has no section for group %q", name, commandGroup[name])
		bound := end
		if section < final {
			bound = starts[section+1]
		}
		assert.Greater(t, at, starts[section], "%q should follow the %q heading", name, helpSections[section].title)
		assert.Less(t, at, bound, "%q should come before the next heading", name)
	}
}

// The built-in commands stay outside the groups, in cobra's own section.
func TestHelpKeepsBuiltinsOutOfTheGroupedSections(t *testing.T) {
	text := helpText(t)
	additional := strings.Index(text, "\nAdditional Commands:\n")
	require.NotEqual(t, -1, additional, "help should keep an Additional Commands section:\n%s", text)
	tail := text[additional:]
	assert.Contains(t, tail, "\n  completion ")
	assert.Contains(t, tail, "\n  help ")
}
