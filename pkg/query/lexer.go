// Package query provides a lexer and recursive-descent parser for the
// PathCollapse analyst DSL, plus an executor that runs queries against a graph.
package query

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType identifies a lexical token.
type TokenType int

const (
	tokEOF TokenType = iota
	tokIdent
	tokNumber
	tokString
	tokColon
	tokDot
	tokGT
	tokLT
	tokGTE
	tokLTE
	tokEQ
	tokNEQ

	// Keywords
	tokFIND
	tokPATHS
	tokFROM
	tokTO
	tokWHERE
	tokORDER
	tokBY
	tokDESC
	tokASC
	tokLIMIT
	tokBREAKPOINTS
	tokFOR
	tokSHOW
	tokDRIFT
	tokSINCE
	tokAND
	tokOR
)

var keywords = map[string]TokenType{
	"FIND":        tokFIND,
	"PATHS":       tokPATHS,
	"FROM":        tokFROM,
	"TO":          tokTO,
	"WHERE":       tokWHERE,
	"ORDER":       tokORDER,
	"BY":          tokBY,
	"DESC":        tokDESC,
	"ASC":         tokASC,
	"LIMIT":       tokLIMIT,
	"BREAKPOINTS": tokBREAKPOINTS,
	"FOR":         tokFOR,
	"SHOW":        tokSHOW,
	"DRIFT":       tokDRIFT,
	"SINCE":       tokSINCE,
	"AND":         tokAND,
	"OR":          tokOR,
}

// tokenTypeNames maps each TokenType constant to a human-readable label used in
// error messages. Entries only exist for tokens that appear in expect() calls.
var tokenTypeNames = map[TokenType]string{
	tokEOF:         "EOF",
	tokIdent:       "identifier",
	tokNumber:      "number",
	tokString:      "string",
	tokColon:       "':'",
	tokDot:         "'.'",
	tokGT:          "'>'",
	tokLT:          "'<'",
	tokGTE:         "'>='",
	tokLTE:         "'<='",
	tokEQ:          "'='",
	tokNEQ:         "'!='",
	tokFIND:        "FIND",
	tokPATHS:       "PATHS",
	tokFROM:        "FROM",
	tokTO:          "TO",
	tokWHERE:       "WHERE",
	tokORDER:       "ORDER",
	tokBY:          "BY",
	tokDESC:        "DESC",
	tokASC:         "ASC",
	tokLIMIT:       "LIMIT",
	tokBREAKPOINTS: "BREAKPOINTS",
	tokFOR:         "FOR",
	tokSHOW:        "SHOW",
	tokDRIFT:       "DRIFT",
	tokSINCE:       "SINCE",
	tokAND:         "AND",
	tokOR:          "OR",
}

// String returns a human-readable name for the token type.
func (tt TokenType) String() string {
	if name, ok := tokenTypeNames[tt]; ok {
		return name
	}
	return fmt.Sprintf("TokenType(%d)", int(tt))
}

// Token is a single unit produced by the lexer.
type Token struct {
	Type    TokenType
	Literal string
	Pos     int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%s, %q, pos=%d)", t.Type, t.Literal, t.Pos)
}

// Lexer tokenises a PathCollapse DSL query string.
type Lexer struct {
	input []rune
	pos   int
}

// NewLexer returns a Lexer for the given query string.
func NewLexer(query string) *Lexer {
	return &Lexer{input: []rune(query)}
}

// Tokenize returns all tokens from the input, ending with tokEOF.
func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.Type == tokEOF {
			break
		}
	}
	return tokens, nil
}

func (l *Lexer) next() (Token, error) {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return Token{Type: tokEOF, Pos: l.pos}, nil
	}

	start := l.pos
	ch := l.input[l.pos]

	switch {
	case ch == ':':
		l.pos++
		return Token{Type: tokColon, Literal: ":", Pos: start}, nil
	case ch == '.':
		l.pos++
		return Token{Type: tokDot, Literal: ".", Pos: start}, nil
	case ch == '>' && l.peek() == '=':
		l.pos += 2
		return Token{Type: tokGTE, Literal: ">=", Pos: start}, nil
	case ch == '<' && l.peek() == '=':
		l.pos += 2
		return Token{Type: tokLTE, Literal: "<=", Pos: start}, nil
	case ch == '!' && l.peek() == '=':
		l.pos += 2
		return Token{Type: tokNEQ, Literal: "!=", Pos: start}, nil
	case ch == '>':
		l.pos++
		return Token{Type: tokGT, Literal: ">", Pos: start}, nil
	case ch == '<':
		l.pos++
		return Token{Type: tokLT, Literal: "<", Pos: start}, nil
	case ch == '=':
		l.pos++
		return Token{Type: tokEQ, Literal: "=", Pos: start}, nil
	case ch == '"' || ch == '\'':
		return l.readString(ch)
	case unicode.IsDigit(ch) || (ch == '-' && l.pos+1 < len(l.input) && unicode.IsDigit(l.input[l.pos+1])):
		return l.readNumber(start)
	case unicode.IsLetter(ch) || ch == '_':
		return l.readIdentOrKeyword(start)
	default:
		return Token{}, fmt.Errorf("lexer: unexpected character %q at pos %d", ch, l.pos)
	}
}

func (l *Lexer) readIdentOrKeyword(start int) (Token, error) {
	for l.pos < len(l.input) && (unicode.IsLetter(l.input[l.pos]) || l.input[l.pos] == '_' || unicode.IsDigit(l.input[l.pos])) {
		l.pos++
	}
	lit := string(l.input[start:l.pos])
	upper := strings.ToUpper(lit)
	if tt, ok := keywords[upper]; ok {
		return Token{Type: tt, Literal: upper, Pos: start}, nil
	}
	return Token{Type: tokIdent, Literal: lit, Pos: start}, nil
}

func (l *Lexer) readNumber(start int) (Token, error) {
	if l.input[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.input) && (unicode.IsDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		l.pos++
	}
	return Token{Type: tokNumber, Literal: string(l.input[start:l.pos]), Pos: start}, nil
}

func (l *Lexer) readString(quote rune) (Token, error) {
	l.pos++ // skip opening quote
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != quote {
		l.pos++
	}
	if l.pos >= len(l.input) {
		return Token{}, fmt.Errorf("lexer: unterminated string at pos %d", start)
	}
	lit := string(l.input[start:l.pos])
	l.pos++ // skip closing quote
	return Token{Type: tokString, Literal: lit, Pos: start}, nil
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
}

func (l *Lexer) peek() rune {
	if l.pos+1 < len(l.input) {
		return l.input[l.pos+1]
	}
	return 0
}
