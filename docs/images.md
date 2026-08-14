# Images

```xml
<Image src="./logo.png" alt="the TML mark" width="16"/>
```

`src` is resolved against the directory of the `.tml` file the element was written in, inside the filesystem the view
was loaded from -- a path that climbs out of it is an error, not a read. PNG, JPEG and GIF are decoded by sniffing the
bytes, so the extension is not what decides.

## The ladder

There is no one way to put a picture in a terminal, so `Image` takes the best one available and falls back until
something works everywhere:

1. **kitty** -- the Kitty graphics protocol. Real pixels. Kitty, Ghostty and WezTerm.
2. **iterm** -- the iTerm2 inline image protocol. Real pixels. iTerm2 and WezTerm.
3. **mosaic** -- half-block characters: `▀` with the top pixel as the foreground and the bottom as the background, so
   one cell carries two pixels. Works in anything that can do colour.
4. **link** -- the alt text as an OSC 8 hyperlink to the file. Every terminal shows the text; one that understands the
   escape makes it clickable.

`protocol="kitty|iterm|mosaic|link|auto"` picks one; `auto` is the default and reads the environment.

Nothing is auto-detected by asking the terminal. A query means writing to the terminal and waiting for the reply, and a
renderer that did that would block on a pipe. So detection is what `TERM`, `TERM_PROGRAM`, `KITTY_WINDOW_ID` and their
neighbours say, and `protocol` is the override for when they are wrong. Half-blocks are what is left when nothing claims
to do better -- never a blank space.

`protocol="link"` never reads the file at all, which is what makes it the fallback that always works: it is also the
right choice for an image that is not on this machine.

## Size

An image takes the width it is given and keeps its shape, because a terminal cell is about twice as tall as it is wide:
64 by 64 pixels at `width="16"` is 8 rows, not 16. Nothing reports the real cell aspect portably, and 2 is close enough
that a square image comes out square instead of stretched.

Both graphics protocols are asked not to move the cursor, and the escape is paired with the blank cells it covers. The
escape itself measures zero -- it is an instruction to the terminal rather than text -- and everything downstream counts
cells, so without the blanks the image would have no footprint and the layout would place other things on top of it.

## Transparency

A transparent pixel is blended onto the theme's own background, since the terminal's real one is not something anything
reports portably. `Options.Dark` is what decides, which is the right answer whenever the terminal and the theme agree --
and making them agree is what the theme is for.
