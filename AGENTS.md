# tml — Terminal Markup Language

XML-based declarative language for defining reusable terminal components: typed properties, slots, layout panels and themes. XAML-flavoured. Compiles to a laid-out tree, painted on a grid of cells by TML's own renderer. Run on the terminal by TML's own program loop.

TML owns the whole path: layout produces rects, `render` paints cells from them, and `term` writes the cells that changed. There is no Bubble Tea, no Lip Gloss and no string compositing. See docs/renderer.md and docs/program.md.

Width is `Options.Measure`, defaulting to the Unicode width of a grapheme cluster. A terminal draws a ZWJ emoji in multiple cells or in a higher count, which depends on whether it agreed to mode 2027. A host that had that conversation supplies its own answer. This view's geometry then agrees with the rest of its screen. The measure governs layout and paint alike.

## Build & Test

```bash
go-toolchain
```

Handles module tidying, vet, tests with coverage, and the build. Never run a bare `go` command, and never pipe or redirect `go-toolchain` output.

## Dependencies

The renderer and the runtime are first-party. Do not add a Charm module (`charm.land/*`, `github.com/charmbracelet/*`) back, for a test either: `internal/vt` is the terminal a test replays output through.

- `github.com/clipperhouse/displaywidth` — grapheme clusters and their widths, behind `cell.Measure`
- `golang.org/x/sys` — raw mode, the window size and the input poll, behind `term`

## Language

- One component per file, rooted at `<Component>` (or one theme, rooted at `<Theme>`). A nested `<Component>` is an error — sharing is `<Import>`, not nesting. DataTemplates may still be nested in their file's component.
- Strict XML 1.1 — every file opens with the XML declaration. Inherited from xml-validator.
- `<Slot>` declares and marks the insertion point in one element. Its children are fallback content.
- Call sites fill named slots with XAML property-element syntax: `<Card.actions>…</Card.actions>`.
- Unknown names are a hard error at analysis time. No arithmetic, no calls.
- A theme is the only place a colour value is written: `<Color name for light dark>`, where `for` says what the colour is used for. A control, a style and a widget refer to it as `{theme.name}`. A colour written in place fails the frame. `style/base.tml` holds the colours the library draws with. See docs/language.md §Colours.
- `itemsSource`/`itemTemplate` on any element repeat a `<DataTemplate>` over a list the host supplies. A repeated thing is therefore a widget rather than a line somebody else already drew. An item's fields are the template's property values, by name. A field nobody declared is an error rather than a blank cell.
- See docs/language.md for the full grammar and the built-in element reference.

## Layout

Two-pass measure/arrange. Implemented panels. Sizing is `auto`, fixed cells, or `*` star share, and a star size propagates upward so an auto-sized ancestor cannot collapse it.

`Grid` declares tracks and each child declares placement through attached properties (`Grid.row`, `Grid.column`, and the span variants). Track solving is fixed, then auto, then star. Grid children sit at coordinates, and a later child covers an earlier one where their cells overlap.

A `Grid` track takes a range (`*[8..32]`), and `collapse` names the columns that give way, in order, when the grid is too narrow. A hidden column takes no width and no gap, and `render` skips a grid child with no area. See docs/language.md.

`Canvas` positions children freely, and fills whatever it is offered rather than shrinking. A widget can name its own default anchor, which is how `<Popup>` centres itself.

`Text` wraps by default. `path` cuts in the middle and keeps the last path element. A clipped Text keeps its line count however narrow it gets. A card that holds a log tail therefore keeps its height when one line runs long. Layout counts a wrapped Text's rows with the same `cell.Measure.Wrap` call the painter draws with.

`Dock` is NOT implemented. It is deliberately absent from `sema.Builtins`, so using it is an unknown-element error rather than a silent blank. Add a panel to that list only once it lays out.

## Widgets

Panels solve constraints. Widgets draw. `widgets/` is the library: Border, Popup, Scrollbox, Button, Textbox, Checkbox, Radio, List, Menu, Table, Image, Rule, ProgressBar, Spinner, Sparkline, Badge. Default registers each one, dropped by `Options.Bare`, and overridable by name. Nothing in it holds state. A `widget.Templated` widget is drawn as another element: a `Button` is a real `<Border>` box in the tree around its content (`layout/template.go`).

Containers go through the public `widget.Composer`/`Arranger` seam, not through the engine, so a widget written outside the language is not a second-class one. Do not special-case a library widget inside `layout/`.

- docs/widgets.md — the library reference, interaction, and the interfaces a widget opts into.
- docs/images.md — the kitty/iterm/mosaic/link ladder, sizing against cell aspect, transparency.

## Interaction

A widget takes a `tooltip`. `UI` counts the inputs. The program runs the wait, and `View.paint` draws the strip over the frame. A tooltip is no element: it takes no input and has no target.

