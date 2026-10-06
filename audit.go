package main

import (
	"fmt"
	"strconv"
)

// NodeReport is the audit result for one SGF node.
type NodeReport struct {
	ID        int    `json:"id"`
	ParentID  int    `json:"parent"` // -1 for the root
	Move      string `json:"move"`   // e.g. "B[dd]", "W[]", "-" if none
	Comment   string `json:"comment,omitempty"`
	OK        bool   `json:"ok"`
	Reason    string `json:"reason,omitempty"`     // why illegal / not replayable
	CapturesB int    `json:"captures_b,omitempty"` // cumulative, only when OK
	CapturesW int    `json:"captures_w,omitempty"`
	state     *State // board after this node; nil when not replayable
}

// Report is the full audit of one SGF file.
type Report struct {
	Size  int           `json:"size"`
	Nodes []*NodeReport `json:"nodes"`
}

// setupProps are explicitly rejected: we do not pretend to support them.
var setupProps = map[string]string{
	"AB": "setup property AB (add black) is not supported",
	"AW": "setup property AW (add white) is not supported",
	"AE": "setup property AE (add empty) is not supported",
	"PL": "setup property PL (player to play) is not supported",
	"HA": "handicap property HA is not supported",
}

// Audit validates the parsed tree and returns per-node reports.
// Structural/root errors reject the whole file with an error.
func Audit(root *Node) (*Report, error) {
	size, err := rootSettings(root)
	if err != nil {
		return nil, err
	}
	rep := &Report{Size: size}
	var walk func(n *Node, st *State, parentID int, parentOK bool)
	walk = func(n *Node, st *State, parentID int, parentOK bool) {
		rec := &NodeReport{ID: len(rep.Nodes), ParentID: parentID, Move: "-"}
		rep.Nodes = append(rep.Nodes, rec)

		if !parentOK {
			rec.Reason = "ancestor node illegal: cannot replay"
		} else {
			// Branch-private state: never shared with siblings.
			nst := st.Clone()
			rec.OK = true
			for _, p := range n.Props {
				if !rec.OK {
					break
				}
				switch p.Ident {
				case "B", "W":
					if len(p.Values) != 1 {
						rec.fail(fmt.Sprintf("property %s expects exactly one value", p.Ident))
						break
					}
					color := Black
					if p.Ident == "W" {
						color = White
					}
					rec.Move = fmt.Sprintf("%s[%s]", p.Ident, p.Values[0])
					if color != nst.ToMove {
						rec.fail(fmt.Sprintf("move %s out of turn: %s to move", p.Ident, colorName(nst.ToMove)))
						break
					}
					if p.Values[0] == "" {
						nst.Pass()
						break
					}
					x, y, ok := parsePoint(p.Values[0], nst.Size)
					if !ok {
						rec.fail(fmt.Sprintf("bad coordinate %q", p.Values[0]))
						break
					}
					if err := nst.PlayMove(x, y); err != nil {
						rec.fail(err.Error())
					}
				case "C":
					if len(p.Values) > 0 {
						rec.Comment = p.Values[0]
					}
				default:
					if msg, bad := setupProps[p.Ident]; bad {
						rec.fail(msg)
					}
					// Unknown / other ordinary properties are skipped.
				}
			}
			if rec.OK {
				rec.state = nst
				rec.CapturesB = nst.CapturesB
				rec.CapturesW = nst.CapturesW
			}
		}

		childState := rec.state
		if childState == nil {
			childState = st // placeholder; children are marked not replayable
		}
		for _, c := range n.Children {
			walk(c, childState, rec.ID, rec.OK)
		}
	}
	walk(root, NewState(size), -1, true)
	return rep, nil
}

func (r *NodeReport) fail(reason string) {
	r.OK = false
	r.Reason = reason
}

func colorName(c int) string {
	if c == Black {
		return "B"
	}
	return "W"
}

func parsePoint(v string, size int) (int, int, bool) {
	if len(v) != 2 || v[0] < 'a' || v[1] < 'a' {
		return 0, 0, false
	}
	x, y := int(v[0]-'a'), int(v[1]-'a')
	if x >= size || y >= size {
		return 0, 0, false
	}
	return x, y, true
}

// rootSettings validates root properties GM/FF/SZ. The root must be an
// empty board with Black to move; any setup property rejects the file.
func rootSettings(root *Node) (int, error) {
	v, ok := root.Find("GM")
	if !ok || len(v) != 1 || v[0] != "1" {
		return 0, fmt.Errorf("root must declare GM[1] (Go)")
	}
	v, ok = root.Find("FF")
	if !ok || len(v) != 1 || v[0] != "4" {
		return 0, fmt.Errorf("root must declare FF[4]")
	}
	size := 19
	if v, ok := root.Find("SZ"); ok {
		if len(v) != 1 {
			return 0, fmt.Errorf("SZ expects one value")
		}
		n, err := strconv.Atoi(v[0])
		if err != nil || n < 2 || n > 19 {
			return 0, fmt.Errorf("SZ must be a square size 2..19, got %q", v[0])
		}
		size = n
	}
	for _, p := range root.Props {
		if msg, bad := setupProps[p.Ident]; bad {
			return 0, fmt.Errorf("root: %s", msg)
		}
	}
	if _, ok := root.Find("B"); ok {
		return 0, fmt.Errorf("root node must not contain a move")
	}
	if _, ok := root.Find("W"); ok {
		return 0, fmt.Errorf("root node must not contain a move")
	}
	return size, nil
}
