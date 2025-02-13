package gohtmlprinter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

type writer interface {
	io.Writer
	io.ByteWriter
	WriteString(string) (int, error)
}

var errPlaintextAbort = errors.New("html: internal error (plaintext abort)")

func Render(w io.Writer, n *html.Node) error {
	filter := func(*html.Node) bool {
		return false
	}

	if x, ok := w.(writer); ok {
		return render(x, n, filter)
	}
	buf := bufio.NewWriter(w)
	if err := render(buf, n, filter); err != nil {
		return err
	}
	return buf.Flush()
}

func RenderButSkipElements(w io.Writer, n *html.Node, filter func(*html.Node) bool) error {
	if x, ok := w.(writer); ok {
		return render(x, n, filter)
	}
	buf := bufio.NewWriter(w)
	if err := render(buf, n, filter); err != nil {
		return err
	}
	return buf.Flush()
}

func render(w writer, n *html.Node, filter func(*html.Node) bool) error {
	err := render1(w, n, filter)
	if err == errPlaintextAbort {
		err = nil
	}
	return err
}

func render1(w writer, n *html.Node, filter func(*html.Node) bool) error {
	// Render non-element nodes; these are the easy cases.
	switch n.Type {
	case html.ErrorNode:
		return errors.New("html: cannot render an ErrorNode node")
	case html.TextNode:
		return escape(w, n.Data)
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := render1(w, c, filter); err != nil {
				return err
			}
		}
		return nil
	case html.ElementNode:
		// No-op.
	case html.CommentNode:
		if _, err := w.WriteString("<!--"); err != nil {
			return err
		}
		if err := escapeComment(w, n.Data); err != nil {
			return err
		}
		if _, err := w.WriteString("-->"); err != nil {
			return err
		}
		return nil
	case html.DoctypeNode:
		if _, err := w.WriteString("<!DOCTYPE "); err != nil {
			return err
		}
		if err := escape(w, n.Data); err != nil {
			return err
		}
		if n.Attr != nil {
			var p, s string
			for _, a := range n.Attr {
				switch a.Key {
				case "public":
					p = a.Val
				case "system":
					s = a.Val
				}
			}
			if p != "" {
				if _, err := w.WriteString(" PUBLIC "); err != nil {
					return err
				}
				if err := writeQuoted(w, p); err != nil {
					return err
				}
				if s != "" {
					if err := w.WriteByte(' '); err != nil {
						return err
					}
					if err := writeQuoted(w, s); err != nil {
						return err
					}
				}
			} else if s != "" {
				if _, err := w.WriteString(" SYSTEM "); err != nil {
					return err
				}
				if err := writeQuoted(w, s); err != nil {
					return err
				}
			}
		}
		return w.WriteByte('>')
	case html.RawNode:
		_, err := w.WriteString(n.Data)
		return err
	default:
		return errors.New("html: unknown node type")
	}

	skipIt := filter(n)

	if !skipIt {
		// Render the <xxx> opening tag.
		if err := w.WriteByte('<'); err != nil {
			return err
		}
		if _, err := w.WriteString(n.Data); err != nil {
			return err
		}
		for _, a := range n.Attr {
			if err := w.WriteByte(' '); err != nil {
				return err
			}
			if a.Namespace != "" {
				if _, err := w.WriteString(a.Namespace); err != nil {
					return err
				}
				if err := w.WriteByte(':'); err != nil {
					return err
				}
			}
			if _, err := w.WriteString(a.Key); err != nil {
				return err
			}
			if _, err := w.WriteString(`="`); err != nil {
				return err
			}
			if err := escapeAttr(w, a.Val); err != nil {
				return err
			}
			if err := w.WriteByte('"'); err != nil {
				return err
			}
		}
		if voidElements[n.Data] {
			if n.FirstChild != nil {
				return fmt.Errorf("html: void element <%s> has child nodes", n.Data)
			}
			_, err := w.WriteString("/>")
			return err
		}
		if err := w.WriteByte('>'); err != nil {
			return err
		}
	}

	// Add initial newline where there is danger of a newline beging ignored.
	if c := n.FirstChild; c != nil && c.Type == html.TextNode && strings.HasPrefix(c.Data, "\n") {
		switch n.Data {
		case "pre", "listing", "textarea":
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
		}
	}

	// Render any child nodes
	if childTextNodesAreLiteral(n) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				if _, err := w.WriteString(c.Data); err != nil {
					return err
				}
			} else {
				if err := render1(w, c, filter); err != nil {
					return err
				}
			}
		}
		if n.Data == "plaintext" {
			// Don't render anything else. <plaintext> must be the
			// last element in the file, with no closing tag.
			return errPlaintextAbort
		}
	} else {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := render1(w, c, filter); err != nil {
				return err
			}
		}
	}

	if !skipIt {
		// Render the </xxx> closing tag.
		if _, err := w.WriteString("</"); err != nil {
			return err
		}
		if _, err := w.WriteString(n.Data); err != nil {
			return err
		}
		return w.WriteByte('>')
	}

	return nil
}

func childTextNodesAreLiteral(n *html.Node) bool {
	// Per WHATWG HTML 13.3, if the parent of the current node is a style,
	// script, xmp, iframe, noembed, noframes, or plaintext element, and the
	// current node is a text node, append the value of the node's data
	// literally. The specification is not explicit about it, but we only
	// enforce this if we are in the HTML namespace (i.e. when the namespace is
	// "").
	// NOTE: we also always include noscript elements, although the
	// specification states that they should only be rendered as such if
	// scripting is enabled for the node (which is not something we track).
	if n.Namespace != "" {
		return false
	}
	switch n.Data {
	case "iframe", "noembed", "noframes", "noscript", "plaintext", "script", "style", "xmp":
		return true
	default:
		return false
	}
}

// writeQuoted writes s to w surrounded by quotes. Normally it will use double
// quotes, but if s contains a double quote, it will use single quotes.
// It is used for writing the identifiers in a doctype declaration.
// In valid HTML, they can't contain both types of quotes.
func writeQuoted(w writer, s string) error {
	var q byte = '"'
	if strings.Contains(s, `"`) {
		q = '\''
	}
	if err := w.WriteByte(q); err != nil {
		return err
	}
	if _, err := w.WriteString(s); err != nil {
		return err
	}
	if err := w.WriteByte(q); err != nil {
		return err
	}
	return nil
}

// Section 12.1.2, "Elements", gives this list of void elements. Void elements
// are those that can't have any contents.
var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"keygen": true, // "keygen" has been removed from the spec, but are kept here for backwards compatibility.
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}
