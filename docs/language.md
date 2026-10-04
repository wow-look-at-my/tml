# The TML language

A `.tml` file is strict XML 1.1. It opens with `<?xml version="1.1" encoding="UTF-8"?>`, is UTF-8 without a BOM, and has no DOCTYPE and no entities beyond the predefined five. That comes from xml-validator and is not negotiable per file.

The default namespace is `urn:tml:v1`. It is a URN on purpose: a name, not a URL, and nothing fetches it.

A file holds exactly one definition, rooted at `<Component>` or `<Theme>`.

## The schema

`tml.schema.xsd` states the grammar of everything on this page except the template body. An editor pointed at it completes and checks a file as it is typed:

```bash
xml-validator --schema tml.schema.xsd app.tml
```

It covers the declarations. That means which children each root takes, in any order, and every attribute each has. It also covers the property type grammar. It states that a required property is never defaulted. It states that a token carries a value, or a light and dark pair, and never one half of it. It also lists every style attribute with its vocabulary.

**The `<Template>` body is not checked, and cannot be.** It names components the file imported and widgets the host bound. A schema knows neither. xml-validator allows only `processContents="strict"`, which demands a declaration for every element it matches. A schema that checked the body therefore rejects every document that imports anything. `sema/analyze.go` checks the body. It is the authority on the whole file at load time.

Smaller rules are also out of reach. A root element in the wrong namespace is accepted, because the validator falls back to a local-name match at the root. `syntax.Parse` catches it. A style attribute whose value is a `{theme.token}` takes the token's word for the vocabulary. What the token holds is known only when the sheet resolves.

CI runs `xml-validator` over every document in the tree, and again with `--schema tml.schema.xsd` over every component. The schema can therefore never grow stricter than the documents this repository ships. `testdata/schema/valid` is the corpus that carries a declaration of each kind.

## Components

```xml
<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Card">
    <Import src="./Badge.tml"/>

    <Property name="title" type="string" required="true"/>
    <Property name="tags"  type="string[]" default=""/>

    <Template>
        <Box style="card">
            <Text>{title}</Text>
            <Slot/>
        </Box>
    </Template>
</Component>
```

A file defines exactly one component, rooted at `<Component>` (or exactly one theme, rooted at `<Theme>`). A component that shares its file with a second one is refused: one file, one component. Reuse is `<Import>`, not nesting.

### Properties

`<Property>` takes `name`, `type`, and either `required="true"` or a `default`. To require a property and default it at the same time is an error, because the default never applies.

Types: `string`, `int`, `bool`, `color`, `length`, `thickness`, `enum(a|b|c)`, and a `[]` suffix for a list of any of them. The suffix spelling exists so a type never needs XML escaping. `list<string>` needs it.

- `color` is a hex triplet (`#e0af68`, `#fff`) or an ANSI index `0-255`. Names are not accepted. A theme token is how a colour gets a name.
- `length` is `auto`, a cell count, or a star share (`*`, `2*`).
- `thickness` is the CSS shorthand: `1`, `1 2`, or `1 2 3 4` clockwise from the top.

A default is evaluated against the theme only, not against the component's other properties, so defaults do not depend on declaration order.

### Imports

`<Import src="./Badge.tml"/>` resolves relative to the importing file and brings that file's definition into scope under its own name. **Imports are not transitive**: a component sees its own definition, its data templates, and only what it imported itself.

An import cycle is harmless and terminates, because importing only makes a name visible. A component that instantiates itself is a different thing and is rejected.

## Slots

`<Slot>` is both the declaration and the insertion point. Its children are fallback content, used when the call site supplies nothing.

```xml
<Slot/>                                        <!-- the default slot -->
<Slot name="actions"/>                         <!-- a named slot -->
<Slot name="actions" required="true"/>         <!-- must be filled -->
<Slot name="actions"><Text>none</Text></Slot>  <!-- with fallback -->
```

A call site fills a named slot with XAML property-element syntax. Untagged children go to the default slot.

```xml
<Card title="Deploy">
    <Text>Ready to ship.</Text>          <!-- default slot -->
    <Card.actions>
        <Button label="OK"/>
    </Card.actions>
</Card>
```

**Slot content is a closure over the call site.** It expands in the caller's scope and the caller's file. It therefore sees the caller's properties and the caller's imports, not the component it is passed into. This is what makes a component substitutable without knowing where it is used.

Components do not nest lexically for anything else either: an instance sees its own properties and the theme, not the enclosing component's names.

## Expressions

The grammar is a name, optionally a theme token, optionally negated:

- `{name}` — a property or a `<For>` variable
- `{theme.accent}` — a theme token
- `{not name}` — negation
- `Hello {name}!` — interpolation in text or in any attribute value
- `{{` and `}}` — literal braces, the only escaping there is

