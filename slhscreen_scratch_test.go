package tml_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

// Scratch: renders slh's real screen against this branch's tml. Not for commit.
func TestSlhScreenRenders(t *testing.T) {
	view, err := tml.Load(os.DirFS("testdata/slh"), "app.tml", tml.Options{Dark: true})
	require.NoError(t, err)

	msgs := sema.RecordListValue([]map[string]sema.Value{
		{
			"lead": sema.BoolValue(false), "indent": sema.StringValue("0"),
			"rail": sema.StringValue(">"), "railStyle": sema.StringValue("role.user"),
			"showHeader": sema.BoolValue(true), "identity": sema.StringValue("you"),
			"identityStyle": sema.StringValue("role.user"), "timestamp": sema.StringValue("14:23:08"),
			"body": sema.ListValue([]string{"do the thing"}), "bodyStyle": sema.StringValue("role.assistant"),
			"notes": sema.ListValue(nil), "showFooter": sema.BoolValue(false),
			"footerLead": sema.StringValue(""), "footer": sema.StringValue(""), "footerWarn": sema.StringValue(""),
		},
		{
			"lead": sema.BoolValue(true), "indent": sema.StringValue("0"),
			"rail": sema.StringValue("*"), "railStyle": sema.StringValue("role.assistant"),
			"showHeader": sema.BoolValue(true), "identity": sema.StringValue("opus-4-6"),
			"identityStyle": sema.StringValue("ui.dim"), "timestamp": sema.StringValue(""),
			"body":  sema.ListValue([]string{"here is the answer", "on two lines"}),
			"notes": sema.ListValue([]string{"! 1 escape sequences removed"}),
			"bodyStyle": sema.StringValue("role.assistant"), "showFooter": sema.BoolValue(true),
			"footerLead": sema.StringValue("+"), "footer": sema.StringValue("18.4k in · 903 out · $0.0143"),
			"footerWarn": sema.StringValue("! interrupted"),
		},
		{
			"lead": sema.BoolValue(false), "indent": sema.StringValue("2"),
			"rail": sema.StringValue("-"), "railStyle": sema.StringValue("role.tool"),
			"showHeader": sema.BoolValue(true), "identity": sema.StringValue("read_file"),
			"identityStyle": sema.StringValue("role.tool"), "timestamp": sema.StringValue(""),
			"body": sema.ListValue([]string{"nested under the turn"}), "bodyStyle": sema.StringValue("role.tool"),
			"notes": sema.ListValue(nil), "showFooter": sema.BoolValue(false),
			"footerLead": sema.StringValue(""), "footer": sema.StringValue(""), "footerWarn": sema.StringValue(""),
		},
	})

	props := tml.Props{
		"messages": msgs,
		"counter":  sema.StringValue("v 2 new"),

		"showPrompt": sema.BoolValue(false), "promptHeight": sema.StringValue("0"),
		"promptTitle": sema.StringValue(""), "promptRows": sema.ListValue(nil),
		"promptChoices": sema.ListValue(nil),

		"notice0": sema.StringValue("· store reopened"), "notice0Style": sema.StringValue("role.system"),
		"notice1": sema.StringValue(""), "notice1Style": sema.StringValue("role.system"),
		"notice2": sema.StringValue(""), "notice2Style": sema.StringValue("role.system"),

		"rule": sema.StringValue(""),
		"bypass": sema.StringValue(""), "activity": sema.StringValue("thinking"),
		"mode": sema.StringValue(""), "goal": sema.StringValue("goal: ship it"),
		"goalStyle": sema.StringValue("ui.dim"), "context": sema.StringValue("38%"),
		"model": sema.StringValue("opus-4-6"), "cost": sema.StringValue("$1.43"),
		"git": sema.StringValue("master"), "gitStyle": sema.StringValue("ui.dim"),
		"ci": sema.StringValue("ok"), "ciStyle": sema.StringValue("ui.dim"),
		"clock": sema.StringValue("14:25"), "dropped": sema.BoolValue(false),
		"transient": sema.StringValue(""),

		"showQueue": sema.BoolValue(false), "queueHeader": sema.StringValue(""),
		"queueHeaderStyle": sema.StringValue("ui.dim"), "queueRows": sema.ListValue(nil),
		"queueHeight": sema.StringValue("0"),

		"composerHeight": sema.StringValue("1"), "composerBoxHeight": sema.StringValue("3"),
		"composerBoxed": sema.BoolValue(true), "composerUnboxed": sema.BoolValue(false),
		"composerAbove": sema.ListValue(nil), "composerBelow": sema.ListValue(nil),
		"caretPrefix": sema.StringValue("> "), "caretBefore": sema.StringValue("typing"),
		"caretCell": sema.StringValue(" "), "caretAfter": sema.StringValue(""),
		"ghost": sema.StringValue(""), "showGhost": sema.BoolValue(false),
		"showCaret": sema.BoolValue(true), "showPlainCell": sema.BoolValue(false),

		"showHints": sema.BoolValue(true),
		"hints":     sema.ListValue([]string{"⏎ send", "Cancel [Ctrl+C]"}),
	}

	out, err := view.Render(props, 72, 20)
	require.NoError(t, err)
	t.Log("\n" + out)
	require.NotEmpty(t, out)
}
