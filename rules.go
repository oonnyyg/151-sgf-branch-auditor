package main

import "fmt"

const (
	Empty = 0
	Black = 1
	White = 2
)

// Board is an immutable square grid; index = y*size+x.
type Board struct {
	Size  int
	Cells []uint8
}

func NewBoard(size int) *Board {
	return &Board{Size: size, Cells: make([]uint8, size*size)}
}

func (b *Board) at(x, y int) uint8 { return b.Cells[y*b.Size+x] }

func (b *Board) equal(o *Board) bool {
	if b == nil || o == nil {
		return b == o
	}
	if b.Size != o.Size || len(b.Cells) != len(o.Cells) {
		return false
	}
	for i := range b.Cells {
		if b.Cells[i] != o.Cells[i] {
			return false
		}
	}
	return true
}

// neighbors returns in-board orthogonal neighbors.
func (b *Board) neighbors(x, y int) [][2]int {
	var out [][2]int
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if nx >= 0 && nx < b.Size && ny >= 0 && ny < b.Size {
			out = append(out, [2]int{nx, ny})
		}
	}
	return out
}

// group collects the connected same-color chain and its liberty count.
func (b *Board) group(x, y int) (stones [][2]int, liberties int) {
	color := b.at(x, y)
	seen := map[int]bool{y*b.Size + x: true}
	libSeen := map[int]bool{}
	stack := [][2]int{{x, y}}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		stones = append(stones, c)
		for _, nb := range b.neighbors(c[0], c[1]) {
			idx := nb[1]*b.Size + nb[0]
			v := b.Cells[idx]
			if v == Empty {
				if !libSeen[idx] {
					libSeen[idx] = true
					liberties++
				}
			} else if v == color && !seen[idx] {
				seen[idx] = true
				stack = append(stack, nb)
			}
		}
	}
	return stones, liberties
}

// ApplyMove places color at (x,y) on a copy of b. It captures adjacent
// opponent groups without liberties first, then rejects suicide.
// Returns the new board and the number of captured stones.
func (b *Board) ApplyMove(x, y int, color uint8) (*Board, int, error) {
	if x < 0 || x >= b.Size || y < 0 || y >= b.Size {
		return nil, 0, fmt.Errorf("move out of board at %c%c", 'a'+byte(x), 'a'+byte(y))
	}
	if b.at(x, y) != Empty {
		return nil, 0, fmt.Errorf("intersection %c%c already occupied", 'a'+byte(x), 'a'+byte(y))
	}
	cells := make([]uint8, len(b.Cells))
	copy(cells, b.Cells)
	nb := &Board{Size: b.Size, Cells: cells}
	nb.Cells[y*b.Size+x] = color

	var opp uint8 = White
	if color == White {
		opp = Black
	}
	captured := 0
	done := map[int]bool{}
	for _, n := range nb.neighbors(x, y) {
		idx := n[1]*b.Size + n[0]
		if nb.Cells[idx] == opp && !done[idx] {
			stones, libs := nb.group(n[0], n[1])
			if libs == 0 {
				for _, s := range stones {
					nb.Cells[s[1]*b.Size+s[0]] = Empty
					done[s[1]*b.Size+s[0]] = true
					captured++
				}
			}
		}
	}
	if _, libs := nb.group(x, y); libs == 0 {
		return nil, 0, fmt.Errorf("suicide move at %c%c is forbidden", 'a'+byte(x), 'a'+byte(y))
	}
	return nb, captured, nil
}

// ParsePoint converts an SGF point like "cd" to coordinates.
// Empty string means pass. Anything else is invalid.
func ParsePoint(v string, size int) (x, y int, pass bool, err error) {
	if v == "" {
		return 0, 0, true, nil
	}
	if len(v) != 2 || v[0] < 'a' || v[1] < 'a' {
		return 0, 0, false, fmt.Errorf("invalid move coordinate %q", v)
	}
	x, y = int(v[0]-'a'), int(v[1]-'a')
	if x >= size || y >= size {
		return 0, 0, false, fmt.Errorf("move %q out of board (size %d)", v, size)
	}
	return x, y, false, nil
}
