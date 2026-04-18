package query

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser is a recursive-descent parser for the PathCollapse DSL.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser returns a Parser ready to parse the given token stream.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse parses a single statement and returns its AST node.
func (p *Parser) Parse() (*Statement, error) {
	tok := p.peek()
	switch tok.Type {
	case tokFIND:
		return p.parseFindStatement()
	case tokSHOW:
		return p.parseShowStatement()
	case tokEOF:
		return nil, fmt.Errorf("parser: empty query")
	default:
		return nil, fmt.Errorf("parser: unexpected token %q at pos %d", tok.Literal, tok.Pos)
	}
}

// ParseQuery lexes and parses q in one step.
func ParseQuery(q string) (*Statement, error) {
	l := NewLexer(q)
	tokens, err := l.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("query parse: lex: %w", err)
	}
	p := NewParser(tokens)
	return p.Parse()
}

func (p *Parser) parseFindStatement() (*Statement, error) {
	p.consume() // FIND

	tok := p.peek()
	switch tok.Type {
	case tokPATHS:
		return p.parseFindPaths()
	case tokBREAKPOINTS:
		return p.parseFindBreakpoints()
	case tokIdent:
		// FIND HIGH_RISK_SERVICE_ACCOUNTS or similar
		return p.parseFindHighRisk()
	default:
		return nil, fmt.Errorf("parser: expected PATHS or BREAKPOINTS after FIND, got %q", tok.Literal)
	}
}

func (p *Parser) parseFindPaths() (*Statement, error) {
	p.consume() // PATHS

	stmt := &Statement{Type: StmtFindPaths, Limit: 0}

	if err := p.expect(tokFROM); err != nil {
		return nil, err
	}
	from, err := p.parseRef()
	if err != nil {
		return nil, fmt.Errorf("parser: FROM ref: %w", err)
	}
	stmt.From = from

	if err := p.expect(tokTO); err != nil {
		return nil, err
	}
	to, err := p.parseRef()
	if err != nil {
		return nil, fmt.Errorf("parser: TO ref: %w", err)
	}
	stmt.To = to

	// Optional clauses.
	for !p.isEOF() {
		switch p.peek().Type {
		case tokWHERE:
			p.consume()
			preds, err := p.parsePredicates()
			if err != nil {
				return nil, err
			}
			stmt.Where = preds
		case tokORDER:
			p.consume()
			if err := p.expect(tokBY); err != nil {
				return nil, err
			}
			field, err := p.expectIdent()
			if err != nil {
				return nil, err
			}
			stmt.OrderBy = field
			if p.peek().Type == tokDESC {
				p.consume()
				stmt.OrderDesc = true
			} else if p.peek().Type == tokASC {
				p.consume()
			}
		case tokLIMIT:
			p.consume()
			n, err := p.expectNumber()
			if err != nil {
				return nil, err
			}
			stmt.Limit = n
		default:
			return nil, fmt.Errorf("parser: unexpected token %q", p.peek().Literal)
		}
	}

	return stmt, nil
}

func (p *Parser) parseFindBreakpoints() (*Statement, error) {
	p.consume() // BREAKPOINTS

	stmt := &Statement{Type: StmtFindBreakpoints, Limit: 0}

	if err := p.expect(tokFOR); err != nil {
		return nil, err
	}
	target, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	stmt.BreakpointsFor = target

	if p.peek().Type == tokLIMIT {
		p.consume()
		n, err := p.expectNumber()
		if err != nil {
			return nil, err
		}
		stmt.Limit = n
	}

	return stmt, nil
}

func (p *Parser) parseFindHighRisk() (*Statement, error) {
	ident, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	return &Statement{
		Type:           StmtFindHighRisk,
		HighRiskTarget: strings.ToLower(ident),
	}, nil
}

func (p *Parser) parseShowStatement() (*Statement, error) {
	p.consume() // SHOW

	tok := p.peek()
	if tok.Type != tokDRIFT {
		return nil, fmt.Errorf("parser: expected DRIFT after SHOW, got %q", tok.Literal)
	}
	p.consume() // DRIFT

	stmt := &Statement{Type: StmtShowDrift, DriftSince: "last_snapshot"}

	if p.peek().Type == tokSINCE {
		p.consume()
		since, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		stmt.DriftSince = since
	}

	return stmt, nil
}

// parseRef parses expressions like "user:alice", "privilege:tier0", or bare "top_paths".
func (p *Parser) parseRef() (*Ref, error) {
	kind, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	if p.peek().Type == tokColon {
		p.consume() // :
		value, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		return &Ref{Kind: kind, Value: value}, nil
	}
	return &Ref{Kind: "", Value: kind}, nil
}

// parsePredicates parses one or more WHERE predicates connected by AND.
func (p *Parser) parsePredicates() ([]Predicate, error) {
	var preds []Predicate
	for {
		pred, err := p.parsePredicate()
		if err != nil {
			return nil, err
		}
		preds = append(preds, pred)
		if p.peek().Type == tokAND {
			p.consume()
			continue
		}
		break
	}
	return preds, nil
}

func (p *Parser) parsePredicate() (Predicate, error) {
	field, err := p.expectIdent()
	if err != nil {
		return Predicate{}, err
	}

	op, err := p.parseOperator()
	if err != nil {
		return Predicate{}, err
	}

	val, err := p.parseValue()
	if err != nil {
		return Predicate{}, err
	}

	return Predicate{Field: field, Operator: op, Value: val}, nil
}

func (p *Parser) parseOperator() (string, error) {
	tok := p.consume()
	switch tok.Type {
	case tokGT:
		return ">", nil
	case tokLT:
		return "<", nil
	case tokGTE:
		return ">=", nil
	case tokLTE:
		return "<=", nil
	case tokEQ:
		return "=", nil
	case tokNEQ:
		return "!=", nil
	default:
		return "", fmt.Errorf("parser: expected comparison operator, got %q at pos %d", tok.Literal, tok.Pos)
	}
}

func (p *Parser) parseValue() (string, error) {
	tok := p.consume()
	switch tok.Type {
	case tokNumber, tokString, tokIdent:
		return tok.Literal, nil
	default:
		return "", fmt.Errorf("parser: expected value, got %q", tok.Literal)
	}
}

// helpers

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) consume() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(tt TokenType) error {
	tok := p.consume()
	if tok.Type != tt {
		return fmt.Errorf("parser: expected %s, got %q at pos %d", tt, tok.Literal, tok.Pos)
	}
	return nil
}

func (p *Parser) expectIdent() (string, error) {
	tok := p.consume()
	if tok.Type != tokIdent {
		return "", fmt.Errorf("parser: expected identifier, got %q at pos %d", tok.Literal, tok.Pos)
	}
	return tok.Literal, nil
}

func (p *Parser) expectNumber() (int, error) {
	tok := p.consume()
	if tok.Type != tokNumber {
		return 0, fmt.Errorf("parser: expected number, got %q", tok.Literal)
	}
	n, err := strconv.Atoi(tok.Literal)
	if err != nil {
		return 0, fmt.Errorf("parser: invalid number %q: %w", tok.Literal, err)
	}
	return n, nil
}

func (p *Parser) isEOF() bool {
	return p.peek().Type == tokEOF
}
