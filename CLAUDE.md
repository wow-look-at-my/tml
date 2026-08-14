# tml — Terminal Markup Language

XML-based declarative language for defining reusable terminal components: typed properties, slots, layout panels and
themes. XAML-flavoured. Compiles to a laid-out tree rendered with Lip Gloss and dropped into a Bubble Tea `View()`.

TML solves layout constraints and produces rects; Lip Gloss owns measurement, styling and compositing. TML emits no ANSI
and does no text wrapping.

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

- One definition per file, rooted at `<Component>` or `<Theme>`; nested `<Component>` elements are file-private helpers.
- Strict XML 1.1 — every file opens with the XML declaration. Inherited from xml-validator, see below.
- `<Slot>` declares and marks the insertion point in one element; its children are fallback content.
- Call sites fill named slots with XAML property-element syntax: `<Card.actions>…</Card.actions>`.
- Expressions are `{name}`, `{name.path}`, `{theme.token}`, `{not name}`, interpolated in text and attribute values.
  Unknown names are a hard error at analysis time. No arithmetic, no calls.
- See docs/language.md for the full grammar and the built-in element reference.

## Layout

Two-pass measure/arrange. Implemented panels: `Stack`, `Grid`, `Box`, `Text`, `Spacer`. Sizing is `auto`, fixed cells, or
`*` star share, and a star size propagates upward so an auto-sized ancestor cannot collapse it.

`Grid` declares tracks and each child declares placement through attached properties (`Grid.row`, `Grid.column`, and the
two span variants). Track solving is fixed, then auto, then star. Grid children are composited through lipgloss layers
rather than joined, since they sit at coordinates.

`Dock` and `Overlay` are NOT implemented. They are deliberately absent from `sema.Builtins`, so using one is an
unknown-element error rather than a silent blank. Add a panel to that list only once it lays out.

## Parsing

Delegated to `github.com/wow-look-at-my/xml-validator` — strict XML 1.1, namespace validation, UTF-8/BOM rejection and
line/col positions. TML contains no XML parser. XSD self-validation is deliberately unused: a template body holds
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
- `syntax/` — AST, loader, `<Import>` resolution, diagnostics
- `sema/` — property types, values, expressions, slot and component analysis, expansion
- `layout/` — constraints, measure/arrange, the panels
- `style/` — theme tokens, named styles, resolution to `lipgloss.Style`
- `render/` — composition of a laid-out tree into terminal output
- `widget/` — host element registry and the bubbles adapter
- `cli/` — cobra commands, one self-registering command per file
- `cmd/tml/` — the CLI binary
- `examples/dashboard/` — a Bubble Tea program whose whole view is TML

## Hot reload

`tml.Watch(ctx, dir, entry, opts, onChange)` reloads on change, by polling modification times rather than subscribing to
filesystem events — editors save by write-then-rename, which replaces the inode an event watcher holds and drops the change
that mattered. A failed reload is delivered to `onChange` as an error and the previous view is left alone; showing it is
the caller's job, and hiding it defeats the point.

## Testing

Golden files live in `testdata/`. An empty golden seeds itself from the run and then FAILS, so a broken renderer can never
bless its own output. Read the diff before trusting a reseeded golden.

`examples/dashboard -frame` renders one frame without a terminal, which is how the example is checked headlessly.
