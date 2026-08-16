package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/render"
)

// rect and size are the JSON spellings of the geometry a test asserts on.
type rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type size struct {
	W int `json:"w"`
	H int `json:"h"`
}

type scroll struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	MaxX int `json:"maxX"`
	MaxY int `json:"maxY"`
}

// element is one id-bearing element of a laid-out frame.
type element struct {
	ID      string   `json:"id"`
	Element string   `json:"element"`
	Action  string   `json:"action,omitempty"`
	Focus   bool     `json:"focus"`
	Rect    rect     `json:"rect"`
	Content size     `json:"content"`
	Clip    rect     `json:"clip"`
	Scroll  scroll   `json:"scroll"`
	Lines   []string `json:"lines"`
}

func toRect(r layout.Rect) rect { return rect{X: r.X, Y: r.Y, W: r.W, H: r.H} }

// find returns the box carrying id, searching depth first so the outermost
// match wins when a component reuses an id inside itself.
func find(box *layout.Box, id string) *layout.Box {
	if box == nil {
		return nil
	}
	if box.ID == id {
		return box
	}
	for _, child := range box.Children {
		if hit := find(child, id); hit != nil {
			return hit
		}
	}
	return nil
}

func ids(box *layout.Box, out *[]string) {
	if box == nil {
		return
	}
	if box.ID != "" {
		*out = append(*out, box.ID)
	}
	for _, child := range box.Children {
		ids(child, out)
	}
}

// describe renders one box on its own and reports where it landed.
func describe(box *layout.Box, sc scroll, keepANSI bool) element {
	text := render.Render(box)
	if !keepANSI {
		text = ansi.Strip(text)
	}
	lines := []string{}
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	return element{
		ID:      box.ID,
		Element: box.Name,
		Action:  box.Action,
		Rect:    toRect(box.Screen),
		Content: size{W: box.Content.W, H: box.Content.H},
		Clip:    toRect(box.Clip),
		Scroll:  sc,
		Lines:   lines,
	}
}

func init() { root.AddCommand(newInspectCmd()) }

// newInspectCmd builds the command fresh, so a caller -- a test especially --
// gets its own flag values rather than whatever the last run left behind.
func newInspectCmd() *cobra.Command {
	var (
		dark          bool
		width, height int
		props         []string
		id            string
		keepANSI      bool
	)

	inspect := &cobra.Command{
		Use:   "inspect <file.tml>",
		Short: "Report where an element landed and what it drew",
		Long: "Lays the view out at the given size and prints JSON describing the\n" +
			"elements that carry an id: the rect they occupy, the content size they\n" +
			"were given, their clip, their scroll position, and the lines they drew.\n" +
			"\n" +
			"With --id it prints that one element and fails if no element carries the\n" +
			"id, naming the ids that do exist. Without it, every id-bearing element is\n" +
			"printed in document order.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			view, parsed, err := loadView(args[0], dark, props)
			if err != nil {
				return err
			}
			box, err := view.Layout(parsed, width, height)
			if err != nil {
				return err
			}

			scrolls := map[string]scroll{}
			for _, t := range view.UI().Targets() {
				scrolls[t.ID] = scroll{X: t.Scroll.X, Y: t.Scroll.Y, MaxX: t.Scroll.MaxX, MaxY: t.Scroll.MaxY}
			}
			focused := map[string]bool{}
			for _, t := range view.UI().Targets() {
				focused[t.ID] = t.Focus
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			if id != "" {
				hit := find(box, id)
				if hit == nil {
					var known []string
					ids(box, &known)
					sort.Strings(known)
					if len(known) == 0 {
						return fmt.Errorf("no element has id %q: this view declares no ids at all", id)
					}
					return fmt.Errorf("no element has id %q: the view declares %s", id, strings.Join(known, ", "))
				}
				el := describe(hit, scrolls[id], keepANSI)
				el.Focus = focused[id]
				return enc.Encode(el)
			}

			var known []string
			ids(box, &known)
			all := make([]element, 0, len(known))
			for _, known := range known {
				hit := find(box, known)
				el := describe(hit, scrolls[known], keepANSI)
				el.Focus = focused[known]
				all = append(all, el)
			}
			return enc.Encode(all)
		},
	}
	inspect.Flags().BoolVar(&dark, "dark", false, "resolve adaptive theme tokens to their dark value")
	inspect.Flags().IntVar(&width, "width", 80, "viewport width in cells")
	inspect.Flags().IntVar(&height, "height", 24, "viewport height in cells")
	inspect.Flags().StringArrayVar(&props, "prop", nil, "set a property as name=value (repeatable)")
	inspect.Flags().StringVar(&id, "id", "", "report only the element carrying this id")
	inspect.Flags().BoolVar(&keepANSI, "ansi", false, "keep the escape sequences in the reported lines")
	return inspect
}
