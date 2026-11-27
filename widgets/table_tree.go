// table_tree.go holds treeNode, one row's place in a table's tree column.

package widgets

import (
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/widget"
)

// treeIndent is the cells of indent for each level of depth.
const treeIndent = 2

// treeGlyphs is the disclosure glyph of each state. A leaf draws a blank of the same width.
var treeGlyphs = map[string]string{"open": "▾", "closed": "▸", "leaf": " "}

// treeNode is a row's depth and its state: open, closed or leaf.
type treeNode struct {
	depth int
	state string
}

// prefix is the cells the node draws before the text of its row: the indent, the glyph and a space.
func (n treeNode) prefix() int { return n.depth*treeIndent + 2 }

// glyphAt is the column of the node's disclosure glyph.
func (n treeNode) glyphAt() int { return n.depth * treeIndent }

// treeNodes reads the tree attribute, one "<depth>:<state>" entry for each row. No attribute is no tree.
func treeNodes(ctx widget.Context, rows int) ([]treeNode, error) {
	raw := ctx.Attrs.List("tree")
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) != rows {
		return nil, ctx.Attrs.Errorf("tree", "names %d entries for %d rows", len(raw), rows)
	}
	nodes := make([]treeNode, len(raw))
	for i, entry := range raw {
		depthText, state, ok := strings.Cut(strings.TrimSpace(entry), ":")
		depth, err := strconv.Atoi(depthText)
		if _, known := treeGlyphs[state]; !ok || err != nil || depth < 0 || !known {
			return nil, ctx.Attrs.Errorf("tree", "entry %q of row %d is not <depth>:<state>, where depth is zero or more and state is open, closed or leaf", entry, i)
		}
		nodes[i] = treeNode{depth: depth, state: state}
	}
	return nodes, nil
}
