package gochess

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type pgnToken int

const (
	pgnEOF pgnToken = iota
	pgnPERIOD
	pgnASTERISK
	pgnLBRACKET
	pgnRBRACKET
	pgnLPAREN
	pgnRPAREN
	pgnLANGLE
	pgnRANGLE
	pgnNAG
	pgnRESULT
	pgnSTRING
	pgnSYMBOL
	pgnINTEGER
	pgnIDENTIFIER
	pgnCOMMENT
	pgnTOKEN
)

const (
	pgnSAN_REGEXP = "^(((O-O|O-O-O)|((P?|[RNBQK])[a-h]?[1-8]?x?[a-h][1-8](=[PRNBQK])?))(\\+|#)?(!|\\?|!!|\\?\\?|\\?!|!\\?)?)$"
	pgnTAG_REGEXP = `^\[?\s*([A-Za-z0-9_]+)\s+"(.*)"\s*\]`
)

var (
	san_re    *regexp.Regexp = regexp.MustCompile(pgnSAN_REGEXP)
	tag_re    *regexp.Regexp = regexp.MustCompile(pgnTAG_REGEXP)
	whiteWins []byte         = []byte("1-0")
	blackWins []byte         = []byte("0-1")
	drawRes   []byte         = []byte("1/2-1/2")
)

// Parser is a parser for the PGN chess games notation
type Parser struct {
	input *bufio.Reader
	line  []byte
}

// Game represents a single parsed PGN game
type Game struct {
	// Tags is a map with the PGN tags. The keys are the tag keys
	Tags map[string]string
	// Moves is initialized after calling ParseMovesText. It contains
	// all the variations of the game as a tree of plies
	Moves Variation
	// PGNText is a verbatim copy of the game PGN
	PGNText []byte
	// MovesText contains just the game moves section from the PGN
	MovesText []byte
}

// Variation represents a singles variation i.e a move sequence in the game
type Variation struct {
	// MoveNumber the move number at which the variation applies
	MoveNumber uint8
	// WhiteMove if true the variation starts with a ply by white, else by black
	WhiteMove bool
	// Plies is a slice with the plies of the variation
	Plies []*Ply
	// Result is the result of the variation. It is one of 1-0, 0-1, 1/2-1/2, *
	Result string
	// Comment has the comments from the PGN that apply to the variation as
	// a whole and do not apply to specific plies
	Comment string
}

// Ply is a single move by white or black
type Ply struct {
	// SAN is the text of the move like e4 or Nf3
	SAN string
	// Nags are pgn annotation for the move
	Nags []uint8
	// Comment is the comment for the move
	Comment string
	// Variations is a slice of alternative moves at this point.
	// In PGN they are represented as RAVs parenthesized variations
	Variations []Variation
}

type token struct {
	typ pgnToken
	val string
}

type tokenizer struct {
	text []byte
}

func (p *Parser) readline() ([]byte, error) {
	if p.line != nil {
		s := p.line
		p.line = nil
		return s, nil
	}
	return p.input.ReadSlice('\n')
}

func (p *Parser) unreadline(line []byte) {
	p.line = line
}

// NewParser returns a new Parser for the input ReadCloser
// The input stream must contain PGN data.
// It is not safe(yet) for concurrent access by multiple goroutines
func NewParser(input io.Reader) *Parser {
	p := &Parser{
		input: bufio.NewReader(input),
	}
	var line []byte
	var err error
	for {
		if line, err = p.readline(); err != nil {
			break
		}
		if matches := matchTagLine(line); matches != nil {
			p.unreadline(line)
			break
		}
	}
	return p
}

func matchTagLine(line []byte) [][]byte {
	if len(line) > 0 && line[0] == '[' {
		return tag_re.FindSubmatch(line)
	}
	return nil
}