Any element takes an `id` and an `action`. Layout asks the UI for each element's state before it measures. It then publishes where everything landed. A click therefore resolves against the frame the user looks at.

A focused widget that implements `widget.Keyed` takes keys before the ring does. A `Textbox` edits itself this way and holds nothing: the edit comes back as a `Changed` event. The host hands the value and the cursor back as attributes.

The focus ring is the widgets that take focus. The pointer also reaches anything else with an id, which is what lets the wheel find a `Scrollbox`. A widget that implements `Focusable` and refuses is disabled and reachable by neither.

A pointer event carries `X`/`Y` inside the control it hit, `-1` from the keyboard. That is how a host tells one row of a `List` from another. A multi-part widget is still one element to the ring, and sub-targets are deliberately not a thing. Turning that `Y` into a row index assumes a row is a line. A `Table` guarantees that only under `wrap="false"`. A cell too wide for its column otherwise takes the next line, and puts every row below it out of step.

`UI().Target(id)` reads an element back out of the last frame. A scrolling region also reports its `Scroll` position and maximum there, because how far the content runs depends on how it wrapped. To follow a growing transcript, ask for a big offset and read back where it stopped.

Content too long to lay out goes through `Scrollbox`'s `contentHeight`. The host hands over the rows on screen, and says how many there are in all. That takes a 10 000-row transcript from 758 ms a frame to a flat 7.2 ms. See docs/widgets.md.

A list of data-template items too long to lay out scrolls through `tml.Window` (`window.go`). `View.ItemHeight` measures one item with the engine. As a result, the host keeps no row count of its own. The window owns the position, both ends and the follow. Its `Slice` is the items and the `Scrollbox` offset. See docs/widgets.md.

## Parsing

Delegated to `github.com/wow-look-at-my/xml-validator`: strict XML 1.1, namespace validation, UTF-8/BOM rejection, and line and column positions on elements and attributes alike. An attribute-scoped diagnostic therefore points at the attribute, not at the element several columns to its left. TML contains no XML parser.

The `<Template>` body is deliberately unchecked. It names components the file imported and widgets the host bound. This validator allows only `processContents="strict"`, which demands a global declaration for every element it matches. `sema/` is the authority on the whole file. Other rules the schema cannot reach: a root in the incorrect namespace, and a style attribute's vocabulary when a `{theme.token}` supplies the value.

## Rendering

- `cell/` is the drawing model.
- `render.Paint` draws every box on the rect layout gave it, so the screen and the published geometry cannot disagree. Text style is inherited, `bg` fills the padding box, and a `Canvas` or `Grid` child covers what is beneath it.
- A widget draws on a `cell.Canvas` (`widget.Native.Draw`). The engine paints a composer's children itself, clipped to the composer's insets. Never return a string from a widget.
- `term.Screen` diffs frames and writes inside one synchronized update. `term.Decoder` reads keys, the pointer and pastes.
- docs/renderer.md covers the model. `widget/widgettest` draws a widget alone for a test.

## The program

`tml.Program` (`program.go`) owns the terminal and the loop. An input value goes to the app first, then to the focus ring, and the ring's `Event`s come back through `Update`. The program steps animations itself and stops its clock when nothing moves. An unconsumed ctrl+c ends it with `ErrInterrupted`. `Update` calls `p.Unchanged()` to skip the frame of a message that changed nothing. See docs/program.md.

## Project structure

The root is the importable package, so a host imports `github.com/wow-look-at-my/tml` directly and the binary lives under `cmd/`.

- `tml.go` — the façade
- `program.go` — `Program`, the loop that owns the terminal, and the `App` a host implements
- `ui.go` — the focus ring, the pointer, and the events a host reads
- `cell/` — styles, colours, the cell buffer, the canvas, text measure and wrap, the SGR encoder
- `term/` — raw mode and size, the input decoder, the frame-diffing screen writer
- `input/` — the key, pointer, paste and resize values
- `internal/vt/` — the small terminal a test replays output through
- `syntax/` — AST, loader, `<Import>` resolution, diagnostics
- `sema/` — property types, values, expressions, slot and component analysis, expansion
- `layout/` — constraints, measure/arrange, the panels
- `style/` — theme colours and tokens, the base theme, named styles, resolution to a cell style and a box model
- `render/` — the painter: a laid-out tree onto a cell buffer
- `widget/` — the widget seam: registry, typed attributes, the `View() string` adapter, and `widgettest`
- `widgets/` — the built-in widget library
- `cli/` — cobra commands, one self-registering command per file: file commands and live-program commands
- `cmd/tml/` — the CLI binary
- `inspect/` — the per-element inspection protocol, its socket and HTTP transports, and the browser inspector's page
- `examples/dashboard/` — a program whose whole view is TML and whose host is a few lines of state
- `examples/gallery/` — every library widget on one interactive screen
- `examples/downloads/` — a mock download manager on a `tml.Live` region, with goldens at widths
- `region.go` — `Live`, the inline region
- `examples/agent/` — a mock coding agent, the proving ground. See docs/agent.md for what it loads on and what it proved
- `tools/shots/` — the Ghostty screenshot capture, its shot list, and the index page the site serves
- `tools/inspector-check/` — the CI check that drives the inspector against a running program, over the socket and in a browser

