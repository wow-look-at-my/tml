package inspect

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/layout"
)

// fake is a Source with a frame built by hand, so the protocol is tested without a terminal program in the way.
type fake struct {
	frame   Frame
	ok      bool
	keys    []string
	clicks  [][2]int
	styles  map[string]map[string]string
	control bool
}

func (f *fake) Frame() (Frame, bool) { return f.frame, f.ok }

// readOnly hides the Controller half, which is how a program that only reports its frames is represented. It wraps
type readOnly struct{ src Source }

func (r readOnly) Frame() (Frame, bool) { return r.src.Frame() }

func (f *fake) Key(key string) error {
	f.keys = append(f.keys, key)
	return nil
}

func (f *fake) Click(x, y int) error {
	f.clicks = append(f.clicks, [2]int{x, y})
	return nil
}

func (f *fake) Restyle(id string, attrs map[string]string) error {
	if f.styles == nil {
		f.styles = map[string]map[string]string{}
	}
	f.styles[id] = attrs
	return nil
}

func (f *fake) Reset() error {
	f.styles = nil
	return nil
}

// box builds a parent-and-child frame: a root with a single child carrying an id.
func box() *layout.Box {
	child := &layout.Box{
		Name: "Text", ID: "status", Action: "refresh", Text: "ready",
		Screen: layout.Rect{X: 2, Y: 3, W: 10, H: 1},
		Clip:   layout.Rect{X: 0, Y: 0, W: 80, H: 24},
	}
	return &layout.Box{
		Name: "Stack", ID: "app",
		Screen:   layout.Rect{X: 0, Y: 0, W: 80, H: 24},
		Clip:     layout.Rect{X: 0, Y: 0, W: 80, H: 24},
		Children: []*layout.Box{child},
	}
}

func newFake() *fake {
	return &fake{
		ok: true,
		frame: Frame{
			Seq: 7, At: time.Unix(1700000000, 0), Width: 80, Height: 24,
			Box:  box(),
			ANSI: "\x1b[31mready\x1b[0m",
			State: map[string]layout.Target{
				"status": {ID: "status", Focus: true, Scroll: layout.Scroll{Y: 2, MaxY: 9}},
			},
		},
	}
}

func TestQueryReportsOneElementByID(t *testing.T) {
	s := NewServer(newFake())

	res := s.Handle(Request{Op: "query", ID: "status"})

	require.Empty(t, res.Error)
	require.NotNil(t, res.Element)
	assert.Equal(t, "status", res.Element.ID)
	assert.Equal(t, "Text", res.Element.Element)
	assert.Equal(t, "refresh", res.Element.Action)
	assert.Equal(t, Rect{X: 2, Y: 3, W: 10, H: 1}, res.Element.Rect)
	assert.True(t, res.Element.Focus, "the frame's state says this element has the keyboard")
	assert.Equal(t, 2, res.Element.Scroll.Y)
	assert.Equal(t, []string{"ready"}, res.Element.Lines)
}

// A typo has to name the mistake. An empty answer would read like an element that drew nothing.
func TestQueryNamesTheIDsThatDoExist(t *testing.T) {
	s := NewServer(newFake())

	res := s.Handle(Request{Op: "query", ID: "nope"})

	assert.Contains(t, res.Error, `no element has id "nope"`)
	assert.Contains(t, res.Error, "app")
	assert.Contains(t, res.Error, "status")
	assert.Nil(t, res.Element)
}

func TestElementsAndIDsCoverEveryIDInDocumentOrder(t *testing.T) {
	s := NewServer(newFake())

	elements := s.Handle(Request{Op: "elements"})
	ids := s.Handle(Request{Op: "ids"})

	require.Len(t, elements.Elements, 2)
	assert.Equal(t, "app", elements.Elements[0].ID)
	assert.Equal(t, "status", elements.Elements[1].ID)
	assert.Equal(t, []string{"app", "status"}, ids.IDs)
}

// The tree carries boxes with no id, which is where a layout mistake usually is.
func TestTreeCarriesEveryBoxWithItsPath(t *testing.T) {
	f := newFake()
	f.frame.Box.Children = append(f.frame.Box.Children, &layout.Box{Name: "Spacer"})
	s := NewServer(f)

	res := s.Handle(Request{Op: "tree"})

	require.NotNil(t, res.Tree)
	assert.Equal(t, "0", res.Tree.Path)
	assert.Equal(t, "app", res.Tree.ID)
	require.Len(t, res.Tree.Children, 2)
	assert.Equal(t, "0.1", res.Tree.Children[1].Path)
	assert.Equal(t, "Spacer", res.Tree.Children[1].Element)
	assert.Empty(t, res.Tree.Children[1].ID, "a box with no id is still in the tree")
}

func TestAtResolvesACellToTheInnermostElement(t *testing.T) {
	s := NewServer(newFake())

	inside := s.Handle(Request{Op: "at", X: 3, Y: 3})
	outside := s.Handle(Request{Op: "at", X: 40, Y: 20})

	assert.Equal(t, "status", inside.Hit)
	assert.True(t, inside.Found)
	assert.Equal(t, "app", outside.Hit, "the root still covers the cell")
}

func TestFrameReportsWhatIsOnScreen(t *testing.T) {
	s := NewServer(newFake())

	plain := s.Handle(Request{Op: "frame"})
	styled := s.Handle(Request{Op: "frame", ANSI: true})

	require.NotNil(t, plain.Frame)
	assert.Equal(t, uint64(7), plain.Frame.Seq)
	assert.Equal(t, 80, plain.Frame.Width)
	assert.Equal(t, "ready", plain.Frame.Text, "the escapes are stripped by default")
	assert.Empty(t, plain.Frame.ANSI)
	assert.Contains(t, styled.Frame.ANSI, "\x1b[31m")
}