There is no arithmetic and there are no calls. Every reference is therefore checked when the view loads, including references inside a branch that is never taken. An unknown name is an error, not an empty string.

**A lone reference keeps its type.** `padding="{gutter}"` stays a thickness. `title="pad {gutter}"` is a string. This is why a typed property can be forwarded to a typed attribute without restating the type.

## Control flow

`if` is an attribute on any element and takes a bool, or a string or list where empty counts as false. An `int` or `color` in an `if` is an error rather than a silent "non-zero".

```xml
<Text if="{subtitle}">{subtitle}</Text>
<Text if="{not subtitle}">no subtitle</Text>

<For each="{tags}" as="tag" index="i">
    <Text>{i}. {tag}</Text>
</For>
```

`<For>` itself renders nothing. Its children repeat. The loop variable and index are visible only inside it.

## Items controls

`<For>` repeats markup the document already wrote. An items control repeats a TEMPLATE over items the PROGRAM supplies. What a row looks like is the document's business. What a row contains is the host's.

Attributes make any element an items control. `itemsSource` takes a list. `itemTemplate` names a `<DataTemplate>`. Neither reaches the rendered element. They say what it contains, not what it is.

```xml
<Component xmlns="urn:tml:v1" name="App">
    <Property name="messages" type="record[]" default=""/>

    <DataTemplate name="Message">
        <Property name="role" type="string" required="true"/>
        <Property name="body" type="string[]" default=""/>
        <Template>
            <Stack>
                <MessageHeader role="{role}"/>
                <For each="{body}" as="line"><Text>{line}</Text></For>
            </Stack>
        </Template>
    </DataTemplate>

    <Template>
        <Scrollbox itemsSource="{messages}" itemTemplate="Message"/>
    </Template>
</Component>
```

A `<DataTemplate>` is written exactly like a `<Component>`. It differs in one way. Nobody writes it as an element, so its property values come from an item instead of from a call site. That is why an `itemTemplate` that names a `<Component>` is an error. A component whose values nothing supplies draws its defaults, once per item, forever.

- A `record` is a value with named fields. It has no literal spelling, because it comes from the program. The document reads its fields. `record[]` is the type of a property an items control repeats over.
- Each item's fields ARE the template's property values, matched by name. A field the template never declared is an error naming both sides, and so is a required property no item supplied. A binding that silently renders nothing is the failure this replaces: a column blank forever because one side called it `spend` and the other `cost`.
- A plain string item supplies one property, `value`, so a list of strings needs no wrapping.
- A template that declares `index` is handed its position. One that does not is not handed a property it never declared.

## Layout

Every element takes `width` and `height`: `auto` (default), a cell count, or a star share.

`auto` sizes to content. A star share takes a weighted slice of what is left after the fixed and auto siblings are placed. A star size propagates upward. A container that holds a star child asks for all the space available on that axis. Without that, the container shrinks to its content, and the star has nothing to fill.

Panels:

- `Stack` — `orientation="vertical|horizontal"` (vertical is the default) and `gap`
- `Grid` — `columns`, `rows`, `gap`, with placement declared per child
- `Canvas` — free positioning: children sit where they are put, and overlap
- `Box` — a single-child decorator for border, padding, background and alignment
- `Text` — character data, wrapped to the width it is given. `overflow="clip"` cuts each line at that width instead, and `overflow="ellipsis"` marks the cut. A clipped Text is as tall as it has lines, however narrow it gets. That keeps a fixed-height card fixed when one line inside it runs long
- `Spacer` — occupies space and draws nothing

`Dock` is **not implemented yet**. It is deliberately absent from the builtin list, so using it is reported as an unknown element rather than silently rendering nothing.

### Canvas

```xml
<Canvas>
    <Stack>…the page…</Stack>
    <Popup title="Confirm" if="{confirming}">…</Popup>
    <Badge label="beta" Canvas.anchor="bottomRight" Canvas.x="-2" Canvas.y="-1"/>
</Canvas>
```

A canvas takes all the space it is offered. One that shrank to its content leaves a child anchored to its bottom-right corner nowhere to sit. Attached properties place each child: `Canvas.x`, `Canvas.y` and `Canvas.anchor` (`topLeft`, `topRight`, `bottomLeft`, `bottomRight`, `center`). The offsets measure from the anchor. A negative one therefore moves back in from the edge it is pinned to.

A child that says nothing sits where its own default anchor puts it, which is how `<Popup>` lands in the middle without being told to. Children paint in document order, so a dialog written last is the one on top.

### Grid

