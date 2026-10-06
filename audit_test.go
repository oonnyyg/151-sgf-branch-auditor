package main

import (
	"strings"
	"testing"
)

func audit(t *testing.T, sgf string) *Report {
	t.Helper()
	root, err := ParseSGF(sgf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rep, err := Audit(root)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return rep
}

func recByMove(rep *Report, move string) *Record {
	for _, r := range rep.Records {
		if r.Move == move {
			return r
		}
	}
	return nil
}

func TestParseEscapes(t *testing.T) {
	root, err := ParseSGF(`(;FF[4]GM[1]SZ[9]C[a\]b\\c];B[aa]C[line\
break])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := root.Props["C"][0]; got != `a]b\c` {
		t.Fatalf("escaped comment = %q", got)
	}
	child := root.Children[0]
	if got := child.Props["C"][0]; got != "linebreak" {
		t.Fatalf("soft line break = %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{
		"", "(;FF[4])(;GM[1])", "(;FF[4]", "(;B[aa]", "(;Baa)", "(;B[aa]",
	} {
		if _, err := ParseSGF(bad); err == nil {
			t.Fatalf("expected parse error for %q", bad)
		}
	}
}

func TestRootRejects(t *testing.T) {
	for _, sgf := range []string{
		"(;GM[2]SZ[9])",
		"(;FF[3]SZ[9])",
		"(;SZ[1])",
		"(;SZ[20])",
		"(;SZ[9:13])",
	} {
		root, err := ParseSGF(sgf)
		if err != nil {
			t.Fatalf("parse %q: %v", sgf, err)
		}
		if _, err := Audit(root); err == nil {
			t.Fatalf("expected audit rejection for %q", sgf)
		}
	}
}

func TestSetupPropsRejected(t *testing.T) {
	for _, prop := range []string{"AB[aa]", "AW[aa]", "AE[aa]", "PL[W]"} {
		rep := audit(t, "(;SZ[9];"+prop+";B[bb])")
		if rep.Records[1].OK {
			t.Fatalf("expected %s to be rejected", prop)
		}
		if rep.Records[2].OK {
			t.Fatalf("descendant of rejected node must be unreplayable")
		}
	}
}

// Black captures two separate white chains with one move.
func TestMultiCapture(t *testing.T) {
	// White chains at (a,b) and (c,b)... build on 5x5:
	// W at ab(0,1) surrounded by B aa, B ac, B bb; W at cb(2,1) by B ca, B cc, B db.
	// Both chains also touch (1,1)=bb? No: keep them separate.
	// Simpler: white stones at (0,0) and (2,0); black plays (1,0) capturing
	// neither... use edges:
	//   W aa, B ab, B ba  -> aa has libs? aa neighbors: ab(B), ba(B) => dead after B ba.
	// Multi-capture: white at (0,1) and (1,0) are one chain. Use two chains:
	//   chain1: W ba; chain2: W da; both only adjacent to B ca at capture time.
	sgf := "(;SZ[5]" +
		";B[bb];W[ba];B[db];W[da]" + // white chains at ba and da
		";B[ab];W[cc];B[bc];W[cd]" + // surround: ba libs aa,ca; da libs ca,ea
		";B[aa];W[dd];B[ea];W[be]" +
		";B[ca])" // captures both ba and da
	rep := audit(t, sgf)
	last := rep.Records[len(rep.Records)-1]
	if !last.OK {
		t.Fatalf("capturing move rejected: %s", last.Reason)
	}
	if last.BlackCaps != 2 {
		t.Fatalf("black captures = %d, want 2", last.BlackCaps)
	}
	if last.Board.at(1, 0) != Empty || last.Board.at(3, 0) != Empty {
		t.Fatalf("captured stones still on board")
	}
}

func TestSuicideForbidden(t *testing.T) {
	// On 2x2: white at ab and ba, black bb; B[aa] has no liberty.
	rep := audit(t, "(;SZ[2];B[bb];W[ab];B[];W[ba];B[aa])")
	last := rep.Records[len(rep.Records)-1]
	if last.OK || !strings.Contains(last.Reason, "suicide") {
		t.Fatalf("expected suicide rejection, got %+v", last)
	}
}

func TestOccupiedAndOutOfBoard(t *testing.T) {
	rep := audit(t, "(;SZ[9];B[aa];W[aa])")
	if rep.Records[2].OK {
		t.Fatalf("occupied intersection accepted")
	}
	rep = audit(t, "(;SZ[9];B[zz])")
	if rep.Records[1].OK {
		t.Fatalf("out-of-board move accepted")
	}
}
