// groups.go declares the help sections commands are listed under.

package cli

import "github.com/spf13/cobra"

const (
	groupDocument = "document"
	groupLive     = "live"
	groupEither   = "either"
)

// init adds the groups in display order. Cobra prints them in the order they are added and sorts each group's
// commands by name.
func init() {
	root.AddGroup(
		&cobra.Group{ID: groupDocument, Title: "Document commands:"},
		&cobra.Group{ID: groupLive, Title: "Live program commands:"},
		&cobra.Group{ID: groupEither, Title: "Document or live program:"},
	)
}
