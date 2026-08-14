// Package render composes a laid-out TML element tree into terminal output.
//
// TML solves layout constraints and produces absolute rects; this package
// hands those rects to Lip Gloss, which owns styling, measurement and
// compositing. Nothing here emits ANSI directly.
package render
