package parser

import (
	"testing"

	"purelang/internal/ast"
)

func parse(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return prog
}

func TestParseImmutableVariable(t *testing.T) {
	prog := parse(t, `name = "Alex"`)
	if len(prog.Stmts) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(prog.Stmts))
	}
	v, ok := prog.Stmts[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("expected VarDecl, got %T", prog.Stmts[0])
	}
	if v.Mutable {
		t.Errorf("expected immutable")
	}
	if v.Name != "name" {
		t.Errorf("got name %q", v.Name)
	}
	s, ok := v.Value.(*ast.StringLiteral)
	if !ok || s.Value != "Alex" {
		t.Errorf("expected string 'Alex' got %v", v.Value)
	}
}

func TestParseMutableVariable(t *testing.T) {
	prog := parse(t, `var count = 0`)
	v, ok := prog.Stmts[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("expected VarDecl, got %T", prog.Stmts[0])
	}
	if !v.Mutable {
		t.Errorf("expected mutable")
	}
}

func TestParseFunctionDecl(t *testing.T) {
	prog := parse(t, `add(a: Int, b: Int) => a + b`)
	fn, ok := prog.Stmts[0].(*ast.FunctionDecl)
	if !ok {
		t.Fatalf("expected FunctionDecl got %T", prog.Stmts[0])
	}
	if fn.Name != "add" {
		t.Errorf("name %q", fn.Name)
	}
	if len(fn.Params) != 2 {
		t.Errorf("params %d", len(fn.Params))
	}
	if fn.ExprBody == nil {
		t.Errorf("expected expr body")
	}
}

func TestParseFunctionBlock(t *testing.T) {
	src := `square(x: Int) {
    x * x
}`
	prog := parse(t, src)
	fn, ok := prog.Stmts[0].(*ast.FunctionDecl)
	if !ok {
		t.Fatalf("expected FunctionDecl got %T", prog.Stmts[0])
	}
	if fn.Body == nil {
		t.Fatalf("expected block body")
	}
	if len(fn.Body.Stmts) != 1 {
		t.Errorf("block stmts %d", len(fn.Body.Stmts))
	}
}

func TestParseDataDecl(t *testing.T) {
	src := `User(name: String, age: Int) {
    adult = age >= 18
    greet => "Hello, $name"
}`
	prog := parse(t, src)
	d, ok := prog.Stmts[0].(*ast.DataDecl)
	if !ok {
		t.Fatalf("expected DataDecl got %T", prog.Stmts[0])
	}
	if d.Name != "User" {
		t.Errorf("name %q", d.Name)
	}
	if len(d.Fields) != 2 {
		t.Errorf("fields %d", len(d.Fields))
	}
	if len(d.ComputedFields) != 2 {
		t.Errorf("computed %d", len(d.ComputedFields))
	}
}

func TestParseMemberAccess(t *testing.T) {
	prog := parse(t, `user.name.first`)
	es, ok := prog.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("expected ExpressionStmt got %T", prog.Stmts[0])
	}
	m, ok := es.Expr.(*ast.MemberExpr)
	if !ok {
		t.Fatalf("expected MemberExpr got %T", es.Expr)
	}
	if m.Property != "first" {
		t.Errorf("got %q", m.Property)
	}
}

func TestParseFunctionCall(t *testing.T) {
	prog := parse(t, `add(1, 2)`)
	es := prog.Stmts[0].(*ast.ExpressionStmt)
	c, ok := es.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr got %T", es.Expr)
	}
	if len(c.Args) != 2 {
		t.Errorf("args %d", len(c.Args))
	}
}

func TestParseIfExpr(t *testing.T) {
	src := `status = if age >= 18 {
    "adult"
} else {
    "minor"
}`
	prog := parse(t, src)
	v := prog.Stmts[0].(*ast.VarDecl)
	if _, ok := v.Value.(*ast.IfExpr); !ok {
		t.Fatalf("expected IfExpr got %T", v.Value)
	}
}

func TestParseListLiteral(t *testing.T) {
	prog := parse(t, `numbers = [1, 2, 3]`)
	v := prog.Stmts[0].(*ast.VarDecl)
	l, ok := v.Value.(*ast.ListLiteral)
	if !ok {
		t.Fatalf("expected ListLiteral got %T", v.Value)
	}
	if len(l.Elements) != 3 {
		t.Errorf("elements %d", len(l.Elements))
	}
}

func TestParseUseDecl(t *testing.T) {
	prog := parse(t, `use std.io`)
	u, ok := prog.Stmts[0].(*ast.UseDecl)
	if !ok {
		t.Fatalf("expected UseDecl got %T", prog.Stmts[0])
	}
	if len(u.Path) != 2 || u.Path[0] != "std" || u.Path[1] != "io" {
		t.Errorf("path %v", u.Path)
	}
}

func TestParseCommandStyleCall(t *testing.T) {
	prog := parse(t, `print "Hello"`)
	es := prog.Stmts[0].(*ast.ExpressionStmt)
	c, ok := es.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr got %T", es.Expr)
	}
	id, ok := c.Callee.(*ast.Identifier)
	if !ok || id.Name != "print" {
		t.Errorf("callee %v", c.Callee)
	}
	if len(c.Args) != 1 {
		t.Fatalf("args %d", len(c.Args))
	}
	if s, ok := c.Args[0].(*ast.StringLiteral); !ok || s.Value != "Hello" {
		t.Errorf("arg %v", c.Args[0])
	}
}
