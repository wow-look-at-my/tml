# The program

A `tml.Program` runs a view on a terminal. It owns raw mode, the input, the loop and the screen. The host owns the state.

```go
type App interface {
    Props() tml.Props
    Update(p *tml.Program, msg tml.Msg) bool
}

err := tml.Run(view, app)
```

`Props` is what the view draws from. The program reads it before every frame. An app never renders anything itself. `Update` takes one message on the program's own goroutine, and reports whether it consumed it.

## What arrives

```text
tml.Resize   the screen's size: one before the first frame, one for each change
tml.Key      one key press:        key.String() is "a", "A", "enter", "ctrl+c", "shift+tab", "space"
tml.Mouse    one pointer event:    press, release, motion or a wheel notch, at a cell
tml.Paste    pasted text, as one value
tml.Event    what the focus ring made of an input value: Activated, FocusMoved, Scrolled, Changed
tml.RepaintMsg  the inspector asking for a new frame
anything     whatever the host sent with p.Send
```

An input value goes to the app first. `Update` returns true to keep it, and the focus ring then never sees it. A value the app leaves alone goes to the ring, and each `tml.Event` the ring reports comes back through `Update`. That order is what lets a host keep a hotkey for itself and still leave typing to the field that has the focus.

A ctrl+c that neither the app nor the ring took ends the program with `tml.ErrInterrupted`. A host cannot leave its user without a way out by forgetting a key.

## What a host sends

```go
p.Send(msg)                   // from any goroutine
p.Go(func() tml.Msg { ... })  // run work off the loop; what it returns is sent
p.After(time.Second, msg)     // once
p.Every(time.Second, msg)     // until the program ends, or until the returned function is called
p.Quit()
p.SetView(view)               // put a reloaded document on screen
p.Copy(text)                  // set the system clipboard, with OSC 52
```

`Copy` writes between a pair of frames, not inside one. A terminal that does not take a later OSC ignores the sequence, and nothing reports that back. A host says what it copied on screen.

There is no command value to return from `Update`. Work that takes time runs on a goroutine and sends its result.

## The clock

The program steps the view's animations by itself. Its clock runs at `ProgramOptions.Interval` while something on screen moves, and stops when nothing does. An idle program writes nothing and wakes for nothing. A host needs a timer only for state of its own.

## Frames

After each batch of messages the program lays the view out at the screen's size and paints it. The screen writer keeps the last frame and writes only the cells that changed, inside one synchronized update. A burst of input is one frame. See docs/renderer.md.

A frame that fails to draw ends the program with that error, once the terminal is restored. A program that keeps its last good frame looks like one that stopped.

## Options

```go
tml.ProgramOptions{
    Input     io.Reader             // nil is os.Stdin
    NoInput   bool                  // read nothing; the terminal stays out of raw mode
    Output    io.Writer             // nil is os.Stdout
    Inline    bool                  // draw in place below the prompt, as tall as the content
    MaxHeight int                   // cap on an inline program's rows
    NoMouse   bool                  // leave the pointer to the terminal
    Interval  time.Duration         // the animation step
    Poll      bool                  // repaint on every interval
    Width, Height int               // a fixed size; zero follows the terminal
    Env       func(string) string   // the environment that says what the terminal shows
}
```

An inline program is what `tml.Live` is built on. Lines printed with `p.Println` go above it and stay in the scrollback. See docs/live.md.

The colour profile follows the terminal. A stream that is not a terminal gets no styling at all.

## Leaving

`Run` returns nil after `Quit`, `tml.ErrInterrupted` after an interrupt, or the error a frame failed with. The terminal goes back however the loop ends. A panic in the app included: the alternate screen, raw mode, the cursor and the reports the program asked for.

## A loop of your own

`View.Render` and `View.Draw` still paint one frame for a host that wants no program: a test, a snapshot, a document. `view.UI().Update(msg)` is the focus ring by hand. The inspector reads such a host and cannot drive it. See docs/inspector.md.
