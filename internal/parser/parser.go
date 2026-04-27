package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"purelang/internal/ast"
	"purelang/internal/lexer"
	"purelang/internal/token"
)

type Parser struct {
	tokens []token.Token
	pos    int
}

func Parse(source string) (*ast.Program, error) {
	toks, err := lexer.Tokenize(source)
	if err != nil {
		return nil, err
	}
	p := &Parser{tokens: toks}
	return p.parseProgram()
}

func ParseTokens(toks []token.Token) (*ast.Program, error) {
	p := &Parser{tokens: toks}
	return p.parseProgram()
}

// ParseExpression parses a single expression from source.
func ParseExpression(source string) (ast.Expr, error) {
	toks, err := lexer.Tokenize(source)
	if err != nil {
		return nil, err
	}
	p := &Parser{tokens: toks}
	p.skipNewlines()
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if p.peek().Type != token.EOF {
		t := p.peek()
		return nil, fmt.Errorf("[%d:%d] unexpected token %s after expression", t.Line, t.Column, t.Type)
	}
	return expr, nil
}

func (p *Parser) peek() token.Token {
	return p.tokens[p.pos]
}

func (p *Parser) peekN(n int) token.Token {
	if p.pos+n >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos+n]
}

// peekSkippingNewlines returns the n-th non-newline token starting from current pos
func (p *Parser) peekSkippingNewlines(n int) token.Token {
	count := 0
	i := p.pos
	for i < len(p.tokens) {
		if p.tokens[i].Type == token.NEWLINE {
			i++
			continue
		}
		if count == n {
			return p.tokens[i]
		}
		count++
		i++
	}
	return p.tokens[len(p.tokens)-1]
}

func (p *Parser) advance() token.Token {
	t := p.tokens[p.pos]
	if t.Type != token.EOF {
		p.pos++
	}
	return t
}

func (p *Parser) skipNewlines() {
	for p.peek().Type == token.NEWLINE || p.peek().Type == token.SEMI {
		p.advance()
	}
}

func (p *Parser) skipTermsOnce() bool {
	skipped := false
	for p.peek().Type == token.NEWLINE || p.peek().Type == token.SEMI {
		skipped = true
		p.advance()
	}
	return skipped
}

func (p *Parser) expect(tt token.Type) (token.Token, error) {
	if p.peek().Type != tt {
		t := p.peek()
		return t, fmt.Errorf("[%d:%d] expected %s, got %s (%q)", t.Line, t.Column, tt, t.Type, t.Value)
	}
	return p.advance(), nil
}

func (p *Parser) parseProgram() (*ast.Program, error) {
	prog := &ast.Program{}
	if len(p.tokens) > 0 {
		prog.Position = ast.Position{Line: p.tokens[0].Line, Column: p.tokens[0].Column}
	}
	p.skipNewlines()
	for p.peek().Type != token.EOF {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			prog.Stmts = append(prog.Stmts, stmt)
		}
		// Require a newline/semi or EOF or '}' separator
		if p.peek().Type == token.EOF {
			break
		}
		if !p.skipTermsOnce() {
			if p.peek().Type == token.EOF {
				break
			}
			t := p.peek()
			return nil, fmt.Errorf("[%d:%d] expected newline between statements, got %s", t.Line, t.Column, t.Type)
		}
	}
	return prog, nil
}

func (p *Parser) parseStatement() (ast.Stmt, error) {
	t := p.peek()
	switch t.Type {
	case token.USE:
		return p.parseUse()
	case token.VAR:
		return p.parseVarDecl()
	case token.RETURN:
		return p.parseReturn()
	case token.FOR:
		return p.parseFor()
	case token.IF:
		return p.parseIfStmt()
	case token.WHEN:
		expr, err := p.parseWhen()
		if err != nil {
			return nil, err
		}
		return expr.(*ast.WhenExpr), nil
	}

	// Look for function/data decl: IDENT (
	if t.Type == token.IDENT && p.peekN(1).Type == token.LPAREN {
		isDecl, err := p.looksLikeDecl()
		if err != nil {
			return nil, err
		}
		if isDecl {
			return p.parseFunctionOrDataDecl()
		}
	}

	// Look for assignment: IDENT =
	if t.Type == token.IDENT && p.peekN(1).Type == token.ASSIGN {
		return p.parseImmutableDecl()
	}

	return p.parseExpressionStmt()
}