```xml
<Grid columns="auto,1*,2*" rows="1,*" gap="1">
    <Text Grid.row="0" Grid.column="1" Grid.columnSpan="2">spans two columns</Text>
</Grid>
```

Tracks are declared on the grid. Placement is declared on each child with XAML-style attached properties: `Grid.row`, `Grid.column`, `Grid.rowSpan`, `Grid.columnSpan`. Track solving runs fixed, then auto, then star. A star track therefore divides only what the other two left behind. The last star track absorbs the rounding, so the grid fills exactly.

An auto track sizes to the widest child confined to it. A child spanning several tracks is excluded from that measurement, because it cannot say which of the tracks it covers must grow.

Placing a child past the last declared track widens the grid with auto tracks rather than dropping the child.

An auto or a star track can carry a range: `*[8..32]`, `auto[..20]`, `*[4..]`. Either end can be left out. A star track that reaches its maximum stops there, and the other star tracks share what it did not take. A lone capped star therefore leaves the rest of the row empty. That keeps a label column near the numbers beside it on a wide terminal. A track below its minimum keeps the minimum, and the grid overflows. A fixed track has no range.

`collapse` lists column indices in the order they give way when the grid is too narrow:

```xml
<Grid columns="*[8..32],16,4,24,11,9" gap="2" collapse="1,4,5,3">
```

While the shown columns do not fit, the next column in the list is hidden. A star column counts its minimum for this. A hidden column takes no width and no gap. A child confined to hidden columns draws nothing. A spanning child keeps the shown part of its span. A column the list does not name never goes. An auto column counts what its content measured. With only fixed and star columns, the decision depends on the width alone. Rows that repeat such a grid then hide the same columns and stay in line.

A grid child fills the cell it is placed in. A `1*` track is therefore how you say "as wide as there is room for". A `Stack` is the other way round. A child keeps its own size across the stack, and `align`/`valign` place it, unless it asks for `width="*"`. The difference is that a grid's tracks were declared by the author and a stack's cross axis was not.

An attached property written on a child of anything other than a `Grid` is an error, as is an unknown one such as `Grid.depth`. To ignore them leaves a layout that quietly disregards what was written.

Grid children sit at coordinates rather than in a line, so the renderer composites them through lipgloss layers instead of joining. Each layer gets a distinct, increasing z in document order. See docs/lipgloss-contract.md for why equal z values are unsafe.

Layout attributes are not forwarded through a component instance: every attribute on `<Card .../>` is a property. A component that wants to be sized declares a `length` property and puts it on its own root, which is what `Card.tml` in `testdata/dashboard` does.

## Themes

```xml
<?xml version="1.1" encoding="UTF-8"?>
<Theme xmlns="urn:tml:v1" name="default">
    <Token name="accent" light="#5f5fd7" dark="#e0af68"/>
    <Token name="gutter" value="1"/>

    <Style name="card"       border="rounded" borderColor="{theme.accent}" padding="0 1"/>
    <Style name="card.title" extends="card"   bold="true"/>
</Theme>
```

A token has either a single `value` or a `light`/`dark` pair, not both and never half a pair. The pair follows `Options.Dark`.

Style attributes: `fg`, `bg`, `bold`, `italic`, `underline`, `strikethrough`, `faint`, `reverse`, `padding`, `margin`, `border`, `borderColor`, `align`, `valign`. Borders are named: `none`, `normal`, `rounded`, `thick`, `double`, `hidden`, `block`, `ascii`.

An element picks a named style with `style="card"` and overrides individual attributes inline.

`extends` is resolved in TML's own attribute model, not by `lipgloss.Style.Inherit`, because Inherit deliberately drops padding and margin. See docs/lipgloss-contract.md.

## Widgets

Everything that is not a panel is a widget. The library in `widgets/` holds buttons, fields, frames, popups, scrolling regions, images and the rest. It is registered by default. `id` and `action` on any element wire it to the host's `Update`. See docs/widgets.md for the reference and for writing your own.

A host binds its own elements the same way, and TML measures, places and draws them without touching their state:

```go
widgets := widget.NewRegistry().Bind("Search", widget.Bubble(&m.input))
view, err := tml.Load(ui, "app.tml", tml.Options{Widgets: widgets})
```

Pass a pointer to `Bubble`. `SetWidth` has a pointer receiver. Without a pointer, the width TML computes lands on a copy and is thrown away.

Widget names are checked when the view loads, so a template naming a widget that was never bound fails there rather than rendering a blank.

TML never routes messages. The host keeps its `Update` and its state. What TML reports is what the user did. `view.UI().Update(msg)` hands back the id and action of whatever was activated, and nothing else. `examples/dashboard` shows the split with a bubbles component. `examples/gallery` shows it with the library.
