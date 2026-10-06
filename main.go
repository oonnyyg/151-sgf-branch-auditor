package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	jsonOut := flag.Bool("json", false, "emit JSON report instead of text")
	query := flag.Int("query", -1, "print board and captures for node ID")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: sgf-audit [-json] [-query N] file.sgf")
		os.Exit(2)
	}
	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	root, err := ParseSGF(string(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, "rejected:", err)
		os.Exit(1)
	}
	rep, err := Audit(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rejected:", err)
		os.Exit(1)
	}

	if *query >= 0 {
		if *query >= len(rep.Nodes) {
			fmt.Fprintln(os.Stderr, "no node with id", *query)
			os.Exit(1)
		}
		rec := rep.Nodes[*query]
		if rec.state == nil {
			fmt.Printf("node %d: not replayable (%s)\n", rec.ID, rec.Reason)
			os.Exit(1)
		}
		fmt.Printf("node %d (parent %d) move %s\n", rec.ID, rec.ParentID, rec.Move)
		fmt.Print(rec.state.BoardString())
		fmt.Printf("captures: B=%d W=%d, %s to move\n",
			rec.CapturesB, rec.CapturesW, colorName(rec.state.ToMove))
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

	fmt.Printf("board size: %d\n", rep.Size)
	for _, rec := range rep.Nodes {
		status := "OK"
		if !rec.OK {
			status = "ILLEGAL: " + rec.Reason
		}
		fmt.Printf("#%d parent=%d move=%s %s\n", rec.ID, rec.ParentID, rec.Move, status)
	}
}
