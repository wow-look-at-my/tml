// table_shape.go holds shape.

package widgets

// shape is a table laid out for one width: the columns it shows, and how wide each is.
type shape struct {
	// shown is the indices of the columns on screen, in order.
	shown  []int
	widths []int
}