// looksLikeDecl: starting at IDENT (, scan to matching ) and look at what follows.
// If `=>` or `{` immediately, it's a decl.
// If params look typed (`name : Type` syntax), it's also a decl.
func (p *Parser) looksLikeDecl() (bool, error) {
	depth := 0
	i := p.pos + 1 // at LPAREN
	if p.tokens[i].Type != token.LPAREN {
		return false, nil
	}
	depth = 1
	i++
	hasColon := false
	for i < len(p.tokens) && depth > 0 {
		switch p.tokens[i].Type {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			depth--
		case token.COLON:
			if depth == 1 {
				hasColon = true
			}
		case token.EOF:
			return false, fmt.Errorf("[%d:%d] unmatched '('", p.tokens[p.pos+1].Line, p.tokens[p.pos+1].Column)
		}
		i++
	}
	// i points after RPAREN
	// skip newlines? In PureLang, body opening brace must follow on same line.
	// For decl with => or { we require it on the same logical line (no newline between).
	if i < len(p.tokens) {
		next := p.tokens[i]
		if next.Type == token.FATARROW || next.Type == token.LBRACE {
			return true, nil
		}
	}
	if hasColon {
		return true, nil
	}
	return false, nil
}

func (p *Parser) parseUse() (ast.Stmt, error) {
	tok := p.advance() // use
	first, err := p.expect(token.IDENT)
	if err != nil {
		return nil, err
	}
	path := []string{first.Value}
	for p.peek().Type == token.DOT {
		p.advance()
		next, err := p.expect(token.IDENT)
		if err != nil {
			return nil, err
		}
		path = append(path, next.Value)
	}
	return &ast.UseDecl{Position: ast.Position{Line: tok.Line, Column: tok.Column}, Path: path}, nil
}

func (p *Parser) parseVarDecl() (ast.Stmt, error) {
	tok := p.advance() // var
	name, err := p.expect(token.IDENT)
	if err != nil {
		return nil, err
	}
	typ := ""
	if p.peek().Type == token.COLON {
		p.advance()
		t, err := p.parseTypeAnnotation()
		if err != nil {
			return nil, err
		}
		typ = t
	}
	if _, err := p.expect(token.ASSIGN); err != nil {
		return nil, err
	}
	val, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.VarDecl{
		Position: ast.Position{Line: tok.Line, Column: tok.Column},
		Name:     name.Value,
		Mutable:  true,
		Type:     typ,
		Value:    val,
	}, nil
}

func (p *Parser) parseImmutableDecl() (ast.Stmt, error) {
	name := p.advance() // ident
	p.advance()         // =
	val, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	// If identifier exists in scope it's a reassign; we represent both as VarDecl with Mutable=false
	// for declaration. But re-assignments to mutable vars should be AssignStmt.
	// For simplicity, emit AssignStmt for re-assign-style use is checked at runtime via env.
	return &ast.VarDecl{
		Position: ast.Position{Line: name.Line, Column: name.Column},
		Name:     name.Value,
		Mutable:  false,
		Value:    val,
	}, nil
}

func (p *Parser) parseReturn() (ast.Stmt, error) {
	tok := p.advance()
	if p.peek().Type == token.NEWLINE || p.peek().Type == token.SEMI || p.peek().Type == token.RBRACE || p.peek().Type == token.EOF {
		return &ast.ReturnStmt{Position: ast.Position{Line: tok.Line, Column: tok.Column}}, nil
	}
	val, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.ReturnStmt{
		Position: ast.Position{Line: tok.Line, Column: tok.Column},
		Value:    val,
	}, nil
}

