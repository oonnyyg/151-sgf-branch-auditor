package main

import (
	"fmt"
	"strconv"
	"strings"
)

// setup or turn-forcing properties are explicitly unsupported.
var rejectedProps = map[string]string{
	"AB": "add black stones (setup) is not supported",
	"AW": "add white stones (setup) is not supported",
	"AE": "clear intersections (setup) is not supported",
	"PL": "specifying the player to move is not supported",
}

// Record is the audit result for one node.
type Record struct {
	ID        int    `json:"id"`
	ParentID  int    `json:"parent"` // -1 for root
	Move      string `json:"move"`   // e.g. "B[cd]", "W[]", "" for root
	Comment   string `json:"comment,omitempty"`
	OK        bool   `json:"ok"`
	Reason    string `json:"reason,omitempty"`
	BlackCaps int    `json:"blackCaptures"`
	WhiteCaps int    `json:"whiteCaptures"`

	Board *Board `json:"-"`
}

// Report is the full audit of one game tree.
type Report struct {
	Size    int       `json:"size"`
	Records []*Record `json:"records"`
}

// state is the per-branch replay state. Every node gets its own copy,
// so sibling branches never share mutable state.
type state struct {
	board     *Board
	toMove    uint8
	blackCaps int
	whiteCaps int
	prevBoard *Board // board before the last move (ko reference)
}

// Audit replays the whole tree and returns per-node records in preorder.
func Audit(root *Node) (*Report, error) {
	size, err := rootSize(root)
	if err != nil {
		return nil, err
	}
	rep := &Report{Size: size}
	init := state{board: NewBoard(size), toMove: Black}
	walk(root, -1, init, rep)
	return rep, nil
}

func rootSize(root *Node) (int, error) {
	if vs, ok := root.Props["GM"]; ok {
		if len(vs) != 1 || strings.TrimSpace(vs[0]) != "1" {
			return 0, fmt.Errorf("GM must be 1 (go), got %q", vs)
		}
	}
	if vs, ok := root.Props["FF"]; ok {
		if len(vs) != 1 || strings.TrimSpace(vs[0]) != "4" {
			return 0, fmt.Errorf("only FF[4] is supported, got %q", vs)
		}
	}
	size := 19
	if vs, ok := root.Props["SZ"]; ok {
		if len(vs) != 1 {
			return 0, fmt.Errorf("SZ expects a single value")
		}
		v := strings.TrimSpace(vs[0])
		if strings.Contains(v, ":") {
			return 0, fmt.Errorf("non-square board SZ[%s] is not supported", v)
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 2 || n > 19 {
			return 0, fmt.Errorf("board size %q out of range 2..19", v)
		}
		size = n
	}
	return size, nil
}

func walk(n *Node, parentID int, st state, rep *Report) {
	rec := &Record{ID: len(rep.Records), ParentID: parentID, OK: true}
	rep.Records = append(rep.Records, rec)

	cur, ok, reason := applyNode(n, st, parentID == -1)
	rec.OK = ok
	rec.Reason = reason
	rec.BlackCaps = cur.blackCaps
	rec.WhiteCaps = cur.whiteCaps
	rec.Board = cur.board
	if mv, isMove := moveProp(n); isMove {
		rec.Move = mv
	}
	if c, ok := n.Props["C"]; ok && len(c) > 0 {
		rec.Comment = c[0]
	}
	if !ok {
		// Descendants cannot be replayed; mark them without applying rules.
		markDead(n, rec.ID, rep)
		return
	}
	for _, ch := range n.Children {
		walk(ch, rec.ID, cur, rep)
	}
}

// markDead records every descendant as unreplayable, still in preorder.
func markDead(n *Node, parentID int, rep *Report) {
	for _, ch := range n.Children {
		rec := &Record{
			ID:       len(rep.Records),
			ParentID: parentID,
			OK:       false,
			Reason:   "ancestor node is illegal; branch cannot be replayed",
		}
		if mv, isMove := moveProp(ch); isMove {
			rec.Move = mv
		}
		if c, ok := ch.Props["C"]; ok && len(c) > 0 {
			rec.Comment = c[0]
		}
		rep.Records = append(rep.Records, rec)
		markDead(ch, rec.ID, rep)
	}
}

func moveProp(n *Node) (string, bool) {
	if vs, ok := n.Props["B"]; ok && len(vs) > 0 {
		return "B[" + vs[0] + "]", true
	}
	if vs, ok := n.Props["W"]; ok && len(vs) > 0 {
		return "W[" + vs[0] + "]", true
	}
	return "", false
}

// applyNode validates one node against the inherited state and returns the
// next state. The input state is never mutated.
func applyNode(n *Node, st state, isRoot bool) (state, bool, string) {
	for prop, msg := range rejectedProps {
		if _, ok := n.Props[prop]; ok {
			return st, false, fmt.Sprintf("property %s: %s", prop, msg)
		}
	}
	if _, hasB := n.Props["B"]; hasB && has(n, "W") {
		return st, false, "node contains both B and W moves"
	}
	if _, hasB := n.Props["B"]; !hasB && !has(n, "W") {
		if isRoot {
			return st, true, "" // empty root: empty board, black to move
		}
		return st, true, "" // annotation-only node: state passes through
	}

	var color uint8
	var vals []string
	if has(n, "B") {
		color = Black
		vals = n.Props["B"]
	} else {
		color = White
		vals = n.Props["W"]
	}
	if len(vals) != 1 {
		return st, false, "move property must have exactly one value"
	}
	if color != st.toMove {
		return st, false, fmt.Sprintf("wrong turn: expected %s to move", colorName(st.toMove))
	}
	x, y, pass, err := ParsePoint(vals[0], st.board.Size)
	if err != nil {
		return st, false, err.Error()
	}

	next := st
	next.prevBoard = st.board
	next.toMove = opponent(st.toMove)

	if pass {
		return next, true, "" // pass is always legal and advances history
	}
	nb, captured, err := st.board.ApplyMove(x, y, color)
	if err != nil {
		return st, false, err.Error()
	}
	// Simple ko: the resulting board must not equal the board from
	// before the opponent's previous move.
	if st.prevBoard != nil && nb.equal(st.prevBoard) {
		return st, false, fmt.Sprintf("simple ko: move at %c%c repeats the board before the opponent's last move", 'a'+byte(x), 'a'+byte(y))
	}
	next.board = nb
	if color == Black {
		next.blackCaps += captured
	} else {
		next.whiteCaps += captured
	}
	return next, true, ""
}

func has(n *Node, prop string) bool {
	_, ok := n.Props[prop]
	return ok
}

func opponent(c uint8) uint8 {
	if c == Black {
		return White
	}
	return Black
}

func colorName(c uint8) string {
	if c == Black {
		return "black"
	}
	return "white"
}
