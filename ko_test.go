package main

import "testing"

// Classic ko diamond on 4x4:
//
//	  a b c d
//	a . X O .   B: ba ab bc cb   W: ca cc db bb
//	b X O X O   B[cb] captures the single white stone bb;
//	c . X O .   immediate W[bb] recapture would restore the
//	d . . . X   board from before B[cb] -> simple ko violation.
const koSetup = "(;SZ[4]" +
	";B[ba];W[ca]" +
	";B[ab];W[cc]" +
	";B[bc];W[db]" +
	";B[dd];W[bb]" + // W bb: only liberty is cb -> legal
	";B[cb]" // captures W bb

func TestSimpleKo(t *testing.T) {
	rep := audit(t, koSetup+";W[bb])")
	last := rep.Records[len(rep.Records)-1]
	if last.OK {
		t.Fatalf("immediate ko recapture accepted")
	}
	capturing := rep.Records[len(rep.Records)-2]
	if !capturing.OK || capturing.BlackCaps != 1 {
		t.Fatalf("capturing move wrong: %+v", capturing)
	}
}

// After a pass by each side the reference position advances, so the
// recapture becomes legal again.
func TestKoDissolvesAfterPasses(t *testing.T) {
	rep := audit(t, koSetup+";W[];B[];W[bb])")
	last := rep.Records[len(rep.Records)-1]
	if !last.OK {
		t.Fatalf("recapture after passes should be legal: %s", last.Reason)
	}
	if last.WhiteCaps != 1 {
		t.Fatalf("white captures = %d, want 1", last.WhiteCaps)
	}
}

func TestPassLegalAndAdvances(t *testing.T) {
	rep := audit(t, "(;SZ[9];B[];W[];B[];W[])")
	for i, r := range rep.Records {
		if !r.OK {
			t.Fatalf("record %d illegal: %s", i, r.Reason)
		}
	}
}

func TestWrongTurn(t *testing.T) {
	rep := audit(t, "(;SZ[9];B[aa];B[bb])")
	if rep.Records[2].OK {
		t.Fatalf("double black accepted")
	}
	rep = audit(t, "(;SZ[9];W[aa])")
	if rep.Records[1].OK {
		t.Fatalf("white-first accepted")
	}
}

func TestUnknownPropsSkipped(t *testing.T) {
	rep := audit(t, "(;SZ[9]XX[foo]PB[me];B[aa]XY[1];W[bb])")
	for i, r := range rep.Records {
		if !r.OK {
			t.Fatalf("record %d illegal: %s", i, r.Reason)
		}
	}
}

// A failing branch must not affect its sibling, and sibling state must be
// fully independent (board, captures, ko history).
func TestBranchIsolation(t *testing.T) {
	sgf := "(;SZ[5];B[cc];W[cd]" +
		"(;B[aa];W[aa])" + // branch A: W[aa] illegal (occupied)
		"(;B[bb];W[bc];B[dd]))" // branch B: fully legal
	rep := audit(t, sgf)
	// preorder: 0 root, 1 Bcc, 2 Wcd, 3 Baa, 4 Waa(bad), 5 Bbb, 6 Wbc, 7 Bdd
	if rep.Records[4].OK {
		t.Fatalf("branch A illegal move accepted")
	}
	for _, id := range []int{5, 6, 7} {
		if !rep.Records[id].OK {
			t.Fatalf("branch B record %d polluted: %s", id, rep.Records[id].Reason)
		}
	}
	if rep.Records[7].Board.at(0, 0) != Empty {
		t.Fatalf("sibling branch leaked stone into branch B")
	}
	if rep.Records[7].Board.at(2, 2) != Black || rep.Records[7].Board.at(2, 3) != White {
		t.Fatalf("parent state missing in branch B")
	}
}

// Captures made in one sibling must not appear in the other's counters.
func TestCaptureCountsIsolated(t *testing.T) {
	sgf := "(;SZ[4];B[ba];W[ab];B[bb]" +
		"(;W[ba];B[da])" + // branch A: W[ba] illegal (occupied)
		"(;W[bc];B[ab])" + // branch B: B[ab] illegal (occupied)
		"(;W[ac];B[cb]))" // branch C legal
	rep := audit(t, sgf)
	// preorder: 0 root, 1 Bba, 2 Wab, 3 Bbb,
	// 4 Wba(bad), 5 Bda(dead), 6 Wbc, 7 Bab(bad), 8 Wac, 9 Bcb
	if rep.Records[4].OK || rep.Records[7].OK {
		t.Fatalf("illegal siblings accepted")
	}
	if !rep.Records[6].OK || !rep.Records[8].OK || !rep.Records[9].OK {
		t.Fatalf("legal branch rejected")
	}
}

func TestPreorderNumbering(t *testing.T) {
	rep := audit(t, "(;SZ[9];B[aa](;W[bb])(;W[cc];B[dd]))")
	want := []struct{ id, parent int }{
		{0, -1}, {1, 0}, {2, 1}, {3, 1}, {4, 3},
	}
	for i, w := range want {
		if rep.Records[i].ID != w.id || rep.Records[i].ParentID != w.parent {
			t.Fatalf("record %d: got id=%d parent=%d, want %+v",
				i, rep.Records[i].ID, rep.Records[i].ParentID, w)
		}
	}
}

func TestDescendantsMarkedUnreplayable(t *testing.T) {
	rep := audit(t, "(;SZ[9];B[aa];W[aa](;B[bb];W[cc])(;B[dd]))")
	// records: 0 root, 1 Baa, 2 Waa(bad), 3 Bbb dead, 4 Wcc dead, 5 Bdd dead
	for _, id := range []int{3, 4, 5} {
		r := rep.Records[id]
		if r.OK || r.Board != nil {
			t.Fatalf("record %d should be unreplayable without board: %+v", id, r)
		}
	}
}
