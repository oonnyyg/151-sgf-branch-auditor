package main

import (
	"fmt"
	"strings"
)

// Property is a single SGF property identifier with its raw values
// (escape sequences already resolved).
type Property struct {
	Ident  string
	Values []string
}

// Node is one SGF node (";" plus its properties).
type Node struct {
	Props    []Property
	Children []*Node
	Parent   *Node
}

// Find returns the values of the first property with the given ident.
func (n *Node) Find(ident string) ([]string, bool) {
	for _, p := range n.Props {
		if p.Ident == ident {
			return p.Values, true
		}
	}
	return nil, false
}

type parser struct {
	data string
	pos  int
}

func (p *parser) err(format string, args ...any) error {
	return fmt.Errorf("sgf parse error at offset %d: %s", p.pos, fmt.Sprintf(format, args...))
}

func (p *parser) peek() byte {
	if p.pos >= len(p.data) {
		return 0
	}
	return p.data[p.pos]
}

func (p *parser) skipWS() {
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.pos++
		} else {
			break
		}
	}
}

// ParseSGF parses a single FF[4] game tree (a collection of exactly one
// top-level game tree) and returns its root node.
func ParseSGF(data string) (*Node, error) {
	data = strings.TrimPrefix(data, "\xEF\xBB\xBF")
	p := &parser{data: data}
	p.skipWS()
	if p.peek() != '(' {
		return nil, p.err("expected '(' at start of game tree")
	}
	root, err := p.parseTree(nil)
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if p.pos != len(p.data) {
		return nil, p.err("trailing data after game tree")
	}
	if root == nil {
		return nil, p.err("empty game tree")
	}
	return root, nil
}

// parseTree parses "(sequence tree...)" and returns the first node of the
// sequence, with variation children attached.
func (p *parser) parseTree(parent *Node) (*Node, error) {
	if p.peek() != '(' {
		return nil, p.err("expected '('")
	}
	p.pos++
	p.skipWS()
	var first, last *Node
	for p.peek() == ';' {
		n, err := p.parseNode()
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = n
			n.Parent = parent
		} else {
			n.Parent = last
			last.Children = append(last.Children, n)
		}
		last = n
		p.skipWS()
	}
	if first == nil {
		return nil, p.err("game tree contains no nodes")
	}
	for p.peek() == '(' {
		child, err := p.parseTree(last)
		if err != nil {
			return nil, err
		}
		last.Children = append(last.Children, child)
		p.skipWS()
	}
	if p.peek() != ')' {
		return nil, p.err("expected ')'")
	}
	p.pos++
	return first, nil
}

func (p *parser) parseNode() (*Node, error) {
	if p.peek() != ';' {
		return nil, p.err("expected ';'")
	}
	p.pos++
	n := &Node{}
	for {
		p.skipWS()
		c := p.peek()
		if c < 'A' || c > 'Z' {
			break
		}
		prop, err := p.parseProperty()
		if err != nil {
			return nil, err
		}
		n.Props = append(n.Props, prop)
	}
	return n, nil
}

func (p *parser) parseProperty() (Property, error) {
	start := p.pos
	for p.peek() >= 'A' && p.peek() <= 'Z' {
		p.pos++
	}
	ident := p.data[start:p.pos]
	if ident == "" {
		return Property{}, p.err("empty property identifier")
	}
	prop := Property{Ident: ident}
	for {
		p.skipWS()
		if p.peek() != '[' {
			break
		}
		p.pos++
		var sb strings.Builder
		for {
			if p.pos >= len(p.data) {
				return Property{}, p.err("unterminated property value for %s", ident)
			}
			c := p.data[p.pos]
			if c == '\\' {
				p.pos++
				if p.pos >= len(p.data) {
					return Property{}, p.err("unterminated escape in %s", ident)
				}
				esc := p.data[p.pos]
				// Escaped line breaks (both \n and \r\n) are removed entirely.
				if esc == '\n' {
					p.pos++
				} else if esc == '\r' {
					p.pos++
					if p.pos < len(p.data) && p.data[p.pos] == '\n' {
						p.pos++
					}
				} else {
					sb.WriteByte(esc)
					p.pos++
				}
				continue
			}
			if c == ']' {
				p.pos++
				break
			}
			sb.WriteByte(c)
			p.pos++
		}
		prop.Values = append(prop.Values, sb.String())
	}
	if len(prop.Values) == 0 {
		return Property{}, p.err("property %s has no value", ident)
	}
	return prop, nil
}
