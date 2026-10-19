// Command printer prints a document and then outlives the drive grace. A command-line tool does this: it prints a table,
// then waits on something else. A printed frame is not a screen, so the drivable guard must leave it alone.
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
	<Template><Text>printed</Text></Template>
</Component>`

func main() {
	view, err := tml.Load(fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(doc)}}, "app.tml", tml.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	if err := view.Print(os.Stdout, nil, 0); err != nil {
		fmt.Fprintln(os.Stderr, "print:", err)
		os.Exit(1)
	}
	time.Sleep(tml.DriveGrace + 2*time.Second)
	fmt.Println("still up")
}
