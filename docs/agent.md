# The agent example

`examples/agent` is a mock coding agent: the shape of an AI harness with a script where the model would be. Nothing in
it talks to a model and nothing in it runs a command. It exists to put the language under the load a real harness
applies, and to make what the language cannot do obvious rather than quietly absent.

```bash
go-toolchain && ./build/agent            # interactive
./build/agent -frame -steps 6            # one frame, no terminal
./build/agent -frame -steps 6 -answer deny
```

Space steps the script, tab moves, enter activates, the wheel scrolls the session, and typing at the prompt reaches the
prompt. The permission beat stops the script until it is answered: a harness that ran the command and then asked would
be lying about the ask.

## What it loads onto the language

- **A transcript that outgrows its viewport**, so scrolling has to be real and the newest turn has to stay on screen.
- **Tool output as cards** — a diff and a test-results table — sitting in the middle of the conversation they belong to.
- **A modal**, over a layout that keeps rendering behind it.
- **Two widgets the language does not ship**, `Transcript` and `Diff`, bound by name and used in the template exactly
  like a built-in.

## What it proved

Every item here is a change the example forced, not a note about one:

- `widget.NewFactory` / `NewSlottedFactory` and an exported `Attrs.Errorf`. Binding a widget from outside the module
  meant writing a factory by hand against unexported helpers, which is not a seam.
- `sema.ListValue`. A host list went in as a comma-joined string and came back split on the commas inside its own
  values.
- `List` truncates a row rather than wrapping it. A wrapped row took two lines and pushed the geometry out of step with
  what was drawn.
- Scrolling clamps to the content, in the widget and in the geometry alike, and the frame reports where it landed
  (`UI().Target(id).Scroll`). Following a growing transcript is otherwise not expressible: only the frame knows how far
  the wrapped content runs.
- A borderless `Table` keeps the gap between its columns. Two full cells were running together into one word.

## What it could not do

The transcript is a widget because TML can repeat over a list of strings but has no way to switch on what a list item
**is**. A heterogeneous conversation -- one line a tool call, the next a message -- is either a widget or a language
feature, and a widget is the honest one today.

The cards are the other half of the same limit. They sit at fixed points in the template, so the host cuts the
transcript where each one arrived and hands back three regions. That keeps the chronology right, and it is the host
doing the interleaving because the template cannot.