// NextGame is an iterator for games.
// It returns a non-nil error on IO failure
// otherwise it returns a parsed game.
// On EOF it returns nil, nil
func (p *Parser) NextGame() (*Game, error) {
	var line []byte
	var err error
	var pgnText []byte

	tags := make(map[string]string)
	for {
		if line, err = p.readline(); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if matches := matchTagLine(line); matches != nil {
			tags[string(matches[1])] = string(matches[2])
			pgnText = append(pgnText, line...)
		} else {
			break
		}
	}
	pgnTagsEndAt := len(pgnText)

	if tline := bytes.TrimSpace(line); len(tline) > 0 {
		pgnText = append(pgnText, line...)
//		fmt.Fprintf(os.Stderr, "Expected an empty line between tags and moves but got '%q'\n", line)
	} else {
		pgnText = append(pgnText, []byte("\n")...)
	}

	for {
		if line, err = p.readline(); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if matches := matchTagLine(line); matches == nil {
			pgnText = append(pgnText, line...)
		} else {
			p.unreadline(line)
			break
		}
	}

	var game *Game
	if len(tags) > 0 {
		game = &Game{
			Tags:      tags,
			PGNText: pgnText,
			MovesText: pgnText[pgnTagsEndAt:],
		}
	}
	return game, nil
}

func (t *tokenizer) next() token {
	var k int
	for k = 0; k < len(t.text); k++ {
		c := t.text[k]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '\v' {
			break
		}
	}
	if k >= len(t.text) {
		t.text = t.text[len(t.text):]
		return token{pgnEOF, ""}
	}
	t.text = t.text[k:]

	// comment
	if t.text[0] == ';' || t.text[0] == '{' {
		delim := byte('\n')
		if t.text[0] == '{' {
			delim = byte('}')
		}
		i := bytes.IndexByte(t.text, delim)
		var tok string
		if i >= 0 {
			tok = string(t.text[1:i])
			t.text = t.text[i+1:]
		} else {
			tok = string(t.text[1:])
			t.text = t.text[len(t.text):]
		}
		return token{pgnCOMMENT, tok}
	}

	// string
	if t.text[0] == '"' {
		var pos int
		for pos = 1; pos < len(t.text); pos++ {
			c := t.text[pos]
			if c == '\\' {
				pos++
				continue
			}
			if c == '"' {
				break
			}
		}
		tok := string(t.text[1:pos])
		t.text = t.text[pos+1:]
		return token{pgnSTRING, tok}
	}

	if bytes.HasPrefix(t.text, whiteWins) {
		t.text = t.text[len(whiteWins):]
		return token{pgnRESULT, "1-0"}
	}
	if bytes.HasPrefix(t.text, blackWins) {
		t.text = t.text[len(blackWins):]
		return token{pgnRESULT, "0-1"}
	}
	if bytes.HasPrefix(t.text, drawRes) {
		t.text = t.text[len(drawRes):]
		return token{pgnRESULT, "1/2-1/2"}
	}

	if t.text[0] == '.' {
		t.text = t.text[1:]
		return token{pgnPERIOD, "."}
	}
	if t.text[0] == '*' {
		t.text = t.text[1:]
		return token{pgnASTERISK, "*"}
	}
	if t.text[0] == '[' {
		t.text = t.text[1:]
		return token{pgnLBRACKET, "["}
	}
	if t.text[0] == ']' {
		t.text = t.text[1:]
		return token{pgnRBRACKET, "]"}
	}
	if t.text[0] == '(' {
		t.text = t.text[1:]
		return token{pgnLPAREN, "("}
	}
	if t.text[0] == ')' {
		t.text = t.text[1:]
		return token{pgnRPAREN, ")"}
	}
	if t.text[0] == '<' {
		t.text = t.text[1:]
		return token{pgnLANGLE, "<"}
	}
	if t.text[0] == '>' {
		t.text = t.text[1:]
		return token{pgnRANGLE, ">"}
	}

	// nag
	if t.text[0] == '$' {
		var pos int
		for pos = 1; pos < len(t.text); pos++ {
			c := t.text[pos]
			if !('0' <= c && c <= '9') {
				break
			}
		}
		tok := string(t.text[1:pos])
		if tok == "" {
			tok = "0"
		}
		t.text = t.text[pos:]
		return token{pgnNAG, tok}
	}

	// symbol
	isSymbol, isInteger, isIdentifier := false, false, false
	var pos int
	for pos = 0; pos < len(t.text); pos++ {
		c := t.text[pos]
		if '0' <= c && c <= '9' {
			isInteger = true
		} else if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '_' {
			isIdentifier = true
		} else if c == '+' || c == '#' || c == '=' || c == ':' || c == '-' {
			isSymbol = true
		} else {
			break
		}
	}
	tok := string(t.text[0:pos])
	t.text = t.text[pos:]
	if isSymbol {
		return token{pgnSYMBOL, tok}
	} else if isIdentifier {
		return token{pgnIDENTIFIER, tok}
	} else if isInteger {
		return token{pgnINTEGER, tok}
	}
	return token{pgnTOKEN, tok}
}

