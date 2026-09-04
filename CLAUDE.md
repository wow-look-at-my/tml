# tml — Terminal Markup Language

XML-based declarative language for defining reusable terminal components: typed properties, slots, layout panels and
themes. XAML-flavoured. Compiles to a laid-out tree rendered with Lip Gloss and dropped into a Bubble Tea `View()`.

TML solves layout constraints and produces rects; Lip Gloss owns styling and compositing. TML emits no ANSI and does no
text wrapping.

Width is `Options.Measure`, defaulting to `lipgloss.Width`. A terminal draws a ZWJ emoji in 2 cells or 6 depending on
whether it agreed to mode 2027, so a host that had that conversation supplies its own answer and this view's geometry
agrees with the rest of its screen. It governs layout; Lip Gloss still measures internally while it paints.

## Build & Test

```bash
go-toolchain
```

Handles module tidying, vet, tests with coverage, and the build. Never run a bare `go` command, and never pipe or redirect
`go-toolchain` output.

## Charm dependencies

Target the v2 line via the `charm.land/*` module paths, not `github.com/charmbracelet/*`. The old paths still resolve on
the proxy but declare a different module path and fail the build. Requires Go >= 1.25.0.

- `charm.land/lipgloss/v2` — styling, measurement, compositing
- `charm.land/bubbletea/v2` — `Model.View()` returns a `tea.View`, not a string
- `charm.land/bubbles/v2` — every component is `Update(tea.Msg) (Model, tea.Cmd)` + `View() string` + `SetWidth(int)`

## Language

- One component per file, rooted at `<Component>` (or one theme, rooted at `<Theme>`); a nested `<Component>` is an error —
  sharing is `<Import>`, never nesting. DataTemplates may still be nested in their file's component.
- Strict XML 1.1 — every file opens with the XML declaration. Inherited from xml-validator, see below.
- `<Slot>` declares and marks the insertion point in one element; its children are fallback content.
- Call sites fill named slots with XAML property-element syntax: `<Card.actions>…</Card.actions>`.
- Expressions are `{name}`, `{name.path}`, `{theme.token}`, `{not name}`, interpolated in text and attribute values.
  Unknown names are a hard error at analysis time. No arithmetic, no calls.
- `itemsSource`/`itemTemplate` on any element repeat a `<DataTemplate>` over a list the host supplies, so a repeated thing
  is a widget rather than a line somebody else already drew. An item's fields are the template's property values, by name;
  a field nobody declared is an error rather than a blank cell.
- See docs/language.md for the full grammar and the built-in element reference.

## Layout

Two-pass measure/arrange. Implemented panels: `Stack`, `Grid`, `Canvas`, `Box`, `Text`, `Spacer`. Sizing is `auto`, fixed
cells, or `*` star share, and a star size propagates upward so an auto-sized ancestor cannot collapse it.

`Grid` declares tracks and each child declares placement through attached properties (`Grid.row`, `Grid.column`, and the
two span variants). Track solving is fixed, then auto, then star. Grid children are composited through lipgloss layers
rather than joined, since they sit at coordinates.

`Canvas` positions children freely through `Canvas.x`, `Canvas.y` and `Canvas.anchor`, and fills whatever it is offered
rather than shrinking. A widget can name its own default anchor, which is how `<Popup>` centres itself.

`Text` wraps by default, and `overflow="clip"` or `overflow="ellipsis"` cuts each line at the width instead. A clipped
Text keeps its line count however narrow it gets, so a card that holds a log tail keeps its height when one line runs
long. Clipping happens in `render`, because `MaxWidth` cuts without marking the cut.

`Dock` is NOT implemented. It is deliberately absent from `sema.Builtins`, so using it is an unknown-element error rather
than a silent blank. Add a panel to that list only once it lays out.

## Widgets

Panels solve constraints; widgets draw. `widgets/` is the library — Border, Popup, Scrollbox, Button, Textbox, Checkbox,
Radio, List, Table, Image, Rule, ProgressBar, Spinner, Sparkline, Badge — registered by default, dropped by
`Options.Bare`, and overridable by name. Nothing in it holds state.

Containers go through the public `widget.Composer`/`Arranger` seam, not through the engine, so a widget written outside
the language is not a second-class one. Do not special-case a library widget inside `layout/`.

- docs/widgets.md — the library reference, interaction, and the interfaces a widget opts into.
- docs/images.md — the kitty/iterm/mosaic/link ladder, sizing against cell aspect, transparency.

## Interaction

`id` and `action` on any element; `view.UI().Update(msg)` turns a Bubble Tea message into `Activated`, `FocusMoved` and
`Scrolled` events carrying that id and action. Layout asks the UI for each element's state before measuring, then
publishes where everything landed, so a click resolves against the frame the user is looking at.

The focus ring is the widgets that take focus; the pointer also reaches anything else with an id, which is what lets the
wheel find a `Scrollbox`. A widget that implements `Focusable` and refuses is disabled and reachable by neither.

A pointer event carries `X`/`Y` inside the control it hit, `-1` from the keyboard. That is how a host tells one row of a
`List` from another: a multi-part widget is still one element to the ring, and sub-targets are deliberately not a thing.

`UI().Target(id)` reads an element back out of the last frame. A scrolling region also reports its `Scroll` position and
maximum there, because how far the content runs depends on how it wrapped — so following a growing transcript is asking
for a big offset and reading back where it stopped.

Content too long to lay out goes through `Scrollbox`'s `contentHeight`: the host hands over the rows on screen and says
how many there are in all, which takes a 10 000-row transcript from 758 ms a frame to a flat 7.2 ms. See docs/widgets.md.

## Parsing

