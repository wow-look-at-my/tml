module github.com/wow-look-at-my/tml

go 1.26.0

require (
	github.com/stretchr/testify v1.12.1
	github.com/wow-look-at-my/xml-validator/reader v0.0.0 // go-toolchain:auto-branch
	github.com/wow-look-at-my/xml-validator/validator v0.0.0 // go-toolchain:auto-branch
)

require github.com/spf13/cobra v1.10.2

require go.yaml.in/yaml/v3 v3.0.5 // indirect

replace charm.land/bubbletea/v2 => github.com/wow-look-at-my/bubbletea/v2 v2.0.0 // go-toolchain:auto-branch

require (
	github.com/clipperhouse/displaywidth v0.11.0
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/wow-look-at-my/go-containers v0.0.0 // go-toolchain:auto-branch
	golang.org/x/sys v0.47.0
)
