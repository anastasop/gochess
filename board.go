package gochess

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	cWHITE = 0x00
	cBLACK = 0x01

	pPAWN   = 1
	pKNIGHT = 2
	pBISHOP = 3
	pROOK   = 4
	pQUEEN  = 5
	pKING   = 6

	fINITIAL = "rnbqkbnr/pppppppp/////PPPPPPPP/RNBQKBNR"
)

var (
	dKNIGHT   [8]int8        = [8]int8{21, 12, 8, 19, -21, -12, -8, -19}
	dDIAGONAL [4]int8        = [4]int8{9, 11, -9, -11}
	dLINES    [4]int8        = [4]int8{1, 10, -1, -10}
	rSANRE    *regexp.Regexp = regexp.MustCompile("^(P?|[RNBQK])([a-h]?[1-8]?)?x?([a-h][1-8])(=[PRNBQK])?")
)

type color uint8

type piece uint8

// Board represents a chess board and tracks pieces positions.
// It provides methods to move pieces and export board positions
// as FEN and GBR
type Board struct {
	sq   [120]piece
	play [8][]piece
	epsq int8
	wksq int8
	bksq int8
	next_move_white bool
}

func colorOf(b bool) color {
	if b {
		return cWHITE
	}
	return cBLACK
}

func (c color) opposite() color {
	if c == cWHITE {
		return cBLACK
	}
	return cWHITE
}

func newPiece(col color, typ uint8, moved bool) piece {
	p := (uint8(col) << 7) | typ
	if moved {
		p |= 0x08
	}
	return piece(p)
}

func (p piece) identify() (col color, typ uint8) {
	return color(p&0x80) >> 7, uint8(p & 0x07)
}

func (p piece) hasMoved() bool {
	return uint8(p&0x08)>>3 == 1
}

func (p piece) markedMoved() piece {
	p |= 0x08
	return p
}

func (p piece) String() string {
	col, typ := p.identify()
	if col == cWHITE {
		return string("_PNBRQK"[typ])
	}
	return string("_pnbrqk"[typ])
}

// NewBoardFromFen returns a Board object initialized with the position of fen
func NewBoardFromFen(fen string) *Board {
	b := &Board{next_move_white: true}
	for i, _ := range b.sq {
		b.sq[i] = 0xff
	}
	fen = strings.TrimSpace(fen)
	ranks := strings.Split(fen+"/////////", "/")[:8]
	repl := strings.NewReplacer(
		"1", "*", "2", "**", "3", "***", "4", "****", "5", "*****",
		"6", "******", "7", "*******", "8", "********")
	for r, rank := range ranks {
		sq := 91 - r*10
		s := (repl.Replace(rank) + "********")[:8]
		for f, p := range s {
			switch p {
			case 'P':
				b.sq[sq+f] = 1
			case 'N':
				b.sq[sq+f] = 2
			case 'B':
				b.sq[sq+f] = 3
			case 'R':
				b.sq[sq+f] = 4
			case 'Q':
				b.sq[sq+f] = 5
			case 'K':
				b.sq[sq+f] = 6
				b.wksq = int8(sq + f)
			case 'p':
				b.sq[sq+f] = 129
			case 'n':
				b.sq[sq+f] = 130
			case 'b':
				b.sq[sq+f] = 131
			case 'r':
				b.sq[sq+f] = 132
			case 'q':
				b.sq[sq+f] = 133
			case 'k':
				b.sq[sq+f] = 134
				b.bksq = int8(sq + f)
			default:
				b.sq[sq+f] = 0
			}
		}
	}
	b.play[0] = b.sq[91:99]
	b.play[1] = b.sq[81:89]
	b.play[2] = b.sq[71:79]
	b.play[3] = b.sq[61:69]
	b.play[4] = b.sq[51:59]
	b.play[5] = b.sq[41:49]
	b.play[6] = b.sq[31:39]
	b.play[7] = b.sq[21:29]
	return b
}

// NewBoard returns a Board object initialized with the standard starting position
func NewBoard() *Board {
	return NewBoardFromFen(fINITIAL)
}

func sq2string(s int8) string {
	b := make([]byte, 2)
	b[0] = byte('a' - 1 + s%10)
	b[1] = byte('1' + ((s / 10) - 2))
	return string(b)
}