func (p *Parser) parseFor() (ast.Stmt, error) {
	tok := p.advance() // for
	name, err := p.expect(token.IDENT)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.IN); err != nil {
		return nil, err
	}
	iter, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.ForStmt{
		Position: ast.Position{Line: tok.Line, Column: tok.Column},
		Var:      name.Value,
		Iterable: iter,
		Body:     body,
	}, nil
}

func (p *Parser) parseIfStmt() (ast.Stmt, error) {
	expr, err := p.parseIf()
	if err != nil {
		return nil, err
	}
	return expr.(*ast.IfExpr), nil
}

func (p *Parser) parseIf() (ast.Expr, error) {
	tok := p.advance() // if
	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	then, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	node := &ast.IfExpr{
		Position: ast.Position{Line: tok.Line, Column: tok.Column},
		Cond:     cond,
		Then:     then,
	}
	// check for else (skipping newlines between } and else)
	save := p.pos
	for p.peek().Type == token.NEWLINE {
		p.advance()
	}
	if p.peek().Type == token.ELSE {
		p.advance()
		if p.peek().Type == token.IF {
			elseExpr, err := p.parseIf()
			if err != nil {
				return nil, err
			}
			node.Else = elseExpr.(*ast.IfExpr)
		} else {
			elseBlock, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			node.Else = elseBlock
		}
	} else {
		p.pos = save
	}
	return node, nil
}

func (p *Parser) parseBlock() (*ast.BlockStmt, error) {
	tok, err := p.expect(token.LBRACE)
	if err != nil {
		return nil, err
	}
	block := &ast.BlockStmt{Position: ast.Position{Line: tok.Line, Column: tok.Column}}
	p.skipNewlines()
	for p.peek().Type != token.RBRACE && p.peek().Type != token.EOF {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
		if p.peek().Type == token.RBRACE {
			break
		}
		if !p.skipTermsOnce() {
			t := p.peek()
			if t.Type == token.RBRACE {
				break
			}
			return nil, fmt.Errorf("[%d:%d] expected newline or '}' in block, got %s", t.Line, t.Column, t.Type)
		}
	}
	if _, err := p.expect(token.RBRACE); err != nil {
		return nil, err
	}
	return block, nil
}

func (p *Parser) parseFunctionOrDataDecl() (ast.Stmt, error) {
	nameTok := p.advance() // ident
	if _, err := p.expect(token.LPAREN); err != nil {
		return nil, err
	}
	params, err := p.parseTypedParams()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RPAREN); err != nil {
		return nil, err
	}

	isData := startsUpper(nameTok.Value)

	pos := ast.Position{Line: nameTok.Line, Column: nameTok.Column}

	if isData {
		fields := make([]ast.DataField, 0, len(params))
		for _, pa := range params {
			fields = append(fields, ast.DataField{Name: pa.Name, Type: pa.Type})
		}
		decl := &ast.DataDecl{Position: pos, Name: nameTok.Value, Fields: fields}
		if p.peek().Type == token.LBRACE {
			p.advance()
			p.skipNewlines()
			for p.peek().Type != token.RBRACE && p.peek().Type != token.EOF {
				cf, err := p.parseComputedField()
				if err != nil {
					return nil, err
				}
				decl.ComputedFields = append(decl.ComputedFields, cf)
				if p.peek().Type == token.RBRACE {
					break
				}
				if !p.skipTermsOnce() {
					break
				}
			}
			if _, err := p.expect(token.RBRACE); err != nil {
				return nil, err
			}
		}
		return decl, nil
	}

	// Function
	fn := &ast.FunctionDecl{Position: pos, Name: nameTok.Value, Params: params}
	if p.peek().Type == token.FATARROW {
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		fn.ExprBody = expr
	} else if p.peek().Type == token.LBRACE {
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		fn.Body = body
	} else {
		t := p.peek()
		return nil, fmt.Errorf("[%d:%d] expected '=>' or '{' in function declaration", t.Line, t.Column)
	}
	return fn, nil
}

