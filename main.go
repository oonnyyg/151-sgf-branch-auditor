package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	jsonOut := flag.Bool("json", false, "emit a JSON report")
	query := flag.Int("query", -1, "print the board and captures for the node with this ID")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: sgf-audit [-json] [-query N] <file.sgf>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	root, err := ParseSGF(strings.TrimPrefix(string(raw), "\uFEFF"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse error, file rejected:", err)
		os.Exit(1)
	}
	rep, err := Audit(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rejected:", err)
		os.Exit(1)
	}

	if *query >= 0 {
		if !printQuery(rep, *query) {
			os.Exit(1)
		}
		return
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	printText(rep)
}

func printText(rep *Report) {
	fmt.Printf("board size: %d, nodes: %d\n", rep.Size, len(rep.Records))
	bad := 0
	for _, r := range rep.Records {
		status := "OK"
		if !r.OK {
			status = "ILLEGAL"
			bad++
		}
		move := r.Move
		if move == "" {
			move = "-"
		}
		fmt.Printf("#%-4d parent=%-4d %-8s %-8s caps B:%d W:%d", r.ID, r.ParentID, move, status, r.BlackCaps, r.WhiteCaps)
		if r.Reason != "" {
			fmt.Printf("  reason: %s", r.Reason)
		}
		if r.Comment != "" {
			fmt.Printf("  C: %s", oneLine(r.Comment))
		}
		fmt.Println()
	}
	fmt.Printf("summary: %d legal, %d illegal\n", len(rep.Records)-bad, bad)
}

func printQuery(rep *Report, id int) bool {
	if id >= len(rep.Records) {
		fmt.Fprintf(os.Stderr, "no node with ID %d (have %d nodes)\n", id, len(rep.Records))
		return false
	}
	r := rep.Records[id]
	fmt.Printf("node #%d parent=%d ok=%v captures B:%d W:%d\n", r.ID, r.ParentID, r.OK, r.BlackCaps, r.WhiteCaps)
	if r.Reason != "" {
		fmt.Println("reason:", r.Reason)
	}
	if r.Board == nil {
		fmt.Println("board unavailable (branch could not be replayed)")
		return true
	}
	for y := 0; y < r.Board.Size; y++ {
		var row []string
		for x := 0; x < r.Board.Size; x++ {
			switch r.Board.at(x, y) {
			case Black:
				row = append(row, "X")
			case White:
				row = append(row, "O")
			default:
				row = append(row, ".")
			}
		}
		fmt.Println(strings.Join(row, " "))
	}
	return true
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
