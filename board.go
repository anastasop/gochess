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

	fINITIAL = "rnbqkbnr/pppppppp/////PPPPPPPP/RNBQKBNR w KQkq - 0 1"

	sSQUARES =
		"a1a2a3a4a5a6a7a8" +
		"b1b2b3b4b5b6b7b8" +
		"c1c2c3c4c5c6c7c8" +
		"d1d2d3d4d5d6d7d8" +
		"e1e2e3e4e5e6e7e8" +
		"f1f2f3f4f5f6f7f8" +
		"g1g2g3g4g5g6g7g8" +
		"h1h2h3h4h5h6h7h8"
)

var (
	dKNIGHT   [8]int8        = [8]int8{21, 12, 8, 19, -21, -12, -8, -19}
	dKING   [8]int8        = [8]int8{9, 11, -9, -11, 1, 10, -1, -10}
	dDIAGONAL [4]int8        = [4]int8{9, 11, -9, -11}
	dSTRAIGHT    [4]int8        = [4]int8{1, 10, -1, -10}
	rSANRE    *regexp.Regexp = regexp.MustCompile("^(P?|[RNBQK])([a-h]?[1-8]?)?x?([a-h][1-8])(=[PRNBQK])?")
)

func sq2string(s int8) string {
	b := make([]byte, 2)
	b[0] = byte('a' - 1 + s % 10)
	b[1] = byte('1' + ((s / 10) - 2))
	return string(b)
}

func string2sq(sq string) int8 {
	return int8(21 + (sq[1]-'1')*10 + sq[0] - 'a')
}

type color uint8

type piece uint8

