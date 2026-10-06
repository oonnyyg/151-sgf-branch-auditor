package main

import "fmt"

const (
	Empty = 0
	Black = 1
	White = 2
)

// State is the per-branch game state. Each branch gets its own copy;
// nothing mutable is shared between sibling branches.
type State struct {
	Size      int
	Board     []int // row-major, Size*Size
	ToMove    int   // Black or White
	CapturesB int   // stones captured by Black
	CapturesW int   // stones captured by White
	PrevBoard []int // board before the opponent's last move (simple ko)
	HasPrev   bool
}

func NewState(size int) *State {
	return &State{
		Size:   size,
		Board:  make([]int, size*size),
		ToMove: Black,
	}
}

// Clone deep-copies the state so a child branch cannot mutate its parent.
func (s *State) Clone() *State {
	c := *s
	c.Board = append([]int(nil), s.Board...)
	if s.PrevBoard != nil {
		c.PrevBoard = append([]int(nil), s.PrevBoard...)
	}
	return &c
}

func opponent(color int) int {
	if color == Black {
		return White
	}
	return Black
}

func (s *State) at(x, y int) int { return s.Board[y*s.Size+x] }

// groupAndLiberties returns the stones of the group containing (x,y) and
// the number of distinct empty points adjacent to it.
func (s *State) groupAndLiberties(x, y int) ([]int, int) {
	color := s.at(x, y)
	size := s.Size
	visited := make([]bool, size*size)
	stack := []int{y*size + x}
	visited[y*size+x] = true
	var stones []int
	libs := make(map[int]bool)
	for len(stack) > 0 {
		idx := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		stones = append(stones, idx)
		cx, cy := idx%size, idx/size
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || ny < 0 || nx >= size || ny >= size {
				continue
			}
			ni := ny*size + nx
			switch s.Board[ni] {
			case Empty:
				libs[ni] = true
			case color:
				if !visited[ni] {
					visited[ni] = true
					stack = append(stack, ni)
				}
			}
		}
	}
	return stones, len(libs)
}

// PlayMove applies a move at (x,y) for the side to move (0-based coords).
// On rule violation it returns an error and leaves the state unchanged.
func (s *State) PlayMove(x, y int) error {
	size := s.Size
	if x < 0 || y < 0 || x >= size || y >= size {
		return fmt.Errorf("move out of board (%c%c)", 'a'+byte(x), 'a'+byte(y))
	}
	if s.at(x, y) != Empty {
		return fmt.Errorf("point %c%c already occupied", 'a'+byte(x), 'a'+byte(y))
	}
	color := s.ToMove
	opp := opponent(color)

	prevBoard := append([]int(nil), s.Board...)
	s.Board[y*size+x] = color

	// Capture adjacent opponent groups without liberties first.
	captured := 0
	seen := make(map[int]bool)
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if nx < 0 || ny < 0 || nx >= size || ny >= size {
			continue
		}
		ni := ny*size + nx
		if s.Board[ni] != opp || seen[ni] {
			continue
		}
		stones, libs := s.groupAndLiberties(nx, ny)
		for _, st := range stones {
			seen[st] = true
		}
		if libs == 0 {
			for _, st := range stones {
				s.Board[st] = Empty
				captured++
			}
		}
	}

	// Then check own liberties: suicide is forbidden.
	_, libs := s.groupAndLiberties(x, y)
	if libs == 0 {
		s.Board = prevBoard
		return fmt.Errorf("suicide move at %c%c", 'a'+byte(x), 'a'+byte(y))
	}

	// Simple ko: resulting board must not equal the board before the
	// opponent's previous move.
	if s.HasPrev && boardsEqual(s.Board, s.PrevBoard) {
		s.Board = prevBoard
		return fmt.Errorf("simple ko violation at %c%c", 'a'+byte(x), 'a'+byte(y))
	}

	if color == Black {
		s.CapturesB += captured
	} else {
		s.CapturesW += captured
	}
	s.PrevBoard = prevBoard
	s.HasPrev = true
	s.ToMove = opp
	return nil
}

// Pass applies a pass: legal, advances turn and ko history.
func (s *State) Pass() {
	s.PrevBoard = append([]int(nil), s.Board...)
	s.HasPrev = true
	s.ToMove = opponent(s.ToMove)
}

func boardsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BoardString renders the board with coordinates for CLI output.
func (s *State) BoardString() string {
	var sb []byte
	sb = append(sb, "  "...)
	for x := 0; x < s.Size; x++ {
		sb = append(sb, ' ', byte('a'+x))
	}
	sb = append(sb, '\n')
	for y := 0; y < s.Size; y++ {
		sb = append(sb, fmt.Sprintf("%2d", y+1)...)
		for x := 0; x < s.Size; x++ {
			var c byte
			switch s.at(x, y) {
			case Black:
				c = 'X'
			case White:
				c = 'O'
			default:
				c = '.'
			}
			sb = append(sb, ' ', c)
		}
		sb = append(sb, fmt.Sprintf(" %d\n", y+1)...)
	}
	sb = append(sb, "  "...)
	for x := 0; x < s.Size; x++ {
		sb = append(sb, ' ', byte('a'+x))
	}
	sb = append(sb, '\n')
	return string(sb)
}
