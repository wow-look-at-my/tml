# Live regions

A `tml.Live` draws a view inline, under the shell prompt, and repaints it in place. It does not take the alternate screen. Other output goes above the region one time and then scrolls into the terminal's own scrollback. A download manager shows its progress this way. The log grows, and a small live block stays at the bottom.

```go
live := tml.NewLive(view, func() tml.Props { return snapshot() }, tml.LiveOptions{})
if err := live.Start(); err != nil {
	return err
}
live.Printf("done %s", name)  // above the region, one time
cmd.Stderr = live             // a child's stderr, line by line
...
err := live.Stop()            // pending lines print, the region goes, the cursor comes back
```

## What the host supplies

- **The view.** Any loaded `*tml.View`. The region lays it out with `View.RenderFit`. As a result, it takes only the rows its content uses. `LiveOptions.MaxHeight` caps it. The default cap is the terminal's height.
- **The props function.** The region calls it on its own goroutine at each repaint, every `LiveOptions.Interval` (100 ms by default). Workers keep their state as they like, for example in atomics. The function reads a snapshot. Nothing else calls it. `Refresh` repaints before the next interval.
- **The lines.** `Println`, `Printf` and `Write` are safe from any goroutine. `Write` holds a partial line until its newline arrives. It drops carriage returns, so a child that draws its own progress cannot break a line in pieces. The lines print in the order they arrive, at the next repaint.

## How it ends

- `Stop` prints the lines still pending, including a partial line, removes the region, and shows the cursor again. A second `Stop` returns the same result.
- **SIGINT** gets the same orderly end. The region takes no input, so ctrl+c stays a signal. A child can also own stdin. `Stop` and `Wait` then report `tml.ErrInterrupted`, and `LiveOptions.OnInterrupt` runs after the terminal is restored. The host decides the exit code there.
- **A frame that fails to render** ends the region with that error. The region does not keep its last good frame on screen, because that frame looks like progress that stopped.
- A line printed after the region ended goes straight to the output. A region that never started prints its lines at `Stop`.

`Done` is closed when the region is gone, for a host that selects on it.

## Colour

Style the view with a theme, as with any other view. The region runs on Bubble Tea, and Bubble Tea reduces each colour to what the terminal supports. `NO_COLOR` removes colour and keeps bold and faint. `TERM=dumb` removes all styling.

A styled row usually picks its style from its data. A named style takes an expression, so a state field selects the style:

```xml
<Text style="label.{state}">{label}</Text>
```

## Not a terminal

The region is for a terminal. When stdout is a pipe or a file, a host prints plain lines instead and does not start a region. The region cannot decide this, because only the host knows what the plain form of its display is.

## The worked example

`examples/downloads` is a mock download manager. Its block has a header, an optional status line, one row per active transfer and a TOTAL row. The rows are a `Grid` with fixed columns, so they line up. On a narrow terminal, `collapse` drops the bar first, then the speed, the ETA and the sizes. See `docs/language.md` for `collapse` and for track ranges. `./build/downloads` runs it. `./build/downloads -frame -width 60` prints one frame without a terminal.

## Driving and inspection

`Start` builds its program with `tml.NewProgram`. As a result, a region is inspectable and drivable like any other tml program. The one-frame height matters to the inspector: `tml frame` reports the rows the region used, not the whole terminal.
