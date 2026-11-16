// grid.go holds the grid code.

package layout

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/syntax"
)

// Grid places children on a track grid. Tracks are declared on the Grid and placement is declared on each child
var attachedGrid = set.Of("Grid.row", "Grid.column", "Grid.rowSpan", "Grid.columnSpan")

// placement is where a child sits on the grid. It defaults to the leading cell, spanning a single track each way.
type placement struct {
	row, column int
	rowSpan     int
	columnSpan  int
}

// readPlacement validates and reads a child's attached properties. parent is the element the attached properties must
func readPlacement(box *Box, parent string) (placement, error) {
	p := placement{rowSpan: 1, columnSpan: 1}
	for name, raw := range box.attrs {
		owner, _, dotted := strings.Cut(name, ".")
		if !dotted {
			continue
		}
		if owner != parent {
			return p, &syntax.Error{Pos: box.Pos, Message: fmt.Sprintf(
				"attached property %q only applies to a child of <%s>, but this is inside <%s>", name, owner, parent)}
		}
		if !attachedGrid.Contains(name) {
			return p, &syntax.Error{Pos: box.Pos, Message: fmt.Sprintf(
				"unknown attached property %q; want Grid.row, Grid.column, Grid.rowSpan or Grid.columnSpan", name)}
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return p, &syntax.Error{Pos: box.Pos, Message: fmt.Sprintf(
				"%s must be a whole number, got %q", name, raw)}
		}
		switch name {
		case "Grid.row":
			p.row = n
		case "Grid.column":
			p.column = n
		case "Grid.rowSpan":
			p.rowSpan = max(1, n)
		case "Grid.columnSpan":
			p.columnSpan = max(1, n)
		}
	}
	return p, nil
}

// track is one grid track. An auto or star track can carry a range, written `*[8..32]`. A zero max is no upper bound.
type track struct {
	sema.Length
	min, max int
}

// clamp keeps a size inside the track's range.
func (t track) clamp(size int) int {
	if t.max > 0 {
		size = min(size, t.max)
	}
	return max(size, t.min)
}

