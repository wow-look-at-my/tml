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

Two-pass measure/arrange. Panels: `Stack`, `Grid`, `Dock`, `Overlay`, `Box`. Sizing is `auto`, fixed cells, or `*` star
share; attached properties carry panel-specific placement (`Grid.row`, `Dock.side`).

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

- `syntax/` — AST, loader, `<Import>` resolution, diagnostics
- `sema/` — property types, values, expressions, slot and component analysis
- `layout/` — constraints, measure/arrange, the panels
- `style/` — theme tokens, named styles, resolution to `lipgloss.Style`
- `render/` — layer tree construction, compositing, hit testing
- `widget/` — native element registry and the bubbles adapter
- `cmd/` — cobra CLI, one self-registering command per file
