# The package's own binaries come first on PATH, as npm run puts them.
export PATH := justfile_directory() + "/node_modules/.bin:" + env_var("PATH")

# The npm script build.
build:
	cd inspect/ui && ts0 build

# The npm script test.
test:
	cd inspect/ui && ts0 test
