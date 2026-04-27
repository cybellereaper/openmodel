package checker

import (
	"fmt"

	"purelang/internal/ast"
	"purelang/internal/types"
)

type scope struct {
	parent  *scope
	vars    map[string]*varInfo
	funcs   map[string]*funcInfo
	datas   map[string]*dataInfo
}

type varInfo struct {
	typ     *types.Type
	mutable bool
}

type funcInfo struct {
	params  []ast.Param
	hasBody bool
}

type dataInfo struct {
	fields    []ast.DataField
	computed  []*ast.ComputedField
}

type Checker struct {
	root   *scope
	errors []string
}

func New() *Checker {
	return &Checker{root: newScope(nil)}
}

func newScope(parent *scope) *scope {
	return &scope{
		parent: parent,
		vars:   map[string]*varInfo{},
		funcs:  map[string]*funcInfo{},
		datas:  map[string]*dataInfo{},
	}
}

func (s *scope) lookupVar(name string) *varInfo {
	for cur := s; cur != nil; cur = cur.parent {
		if v, ok := cur.vars[name]; ok {
			return v
		}
	}
	return nil
}

func (s *scope) lookupFunc(name string) *funcInfo {
	for cur := s; cur != nil; cur = cur.parent {
		if v, ok := cur.funcs[name]; ok {
			return v
		}
	}
	return nil
}

func (s *scope) lookupData(name string) *dataInfo {
	for cur := s; cur != nil; cur = cur.parent {
		if v, ok := cur.datas[name]; ok {
			return v
		}
	}
	return nil
}

func builtinFuncs() map[string]*funcInfo {
	return map[string]*funcInfo{
		"print":   {params: []ast.Param{{Name: "v"}}, hasBody: true},
		"println": {params: []ast.Param{{Name: "v"}}, hasBody: true},
	}
}

func (c *Checker) addBuiltins() {
	for name, info := range builtinFuncs() {
		c.root.funcs[name] = info
	}
}

func (c *Checker) errorf(node ast.Node, format string, args ...interface{}) {
	l, col := node.Pos()
	c.errors = append(c.errors, fmt.Sprintf("[%d:%d] %s", l, col, fmt.Sprintf(format, args...)))
}

// Check runs over multiple programs (for projects).
func (c *Checker) Check(programs ...*ast.Program) []string {
	c.addBuiltins()
	// First pass: collect declarations.
	for _, prog := range programs {
		for _, stmt := range prog.Stmts {
			c.collectDecl(c.root, stmt)
		}
	}
	for _, prog := range programs {
		for _, stmt := range prog.Stmts {
			c.checkStmt(c.root, stmt)
		}
	}
	return c.errors
}

func (c *Checker) collectDecl(s *scope, stmt ast.Stmt) {
	switch n := stmt.(type) {
	case *ast.FunctionDecl:
		s.funcs[n.Name] = &funcInfo{params: n.Params, hasBody: true}
	case *ast.DataDecl:
		s.datas[n.Name] = &dataInfo{fields: n.Fields, computed: n.ComputedFields}
		// register data as callable for construction
		params := make([]ast.Param, len(n.Fields))
		for i, f := range n.Fields {
			params[i] = ast.Param{Name: f.Name, Type: f.Type}
		}
		s.funcs[n.Name] = &funcInfo{params: params, hasBody: true}
	}
}

func (c *Checker) checkStmt(s *scope, stmt ast.Stmt) {
	switch n := stmt.(type) {
	case *ast.UseDecl:
		// noop
	case *ast.VarDecl:
		t := c.checkExpr(s, n.Value)
		if existing, ok := s.vars[n.Name]; ok {
			if !existing.mutable {
				c.errorf(n, "cannot reassign immutable variable %q", n.Name)
				return
			}
			s.vars[n.Name] = &varInfo{typ: t, mutable: existing.mutable}
			return
		}
		s.vars[n.Name] = &varInfo{typ: t, mutable: n.Mutable}
	case *ast.AssignStmt:
		switch tgt := n.Target.(type) {
		case *ast.Identifier:
			info := s.lookupVar(tgt.Name)
			if info == nil {
				c.errorf(n, "unknown variable %q", tgt.Name)
				return
			}
			if !info.mutable {
				c.errorf(n, "cannot reassign immutable variable %q", tgt.Name)
				return
			}
			c.checkExpr(s, n.Value)
		default:
			c.checkExpr(s, n.Value)
		}
	case *ast.FunctionDecl:
		fnScope := newScope(s)
		for _, p := range n.Params {
			fnScope.vars[p.Name] = &varInfo{typ: types.FromName(p.Type), mutable: false}
		}
		if n.ExprBody != nil {
			c.checkExpr(fnScope, n.ExprBody)
		}
		if n.Body != nil {
			for _, st := range n.Body.Stmts {
				c.checkStmt(fnScope, st)
			}
		}
	case *ast.DataDecl:
		// Check computed fields in scope with fields
		dScope := newScope(s)
		for _, f := range n.Fields {
			dScope.vars[f.Name] = &varInfo{typ: types.FromName(f.Type), mutable: false}
		}
		// also let computed fields reference each other
		for _, cf := range n.ComputedFields {
			dScope.vars[cf.Name] = &varInfo{typ: types.Any, mutable: false}
		}
		for _, cf := range n.ComputedFields {
			c.checkExpr(dScope, cf.Body)
		}
	case *ast.ExpressionStmt:
		c.checkExpr(s, n.Expr)
	case *ast.ReturnStmt:
		if n.Value != nil {
			c.checkExpr(s, n.Value)
		}
	case *ast.ForStmt:
		c.checkExpr(s, n.Iterable)
		bodyScope := newScope(s)
		bodyScope.vars[n.Var] = &varInfo{typ: types.Any, mutable: false}
		for _, st := range n.Body.Stmts {
			c.checkStmt(bodyScope, st)
		}
	case *ast.IfExpr:
		t := c.checkExpr(s, n.Cond)
		if t != nil && t.Kind != types.KindBool && t.Kind != types.KindAny {
			c.errorf(n, "if condition must be Bool, got %s", t)
		}
		for _, st := range n.Then.Stmts {
			c.checkStmt(newScope(s), st)
		}
		if n.Else != nil {
			c.checkStmt(s, n.Else)
		}
	case *ast.BlockStmt:
		bs := newScope(s)
		for _, st := range n.Stmts {
			c.checkStmt(bs, st)
		}
	}
}

