# Screenshots

`tools/shots` runs each example in a real terminal and takes a picture of it. The terminal is Ghostty, on a virtual X display, and the picture is its window. `tools/shots/shots.mjs` lists what to run and what state to get it into. `capture.mjs` is the machinery.

```bash
go-toolchain                                   # build the examples first
node tools/shots/capture.mjs                   # -> build/shots/*.png and index.html
```

The tool needs these on the machine, and names the ones it cannot find:

```bash
sudo add-apt-repository -y ppa:mkasberg/ghostty-ubuntu
sudo apt-get install -y ghostty xvfb xdotool imagemagick libgl1 libgl1-mesa-dri libegl1
```

Ghostty draws with OpenGL. Mesa answers that in software, so no GPU is needed. No window opens on the machine's own screen.

## Why Ghostty

Ghostty draws box, block and braille characters itself, at the size of the cell. A terminal that takes those glyphs from a font leaves gaps where borders meet, and seams inside a bar. Ghostty also speaks the kitty graphics protocol, so the `<Image>` shot shows a real image. It carries its own font. The pictures do not depend on the fonts of the machine.

The capture starts Ghostty with no configuration file. The options in `capture.mjs` are everything the picture depends on.

## Why a terminal and not a frame

Both examples already render one frame to stdout with `-frame`, and `testdata/*.golden` pins that text. That is the mechanical check. It stays the one CI fails on. But a frame is a string. It proves nothing about whether the program draws in a terminal, answers a key press from a keyboard, or picks colours a terminal can show. The image fallback is the clearest case.

`<Image>` asks the terminal what it is, then chooses between the kitty protocol, half-blocks and a link. In a golden the result is text either way.

So the screenshots cover what the goldens cannot, and typed keys drive one of them (`keys` in the shot list) to keep the input path honest.

## What a shot declares

- `args` put the program in a state before the terminal is attached.
- `keys` are typed into the running terminal afterwards, with `xdotool`.
- `expect` is text the program has to have painted before the picture counts. A program that died on its arguments still leaves a terminal to photograph. A black rectangle published as a screenshot is worse than a failed build.
- `rows` is the terminal to shoot in. These layouts fill the terminal they are given. A page shot in more rows than it has content for is mostly empty space. One shot in fewer rows drops whatever was pinned to the bottom.

The capture reads what the program painted through the program's own inspection socket, with `tml frame`. That answers both questions a picture depends on. The frame's size is the size the terminal gave the program. The capture fails when that is not the size it asked for. The frame's text is what `expect` is matched against. A program that never painted answers nothing, and the shot fails.

The inspector reports the program, not the terminal. So the capture also counts the colours in the picture, and refuses one that shows an empty window.

## Publishing

CI captures on every push and publishes `build/shots/` to a buildhost site, per branch:

- `https://sites.pazer.build/tml/` -- master
- `https://sites.pazer.build/tml/@<branch>/` -- any other branch

Until master published once, the bare project URL falls back to whichever branch does have a site. The README's pictures therefore work before the first merge, and switch to master's afterwards.

The project is the repository's name, because that is what buildhost's OIDC trust covers. A token minted for `repo:wow-look-at-my/tml` writes project `tml` and anything namespaced beneath it. Any other name answers `404 project not found` rather than creates the project.

A branch build's index page shows each picture beside master's copy of it. What a change did to the rendering is therefore a scroll, not an archaeology exercise. The README embeds master's, which is where the showcase comes from.

The site is published with `public: true`. tml is a private repo. GitHub's image proxy cannot fetch a picture it needs a credential for. Without `public: true` the README shows nothing. What lands there is pictures of example programs, and the index page that lists them. That directory is the whole privacy boundary. Nothing else may be written into it.

There is deliberately no pixel comparison. Software rendering differs between Mesa versions. A pixel gate therefore fails on where it ran, not on what changed. The goldens are the mechanical check. These pictures are for the eye.