// Board represents a chess board and tracks pieces positions.
// It provides methods to move pieces and export board positions
// as FEN and GBR
type Board struct {
	sq              [120]piece
	play            [8][]piece
	epsq            int8
	wksq            int8
	bksq            int8
	activeMove color
	lastSAN string
	MoveWhite bool
	MoveNumber uint8
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

func (c color) String() string {
	if c == cWHITE {
		return "w"
	}
	return "b"
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
func NewBoardFromFen(fen string) (*Board, error) {
	parts := strings.Fields(fen)
	if parts == nil || len(parts) == 0 {
		return nil, fmt.Errorf("fen is wrong")
	}
	b := new(Board)

	b.activeMove = colorOf(parts[1] == "w")
//	if parts[3] != "-" {
//		b.epsq = string2sq(parts[3])
//	}
	// TODO castling
	if n, err := strconv.Atoi(parts[5]); err == nil {
		b.MoveNumber = uint8(n)
	}

	for i, _ := range b.sq {
		b.sq[i] = 0xff
	}
	repr := strings.TrimSpace(parts[0])
	ranks := strings.Split(repr+"/////////", "/")[:8]
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
	return b, nil
}

// NewBoard returns a Board object initialized with the standard starting position
func NewBoard() *Board {
	b, _ := NewBoardFromFen(fINITIAL)
	return b
}

func (b *Board) attackersOf(sq int8, col color) []int8 {
	s := make([]int8, 0)
	for _, d := range dKNIGHT {
		from := sq + d
		piece := b.sq[from]
		if c, t := piece.identify(); c == col && t == pKNIGHT {
			s = append(s, from)
		}
	}
	for _, d := range dKING {
		from := sq + d
		piece := b.sq[from]
		if c, t := piece.identify(); c == col && t == pKING {
			s = append(s, from)
		}
	}
	for _, d := range dDIAGONAL {
		for from := sq + d; b.sq[from] != 0xff; from += d {
			if piece := b.sq[from]; piece != 0 {
				if c, t := piece.identify(); c == col && (t == pQUEEN || t == pBISHOP) {
					s = append(s, from)
				}
				break
			}
		}
	}
	for _, d := range dSTRAIGHT {
		for from := sq + d; b.sq[from] != 0xff; from += d {
			if piece := b.sq[from]; piece != 0 {
				if c, t := piece.identify(); c == col && (t == pQUEEN || t == pROOK) {
					s = append(s, from)
				}
				break
			}
		}
	}
	pawnCaptures := dDIAGONAL[2:]
	if col == cBLACK {
		pawnCaptures = dDIAGONAL[0:2]
	}
	for _, d := range pawnCaptures {
		from := sq + d
		piece := b.sq[from]
		if c, t := piece.identify(); c == col && t == pPAWN {
			s = append(s, from)
		}
	}
	return s
}

func (b *Board) piecesMovableTo(sq int8, col color) []int8 {
	s := b.attackersOf(sq, col)
	direction := int8(-1)
	if col == cBLACK {
		direction = int8(1)
	}
	if from := sq + direction*10; b.sq[from] != 0xff {
		if b.sq[from] == 0 {
			from += direction * 10
			if c, t := b.sq[from].identify(); c == col && t == pPAWN && !b.sq[from].hasMoved() {
				s = append(s, from)
			}
		} else if c, t := b.sq[from].identify(); c == col && t == pPAWN {
			s = append(s, from)
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
	err := b.makeMoveFor(san, b.activeMove)
	if err == nil {
		if b.activeMove == cBLACK {
			b.MoveNumber++
		}
		b.activeMove = b.activeMove.opposite()
		b.MoveWhite = !b.MoveWhite
		b.lastSAN = san
	}
	return err
}

// LastMove returns the last move made on the board.
// In other words the position on the board resulted after this move
func (b *Board) LastMove() (san string, white bool, number uint8) {
	san = b.lastSAN
	white = b.activeMove == cBLACK;
	if white {
		number = b.MoveNumber
	} else {
		number = b.MoveNumber - 1
	}
	return
}

// SetTurn sets who makes the next move
func (b *Board) SetTurn(whiteMove bool) {
	b.activeMove = colorOf(whiteMove)
	b.MoveWhite = whiteMove
}

func (b *Board) makeMoveFor(san string, activeMove color) error {
	if san == "--" {
		return nil
	}
	if strings.HasPrefix(san, "O-O-O") {
		if activeMove == cWHITE {
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
		if activeMove == cWHITE {
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
	tosq := string2sq(dsq)

	candidates := b.piecesMovableTo(tosq, activeMove)
	if candidates == nil {
		return fmt.Errorf("no candidates to move for: SAN %s", san)
	}

	qualified := make([]int8, 0)
	for _, candidate := range candidates {
		if _, typ := b.sq[candidate].identify(); typ == pieceTyp {
			if fromHint == "" || strings.Index(sq2string(candidate), fromHint) >= 0 {
				if b.tryMove(true, activeMove, candidate, tosq, promotes) == nil {
					qualified = append(qualified, candidate)
				}
			}
		}
	}
	if len(qualified) != 1 {
//		fmt.Println("There were ", len(candidates), " ", candidates)
//		for _, sq := range candidates {
//			fmt.Println("\t", sq2string(sq))
//		}
		return fmt.Errorf("there are %d candidate moves for %d %s", len(qualified), b.MoveNumber, san)
	}
	return b.tryMove(false, activeMove, qualified[0], tosq, promotes)
}

func (b *Board) tryMove(try bool, activeMove color, csq, tosq int8, promotes string) error {
	step := int8(-10)
	if activeMove == cWHITE {
		step = int8(10)
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
			cPiece = newPiece(activeMove, uint8(strings.Index("PNBRQK", promotes[1:2])+1), true)
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
	if activeMove == cWHITE {
		ksq = b.wksq
	}
	attackers := b.attackersOf(ksq, activeMove.opposite())
	if len(attackers) != 0 {
		return fmt.Errorf("invalid move. King at %v is attacked by %v", sq2string(ksq), sq2string(attackers[0]))
	}
	return nil
}

// Fen returns the board position as a standard FEN string see http://en.wikipedia.org/wiki/Forsyth%E2%80%93Edwards_Notation
func (b *Board) Fen() string {
	fen := ""

	// piece placement
	for r, rank := range b.play {
		nempty := 0
		for _, piece := range rank {
			if t := piece.String(); t == "_" {
				nempty++
			} else {
				if nempty > 0 {
					fen += strconv.Itoa(nempty)
				}
				nempty = 0
				fen += t
			}
		}
		if nempty > 0 {
			fen += strconv.Itoa(nempty)
		}
		nempty = 0
		if r != 7 {
			fen += "/"
		}
	}

	// active color
	fen += " " + b.activeMove.String()

	// castling availability
	av := " "
	if !b.sq[b.wksq].hasMoved() {
		if !b.sq[21].hasMoved() {
			av += "Q"
		}
		if !b.sq[28].hasMoved() {
			av += "K"
		}
	}
	if !b.sq[b.bksq].hasMoved() {
		if !b.sq[91].hasMoved() {
			av += "q"
		}
		if !b.sq[98].hasMoved() {
			av += "k"
		}
	}
	if av == " " {
		av = " -"
	}
	fen += av
	
	// en passant target
	if b.epsq == 0 {
		fen += " -"
	} else {
		fen += "  " + sq2string(b.epsq)
	}

	// halfmoves TODO
	fen += " 0"

	// full moves
	fen += " " + strconv.Itoa(int(b.MoveNumber))

	return fen
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Gbr returns the GBR code for the position see http://en.wikipedia.org/wiki/GBR_code
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

// String is equivalent to Fen()
func (b *Board) String() string {
	return b.Fen()
}

/*
func (b *Board) Img(mapping map[string]image.Image) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	tmpl := "wbpnbrqk"
	for r, rank := range b.play {
		for f, piece := range rank {
			sqColor := (f + (r % 2)) % 2
			var imgKey string
			if piece == 0 {
				imgKey = tmpl[sqColor: sqColor + 1] 
			} else {
				c, t := piece.identify()
				imgKey = tmpl[c: c + 1] + tmpl[1 + t: 2 + t] + tmpl[sqColor: sqColor + 1]
			}
			p := image.Pt(f * 32, r * 32)
			draw.Draw(img, image.Rect(p.X, p.Y, p.X + 32, p.Y + 32), mapping[imgKey], image.ZP, draw.Src)
		}
	}
	return img
}
*/