func (b *Board) attackers(sq int8, col color, includePawnMoves bool) []int8 {
	s := make([]int8, 0)

	for _, d := range dKNIGHT {
		to := sq + d
		piece := b.sq[to]
		if c, t := piece.identify(); c == col && t == pKNIGHT {
			s = append(s, to)
		}
	}
	for _, d := range dDIAGONAL {
		for to := sq + d; b.sq[to] != 0xff; to += d {
			if piece := b.sq[to]; piece != 0 {
				if c, t := piece.identify(); c == col && (t == pQUEEN || t == pBISHOP) {
					s = append(s, to)
				}
				break
			}
		}
	}
	for _, d := range dDIAGONAL {
		to := sq + d
		if piece := b.sq[to]; piece != 0 {
			if c, t := piece.identify(); c == col && t == pKING {
				s = append(s, to)
			}
		}
	}
	for _, d := range dLINES {
		for to := sq + d; b.sq[to] != 0xff; to += d {
			if piece := b.sq[to]; piece != 0 {
				if c, t := piece.identify(); c == col && (t == pQUEEN || t == pROOK) {
					s = append(s, to)
				}
				break
			}
		}
	}
	for _, d := range dLINES {
		to := sq + d
		if piece := b.sq[to]; piece != 0 {
			if c, t := piece.identify(); c == col && t == pKING {
				s = append(s, to)
			}
		}
	}

	direction := int8(1)
	if col == cWHITE {
		direction = int8(-1)
	}
	if to := sq + direction*11; b.sq[to] != 0xff {
		if c, t := b.sq[to].identify(); c == col && t == pPAWN {
			s = append(s, to)
		}
	}
	if to := sq + direction*9; b.sq[to] != 0xff {
		if c, t := b.sq[to].identify(); c == col && t == pPAWN {
			s = append(s, to)
		}
	}
	if includePawnMoves {
		if to := sq + direction*10; b.sq[to] != 0xff {
			if b.sq[to] == 0 {
				to += direction * 10
				if c, t := b.sq[to].identify(); c == col && t == pPAWN && !b.sq[to].hasMoved() {
					s = append(s, to)
				}
			} else if c, t := b.sq[to].identify(); c == col && t == pPAWN {
				s = append(s, to)
			}
		}
	}
	return s
}

// MakeMove makes a move on the board
// If the move is illegal like a king move to a checked square or the move is ambiguous
// as if two pieces can move to the same square, then it returns an error and the board
// does not record the move. The board keeps track of which color moved previously and
// alternates
func (b *Board) MakeMove(san string) error {
	err := b.makeMoveFor(san, b.next_move_white)
	b.next_move_white = !b.next_move_white
	return err
}

// SetTurn sets who makes the next move
func (b *Board) SetTurn(whiteMove bool) {
	b.next_move_white = whiteMove
}

func (b *Board) makeMoveFor(san string, whiteMove bool) error {
	if san == "--" {
		return nil
	}
	if strings.HasPrefix(san, "O-O-O") {
		if whiteMove {
			b.sq[21], b.sq[22], b.sq[23], b.sq[24], b.sq[25] = 0, 0, newPiece(cWHITE, pKING, true), newPiece(cWHITE, pROOK, true), 0
			b.wksq = 23
		} else {
			b.sq[91], b.sq[92], b.sq[93], b.sq[94], b.sq[95] = 0, 0, newPiece(cBLACK, pKING, true), newPiece(cBLACK, pROOK, true), 0
			b.bksq = 93
		}
		b.epsq = 0
		return nil
	}
	if strings.HasPrefix(san, "O-O") {
		if whiteMove {
			b.sq[25], b.sq[26], b.sq[27], b.sq[28] = 0, newPiece(cWHITE, pROOK, true), newPiece(cWHITE, pKING, true), 0
			b.wksq = 27
		} else {
			b.sq[95], b.sq[96], b.sq[97], b.sq[98] = 0, newPiece(cBLACK, pROOK, true), newPiece(cBLACK, pKING, true), 0
			b.bksq = 97
		}
		b.epsq = 0
		return nil
	}

	matches := rSANRE.FindStringSubmatch(san)
	if matches == nil || len(matches) != 5 {
		return fmt.Errorf("san %q is not a valid move", san)
	}
	piece, fromHint, dsq, promotes := matches[1], matches[2], matches[3], matches[4]
	if piece == "" {
		piece = "P"
		if fromHint == "" {
			fromHint = dsq[:1]
		}
	}
	pieceTyp := uint8(strings.Index("PNBRQK", piece) + 1)
	tosq := int8(21 + (dsq[1]-'1')*10 + dsq[0] - 'a')

	attackers := b.attackers(tosq, colorOf(whiteMove), true)
	if attackers == nil {
		return fmt.Errorf("no attackers: SAN %s", san)
	}

	candidates := make([]int8, 0)
	for _, attacker := range attackers {
		if _, typ := b.sq[attacker].identify(); typ == pieceTyp {
			if fromHint == "" || strings.Index(sq2string(attacker), fromHint) >= 0 {
				if b.tryMove(true, whiteMove, attacker, tosq, promotes) == nil {
					candidates = append(candidates, attacker)
				}
			}
		}
	}
	if len(candidates) != 1 {
		return fmt.Errorf("there are %d candidate moves for %s", len(candidates), san)
	}
	return b.tryMove(false, whiteMove, candidates[0], tosq, promotes)
}