## Inspection

Answers questions about the frame `Render` painted, one element at a time: rectangle, content size, clip, scroll, focus, and drawn text by id. **There is nothing to wire and nothing to turn on.** Every `Load` adopts its view into the process's inspector, and opens a socket. That socket is `$XDG_RUNTIME_DIR/tml/<pid>.sock`, in a 0700 directory, with no variable set and no flag passed. `TML_INSPECT_SOCKET` overrides the path and `TML_INSPECT_DIR` the directory. Neither is a switch.

A view that cannot be served fails `Load` rather than running unreachable. `tml query` finds the program by dialling what is in that directory (`tml list` names them all).

`tml.NewProgram` is the thing a host does. Driving needs a running `*tml.Program`. A host that paints from a loop of its own, with `View.Render`, is readable and not drivable. `drivable.go` takes such a program down after `DriveGrace`, rather than leave the debugger half working. `testdata/undrivable` is the program that proves the guard still fires. An app that caches what `Props` returns must drop that cache on `tml.RepaintMsg`.

A restyle fails by name when no new frame follows it. `tml serve` opens a browser inspector on the same protocol. There a click selects, and a drag rewrites the element's attributes as a real layout override. `tml capture` writes that same page over a frozen frame as one self-contained HTML file. The frame comes from a running program, or from a document laid out at a given size.

`inspect.Snapshot` asks the questions the page asks on load. A capture cannot report what the inspector does not. The driving controls are absent there, because a keystroke and a restyle both need the program. See docs/inspector.md.

The page decides NOTHING about layout. Which element covers a cell is `op=hits`, `inspect.At`'s answer for every cell in one response. A capture therefore resolves a click by lookup, not by a second hit test of its own. Its scripts are TypeScript under `inspect/ui/src`. [ts0](https://github.com/wow-look-at-my/ts0) (`pnpm build`) compiles them into the committed `inspect/ui/inspector.js` that `go:embed` reads. CI recompiles that file and fails on a diff.

A test waits on the screen rather than sleeping. `query --await REGEX` and `--await-gone` block until the field matches, and fail with what the element last drew. `frame --since` blocks until the next paint. `frame --max-width` measures the widest line in display cells, which is the number that catches a region painting past its own edge.

## Live regions

`tml.Live` (`region.go`) is an inline `tml.Program` with no input. It pins a view below the log and repaints it from a props function on an interval. `Println`/`Printf`/`Write` queue lines above it. The loop's own goroutine writes them, in the order they were queued, before the frame that follows. SIGINT becomes an orderly close that reports `ErrInterrupted`. `View.DrawFit` lays a view out as tall as its content.

`View.Print` (`print.go`) writes a view to a stream as a document: plain on a pipe, and outside the drivable guard. See docs/live.md.

See docs/live.md.

## Hot reload

`tml.Watch(ctx, dir, entry, opts, onChange)` reloads on change. It polls modification times rather than subscribes to filesystem events. Editors save by write-then-rename, which replaces the inode an event watcher holds, and drops the change that mattered. A failed reload reaches `onChange` as an error. The previous view stays. To show that error is the caller's job.

To hide it defeats the point.

## Testing

Tests run in parallel by default. The inspector, the drivable guard and the socket environment belong to the PROCESS. A test that resets or reads any of them calls `t.Serial()` first. See `inspector_test.go` and `cli/await_test.go`. Without that call, the frame a test waits on is whatever another test painted. It then fails as "the program has not painted a frame yet".

Golden files live in `testdata/`. An empty golden seeds itself from the run and then FAILS. As a result, a broken renderer can never bless its own output. Read the diff before trusting a reseeded golden.

Either example renders one frame without a terminal with `-frame`, which is how they are checked headlessly. The gallery's goldens are the widget library's regression net. They are stripped of colour to stay readable. The test pins the terminal to one with no graphics protocol, so the image lands on half-blocks wherever it runs.

`tools/shots` photographs the examples in a REAL terminal: Ghostty on a virtual display, read back through the inspection socket. CI publishes the pictures per branch to a buildhost site the README embeds. Goldens are the mechanical check. The pictures cover what a string cannot show, like which image protocol the terminal took. See docs/screenshots.md.

`tools/inspector-check/check.mjs` is the end-to-end net for the inspection layer. It runs `build/agent` under a pty, then queries and drives it through `tml`. It then drives the browser inspector with a pointer, and reads the result back off the socket. CI runs it after the screenshots, on a Chromium that Playwright installs.
