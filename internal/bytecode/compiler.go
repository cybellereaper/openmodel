package bytecode

import (
	"fmt"

	"purelang/internal/ast"
)

// Compile turns a Program into a top-level Chunk.
type compiler struct {
	chunk    *Chunk
	scopes   []map[string]int
	localCnt int
	parent   *compiler
}

// CompileProgram compiles a program into a top-level chunk.
func CompileProgram(prog *ast.Program) (*Chunk, error) {
	c := &compiler{chunk: &Chunk{}, scopes: nil}
	for _, s := range prog.Stmts {
		if err := c.compileStmt(s); err != nil {
			return nil, err
		}
	}
	c.emit(OpHalt, 0, 0)
	return c.chunk, nil
}

func (c *compiler) emit(op Op, arg int, line int) int {
	c.chunk.Code = append(c.chunk.Code, Instruction{Op: op, Arg: arg})
	c.chunk.Lines = append(c.chunk.Lines, line)
	return len(c.chunk.Code) - 1
}

func (c *compiler) addConst(k Constant) int {
	c.chunk.Constants = append(c.chunk.Constants, k)
	return len(c.chunk.Constants) - 1
}

func (c *compiler) compileStmt(s ast.Stmt) error {
	switch n := s.(type) {
	case *ast.UseDecl:
		return nil
	case *ast.VarDecl:
		if err := c.compileExpr(n.Value); err != nil {
			return err
		}
		idx := c.addConst(Constant{Kind: ConstString, Str: n.Name})
		l, _ := n.Pos()
		c.emit(OpDefGlobal, idx, l)
		return nil
	case *ast.AssignStmt:
		if id, ok := n.Target.(*ast.Identifier); ok {
			if err := c.compileExpr(n.Value); err != nil {
				return err
			}
			idx := c.addConst(Constant{Kind: ConstString, Str: id.Name})
			l, _ := n.Pos()
			c.emit(OpSetGlobal, idx, l)
			return nil
		}
		return fmt.Errorf("only identifier assignment is supported in vm")
	case *ast.ExpressionStmt:
		if err := c.compileExpr(n.Expr); err != nil {
			return err
		}
		l, _ := n.Pos()
		c.emit(OpPop, 0, l)
		return nil
	case *ast.FunctionDecl:
		return c.compileFunctionDecl(n)
	case *ast.ReturnStmt:
		if n.Value != nil {
			if err := c.compileExpr(n.Value); err != nil {
				return err
			}
		} else {
			l, _ := n.Pos()
			c.emit(OpNull, 0, l)
		}
		l, _ := n.Pos()
		c.emit(OpReturn, 0, l)
		return nil
	case *ast.IfExpr:
		_, err := c.compileIf(n, false)
		return err
	case *ast.BlockStmt:
		for _, st := range n.Stmts {
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("vm: unsupported statement %T", s)
}

func (c *compiler) compileFunctionDecl(n *ast.FunctionDecl) error {
	sub := &compiler{chunk: &Chunk{}, parent: c}
	sub.beginScope()
	for _, p := range n.Params {
		sub.declareLocal(p.Name)
	}
	if n.ExprBody != nil {
		if err := sub.compileExpr(n.ExprBody); err != nil {
			return err
		}
		l, _ := n.Pos()
		sub.emit(OpReturn, 0, l)
	} else if n.Body != nil {
		for _, st := range n.Body.Stmts {
			if err := sub.compileStmt(st); err != nil {
				return err
			}
		}
		l, _ := n.Pos()
		sub.emit(OpNull, 0, l)
		sub.emit(OpReturn, 0, l)
	}
	proto := &FunctionProto{
		Name:       n.Name,
		Arity:      len(n.Params),
		ParamNames: paramNames(n.Params),
		Chunk:      sub.chunk,
	}
	idx := c.addConst(Constant{Kind: ConstFunction, Func: proto})
	l, _ := n.Pos()
	c.emit(OpConst, idx, l)
	nameIdx := c.addConst(Constant{Kind: ConstString, Str: n.Name})
	c.emit(OpDefGlobal, nameIdx, l)
	return nil
}

func paramNames(p []ast.Param) []string {
	out := make([]string, len(p))
	for i, par := range p {
		out[i] = par.Name
	}
	return out
}

func (c *compiler) beginScope() { c.scopes = append(c.scopes, map[string]int{}) }
func (c *compiler) endScope()   { c.scopes = c.scopes[:len(c.scopes)-1] }

func (c *compiler) declareLocal(name string) int {
	idx := c.localCnt
	c.localCnt++
	c.scopes[len(c.scopes)-1][name] = idx
	return idx
}

func (c *compiler) resolveLocal(name string) (int, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if v, ok := c.scopes[i][name]; ok {
			return v, true
		}
	}
	return 0, false
}

func (c *compiler) compileIf(n *ast.IfExpr, asExpr bool) (int, error) {
	if err := c.compileExpr(n.Cond); err != nil {
		return 0, err
	}
	l, _ := n.Pos()
	jmpFalse := c.emit(OpJumpIfFalse, 0, l)
	if err := c.compileBlockExpr(n.Then, asExpr); err != nil {
		return 0, err
	}
	jmpEnd := c.emit(OpJump, 0, l)
	c.chunk.Code[jmpFalse].Arg = len(c.chunk.Code)
	if n.Else != nil {
		switch e := n.Else.(type) {
		case *ast.BlockStmt:
			if err := c.compileBlockExpr(e, asExpr); err != nil {
				return 0, err
			}
		case *ast.IfExpr:
			if _, err := c.compileIf(e, asExpr); err != nil {
				return 0, err
			}
		}
	} else if asExpr {
		c.emit(OpNull, 0, l)
	}
	c.chunk.Code[jmpEnd].Arg = len(c.chunk.Code)
	return 0, nil
}

// compileBlockExpr compiles the block. If asExpr is true, leaves the value of
// the last expression statement on the stack (popping the rest); else pops
// every result.
func (c *compiler) compileBlockExpr(b *ast.BlockStmt, asExpr bool) error {
	if !asExpr {
		for _, st := range b.Stmts {
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		return nil
	}
	for i, st := range b.Stmts {
		isLast := i == len(b.Stmts)-1
		if isLast {
			if es, ok := st.(*ast.ExpressionStmt); ok {
				if err := c.compileExpr(es.Expr); err != nil {
					return err
				}
				return nil
			}
		}
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	if len(b.Stmts) == 0 {
		c.emit(OpNull, 0, 0)
	} else {
		c.emit(OpNull, 0, 0)
	}
	return nil
}

func (c *compiler) compileExpr(e ast.Expr) error {
	switch n := e.(type) {
	case *ast.IntLiteral:
		l, _ := n.Pos()
		c.emit(OpConst, c.addConst(Constant{Kind: ConstInt, Int: n.Value}), l)
	case *ast.FloatLiteral:
		l, _ := n.Pos()
		c.emit(OpConst, c.addConst(Constant{Kind: ConstFloat, Float: n.Value}), l)
	case *ast.StringLiteral:
		l, _ := n.Pos()
		c.emit(OpConst, c.addConst(Constant{Kind: ConstString, Str: n.Value}), l)
	case *ast.BoolLiteral:
		l, _ := n.Pos()
		if n.Value {
			c.emit(OpTrue, 0, l)
		} else {
			c.emit(OpFalse, 0, l)
		}
	case *ast.NullLiteral:
		l, _ := n.Pos()
		c.emit(OpNull, 0, l)
	case *ast.Identifier:
		l, _ := n.Pos()
		if idx, ok := c.resolveLocal(n.Name); ok {
			c.emit(OpGetLocal, idx, l)
			return nil
		}
		c.emit(OpGetGlobal, c.addConst(Constant{Kind: ConstString, Str: n.Name}), l)
	case *ast.ListLiteral:
		for _, el := range n.Elements {
			if err := c.compileExpr(el); err != nil {
				return err
			}
		}
		l, _ := n.Pos()
		c.emit(OpList, len(n.Elements), l)
	case *ast.UnaryExpr:
		if err := c.compileExpr(n.Operand); err != nil {
			return err
		}
		l, _ := n.Pos()
		switch n.Op {
		case "-":
			c.emit(OpNeg, 0, l)
		case "!":
			c.emit(OpNot, 0, l)
		}
	case *ast.BinaryExpr:
		return c.compileBinary(n)
	case *ast.CallExpr:
		if err := c.compileExpr(n.Callee); err != nil {
			return err
		}
		for _, a := range n.Args {
			if err := c.compileExpr(a); err != nil {
				return err
			}
		}
		l, _ := n.Pos()
		c.emit(OpCall, len(n.Args), l)
	case *ast.MemberExpr:
		if err := c.compileExpr(n.Target); err != nil {
			return err
		}
		l, _ := n.Pos()
		idx := c.addConst(Constant{Kind: ConstString, Str: n.Property})
		if n.Safe {
			c.emit(OpSafeMember, idx, l)
		} else {
			c.emit(OpMember, idx, l)
		}
	case *ast.IndexExpr:
		if err := c.compileExpr(n.Target); err != nil {
			return err
		}
		if err := c.compileExpr(n.Index); err != nil {
			return err
		}
		l, _ := n.Pos()
		c.emit(OpIndex, 0, l)
	case *ast.IfExpr:
		_, err := c.compileIf(n, true)
		return err
	default:
		return fmt.Errorf("vm: unsupported expression %T", e)
	}
	return nil
}

func (c *compiler) compileBinary(n *ast.BinaryExpr) error {
	if n.Op == "&&" {
		if err := c.compileExpr(n.Left); err != nil {
			return err
		}
		l, _ := n.Pos()
		jmp := c.emit(OpJumpIfFalseNoPop, 0, l)
		c.emit(OpPop, 0, l)
		if err := c.compileExpr(n.Right); err != nil {
			return err
		}
		c.chunk.Code[jmp].Arg = len(c.chunk.Code)
		return nil
	}
	if n.Op == "||" {
		if err := c.compileExpr(n.Left); err != nil {
			return err
		}
		l, _ := n.Pos()
		jmp := c.emit(OpJumpIfTrueNoPop, 0, l)
		c.emit(OpPop, 0, l)
		if err := c.compileExpr(n.Right); err != nil {
			return err
		}
		c.chunk.Code[jmp].Arg = len(c.chunk.Code)
		return nil
	}
	if n.Op == "?:" {
		if err := c.compileExpr(n.Left); err != nil {
			return err
		}
		if err := c.compileExpr(n.Right); err != nil {
			return err
		}
		l, _ := n.Pos()
		c.emit(OpElvis, 0, l)
		return nil
	}
	if err := c.compileExpr(n.Left); err != nil {
		return err
	}
	if err := c.compileExpr(n.Right); err != nil {
		return err
	}
	l, _ := n.Pos()
	switch n.Op {
	case "+":
		c.emit(OpAdd, 0, l)
	case "-":
		c.emit(OpSub, 0, l)
	case "*":
		c.emit(OpMul, 0, l)
	case "/":
		c.emit(OpDiv, 0, l)
	case "%":
		c.emit(OpMod, 0, l)
	case "==":
		c.emit(OpEq, 0, l)
	case "!=":
		c.emit(OpNeq, 0, l)
	case "<":
		c.emit(OpLt, 0, l)
	case "<=":
		c.emit(OpLte, 0, l)
	case ">":
		c.emit(OpGt, 0, l)
	case ">=":
		c.emit(OpGte, 0, l)
	default:
		return fmt.Errorf("vm: unsupported binary op %q", n.Op)
	}
	return nil
}
