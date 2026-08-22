package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/wow-look-at-my/tml/inspect"
)

// pollInterval is how often an await asks again. A question over a unix socket
// costs nothing next to a frame, so this is short enough that a test does not
// wait noticeably longer than the program took.
const pollInterval = 50 * time.Millisecond

// awaitField asks one element until its field matches, and answers with the
// element it matched on.
//
// A test asserts about a screen that is still changing, so its real question is
// "has it happened yet". A sleep answers that by guessing at how fast the
// machine is, and the guess is wrong on somebody else's machine.
//
// gone inverts the match, for something that has to stop being on screen.
func awaitField(id string, ansi bool, field, pattern string, gone bool, timeout time.Duration) (*inspect.Element, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("--await %q is not a regular expression: %w", pattern, err)
	}
	deadline := time.Now().Add(timeout)
	var (
		drew   string
		answer error
		asked  bool
	)
	for {
		res, err := ask("", inspect.Request{Op: "query", ID: id, ANSI: ansi})
		switch {
		case err != nil:
			// The program may still be starting, or the frame may not declare
			// this id yet. Both are answered by asking again, so the error is
			// kept for the deadline to report rather than returned here.
			answer = err
		case res.Element != nil:
			value, err := fieldValue(*res.Element, field)
			if err != nil {
				return nil, err
			}
			asked, drew, answer = true, value, nil
			if re.MatchString(value) != gone {
				return res.Element, nil
			}
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(pollInterval)
	}
	if !asked {
		return nil, fmt.Errorf("%q was never answered within %s: %w", id, timeout, answer)
	}
	if gone {
		return nil, fmt.Errorf("%q still matched /%s/ after %s. It drew:\n%s", id, pattern, timeout, drew)
	}
	return nil, fmt.Errorf("%q never matched /%s/ within %s. It last drew:\n%s", id, pattern, timeout, drew)
}

// fieldValue is one field of an element as text, which is both what --field
// prints and what --await matches against.
func fieldValue(el inspect.Element, field string) (string, error) {
	switch field {
	case "", "text":
		return el.Text, nil
	case "lines":
		return strings.Join(el.Lines, "\n"), nil
	case "x":
		return fmt.Sprint(el.Rect.X), nil
	case "y":
		return fmt.Sprint(el.Rect.Y), nil
	case "w":
		return fmt.Sprint(el.Rect.W), nil
	case "h":
		return fmt.Sprint(el.Rect.H), nil
	case "focus":
		return fmt.Sprint(el.Focus), nil
	case "action":
		return el.Action, nil
	case "element":
		return el.Element, nil
	default:
		return "", fmt.Errorf("unknown field %q; want one of text, lines, x, y, w, h, focus, action, element", field)
	}
}

// printField writes one value bare, so a test can compare it without a JSON
// parser in the way.
func printField(w io.Writer, el inspect.Element, field string) error {
	value, err := fieldValue(el, field)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, value)
	return err
}