// parseTypeAnnotation parses type names, optionally suffixed with `?` for nullable.
// e.g. Int, Int?, List, String?
func (p *Parser) parseTypeAnnotation() (string, error) {
	tt, err := p.expect(token.IDENT)
	if err != nil {
		return "", err
	}
	name := tt.Value
	if p.peek().Type == token.QUESTION {
		p.advance()
		name += "?"
	}
	return name, nil
}

func (p *Parser) parseTypedParams() ([]ast.Param, error) {
	var params []ast.Param
	p.skipNewlines()
	if p.peek().Type == token.RPAREN {
		return params, nil
	}
	for {
		p.skipNewlines()
		nameTok, err := p.expect(token.IDENT)
		if err != nil {
			return nil, err
		}
		typ := ""
		if p.peek().Type == token.COLON {
			p.advance()
			t, err := p.parseTypeAnnotation()
			if err != nil {
				return nil, err
			}
			typ = t
		}
		params = append(params, ast.Param{Name: nameTok.Value, Type: typ})
		p.skipNewlines()
		if p.peek().Type == token.COMMA {
			p.advance()
			p.skipNewlines()
			continue
		}
		break
	}
	return params, nil
}

func (p *Parser) parseComputedField() (*ast.ComputedField, error) {
	nameTok, err := p.expect(token.IDENT)
	if err != nil {
		return nil, err
	}
	if p.peek().Type == token.ASSIGN || p.peek().Type == token.FATARROW {
		p.advance()
	} else {
		t := p.peek()
		return nil, fmt.Errorf("[%d:%d] expected '=' or '=>' in computed field", t.Line, t.Column)
	}
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.ComputedField{
		Position: ast.Position{Line: nameTok.Line, Column: nameTok.Column},
		Name:     nameTok.Value,
		Body:     expr,
	}, nil
}

