package main

import (
	"testing"

	"github.com/wow-look-at-my/tml/cell"
)

func TestProbe(t *testing.T) {
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 96, 30
	for range 8 {
		m.step()
	}
	out := "\n=== permission\n" + cell.Strip(m.render())
	m.answer(true)
	for range 3 {
		m.step()
	}
	out += "\n=== finished\n" + cell.Strip(m.render())
	t.Fatal(out)
}