// ParseMovesText parses the moves text of a game
// and converts it to a tree of plies. It must be called
// explicitly for each game
func (game *Game) ParseMovesText() error {
	t := &tokenizer{
		text: game.MovesText,
	}
	return t.generatePlies(&game.Moves, false, 1, true)
}

func boolAsColor(b bool) string {
	if b {
		return "white"
	}
	return "black"
}

func (t *tokenizer) generatePlies(variation *Variation, inRav bool, thisMoveNumber uint8, thisPlyWhite bool) error {
	var ply *Ply

	for token := t.next(); ; token = t.next() {
	loop:
		switch token.typ {
		case pgnRPAREN:
			if inRav {
				variation.Result = "*"
				return nil
			} else {
				return fmt.Errorf("non matched ')'")
			}

		case pgnRESULT, pgnASTERISK:
			variation.Result = token.val
			if !inRav {
				return nil
			}

		case pgnEOF:
			if inRav {
				return fmt.Errorf("non closing RAV. unexpected EOF")
			} else {
				variation.Result = "*"
				return nil
			}

		case pgnINTEGER:
			n, _ := strconv.Atoi(token.val)
			i := 0
			for token = t.next(); token.typ == pgnPERIOD; token = t.next() {
				i++
			}
			m, p := uint8(n), i == 1
			if variation.MoveNumber != 0 {
				if m != thisMoveNumber {
					return fmt.Errorf("move number mismatch. Expected %d got %d", thisMoveNumber, m)
				}
				if p != thisPlyWhite {
					return fmt.Errorf("move order mismatch. Expected %d got %d", boolAsColor(thisPlyWhite), boolAsColor(p))
				}
			} else {
				variation.MoveNumber = m
				variation.WhiteMove = p
			}
			thisMoveNumber = m
			thisPlyWhite = i == 1
			goto loop

		case pgnSYMBOL, pgnIDENTIFIER:
			var SAN string
			if token.val == "--" {
				SAN = "--"
			} else {
				if !san_re.MatchString(token.val) {
					return fmt.Errorf("mismatched SAN '%s'", token.val)
				}
				SAN = token.val
				if strings.HasSuffix(SAN, "!!") {
					SAN = SAN[:len(SAN)-2]
					ply.Nags = append(ply.Nags, 3)
				} else if strings.HasSuffix(SAN, "??") {
					SAN = SAN[:len(SAN)-2]
					ply.Nags = append(ply.Nags, 4)
				} else if strings.HasSuffix(SAN, "!?") {
					SAN = SAN[:len(SAN)-2]
					ply.Nags = append(ply.Nags, 5)
				} else if strings.HasSuffix(SAN, "?!") {
					SAN = SAN[:len(SAN)-2]
					ply.Nags = append(ply.Nags, 6)
				} else if strings.HasSuffix(SAN, "!") {
					SAN = SAN[:len(SAN)-1]
					ply.Nags = append(ply.Nags, 1)
				} else if strings.HasSuffix(SAN, "?") {
					SAN = SAN[:len(SAN)-1]
					ply.Nags = append(ply.Nags, 2)
				}
			}
			ply = &Ply{SAN: SAN}
			variation.Plies = append(variation.Plies, ply)
			if variation.MoveNumber == 0 {
				variation.MoveNumber = thisMoveNumber
				variation.WhiteMove = thisPlyWhite
			}
			if thisPlyWhite = !thisPlyWhite; thisPlyWhite {
				thisMoveNumber++
			}

		case pgnNAG:
			nag, _ := strconv.Atoi(token.val)
			ply.Nags = append(ply.Nags, uint8(nag))

		case pgnCOMMENT:
			if ply != nil {
				ply.Comment += token.val
			} else {
				variation.Comment += token.val
			}

		case pgnLPAREN:
			var v Variation
			if err := t.generatePlies(&v, true, thisMoveNumber, thisPlyWhite); err == nil {
				ply.Variations = append(ply.Variations, v)
			} else {
				return fmt.Errorf("cannot parse RAV section: %s", err)
			}

		default:
			return fmt.Errorf("unexpected token '%d:%s'", token.typ, token.val)
		}
	}
	return nil
}
