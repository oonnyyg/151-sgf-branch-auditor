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

func nodeByID(t *testing.T, rep *Report, id int) *NodeReport {
	t.Helper()
	for _, n := range rep.Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("no node id %d", id)
	return nil
}

func TestParseEscapesAndUnknownProps(t *testing.T) {
	rep := audit(t, "(;GM[1]FF[4]SZ[9]XX[foo]C[hi\\] there\\\\];B[dd]C[line1\\\nline2])")
	if len(rep.Nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(rep.Nodes))
	}
	if rep.Nodes[0].Comment != `hi] there\` {
		t.Fatalf("bad comment %q", rep.Nodes[0].Comment)
	}
	if rep.Nodes[1].Comment != "line1line2" {
		t.Fatalf("bad soft-break comment %q", rep.Nodes[1].Comment)
	}
}

func TestRejectStructure(t *testing.T) {
	for _, bad := range []string{
		"", "(;)", "(;GM[2])", "(;FF[3])", "(;SZ[1])", "(;SZ[20])", "(;SZ[9x9])",
		"(;SZ[9]AB[dd])", "(;SZ[9]PL[W])", "(;SZ[9]HA[2])", "(;SZ[9];B[dd]", "(;SZ[9];B)",
		"(;SZ[9];B[dd])(;SZ[9])",
	} {
		if _, err := ParseSGF(bad); err == nil {
			root, perr := ParseSGF(bad)
			if perr != nil {
				continue
			}
			if _, aerr := Audit(root); aerr == nil {
				t.Errorf("expected rejection for %q", bad)
			}
		}
	}
}

func TestMultiCapture(t *testing.T) {
	// Two separate one-stone black groups (bb and bd), each reduced to the
	// single shared liberty bc, both captured by one white move on 5x5.
	rep := audit(t, "(;GM[1]FF[4]SZ[5]"+
		";B[bb];W[ab];B[bd];W[ba]"+
		";B[ee];W[cb];B[ea];W[ad]"+
		";B[ec];W[be];B[eb];W[cd]"+
		";B[ed];W[bc])") // W bc captures bb and bd
	last := rep.Nodes[len(rep.Nodes)-1]
	if !last.OK {
		t.Fatalf("capture move illegal: %s", last.Reason)
	}
	if last.CapturesW != 2 {
		t.Fatalf("want 2 captures, got %d", last.CapturesW)
	}
	if last.state.at(1, 1) != Empty || last.state.at(3, 1) != Empty {
		t.Fatal("captured stones still on board")
	}
}

func TestSuicideAndOccupiedAndOutOfBoard(t *testing.T) {
	rep := audit(t, "(;GM[1]FF[4]SZ[5];B[bb];W[bb])")
	if rep.Nodes[2].OK {
		t.Fatal("occupied point accepted")
	}
	rep = audit(t, "(;GM[1]FF[4]SZ[5];B[ff])")
	if rep.Nodes[1].OK {
		t.Fatal("out-of-board move accepted")
	}
	// Suicide: W plays aa with B on ba and ab.
	rep = audit(t, "(;GM[1]FF[4]SZ[5];B[ba];W[cc];B[ab];W[aa])")
	if rep.Nodes[4].OK {
		t.Fatal("suicide move accepted")
	}
	if !strings.Contains(rep.Nodes[4].Reason, "suicide") {
		t.Fatalf("unexpected reason %q", rep.Nodes[4].Reason)
	}
}

func TestSimpleKo(t *testing.T) {
	// Classic ko shape on 5x5:
	//  B: cb, bc? build: B bb, W ba... use standard:
	// Black: ba? Let's set: B bb, B cb, B cc? Construct:
	//   . X O . .
	//   X O . O .
	//   . X O . .
	// Black stones: ab, ba, bc, cb. White: ac, bb, ca, cc? White captures at...
	// Use well-known sequence:
	// B bb ; W ba ; B cb ; W ca ; B bc ; W db ; B cc ; W dc ; B bd?? too long.
	// Direct scripted shape: B: bb,cb,bc,cc? White: ba,ca,db,dc then W cc captures? Let's
	// just test ko via state machine directly.
	s := NewState(5)
	// Build ko: B at (1,1),(0,2),(2,2),(1,3); W at (0,1)... use coords x,y.
	// Shape (x,y): B (1,0),(0,1),(2,1),(1,2)=? That's a diamond; W stone inside
	// captured by B playing (1,1), then W recapture at (1,1)? W needs its own
	// surrounding stones. Classic ko:
	//   B at b1? Use: B (1,0),(0,1),(2,1); W (1,1)? no.
	// Ko: B stones (1,0),(0,1),(2,1),(1,2)? and W stones (3,0)... simpler:
	// B: (1,0),(0,1),(2,1),(1,2)?? white at (1,1) is dead already.
	// Standard: B (0,1),(1,0),(1,2),(2,1) vs W (1,1),(2,0),(2,2),(3,1):
	// W plays (1,1)?? Let's place: W at (1,1) with B around except (2,1);
	// W stones (2,0),(2,2),(3,1); B stones (1,0),(0,1),(1,2),(2,1)?
	// Then B captures (1,1) by playing? (1,1) is W already... skip shapes,
	// play moves alternating legally:
	moves := [][2]int{
		{1, 0}, {2, 0}, // B W
		{0, 1}, {3, 1}, // B W
		{1, 2}, {2, 2}, // B W
		{4, 4}, {1, 1}, // B filler, W plays into shape capturing?
	}
	for _, m := range moves {
		if err := s.PlayMove(m[0], m[1]); err != nil {
			t.Fatalf("setup move %v: %v", m, err)
		}
	}
	// Now B at (2,1) captures W at (1,1)? B stones: (1,0),(0,1),(1,2) plus
	// (2,1) to be played; W stone (1,1) liberties: (2,1) only -> B (2,1) captures.
	if err := s.PlayMove(2, 1); err != nil {
		t.Fatalf("ko capture: %v", err)
	}
	// W recapturing immediately at (1,1) is a simple-ko violation.
	if err := s.PlayMove(1, 1); err == nil || !strings.Contains(err.Error(), "ko") {
		t.Fatalf("expected ko violation, got %v", err)
	}
	// W passes (legal, advances history), B passes, then recapture is legal.
	s.Pass()
	s.Pass()
	if err := s.PlayMove(1, 1); err != nil {
		t.Fatalf("recapture after passes should be legal: %v", err)
	}
}

func TestBranchIsolation(t *testing.T) {
	// Main line: B dd, W qq(out of board on 9x9? q is beyond i.. size 9 -> a..i).
	// Variation A: illegal move; variation B from same parent must still work.
	sgf := "(;GM[1]FF[4]SZ[9];B[dd]" +
		"(;W[dd];B[ff])" + // illegal: occupied
		"(;W[ff];B[ee]))" // legal alternative
	rep := audit(t, sgf)
	// IDs preorder: 0 root, 1 B[dd], 2 W[dd] illegal, 3 B[ff] not replayable,
	// 4 W[ff] ok, 5 B[ee] ok.
	if rep.Nodes[2].OK {
		t.Fatal("occupied move should be illegal")
	}
	if rep.Nodes[3].OK || !strings.Contains(rep.Nodes[3].Reason, "cannot replay") {
		t.Fatalf("descendant of illegal node should be not replayable: %+v", rep.Nodes[3])
	}
	if !rep.Nodes[4].OK || !rep.Nodes[5].OK {
		t.Fatal("sibling branch polluted by illegal variation")
	}
	if rep.Nodes[5].state.at(3, 3) != Black || rep.Nodes[5].state.at(5, 5) != White {
		t.Fatal("sibling branch board state wrong")
	}
	// Parent state must be untouched: node 1 board has only B dd.
	if rep.Nodes[1].state.at(5, 5) != Empty {
		t.Fatal("parent state mutated by child branch")
	}
}

func TestPassAndTurnOrder(t *testing.T) {
	rep := audit(t, "(;GM[1]FF[4]SZ[9];B[];W[];B[aa])")
	for _, n := range rep.Nodes {
		if !n.OK {
			t.Fatalf("node %d should be ok: %s", n.ID, n.Reason)
		}
	}
	rep = audit(t, "(;GM[1]FF[4]SZ[9];B[aa];B[bb])")
	if rep.Nodes[2].OK {
		t.Fatal("out-of-turn move accepted")
	}
}

func TestSetupPropsRejectedInBody(t *testing.T) {
	rep := audit(t, "(;GM[1]FF[4]SZ[9];B[aa];W[bb]AB[cc])")
	if rep.Nodes[2].OK {
		t.Fatal("AB in body should be rejected")
	}
	if !strings.Contains(rep.Nodes[2].Reason, "not supported") {
		t.Fatalf("unexpected reason %q", rep.Nodes[2].Reason)
	}
}
