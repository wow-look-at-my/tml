module github.com/wow-look-at-my/tml

go 1.26

require (
	charm.land/lipgloss/v2 v2.0.6
	github.com/stretchr/testify v1.12.1
	github.com/wow-look-at-my/xml-validator/validator v0.0.0-20260816073403-a21628c1fff2 // go-toolchain:auto-branch
)

require (
	charm.land/bubbles/v2 v2.1.1
	charm.land/bubbletea/v2 v2.0.8
	github.com/charmbracelet/x/ansi v0.11.8
	github.com/spf13/cobra v1.10.2
)

require go.yaml.in/yaml/v3 v3.0.5 // indirect

replace charm.land/bubbletea/v2 => github.com/wow-look-at-my/bubbletea/v2 v2.0.0-20260823065234-810d805a9f3e // go-toolchain:auto-branch=slh

require (
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260811164956-006e29f97886 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/wow-look-at-my/go-containers v0.0.0-20260820210621-2e1261867045 // go-toolchain:auto-branch
	github.com/wow-look-at-my/xml-validator/reader v0.0.0-20260816073403-a21628c1fff2 // indirect; go-toolchain:auto-branch
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