func (c *Checker) checkExpr(s *scope, expr ast.Expr) *types.Type {
	switch n := expr.(type) {
	case *ast.IntLiteral:
		return types.Int
	case *ast.FloatLiteral:
		return types.Float
	case *ast.StringLiteral:
		return types.String
	case *ast.BoolLiteral:
		return types.Bool
	case *ast.NullLiteral:
		return types.Null
	case *ast.Identifier:
		if v := s.lookupVar(n.Name); v != nil {
			return v.typ
		}
		if f := s.lookupFunc(n.Name); f != nil {
			_ = f
			return types.Function()
		}
		if d := s.lookupData(n.Name); d != nil {
			_ = d
			return types.Data(n.Name)
		}
		c.errorf(n, "unknown identifier %q", n.Name)
		return types.Any
	case *ast.ListLiteral:
		var elem *types.Type = types.Any
		for _, e := range n.Elements {
			t := c.checkExpr(s, e)
			if elem == types.Any {
				elem = t
			}
		}
		return types.ListOf(elem)
	case *ast.UnaryExpr:
		t := c.checkExpr(s, n.Operand)
		return t
	case *ast.BinaryExpr:
		l := c.checkExpr(s, n.Left)
		r := c.checkExpr(s, n.Right)
		switch n.Op {
		case "+", "-", "*", "/", "%":
			if l == nil || r == nil {
				return types.Any
			}
			if n.Op == "+" && (l.Kind == types.KindString || r.Kind == types.KindString) {
				return types.String
			}
			if l.Kind == types.KindAny || r.Kind == types.KindAny {
				return types.Any
			}
			if l.Kind == types.KindFloat || r.Kind == types.KindFloat {
				return types.Float
			}
			if l.Kind == types.KindInt && r.Kind == types.KindInt {
				return types.Int
			}
			c.errorf(n, "operator %q requires numeric operands, got %s and %s", n.Op, l, r)
			return types.Any
		case "==", "!=", "<", "<=", ">", ">=", "&&", "||":
			return types.Bool
		}
		return types.Any
	case *ast.CallExpr:
		// Handle direct function call by name
		if id, ok := n.Callee.(*ast.Identifier); ok {
			if f := s.lookupFunc(id.Name); f != nil {
				if len(f.params) != len(n.Args) {
					c.errorf(n, "function %q expects %d arguments, got %d", id.Name, len(f.params), len(n.Args))
				}
				for _, a := range n.Args {
					c.checkExpr(s, a)
				}
				if d := s.lookupData(id.Name); d != nil {
					_ = d
					return types.Data(id.Name)
				}
				return types.Any
			}
			c.errorf(n, "call to unknown function %q", id.Name)
			for _, a := range n.Args {
				c.checkExpr(s, a)
			}
			return types.Any
		}
		c.checkExpr(s, n.Callee)
		for _, a := range n.Args {
			c.checkExpr(s, a)
		}
		return types.Any
	case *ast.MemberExpr:
		t := c.checkExpr(s, n.Target)
		// If target is a known data type, validate field
		if t != nil && t.Kind == types.KindData {
			if d := s.lookupData(t.Name); d != nil {
				for _, f := range d.fields {
					if f.Name == n.Property {
						return types.FromName(f.Type)
					}
				}
				for _, cf := range d.computed {
					if cf.Name == n.Property {
						return types.Any
					}
				}
				c.errorf(n, "type %s has no field %q", t.Name, n.Property)
				return types.Any
			}
		}
		return types.Any
	case *ast.IndexExpr:
		c.checkExpr(s, n.Target)
		c.checkExpr(s, n.Index)
		return types.Any
	case *ast.IfExpr:
		t := c.checkExpr(s, n.Cond)
		if t != nil && t.Kind != types.KindBool && t.Kind != types.KindAny {
			c.errorf(n, "if condition must be Bool, got %s", t)
		}
		for _, st := range n.Then.Stmts {
			c.checkStmt(newScope(s), st)
		}
		if n.Else != nil {
			c.checkStmt(s, n.Else)
		}
		return types.Any
	case *ast.BlockStmt:
		bs := newScope(s)
		for _, st := range n.Stmts {
			c.checkStmt(bs, st)
		}
		return types.Any
	}
	return types.Any
}
