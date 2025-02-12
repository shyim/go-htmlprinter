package gohtmlprinter

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestEscapingInAttributes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			`<html><head><meta content="foo bar"/></head><body></body></html>`,
			`<html><head><meta content="foo bar"/></head><body></body></html>`,
		},
		{
			`<html><head><meta content="$tc('bla')"/></head><body></body></html>`,
			`<html><head><meta content="$tc('bla')"/></head><body></body></html>`,
		},
	}

	for _, c := range cases {
		parsed, _ := html.Parse(strings.NewReader(c.in))
		var buf bytes.Buffer
		Render(&buf, parsed)

		if got := buf.String(); got != c.want {
			t.Fatalf("Render(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
