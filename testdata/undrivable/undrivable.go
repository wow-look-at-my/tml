// Command undrivable is the program the drivable guard exists to stop. It loads a document and then paints it from a
// loop of its own, so the inspector can read it and cannot drive it.
package main

import (
	"fmt"
	"os"
	"testing/fstest"
	"time"

	"github.com/wow-look-at-my/tml"
)

const doc = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="App">
	<Template><Stack id="app" width="20"><Text id="hello">hello</Text></Stack></Template>
</Component>`

func main() {
	view, err := tml.Load(fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(doc)}},
		"app.tml", tml.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	// The whole point of this program: View.Render in a loop, not tml.Run.
	if _, err := view.Render(nil, 40, 10); err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
	time.Sleep(time.Hour)
}