// parseTracks reads a track list of auto, fixed and star entries. An empty list is a single auto track, so a Grid with no columns
func parseTracks(spec string) ([]track, error) {
	if strings.TrimSpace(spec) == "" {
		return []track{{Length: sema.Length{Kind: sema.LengthAuto}}}, nil
	}
	var tracks []track
	for _, field := range strings.Split(spec, ",") {
		t, err := parseTrack(strings.TrimSpace(field))
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// parseTrack reads one entry: a length, then an optional `[min..max]` range where either end can be left out.
func parseTrack(field string) (track, error) {
	spec, bounds, ranged := strings.Cut(field, "[")
	length, err := sema.ParseLength(strings.TrimSpace(spec))
	if err != nil {
		return track{}, err
	}
	t := track{Length: length}
	if !ranged {
		return t, nil
	}
	if length.Kind == sema.LengthCells {
		return track{}, fmt.Errorf("track %q: a fixed track has no range", field)
	}
	inner, closed := strings.CutSuffix(bounds, "]")
	low, high, dotted := strings.Cut(inner, "..")
	if !closed || !dotted {
		return track{}, fmt.Errorf("track %q: a range is written [min..max]", field)
	}
	if t.min, err = rangeEnd(low); err != nil {
		return track{}, fmt.Errorf("track %q: %w", field, err)
	}
	if t.max, err = rangeEnd(high); err != nil {
		return track{}, fmt.Errorf("track %q: %w", field, err)
	}
	if t.max > 0 && t.min > t.max {
		return track{}, fmt.Errorf("track %q: the minimum is larger than the maximum", field)
	}
	return t, nil
}

// rangeEnd reads one end of a track range. An empty end is no bound.
func rangeEnd(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("a range end must be a whole number, got %q", raw)
	}
	return n, nil
}

// parseCollapse reads `collapse`: column indices, in the order they give way when the grid is too narrow for all of them.
func parseCollapse(spec string, columns int) ([]int, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	seen := set.New[int]()
	var order []int
	for _, field := range strings.Split(spec, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || n < 0 || n >= columns {
			return nil, fmt.Errorf("%q is not a column index from 0 to %d", strings.TrimSpace(field), columns-1)
		}
		if seen.Contains(n) {
			return nil, fmt.Errorf("column %d is named twice", n)
		}
		seen.Add(n)
		order = append(order, n)
	}
	return order, nil
}

// initGrid parses the track lists and reads every child's placement. Tracks are widened to cover a child placed past
func initGrid(box *Box) error {
	cols, err := parseTracks(box.attrs["columns"])
	if err != nil {
		return &syntax.Error{Pos: box.Pos, Message: fmt.Sprintf("<Grid> columns: %v", err)}
	}
	rows, err := parseTracks(box.attrs["rows"])
	if err != nil {
		return &syntax.Error{Pos: box.Pos, Message: fmt.Sprintf("<Grid> rows: %v", err)}
	}

	for _, child := range box.Children {
		child.place, err = readPlacement(child, "Grid")
		if err != nil {
			return err
		}
		for len(cols) < child.place.column+child.place.columnSpan {
			cols = append(cols, track{Length: sema.Length{Kind: sema.LengthAuto}})
		}
		for len(rows) < child.place.row+child.place.rowSpan {
			rows = append(rows, track{Length: sema.Length{Kind: sema.LengthAuto}})
		}
	}
	box.cols, box.rows = cols, rows
	if box.collapse, err = parseCollapse(box.attrs["collapse"], len(cols)); err != nil {
		return &syntax.Error{Pos: box.Pos, Message: fmt.Sprintf("<Grid> collapse: %v", err)}
	}
	return nil
}

func (e *Engine) measureGrid(box *Box, inner Constraints) Size {
	gap := box.Gap()
	for _, child := range box.Children {
		e.measure(child, inner)
	}

	// An auto track is as wide as the widest child confined to it. A child that spans several tracks is left out of this:
	box.autoWidths = fixedSizes(box.cols)
	box.autoHeights = fixedSizes(box.rows)
	for _, child := range box.Children {
		if child.place.columnSpan == 1 && box.cols[child.place.column].Kind == sema.LengthAuto {
			box.autoWidths[child.place.column] = max(box.autoWidths[child.place.column], child.desired.W)
		}
	}
	clampAuto(box.cols, box.autoWidths)

	shown := box.visibleColumns(inner.MaxW)
	e.measureInTracks(box, inner, shown, gap)
	for _, child := range box.Children {
		if child.place.rowSpan == 1 && box.rows[child.place.row].Kind == sema.LengthAuto {
			box.autoHeights[child.place.row] = max(box.autoHeights[child.place.row], child.desired.H)
		}
	}
	clampAuto(box.rows, box.autoHeights)

	content := Size{
		W: shownSum(box.autoWidths, shown, gap),
		H: shownSum(box.autoHeights, allShown(len(box.rows)), gap),
	}
	// A star track divides whatever the grid is given, so the grid asks for all the space available on that axis.
	if hasStar(box.cols, shown) {
		content.W = max(content.W, inner.MaxW)
	}
	if hasStar(box.rows, allShown(len(box.rows))) {
		content.H = max(content.H, inner.MaxH)
	}
	return content
}

// measureInTracks measures each child again against the columns it sits in, once the grid knows its width. A row's
// height then comes from text wrapped at the width its cell has, not at the width of the whole grid.
func (e *Engine) measureInTracks(box *Box, inner Constraints, shown []bool, gap int) {
	if inner.MaxW <= 0 {
		return
	}
	widths := solveTracks(box.cols, box.autoWidths, shown, inner.MaxW, gap)
	for _, child := range box.Children {
		width := span(widths, shown, child.place.column, child.place.columnSpan, gap)
		// A child in hidden columns has no cells, and no cells is no bound to a measure.
		if width <= 0 || width == inner.MaxW {
			continue
		}
		e.measure(child, Constraints{MaxW: width, MaxH: inner.MaxH})
	}
}

func (e *Engine) arrangeGrid(box *Box) {
	gap := box.Gap()
	shownCols := box.visibleColumns(box.Content.W)
	shownRows := allShown(len(box.rows))
	widths := solveTracks(box.cols, box.autoWidths, shownCols, box.Content.W, gap)
	heights := solveTracks(box.rows, box.autoHeights, shownRows, box.Content.H, gap)

	x := offsets(widths, shownCols, gap)
	y := offsets(heights, shownRows, gap)

	for _, child := range box.Children {
		// A child in collapsed columns only is arranged into nothing, and nothing is what the renderer draws for it.
		e.arrange(child, Rect{
			X: x[child.place.column],
			Y: y[child.place.row],
			W: span(widths, shownCols, child.place.column, child.place.columnSpan, gap),
			H: span(heights, shownRows, child.place.row, child.place.rowSpan, gap),
		})
	}
}

// visibleColumns drops columns in `collapse` order until the rest fit in the width. A star column counts its minimum. An
// unbounded width drops nothing.
func (b *Box) visibleColumns(available int) []bool {
	shown := allShown(len(b.cols))
	if available <= 0 {
		return shown
	}
	gap := b.Gap()
	need := func() int {
		total, count := 0, 0
		for i, t := range b.cols {
			if !shown[i] {
				continue
			}
			count++
			if t.Kind == sema.LengthStar {
				total += t.min
			} else {
				total += b.autoWidths[i]
			}
		}
		return total + gap*max(0, count-1)
	}
	for _, column := range b.collapse {
		if need() <= available {
			break
		}
		shown[column] = false
	}
	return shown
}

// solveTracks turns the shown tracks into cell counts. A fixed track takes what it asked for, an auto track what it
// measured, and the star tracks share the rest by weight. A hidden track is zero wide.
func solveTracks(tracks []track, measured []int, shown []bool, available, gap int) []int {
	sizes := make([]int, len(tracks))
	used, count := 0, 0
	var stars []int
	for i, t := range tracks {
		if !shown[i] {
			continue
		}
		count++
		if t.Kind == sema.LengthStar {
			stars = append(stars, i)
			continue
		}
		sizes[i] = measured[i]
		used += sizes[i]
	}
	shareStars(tracks, stars, sizes, max(0, available-used-gap*max(0, count-1)))
	return sizes
}

// shareStars divides leftover by weight. A track that lands outside its range keeps the nearer bound, and the other
// tracks divide what is left again. The last open track takes the rounding, so an unbounded row of stars fills exactly.
func shareStars(tracks []track, open, sizes []int, leftover int) {
	for len(open) > 0 {
		weight := 0
		for _, i := range open {
			weight += tracks[i].Weight
		}
		remaining, remainingWeight := leftover, weight
		for _, i := range open {
			if remainingWeight == tracks[i].Weight {
				sizes[i] = remaining
			} else {
				sizes[i] = leftover * tracks[i].Weight / weight
			}
			remaining -= sizes[i]
			remainingWeight -= tracks[i].Weight
		}
		var next []int
		for _, i := range open {
			if bounded := tracks[i].clamp(sizes[i]); bounded != sizes[i] {
				sizes[i] = bounded
				leftover = max(0, leftover-bounded)
				continue
			}
			next = append(next, i)
		}
		if len(next) == len(open) {
			return
		}
		open = next
	}
}

// clampAuto keeps each auto track's measured size inside its range.
func clampAuto(tracks []track, measured []int) {
	for i, t := range tracks {
		if t.Kind == sema.LengthAuto {
			measured[i] = t.clamp(measured[i])
		}
	}
}

// offsets is where each track starts. A hidden track starts where the next shown one does and takes no gap.
func offsets(sizes []int, shown []bool, gap int) []int {
	out := make([]int, len(sizes))
	at := 0
	for i, size := range sizes {
		out[i] = at
		if shown[i] {
			at += size + gap
		}
	}
	return out
}

// span is the size of a run of tracks, including the gaps swallowed between the shown ones.
func span(sizes []int, shown []bool, start, count, gap int) int {
	total, seen := 0, 0
	for i := start; i < start+count && i < len(sizes); i++ {
		if !shown[i] {
			continue
		}
		if seen > 0 {
			total += gap
		}
		total += sizes[i]
		seen++
	}
	return total
}

// fixedSizes seeds the track sizes with the ones already known from the declaration.
func fixedSizes(tracks []track) []int {
	sizes := make([]int, len(tracks))
	for i, t := range tracks {
		if t.Kind == sema.LengthCells {
			sizes[i] = t.Cells
		}
	}
	return sizes
}

// shownSum is the size of the shown tracks and the gaps between them.
func shownSum(sizes []int, shown []bool, gap int) int {
	total, count := 0, 0
	for i, size := range sizes {
		if shown[i] {
			total += size
			count++
		}
	}
	return total + gap*max(0, count-1)
}

func allShown(n int) []bool {
	shown := make([]bool, n)
	for i := range shown {
		shown[i] = true
	}
	return shown
}

func hasStar(tracks []track, shown []bool) bool {
	for i, t := range tracks {
		if shown[i] && t.Kind == sema.LengthStar {
			return true
		}
	}
	return false
}
