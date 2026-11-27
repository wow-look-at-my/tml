// area.go holds the areaWidget type.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var areaAttrs = []string{}

// areaWidget is a region the pointer reaches and nothing else: it draws no cell and keeps no cell for itself.
type areaWidget struct{}

func newArea(widget.Context) (widget.Native, error) { return areaWidget{}, nil }

func (areaWidget) Insets() sema.Thickness { return sema.Thickness{} }

// Measure is only reached when the element is empty, and an empty area is no cells.
func (areaWidget) Measure(_, _ int) (int, int) { return 0, 0 }

func (areaWidget) Draw(cell.Canvas) {}
