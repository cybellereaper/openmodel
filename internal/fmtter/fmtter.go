// Package fmtter implements `pr fmt`: a deterministic pretty-printer for
// PureLang source code. It produces canonical formatting from the AST.
package fmtter

import (
	"fmt"
	"strconv"
	"strings"

	"purelang/internal/ast"
	"purelang/internal/parser"
)

// Format takes PureLang source and returns the canonically formatted source.
func Format(source string) (string, error) {
	prog, err := parser.Parse(source)
	if err != nil {
		return "", err
	}
	p := &printer{indent: 0}
	p.formatProgram(prog)
	return p.out.String(), nil
}

type printer struct {
	out    strings.Builder
	indent int
}

const indentStr = "    "

func (p *printer) writeIndent() {
	for i := 0; i < p.indent; i++ {
		p.out.WriteString(indentStr)
	}
}

func (p *printer) formatProgram(prog *ast.Program) {
	for i, s := range prog.Stmts {
		if i > 0 {
			if needBlankBefore(prog.Stmts[i-1], s) {
				p.out.WriteString("\n")
			}
		}
		p.writeIndent()
		p.formatStmt(s)
		p.out.WriteString("\n")
	}
}

func needBlankBefore(prev, cur ast.Stmt) bool {
	// A blank line appears between a `use` group and the rest, or before
	// each function/data declaration.
	_, prevUse := prev.(*ast.UseDecl)
	_, curUse := cur.(*ast.UseDecl)
	if prevUse && !curUse {
		return true
	}
	switch cur.(type) {
	case *ast.FunctionDecl, *ast.DataDecl:
		return true
	}
	return false
}

func (p *printer) formatStmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.UseDecl:
		fmt.Fprintf(&p.out, "use %s", strings.Join(n.Path, "."))
	case *ast.VarDecl:
		if n.Mutable {
			p.out.WriteString("var ")
		}
		p.out.WriteString(n.Name)
		if n.Type != "" {
			fmt.Fprintf(&p.out, ": %s", n.Type)
		}
		p.out.WriteString(" = ")
		p.formatExpr(n.Value)
	case *ast.AssignStmt:
		p.formatExpr(n.Target)
		p.out.WriteString(" = ")
		p.formatExpr(n.Value)
	case *ast.ReturnStmt:
		p.out.WriteString("return")
		if n.Value != nil {
			p.out.WriteString(" ")
			p.formatExpr(n.Value)
		}
	case *ast.FunctionDecl:
		p.formatFunctionDecl(n)
	case *ast.DataDecl:
		p.formatDataDecl(n)
	case *ast.ExpressionStmt:
		p.formatExpr(n.Expr)
	case *ast.IfExpr:
		p.formatIf(n)
	case *ast.WhenExpr:
		p.formatWhen(n)
	case *ast.ForStmt:
		p.formatFor(n)
	case *ast.BlockStmt:
		p.formatBlock(n)
	}
}

func (p *printer) formatFunctionDecl(n *ast.FunctionDecl) {
	fmt.Fprintf(&p.out, "%s(", n.Name)
	for i, par := range n.Params {
		if i > 0 {
			p.out.WriteString(", ")
		}
		p.out.WriteString(par.Name)
		if par.Type != "" {
			fmt.Fprintf(&p.out, ": %s", par.Type)
		}
	}
	p.out.WriteString(")")
	if n.ExprBody != nil {
		p.out.WriteString(" => ")
		p.formatExpr(n.ExprBody)
		return
	}
	if n.Body != nil {
		p.out.WriteString(" ")
		p.formatBlock(n.Body)
	}
}

func (p *printer) formatDataDecl(n *ast.DataDecl) {
	fmt.Fprintf(&p.out, "%s(", n.Name)
	for i, f := range n.Fields {
		if i > 0 {
			p.out.WriteString(", ")
		}
		p.out.WriteString(f.Name)
		if f.Type != "" {
			fmt.Fprintf(&p.out, ": %s", f.Type)
		}
	}
	p.out.WriteString(")")
	if len(n.ComputedFields) > 0 {
		p.out.WriteString(" {\n")
		p.indent++
		for _, cf := range n.ComputedFields {
			p.writeIndent()
			fmt.Fprintf(&p.out, "%s => ", cf.Name)
			p.formatExpr(cf.Body)
			p.out.WriteString("\n")
		}
		p.indent--
		p.writeIndent()
		p.out.WriteString("}")
	}
}

