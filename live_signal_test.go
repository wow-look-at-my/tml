package tml_test

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// A host that owns SIGINT keeps its region on screen through the signal, and the region ends only on Stop.
func TestLiveWithHostSignalsSurvivesSIGINT(t *testing.T) {
	t.Serial("the test sends SIGINT to the whole process, which every running program would also receive")
	exclusive(t)
	host := make(chan os.Signal, 1)
	signal.Notify(host, os.Interrupt)
	defer signal.Stop(host)

	view, err := tml.Load(fstest.MapFS{"block.tml": &fstest.MapFile{Data: []byte(liveBlock)}}, "block.tml", tml.Options{})
	require.NoError(t, err)
	out := &syncBuffer{}
	live := tml.NewLive(view, blockProps(), tml.LiveOptions{
		Interval:    5 * time.Millisecond,
		Output:      out,
		Width:       liveWidth,
		Height:      liveHeight,
		Env:         envOf(colourEnv),
		HostSignals: true,
		OnInterrupt: func() { t.Error("OnInterrupt ran for a signal the host owns") },
	})
	require.NoError(t, live.Start())
	waitDrawn(t, out, "alpha")

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	select {
	case <-host:
	case <-time.After(5 * time.Second):
		t.Fatal("the host never received SIGINT")
	}

	live.Println("still here")
	waitDrawn(t, out, "still here")
	require.NoError(t, live.Stop(), "the region ends on Stop, not on the signal")
}