func (b *Board) tryMove(try bool, whiteMove bool, csq, tosq int8, promotes string) error {
	colorMove, step := color(cBLACK), int8(-10)
	if whiteMove {
		colorMove, step = color(cWHITE), int8(10)
	}

	// TODO rollback ep moves
	if try {
		defer func(tosq, csq, epsq, wksq, bksq int8, tosqp, csqp piece) {
			b.sq[tosq] = tosqp
			b.sq[csq] = csqp
			b.epsq = epsq
			b.wksq = wksq
			b.bksq = bksq
		}(tosq, csq, b.epsq, b.wksq, b.bksq, b.sq[tosq], b.sq[csq])
	}

	cPiece := b.sq[csq]
	if c, t := cPiece.identify(); t == pPAWN {
		if !cPiece.hasMoved() && csq+2*step == tosq {
			b.epsq = tosq - step
		} else {
			if tosq == b.epsq {
				b.sq[b.epsq-step] = 0
			}
			b.epsq = 0
		}
		if promotes != "" {
			cPiece = newPiece(colorMove, uint8(strings.Index("PNBRQK", promotes[1:2])+1), true)
		}
		b.sq[tosq] = cPiece.markedMoved()
	} else {
		b.sq[tosq] = cPiece.markedMoved()
		b.epsq = 0
		if t == pKING {
			if c == cBLACK {
				b.bksq = tosq
			} else {
				b.wksq = tosq
			}
		}
	}
	b.sq[csq] = 0

	ksq := b.bksq
	if whiteMove {
		ksq = b.wksq
	}
	attackers := b.attackers(ksq, colorMove.opposite(), false)
	if len(attackers) != 0 {
		return fmt.Errorf("invalid move. King at %v is attacked by %v", sq2string(ksq), sq2string(attackers[0]))
	}
	return nil
}

// Fen returns the board position as a standard FEN string see http://en.wikipedia.org/wiki/Forsyth%E2%80%93Edwards_Notation
func (b *Board) Fen() string {
	s := ""
	for _, rank := range b.play {
		nempty := 0
		for _, piece := range rank {
			if t := piece.String(); t == "_" {
				nempty++
			} else {
				if nempty > 0 {
					s += strconv.Itoa(nempty)
				}
				nempty = 0
				s += t
			}
		}
		if nempty > 0 {
			s += strconv.Itoa(nempty)
		}
		nempty = 0
		s += "/"
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Fen returns the GBR code for the position see http://en.wikipedia.org/wiki/GBR_code
func (b *Board) Gbr() string {
	var white, black [7]int
	for _, rank := range b.play {
		for _, piece := range rank {
			if col, typ := piece.identify(); col == cWHITE {
				white[typ]++
			} else {
				black[typ]++
			}
		}
	}
	return fmt.Sprintf("%1d%1d%1d%1d.%1d%1d",
		min(white[pQUEEN]+3*black[pQUEEN], 9),
		min(white[pROOK]+3*black[pROOK], 9),
		min(white[pBISHOP]+3*black[pBISHOP], 9),
		min(white[pKNIGHT]+3*black[pKNIGHT], 9),
		min(white[pPAWN], 9),
		min(black[pPAWN], 9))
}

// String returns a string representation of the board better suited for debugging
func (b *Board) String() string {
	s := ""
	for _, rank := range b.play {
		for _, piece := range rank {
			s += piece.String()
		}
		s += "/"
	}
	return s
}