func (p *Parser) parseExpressionStmt() (ast.Stmt, error) {
	t := p.peek()

	// Check command-style call: IDENT followed by value expression on same line
	// without operator/paren/dot/bracket.
	if t.Type == token.IDENT && p.isCommandStyleCall() {
		ident := p.advance()
		var args []ast.Expr
		for {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().Type == token.COMMA {
				p.advance()
				continue
			}
			break
		}
		call := &ast.CallExpr{
			Position: ast.Position{Line: ident.Line, Column: ident.Column},
			Callee:   &ast.Identifier{Position: ast.Position{Line: ident.Line, Column: ident.Column}, Name: ident.Value},
			Args:     args,
		}
		return &ast.ExpressionStmt{
			Position: ast.Position{Line: ident.Line, Column: ident.Column},
			Expr:     call,
		}, nil
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	// Check for assignment: expr = expr
	if p.peek().Type == token.ASSIGN {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		l, c := expr.Pos()
		return &ast.AssignStmt{
			Position: ast.Position{Line: l, Column: c},
			Target:   expr,
			Value:    val,
		}, nil
	}
	l, c := expr.Pos()
	return &ast.ExpressionStmt{
		Position: ast.Position{Line: l, Column: c},
		Expr:     expr,
	}, nil
}

// isCommandStyleCall checks if current IDENT followed by something starting expression on same line.
// Command style: print "Hello" or print user.name
// Not command style: print(...), print.foo, print + 1, print = 1, print
func (p *Parser) isCommandStyleCall() bool {
	if p.peek().Type != token.IDENT {
		return false
	}
	first := p.peek()
	next := p.peekN(1)
	// Must be on same line
	if next.Line != first.Line {
		return false
	}
	switch next.Type {
	case token.STRING, token.INT, token.FLOAT, token.IDENT,
		token.TRUE, token.FALSE, token.NULL, token.LBRACK,
		token.MINUS, token.BANG:
		// Now ensure it's not followed by operator situation: 
		// `foo bar` -> command. `foo + bar` -> next is PLUS, not in list, returns false. good.
		// `foo - bar` -> MINUS could be subtraction or unary. 
		//   We treat `print -1` as command: print(-1). And `count - 1` as subtraction.
		//   Heuristic: if first IDENT is "print" or "println"... no, we want to be general.
		//   Actually, with unique keywords like `print`, the language convention says it's
		//   command-style. To keep simple: only treat as command style if the IDENT itself
		//   is a known command-style identifier OR if there's clearly no operator chain.
		//   For MVP, accept STRING/IDENT/INT/FLOAT/TRUE/FALSE/NULL/LBRACK as args.
		//   For MINUS: treat as command call to be safe (rare to have `x - 1` as statement,
		//   normally it's `x = x - 1` or expression-stmt where x - 1 returns value).
		//   Actually, statements like `x - 1` are expression statements that compute value
		//   but don't use it. Programs unlikely. Let's go with command style for MINUS too.
		return true
	}
	return false
}

// parseExpression with precedence climbing
func (p *Parser) parseExpression() (ast.Expr, error) {
	return p.parseElvis()
}

func (p *Parser) parseElvis() (ast.Expr, error) {
	left, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == token.ELVIS {
		op := p.advance()
		right, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{
			Position: ast.Position{Line: op.Line, Column: op.Column},
			Op:       "?:", Left: left, Right: right,
		}
	}
	return left, nil
}

func (p *Parser) parseOr() (ast.Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == token.OR {
		op := p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{
			Position: ast.Position{Line: op.Line, Column: op.Column},
			Op:       "||", Left: left, Right: right,
		}
	}
	return left, nil
}

func (p *Parser) parseAnd() (ast.Expr, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == token.AND {
		op := p.advance()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{
			Position: ast.Position{Line: op.Line, Column: op.Column},
			Op:       "&&", Left: left, Right: right,
		}
	}
	return left, nil
}

func (p *Parser) parseEquality() (ast.Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == token.EQ || p.peek().Type == token.NEQ {
		op := p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{
			Position: ast.Position{Line: op.Line, Column: op.Column},
			Op:       op.Value, Left: left, Right: right,
		}
	}
	return left, nil
}

func (p *Parser) parseComparison() (ast.Expr, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	for {
		tt := p.peek().Type
		if tt == token.LT || tt == token.LTE || tt == token.GT || tt == token.GTE {
			op := p.advance()
			right, err := p.parseAdditive()
			if err != nil {
				return nil, err
			}
			left = &ast.BinaryExpr{
				Position: ast.Position{Line: op.Line, Column: op.Column},
				Op:       op.Value, Left: left, Right: right,
			}
			continue
		}
		break
	}
	return left, nil
}

func (p *Parser) parseAdditive() (ast.Expr, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == token.PLUS || p.peek().Type == token.MINUS {
		op := p.advance()
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{
			Position: ast.Position{Line: op.Line, Column: op.Column},
			Op:       op.Value, Left: left, Right: right,
		}
	}
	return left, nil
}

func (p *Parser) parseMultiplicative() (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == token.STAR || p.peek().Type == token.SLASH || p.peek().Type == token.PERCENT {
		op := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{
			Position: ast.Position{Line: op.Line, Column: op.Column},
			Op:       op.Value, Left: left, Right: right,
		}
	}
	return left, nil
}

func (p *Parser) parseUnary() (ast.Expr, error) {
	if p.peek().Type == token.MINUS || p.peek().Type == token.BANG {
		op := p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{
			Position: ast.Position{Line: op.Line, Column: op.Column},
			Op:       op.Value, Operand: operand,
		}, nil
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() (ast.Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.peek().Type {
		case token.LPAREN:
			lp := p.advance()
			var args []ast.Expr
			p.skipNewlines()
			if p.peek().Type != token.RPAREN {
				for {
					p.skipNewlines()
					a, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					p.skipNewlines()
					if p.peek().Type == token.COMMA {
						p.advance()
						continue
					}
					break
				}
			}
			if _, err := p.expect(token.RPAREN); err != nil {
				return nil, err
			}
			expr = &ast.CallExpr{
				Position: ast.Position{Line: lp.Line, Column: lp.Column},
				Callee:   expr,
				Args:     args,
			}
		case token.DOT:
			p.advance()
			id, err := p.expect(token.IDENT)
			if err != nil {
				return nil, err
			}
			expr = &ast.MemberExpr{
				Position: ast.Position{Line: id.Line, Column: id.Column},
				Target:   expr,
				Property: id.Value,
			}
		case token.QDOT:
			p.advance()
			id, err := p.expect(token.IDENT)
			if err != nil {
				return nil, err
			}
			expr = &ast.MemberExpr{
				Position: ast.Position{Line: id.Line, Column: id.Column},
				Target:   expr,
				Property: id.Value,
				Safe:     true,
			}
		case token.LBRACK:
			lb := p.advance()
			idx, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.RBRACK); err != nil {
				return nil, err
			}
			expr = &ast.IndexExpr{
				Position: ast.Position{Line: lb.Line, Column: lb.Column},
				Target:   expr,
				Index:    idx,
			}
		default:
			return expr, nil
		}
	}
}

func (p *Parser) parsePrimary() (ast.Expr, error) {
	t := p.peek()
	switch t.Type {
	case token.INT:
		p.advance()
		v, err := strconv.ParseInt(t.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("[%d:%d] invalid int literal %q", t.Line, t.Column, t.Value)
		}
		return &ast.IntLiteral{Position: ast.Position{Line: t.Line, Column: t.Column}, Value: v}, nil
	case token.FLOAT:
		p.advance()
		v, err := strconv.ParseFloat(t.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("[%d:%d] invalid float literal %q", t.Line, t.Column, t.Value)
		}
		return &ast.FloatLiteral{Position: ast.Position{Line: t.Line, Column: t.Column}, Value: v}, nil
	case token.STRING:
		p.advance()
		return &ast.StringLiteral{Position: ast.Position{Line: t.Line, Column: t.Column}, Value: t.Value}, nil
	case token.TRUE:
		p.advance()
		return &ast.BoolLiteral{Position: ast.Position{Line: t.Line, Column: t.Column}, Value: true}, nil
	case token.FALSE:
		p.advance()
		return &ast.BoolLiteral{Position: ast.Position{Line: t.Line, Column: t.Column}, Value: false}, nil
	case token.NULL:
		p.advance()
		return &ast.NullLiteral{Position: ast.Position{Line: t.Line, Column: t.Column}}, nil
	case token.IDENT:
		p.advance()
		return &ast.Identifier{Position: ast.Position{Line: t.Line, Column: t.Column}, Name: t.Value}, nil
	case token.LPAREN:
		p.advance()
		p.skipNewlines()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.skipNewlines()
		if _, err := p.expect(token.RPAREN); err != nil {
			return nil, err
		}
		return expr, nil
	case token.LBRACK:
		return p.parseListLiteral()
	case token.IF:
		return p.parseIf()
	case token.WHEN:
		return p.parseWhen()
	}
	return nil, fmt.Errorf("[%d:%d] unexpected token %s (%q)", t.Line, t.Column, t.Type, t.Value)
}

func (p *Parser) parseWhen() (ast.Expr, error) {
	tok := p.advance() // when
	pos := ast.Position{Line: tok.Line, Column: tok.Column}
	w := &ast.WhenExpr{Position: pos}
	if p.peek().Type != token.LBRACE {
		subj, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		w.Subject = subj
	}
	if _, err := p.expect(token.LBRACE); err != nil {
		return nil, err
	}
	p.skipNewlines()
	for p.peek().Type != token.RBRACE && p.peek().Type != token.EOF {
		c, err := p.parseWhenCase()
		if err != nil {
			return nil, err
		}
		w.Cases = append(w.Cases, c)
		if p.peek().Type == token.RBRACE {
			break
		}
		if !p.skipTermsOnce() {
			break
		}
	}
	if _, err := p.expect(token.RBRACE); err != nil {
		return nil, err
	}
	return w, nil
}

func (p *Parser) parseWhenCase() (*ast.WhenCase, error) {
	t := p.peek()
	c := &ast.WhenCase{Position: ast.Position{Line: t.Line, Column: t.Column}}
	if p.peek().Type == token.ELSE {
		p.advance()
		c.IsElse = true
	} else {
		for {
			pat, err := p.parsePattern()
			if err != nil {
				return nil, err
			}
			c.Patterns = append(c.Patterns, pat)
			if p.peek().Type == token.COMMA {
				p.advance()
				p.skipNewlines()
				continue
			}
			break
		}
		if p.peek().Type == token.IF {
			p.advance()
			g, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			c.Guard = g
		}
	}
	if _, err := p.expect(token.FATARROW); err != nil {
		return nil, err
	}
	if p.peek().Type == token.LBRACE {
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		c.Body = body
	} else {
		body, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		l, col := body.Pos()
		c.Body = &ast.ExpressionStmt{
			Position: ast.Position{Line: l, Column: col},
			Expr:     body,
		}
	}
	return c, nil
}

// parsePattern is currently a value-equality pattern (any expression).
// `_` identifier is treated as wildcard.
func (p *Parser) parsePattern() (ast.Expr, error) {
	return p.parseExpression()
}

func (p *Parser) parseListLiteral() (ast.Expr, error) {
	lb := p.advance() // [
	var elems []ast.Expr
	p.skipNewlines()
	if p.peek().Type != token.RBRACK {
		for {
			p.skipNewlines()
			e, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			elems = append(elems, e)
			p.skipNewlines()
			if p.peek().Type == token.COMMA {
				p.advance()
				continue
			}
			break
		}
	}
	if _, err := p.expect(token.RBRACK); err != nil {
		return nil, err
	}
	return &ast.ListLiteral{
		Position: ast.Position{Line: lb.Line, Column: lb.Column},
		Elements: elems,
	}, nil
}

func startsUpper(s string) bool {
	if s == "" {
		return false
	}
	r := []rune(s)[0]
	return unicode.IsUpper(r)
}

// PrintExpr returns a debug string for an expr (used in tests).
func PrintExpr(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.IntLiteral:
		return strconv.FormatInt(v.Value, 10)
	case *ast.FloatLiteral:
		return strconv.FormatFloat(v.Value, 'g', -1, 64)
	case *ast.StringLiteral:
		return strconv.Quote(v.Value)
	case *ast.BoolLiteral:
		return strconv.FormatBool(v.Value)
	case *ast.NullLiteral:
		return "null"
	case *ast.Identifier:
		return v.Name
	case *ast.BinaryExpr:
		return "(" + PrintExpr(v.Left) + " " + v.Op + " " + PrintExpr(v.Right) + ")"
	case *ast.UnaryExpr:
		return "(" + v.Op + PrintExpr(v.Operand) + ")"
	case *ast.CallExpr:
		args := make([]string, len(v.Args))
		for i, a := range v.Args {
			args[i] = PrintExpr(a)
		}
		return PrintExpr(v.Callee) + "(" + strings.Join(args, ", ") + ")"
	case *ast.MemberExpr:
		return PrintExpr(v.Target) + "." + v.Property
	case *ast.ListLiteral:
		parts := make([]string, len(v.Elements))
		for i, e := range v.Elements {
			parts[i] = PrintExpr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("<%T>", e)
}
