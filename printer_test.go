package gohtmlprinter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		{
			`<html><head><meta name="viewport" content="width=device-width, initial-scale=1"/></head><body></body></html>`,
			`<html><head><meta name="viewport" content="width=device-width, initial-scale=1"/></head><body></body></html>`,
		},
		{
			`<strong>{{ $tc('sw-product.detailBase.bundleVirtualAvailability') }}</strong>`,
			`<html><head></head><body><strong>{{ $tc('sw-product.detailBase.bundleVirtualAvailability') }}</strong></body></html>`,
		},
	}

	for _, c := range cases {
		parsed, _ := html.Parse(strings.NewReader(c.in))
		var buf bytes.Buffer
		Render(&buf, parsed)

		assert.Equal(t, c.want, buf.String())
	}
}

func TestSkipElements(t *testing.T) {
	input := `<html><head><meta name="viewport" content="width=device-width, initial-scale=1"/></head><body></body></html>`
	output := `<meta name="viewport" content="width=device-width, initial-scale=1"/>`

	parsed, _ := html.Parse(strings.NewReader(input))

	var buf bytes.Buffer

	RenderButSkipElements(&buf, parsed, func(n *html.Node) bool {
		return n.Data == "html" || n.Data == "head" || n.Data == "body"
	})

	assert.Equal(t, output, buf.String())
}