// A program that has not painted is a real state, and saying so beats an empty frame that reads like an empty screen.
func TestEveryReadSaysWhenThereIsNoFrameYet(t *testing.T) {
	s := NewServer(&fake{ok: false})

	for _, op := range []string{"query", "elements", "ids", "tree", "frame", "at"} {
		res := s.Handle(Request{Op: op, ID: "status"})
		assert.Contains(t, res.Error, "has not painted", "op %s", op)
	}
}

func TestDrivingOperationsReachTheProgram(t *testing.T) {
	f := newFake()
	s := NewServer(f)

	require.True(t, s.Handle(Request{Op: "key", Key: "enter"}).OK)
	require.True(t, s.Handle(Request{Op: "click", X: 4, Y: 5}).OK)
	require.True(t, s.Handle(Request{Op: "restyle", ID: "status", Attrs: map[string]string{"width": "20"}}).OK)

	assert.Equal(t, []string{"enter"}, f.keys)
	assert.Equal(t, [][2]int{{4, 5}}, f.clicks)
	assert.Equal(t, map[string]string{"width": "20"}, f.styles["status"])

	require.True(t, s.Handle(Request{Op: "reset"}).OK)
	assert.Nil(t, f.styles)
}

// A read-only program says so rather than accepting input and dropping it.
func TestAReadOnlyProgramRefusesTheDrivingOperations(t *testing.T) {
	s := NewServer(readOnly{src: newFake()})

	for _, req := range []Request{
		{Op: "key", Key: "enter"},
		{Op: "click"},
		{Op: "restyle", ID: "status", Attrs: map[string]string{"width": "2"}},
		{Op: "reset"},
	} {
		res := s.Handle(req)
		assert.Contains(t, res.Error, "read-only", "op %s", req.Op)
		assert.False(t, res.OK)
	}
}

func TestBadRequestsNameWhatWasWanted(t *testing.T) {
	s := NewServer(newFake())

	assert.Contains(t, s.Handle(Request{Op: "wat"}).Error, "unknown op")
	assert.Contains(t, s.Handle(Request{}).Error, "no op given")
	assert.Contains(t, s.Handle(Request{Op: "query"}).Error, "needs an id")
	assert.Contains(t, s.Handle(Request{Op: "key"}).Error, "key name")
	assert.Contains(t, s.Handle(Request{Op: "restyle"}).Error, "needs an id")
	assert.Contains(t, s.Handle(Request{Op: "restyle", ID: "status"}).Error, "at least one attribute")
}

func TestSocketAnswersOneRequestPerLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inspect.sock")
	s := NewServer(newFake())
	require.NoError(t, s.ListenSocket(path))
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(bufio.NewReader(conn))
	require.NoError(t, enc.Encode(Request{Op: "query", ID: "status"}))

	var first Response
	require.NoError(t, dec.Decode(&first))
	require.NotNil(t, first.Element)
	assert.Equal(t, "status", first.Element.ID)

	// The same connection answers again, which is what lets a watcher hold a single open instead of reconnecting per question.
	require.NoError(t, enc.Encode(Request{Op: "ids"}))
	var second Response
	require.NoError(t, dec.Decode(&second))
	assert.Equal(t, []string{"app", "status"}, second.IDs)
}

func TestSocketRefusesAPathAlreadyServed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inspect.sock")
	first := NewServer(newFake())
	require.NoError(t, first.ListenSocket(path))
	t.Cleanup(func() { require.NoError(t, first.Close()) })

	err := NewServer(newFake()).ListenSocket(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already served by a live program")
}

func TestHTTPServesThePageAndTheSameRPC(t *testing.T) {
	srv := httptest.NewServer(HTTPHandler(NewServer(newFake())))
	t.Cleanup(srv.Close)

	page, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer page.Body.Close()
	assert.Equal(t, http.StatusOK, page.StatusCode)

	res, err := http.Post(srv.URL+"/rpc", "application/json",
		strings.NewReader(`{"op":"query","id":"status"}`))
	require.NoError(t, err)
	defer res.Body.Close()

	var answer Response
	require.NoError(t, json.NewDecoder(res.Body).Decode(&answer))
	require.NotNil(t, answer.Element)
	assert.Equal(t, "status", answer.Element.ID)
	assert.Equal(t, "ready", answer.Element.Text)
}

func TestHTTPStreamPushesTheFrame(t *testing.T) {
	srv := httptest.NewServer(HTTPHandler(NewServer(newFake())))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/events")
	require.NoError(t, err)
	defer res.Body.Close()

	reader := bufio.NewReader(res.Body)
	var payload string
	for range 4 {
		line, err := reader.ReadString('\n')
		require.NoError(t, err)
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			payload = after
			break
		}
	}
	require.NotEmpty(t, payload, "the stream sent no frame")

	var frame FrameInfo
	require.NoError(t, json.Unmarshal([]byte(payload), &frame))
	assert.Equal(t, uint64(7), frame.Seq)
	assert.Equal(t, "ready", frame.Text)
}

func TestRPCTakesPostOnly(t *testing.T) {
	srv := httptest.NewServer(HTTPHandler(NewServer(newFake())))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/rpc")
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
}