func (p *printer) formatBlock(b *ast.BlockStmt) {
	p.out.WriteString("{\n")
	p.indent++
	for _, s := range b.Stmts {
		p.writeIndent()
		p.formatStmt(s)
		p.out.WriteString("\n")
	}
	p.indent--
	p.writeIndent()
	p.out.WriteString("}")
}

func (p *printer) formatIf(n *ast.IfExpr) {
	p.out.WriteString("if ")
	p.formatExpr(n.Cond)
	p.out.WriteString(" ")
	p.formatBlock(n.Then)
	if n.Else != nil {
		switch e := n.Else.(type) {
		case *ast.IfExpr:
			p.out.WriteString(" else ")
			p.formatIf(e)
		case *ast.BlockStmt:
			p.out.WriteString(" else ")
			p.formatBlock(e)
		}
	}
}

func (p *printer) formatFor(n *ast.ForStmt) {
	fmt.Fprintf(&p.out, "for %s in ", n.Var)
	p.formatExpr(n.Iterable)
	p.out.WriteString(" ")
	p.formatBlock(n.Body)
}

func (p *printer) formatWhen(n *ast.WhenExpr) {
	p.out.WriteString("when")
	if n.Subject != nil {
		p.out.WriteString(" ")
		p.formatExpr(n.Subject)
	}
	p.out.WriteString(" {\n")
	p.indent++
	for _, c := range n.Cases {
		p.writeIndent()
		if c.IsElse {
			p.out.WriteString("else")
		} else {
			for i, pat := range c.Patterns {
				if i > 0 {
					p.out.WriteString(", ")
				}
				p.formatExpr(pat)
			}
			if c.Guard != nil {
				p.out.WriteString(" if ")
				p.formatExpr(c.Guard)
			}
		}
		p.out.WriteString(" => ")
		switch b := c.Body.(type) {
		case *ast.ExpressionStmt:
			p.formatExpr(b.Expr)
		case *ast.BlockStmt:
			p.formatBlock(b)
		default:
			p.formatStmt(c.Body)
		}
		p.out.WriteString("\n")
	}
	p.indent--
	p.writeIndent()
	p.out.WriteString("}")
}

func (p *printer) formatExpr(e ast.Expr) {
	switch n := e.(type) {
	case *ast.IntLiteral:
		p.out.WriteString(strconv.FormatInt(n.Value, 10))
	case *ast.FloatLiteral:
		p.out.WriteString(strconv.FormatFloat(n.Value, 'g', -1, 64))
	case *ast.StringLiteral:
		p.out.WriteString(strconv.Quote(n.Value))
	case *ast.BoolLiteral:
		if n.Value {
			p.out.WriteString("true")
		} else {
			p.out.WriteString("false")
		}
	case *ast.NullLiteral:
		p.out.WriteString("null")
	case *ast.Identifier:
		p.out.WriteString(n.Name)
	case *ast.ListLiteral:
		p.out.WriteString("[")
		for i, el := range n.Elements {
			if i > 0 {
				p.out.WriteString(", ")
			}
			p.formatExpr(el)
		}
		p.out.WriteString("]")
	case *ast.UnaryExpr:
		p.out.WriteString(n.Op)
		p.formatExpr(n.Operand)
	case *ast.BinaryExpr:
		p.formatExpr(n.Left)
		fmt.Fprintf(&p.out, " %s ", n.Op)
		p.formatExpr(n.Right)
	case *ast.CallExpr:
		p.formatExpr(n.Callee)
		p.out.WriteString("(")
		for i, a := range n.Args {
			if i > 0 {
				p.out.WriteString(", ")
			}
			p.formatExpr(a)
		}
		p.out.WriteString(")")
	case *ast.MemberExpr:
		p.formatExpr(n.Target)
		if n.Safe {
			p.out.WriteString("?.")
		} else {
			p.out.WriteString(".")
		}
		p.out.WriteString(n.Property)
	case *ast.IndexExpr:
		p.formatExpr(n.Target)
		p.out.WriteString("[")
		p.formatExpr(n.Index)
		p.out.WriteString("]")
	case *ast.IfExpr:
		p.formatIf(n)
	case *ast.WhenExpr:
		p.formatWhen(n)
	case *ast.BlockStmt:
		p.formatBlock(n)
	}
}
