package main

import (
	"fmt"
	"strings"
)

// Node is one SGF node in the game tree.
type Node struct {
	Props    map[string][]string
	Children []*Node
	Parent   *Node
}

// ParseSGF parses a collection that must contain exactly one game tree.
func ParseSGF(data string) (*Node, error) {
	p := &parser{s: data}
	p.skipWS()
	if p.eof() {
		return nil, fmt.Errorf("empty input")
	}
	root, err := p.gameTree(nil)
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if !p.eof() {
		return nil, fmt.Errorf("unexpected content after game tree at offset %d", p.i)
	}
	return root, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) eof() bool  { return p.i >= len(p.s) }
func (p *parser) peek() byte { return p.s[p.i] }

func (p *parser) skipWS() {
	for !p.eof() {
		c := p.peek()
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.i++
		} else {
			break
		}
	}
}

// gameTree parses "(" sequence variation* ")".
func (p *parser) gameTree(parent *Node) (*Node, error) {
	if p.eof() || p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' at offset %d", p.i)
	}
	p.i++
	p.skipWS()
	var head, tail *Node
	for !p.eof() && p.peek() == ';' {
		n, err := p.node()
		if err != nil {
			return nil, err
		}
		if head == nil {
			head = n
		} else {
			tail.Children = append(tail.Children, n)
			n.Parent = tail
		}
		tail = n
		p.skipWS()
	}
	if head == nil {
		return nil, fmt.Errorf("game tree without nodes at offset %d", p.i)
	}
	if parent != nil {
		parent.Children = append(parent.Children, head)
		head.Parent = parent
	}
	for !p.eof() && p.peek() == '(' {
		if _, err := p.gameTree(tail); err != nil {
			return nil, err
		}
		p.skipWS()
	}
	if p.eof() || p.peek() != ')' {
		return nil, fmt.Errorf("expected ')' at offset %d", p.i)
	}
	p.i++
	return head, nil
}

func (p *parser) node() (*Node, error) {
	p.i++ // consume ';'
	n := &Node{Props: map[string][]string{}}
	for {
		p.skipWS()
		if p.eof() || !isUpper(p.peek()) {
			return n, nil
		}
		start := p.i
		for !p.eof() && isUpper(p.peek()) {
			p.i++
		}
		ident := p.s[start:p.i]
		p.skipWS()
		if p.eof() || p.peek() != '[' {
			return nil, fmt.Errorf("property %q without value at offset %d", ident, p.i)
		}
		for !p.eof() && p.peek() == '[' {
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			n.Props[ident] = append(n.Props[ident], v)
			p.skipWS()
		}
	}
}

// value parses "[...]" handling backslash escapes. An escaped char is taken
// literally; escaped newlines (soft line breaks) are removed.
func (p *parser) value() (string, error) {
	p.i++ // consume '['
	var b strings.Builder
	for !p.eof() {
		c := p.peek()
		if c == '\\' {
			p.i++
			if p.eof() {
				return "", fmt.Errorf("unterminated escape in property value")
			}
			n := p.peek()
			if n == '\n' {
				p.i++
				continue
			}
			if n == '\r' {
				p.i++
				if !p.eof() && p.peek() == '\n' {
					p.i++
				}
				continue
			}
			b.WriteByte(n)
			p.i++
			continue
		}
		if c == ']' {
			p.i++
			return b.String(), nil
		}
		b.WriteByte(c)
		p.i++
	}
	return "", fmt.Errorf("unterminated property value")
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
