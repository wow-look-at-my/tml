# Images

```xml
<Image src="./logo.png" alt="the TML mark" width="16"/>
```

`src` resolves against the directory of the `.tml` file that holds the element. The lookup stays inside the filesystem the view loaded from. A path that climbs out of it is an error, not a read. PNG, JPEG and GIF decode by a sniff of the bytes. The extension is not what decides.

## The ladder

There is no one way to put a picture in a terminal. `Image` takes the best one available, and falls back until something works everywhere:

1. **kitty** -- the Kitty graphics protocol. Real pixels. Kitty, Ghostty and WezTerm.
2. **iterm** -- the iTerm2 inline image protocol. Real pixels. iTerm2 and WezTerm.
3. **mosaic** -- half-block characters: `▀` with the top pixel as the foreground and the bottom as the background. As a result, one cell carries a couple of pixels. Works in anything that can do colour.
4. **link** -- the alt text as an OSC 8 hyperlink to the file. Every terminal shows the text. One that understands the escape makes it clickable.

`protocol="kitty|iterm|mosaic|link|auto"` picks one. `auto` is the default. It reads the environment.

Nothing is auto-detected by asking the terminal. A query writes to the terminal and waits for the reply. A renderer that does that blocks on a pipe. So detection is what `TERM`, `TERM_PROGRAM`, `KITTY_WINDOW_ID` and their neighbours say. `protocol` is the override for when they are wrong. Half-blocks are what is left when nothing claims to do better. The answer is never a blank space.

`protocol="link"` never reads the file at all, which makes it the fallback that always works. It is also the right choice for an image that is not on this machine.

## Size

An image takes the width it is given and keeps its shape, because a terminal cell is about twice as tall as it is wide. 64 by many pixels at `width="16"` is several rows, not 16. Nothing reports the real cell aspect portably. 2 is close enough that a square image comes out square instead of stretched.

Both graphics protocols are asked not to move the cursor. The escape then pairs with the blank cells it covers. The escape itself measures zero, because it is an instruction to the terminal rather than text. Everything downstream counts cells. Without the blanks the image has no footprint, and the layout puts other things on top of it.

## Transparency

A transparent pixel is blended onto the theme's own background, since the terminal's real one is not something anything reports portably. `Options.Dark` is what decides. That is the right answer whenever the terminal and the theme agree. To make them agree is what the theme is for.