Delegated to `github.com/wow-look-at-my/xml-validator` — strict XML 1.1, namespace validation, UTF-8/BOM rejection and
line/col positions on elements and attributes alike, so an attribute-scoped diagnostic points at the attribute rather
than at the element several columns to its left. TML contains no XML parser. XSD self-validation is deliberately unused: a template body holds
arbitrary user component names, which `processContents="strict"` cannot express.

## Gotchas that decide the design

- `Style.Inherit` skips padding and margin, so the style cascade lives in TML's own model, not in Lip Gloss.
- Layer z-index is global, not parent-scoped, and equal z values sort unstably — the renderer hands out distinct
  increasing z in document order.
- docs/lipgloss-contract.md covers both in full, pinned by `render/lipgloss_contract_test.go`.

## Project structure

The root is the importable package, so a Bubble Tea app imports `github.com/wow-look-at-my/tml` directly and the binary
lives under `cmd/`.

- `tml.go` — the façade: `Load`, `View`, `Props`, `Options`
- `ui.go` — the focus ring, the pointer, and the events a host reads
- `syntax/` — AST, loader, `<Import>` resolution, diagnostics
- `sema/` — property types, values, expressions, slot and component analysis, expansion
- `layout/` — constraints, measure/arrange, the panels
- `style/` — theme tokens, named styles, resolution to `lipgloss.Style`
- `render/` — composition of a laid-out tree into terminal output
- `widget/` — the widget seam: registry, typed attributes, the bubbles adapter
- `widgets/` — the built-in widget library
- `cli/` — cobra commands, one self-registering command per file: file commands (`check`, `tree`, `render`, `inspect`) and live-program commands (`query`, `ids`, `list`, `serve`, …)
- `cmd/tml/` — the one CLI binary
- `inspect/` — the per-element inspection protocol, its socket and HTTP transports, and the browser inspector's page
- `examples/dashboard/` — a Bubble Tea program whose whole view is TML
- `examples/gallery/` — every library widget on one interactive screen
- `examples/agent/` — a mock coding agent, the proving ground; see docs/agent.md for what it loads on and what it proved
- `tools/shots/` — the ttyd screenshot capture, its shot list, and the index page the site serves
- `tools/inspector-check/` — the CI check that drives the inspector against a running program, over the socket and in a browser

## Inspection

Answers questions about the frame `Render` painted, one element at a time: rectangle, content size, clip, scroll, focus,
and drawn text by id. **There is nothing to wire and nothing to turn on.** Every `Load` adopts its view into the
process's one inspector and opens a socket — `$XDG_RUNTIME_DIR/tml/<pid>.sock`, in a 0700 directory, with no variable
set and no flag passed. `TML_INSPECT_SOCKET` overrides the path and `TML_INSPECT_DIR` the directory; neither is a
switch, and a view that cannot be served fails `Load` rather than running unreachable. `tml query` finds the program by
dialling what is in that directory (`tml list` names them all).

`tml.NewProgram` is the one thing a host does: driving needs a running `*tea.Program`, which cannot exist before the host
builds it. A program built with `tea.NewProgram` is readable and not drivable, and `drivable.go` takes it down after
`DriveGrace` rather than leave the debugger half working — `testdata/undrivable` is the program that proves the guard
still fires. A model that caches its frame must invalidate on `tml.RepaintMsg` — the one line a host writes, and a
restyle fails by name when it is missing. `tml serve` opens a browser inspector on the same protocol, where a click
selects and a drag rewrites the element's attributes as a real layout override. `tml capture` writes that same page over
a frozen frame as one self-contained HTML file, from a running program or from a document laid out at a given size:
`inspect.Snapshot` asks the questions the page asks on load, so a capture cannot report what the inspector does not. The
driving controls are absent there, because a keystroke and a restyle both need the program. See docs/inspector.md.

A test waits on the screen rather than sleeping: `query --await REGEX` and `--await-gone` block until the field matches,
failing with what the element last drew, and `frame --since` blocks until the next paint. `frame --max-width` measures
the widest line in display cells, which is the number that catches a region painting past its own edge.

## Hot reload

`tml.Watch(ctx, dir, entry, opts, onChange)` reloads on change, by polling modification times rather than subscribing to
filesystem events — editors save by write-then-rename, which replaces the inode an event watcher holds and drops the change
that mattered. A failed reload is delivered to `onChange` as an error and the previous view is left alone; showing it is
the caller's job, and hiding it defeats the point.

## Testing

Tests run in parallel by default. The inspector, the drivable guard and the socket environment belong to the PROCESS, so a
test that resets or reads any of them calls `t.Serial()` first — see `inspector_test.go` and `cli/await_test.go`. Without it
the frame a test waits on is whatever another test painted, which fails as "the program has not painted a frame yet".

Golden files live in `testdata/`. An empty golden seeds itself from the run and then FAILS, so a broken renderer can never
bless its own output. Read the diff before trusting a reseeded golden.

Either example renders one frame without a terminal with `-frame`, which is how they are checked headlessly. The
gallery's goldens are the widget library's regression net: they are stripped of colour to stay readable, and the test
pins the terminal to one with no graphics protocol so the image lands on half-blocks wherever it runs.

`tools/shots` photographs the examples in a REAL terminal — ttyd on a PTY, xterm.js in Chromium — and CI publishes the
pictures per branch to a buildhost site the README embeds. Goldens are the mechanical check; the pictures cover what a
string cannot show, like which image protocol the terminal took. See docs/screenshots.md.

`tools/inspector-check/check.mjs` is the end-to-end net for the inspection layer: it runs `build/agent` under a pty,
queries and drives it through `tml`, then drives the browser inspector with a pointer and reads the result back off
the socket. CI runs it after the screenshots, on the browser that step already installed.
