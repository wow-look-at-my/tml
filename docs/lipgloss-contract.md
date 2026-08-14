# What TML delegates to Lip Gloss, and the traps in it

TML solves layout constraints and produces rects. Lip Gloss owns measurement, styling and compositing. TML emits no ANSI
and does no text wrapping. `render/lipgloss_contract_test.go` pins every behaviour below, so an upstream change fails a
test instead of producing garbled frames.

Target is `charm.land/lipgloss/v2` v2.0.6. The v2 modules moved off the `github.com/charmbracelet/...` path: the old path
still resolves on the proxy, but the served `go.mod` declares `charm.land/lipgloss/v2` and the build fails with a module
path mismatch. Lip Gloss v2 also requires Go >= 1.25.0.

## Measurement

`lipgloss.Width`, `Height` and `Size` count display cells and ignore ANSI, and account for wide runes (`世` is 2 cells).
This is the whole leaf measure pass — TML never counts bytes or runes itself.

`Style.Width(n)` wraps text and pads every line to `n`. It does not truncate. This is what renders a text leaf once
layout has assigned it a rect.

## The cascade does NOT come from Style.Inherit

`Style.Inherit` fills unset fields from a parent, but **deliberately skips padding and margin** — see the explicit
`continue` branches in lipgloss `style.go`. `<Style extends="...">` therefore cannot be `Inherit` alone.

TML resolves the cascade in its own style model and emits one fully-resolved `lipgloss.Style` at the leaf. This is
required anyway: padding and margin change an element's size, so layout needs them at measure time, long before any
`lipgloss.Style` exists.

## Compositing is Compositor, not Canvas

`Canvas` is the low-level cell buffer: `NewCanvas(width, height int)`, `SetCell`, `Compose`, `Render`. It has no hit
testing.

`Compositor` is the layer engine TML targets:

- `NewCompositor(layers ...*Layer)` flattens eagerly; `AddLayers` and `Refresh` re-flatten.
- `Hit(x, y) LayerHit` returns the topmost layer at a point, which is how a mouse event routes back to a TML element.
- **`Hit` ignores layers with an empty ID**, so elements opt into mouse routing by having an id; nothing else is indexed.

## Two layer traps that decide the renderer's design

**Layer coordinates are parent-relative.** `flattenRecursive` accumulates `absX := layer.x + parentX`. Arrange can emit
parent-relative rects and never track absolute screen coordinates.

**But z-index is NOT parent-scoped, and equal z is unordered.** `flatten` sorts with
`slices.SortFunc(..., a.layer.z - b.layer.z)` — each layer's own `z`, never an accumulated one. A parent's z creates no
stacking context: a nested layer with a high z paints above an unrelated top-level layer with a lower z. And
`slices.SortFunc` is not stable, so two layers sharing a z have unspecified paint order.

Consequence: the renderer allocates z from a single tree-wide counter in document order, so every layer gets a distinct,
increasing z. Sibling order is never left to the sort. `Overlay` raises the counter into a higher band for its children.

## Layer size comes from content

A layer has no width/height setter — `Width()`/`Height()` derive from the content string, unioned with child bounds. TML
must render each node to exactly its arranged size (via `Style.Width`/`Height`), and the layer bounds then follow.

## Colour and golden files

`Style.Render` emits ANSI for any colour that is set, regardless of TTY; downsampling happens later at the writer. So
golden files either avoid colour entirely or compare after stripping ANSI. An unstyled `Render` returns plain text, which
is why layout goldens are colourless.
