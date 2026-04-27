package runtime

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"purelang/internal/ast"
	"purelang/internal/parser"
)

type Interpreter struct {
	globals *Environment
	out     io.Writer
}

func New() *Interpreter {
	return NewWithWriter(os.Stdout)
}

func NewWithWriter(w io.Writer) *Interpreter {
	i := &Interpreter{
		globals: NewEnvironment(nil),
		out:     w,
	}
	i.installBuiltins()
	return i
}

func (i *Interpreter) Globals() *Environment { return i.globals }

func (i *Interpreter) installBuiltins() {
	i.globals.Define("print", Value{
		Kind: VBuiltin,
		Builtin: &BuiltinFunction{
			Name: "print",
			Fn: func(args []Value) (Value, error) {
				parts := make([]string, len(args))
				for j, a := range args {
					parts[j] = a.String()
				}
				fmt.Fprintln(i.out, strings.Join(parts, " "))
				return Null(), nil
			},
		},
	}, false)
	i.globals.Define("println", Value{
		Kind: VBuiltin,
		Builtin: &BuiltinFunction{
			Name: "println",
			Fn: func(args []Value) (Value, error) {
				parts := make([]string, len(args))
				for j, a := range args {
					parts[j] = a.String()
				}
				fmt.Fprintln(i.out, strings.Join(parts, " "))
				return Null(), nil
			},
		},
	}, false)
}

// returnSignal used to unwind from return statements.
type returnSignal struct {
	value Value
}

func (returnSignal) Error() string { return "return" }

// RegisterDeclarations pre-registers top-level declarations from a program (no execution).
func (i *Interpreter) RegisterDeclarations(prog *ast.Program) error {
	for _, stmt := range prog.Stmts {
		switch n := stmt.(type) {
		case *ast.FunctionDecl:
			fn := &FunctionValue{
				Name:     n.Name,
				Params:   n.Params,
				Body:     n.Body,
				ExprBody: n.ExprBody,
				Env:      i.globals,
			}
			i.globals.Define(n.Name, Value{Kind: VFunction, Func: fn}, false)
		case *ast.DataDecl:
			dt := &DataTypeValue{
				Name:           n.Name,
				Fields:         n.Fields,
				ComputedFields: n.ComputedFields,
			}
			i.globals.Define(n.Name, Value{Kind: VDataType, DataType: dt}, false)
		}
	}
	return nil
}

// Run executes a program in the global environment.
func (i *Interpreter) Run(prog *ast.Program) error {
	for _, stmt := range prog.Stmts {
		if _, err := i.execStmt(i.globals, stmt); err != nil {
			if _, ok := err.(returnSignal); ok {
				return nil
			}
			return err
		}
	}
	return nil
}

// RunSkippingDecls is similar to Run but skips re-registering declarations
// when they were already registered by RegisterDeclarations.
func (i *Interpreter) RunSkippingDecls(prog *ast.Program) error {
	for _, stmt := range prog.Stmts {
		switch stmt.(type) {
		case *ast.FunctionDecl, *ast.DataDecl:
			continue
		}
		if _, err := i.execStmt(i.globals, stmt); err != nil {
			if _, ok := err.(returnSignal); ok {
				return nil
			}
			return err
		}
	}
	return nil
}

func (i *Interpreter) execStmt(env *Environment, stmt ast.Stmt) (Value, error) {
	switch n := stmt.(type) {
	case *ast.UseDecl:
		return Null(), nil
	case *ast.VarDecl:
		val, err := i.evalExpr(env, n.Value)
		if err != nil {
			return Null(), err
		}
		if env.IsDefinedHere(n.Name) {
			if !env.IsMutable(n.Name) {
				return Null(), i.errorf(n, "cannot reassign immutable variable %q", n.Name)
			}
			if err := env.Assign(n.Name, val); err != nil {
				return Null(), i.errorf(n, "%v", err)
			}
			return Null(), nil
		}
		env.Define(n.Name, val, n.Mutable)
		return Null(), nil
	case *ast.AssignStmt:
		val, err := i.evalExpr(env, n.Value)
		if err != nil {
			return Null(), err
		}
		switch tgt := n.Target.(type) {
		case *ast.Identifier:
			if err := env.Assign(tgt.Name, val); err != nil {
				return Null(), i.errorf(n, "%v", err)
			}
		default:
			return Null(), i.errorf(n, "invalid assignment target")
		}
		return Null(), nil
	case *ast.FunctionDecl:
		fn := &FunctionValue{
			Name:     n.Name,
			Params:   n.Params,
			Body:     n.Body,
			ExprBody: n.ExprBody,
			Env:      env,
		}
		env.Define(n.Name, Value{Kind: VFunction, Func: fn}, false)
		return Null(), nil
	case *ast.DataDecl:
		dt := &DataTypeValue{
			Name:           n.Name,
			Fields:         n.Fields,
			ComputedFields: n.ComputedFields,
		}
		env.Define(n.Name, Value{Kind: VDataType, DataType: dt}, false)
		return Null(), nil
	case *ast.ExpressionStmt:
		return i.evalExpr(env, n.Expr)
	case *ast.ReturnStmt:
		var val Value = Null()
		if n.Value != nil {
			v, err := i.evalExpr(env, n.Value)
			if err != nil {
				return Null(), err
			}
			val = v
		}
		return val, returnSignal{value: val}
	case *ast.ForStmt:
		iter, err := i.evalExpr(env, n.Iterable)
		if err != nil {
			return Null(), err
		}
		switch iter.Kind {
		case VList:
			for _, item := range iter.List {
				bodyEnv := NewEnvironment(env)
				bodyEnv.Define(n.Var, item, false)
				if err := i.execBlock(bodyEnv, n.Body); err != nil {
					return Null(), err
				}
			}
		default:
			return Null(), i.errorf(n, "cannot iterate over %s", iter.TypeName())
		}
		return Null(), nil
	case *ast.IfExpr:
		return i.evalIf(env, n)
	case *ast.BlockStmt:
		return i.evalBlock(env, n)
	}
	return Null(), nil
}

func (i *Interpreter) execBlock(env *Environment, block *ast.BlockStmt) error {
	for _, st := range block.Stmts {
		if _, err := i.execStmt(env, st); err != nil {
			return err
		}
	}
	return nil
}

func (i *Interpreter) evalBlock(env *Environment, block *ast.BlockStmt) (Value, error) {
	var last Value = Null()
	for _, st := range block.Stmts {
		v, err := i.execStmt(env, st)
		if err != nil {
			return v, err
		}
		last = v
	}
	return last, nil
}

func (i *Interpreter) evalIf(env *Environment, n *ast.IfExpr) (Value, error) {
	cond, err := i.evalExpr(env, n.Cond)
	if err != nil {
		return Null(), err
	}
	if cond.Truthy() {
		return i.evalBlock(NewEnvironment(env), n.Then)
	}
	if n.Else != nil {
		switch e := n.Else.(type) {
		case *ast.BlockStmt:
			return i.evalBlock(NewEnvironment(env), e)
		case *ast.IfExpr:
			return i.evalIf(env, e)
		}
	}
	return Null(), nil
}

func (i *Interpreter) evalExpr(env *Environment, expr ast.Expr) (Value, error) {
	switch n := expr.(type) {
	case *ast.IntLiteral:
		return IntVal(n.Value), nil
	case *ast.FloatLiteral:
		return FloatVal(n.Value), nil
	case *ast.StringLiteral:
		return i.interpolateString(env, n)
	case *ast.BoolLiteral:
		return BoolVal(n.Value), nil
	case *ast.NullLiteral:
		return Null(), nil
	case *ast.Identifier:
		v, ok := env.Get(n.Name)
		if !ok {
			return Null(), i.errorf(n, "unknown identifier %q", n.Name)
		}
		return v, nil
	case *ast.ListLiteral:
		vals := make([]Value, 0, len(n.Elements))
		for _, e := range n.Elements {
			v, err := i.evalExpr(env, e)
			if err != nil {
				return Null(), err
			}
			vals = append(vals, v)
		}
		return ListVal(vals), nil
	case *ast.UnaryExpr:
		v, err := i.evalExpr(env, n.Operand)
		if err != nil {
			return Null(), err
		}
		switch n.Op {
		case "-":
			switch v.Kind {
			case VInt:
				return IntVal(-v.Int), nil
			case VFloat:
				return FloatVal(-v.Float), nil
			}
			return Null(), i.errorf(n, "cannot negate %s", v.TypeName())
		case "!":
			return BoolVal(!v.Truthy()), nil
		}
		return Null(), i.errorf(n, "unknown unary op %q", n.Op)
	case *ast.BinaryExpr:
		return i.evalBinary(env, n)
	case *ast.CallExpr:
		return i.evalCall(env, n)
	case *ast.MemberExpr:
		return i.evalMember(env, n)
	case *ast.IndexExpr:
		target, err := i.evalExpr(env, n.Target)
		if err != nil {
			return Null(), err
		}
		idx, err := i.evalExpr(env, n.Index)
		if err != nil {
			return Null(), err
		}
		if target.Kind != VList {
			return Null(), i.errorf(n, "cannot index %s", target.TypeName())
		}
		if idx.Kind != VInt {
			return Null(), i.errorf(n, "list index must be Int")
		}
		i64 := idx.Int
		if i64 < 0 || int(i64) >= len(target.List) {
			return Null(), i.errorf(n, "list index out of range: %d", i64)
		}
		return target.List[i64], nil
	case *ast.IfExpr:
		return i.evalIf(env, n)
	case *ast.BlockStmt:
		return i.evalBlock(NewEnvironment(env), n)
	}
	return Null(), fmt.Errorf("cannot evaluate %T", expr)
}

func (i *Interpreter) evalBinary(env *Environment, n *ast.BinaryExpr) (Value, error) {
	if n.Op == "&&" {
		l, err := i.evalExpr(env, n.Left)
		if err != nil {
			return Null(), err
		}
		if !l.Truthy() {
			return BoolVal(false), nil
		}
		r, err := i.evalExpr(env, n.Right)
		if err != nil {
			return Null(), err
		}
		return BoolVal(r.Truthy()), nil
	}
	if n.Op == "||" {
		l, err := i.evalExpr(env, n.Left)
		if err != nil {
			return Null(), err
		}
		if l.Truthy() {
			return BoolVal(true), nil
		}
		r, err := i.evalExpr(env, n.Right)
		if err != nil {
			return Null(), err
		}
		return BoolVal(r.Truthy()), nil
	}
	l, err := i.evalExpr(env, n.Left)
	if err != nil {
		return Null(), err
	}
	r, err := i.evalExpr(env, n.Right)
	if err != nil {
		return Null(), err
	}
	switch n.Op {
	case "+":
		if l.Kind == VString || r.Kind == VString {
			return StringVal(l.String() + r.String()), nil
		}
		if l.Kind == VFloat || r.Kind == VFloat {
			return FloatVal(toFloat(l) + toFloat(r)), nil
		}
		if l.Kind == VInt && r.Kind == VInt {
			return IntVal(l.Int + r.Int), nil
		}
	case "-":
		if l.Kind == VFloat || r.Kind == VFloat {
			return FloatVal(toFloat(l) - toFloat(r)), nil
		}
		if l.Kind == VInt && r.Kind == VInt {
			return IntVal(l.Int - r.Int), nil
		}
	case "*":
		if l.Kind == VFloat || r.Kind == VFloat {
			return FloatVal(toFloat(l) * toFloat(r)), nil
		}
		if l.Kind == VInt && r.Kind == VInt {
			return IntVal(l.Int * r.Int), nil
		}
	case "/":
		if l.Kind == VFloat || r.Kind == VFloat {
			d := toFloat(r)
			if d == 0 {
				return Null(), i.errorf(n, "division by zero")
			}
			return FloatVal(toFloat(l) / d), nil
		}
		if l.Kind == VInt && r.Kind == VInt {
			if r.Int == 0 {
				return Null(), i.errorf(n, "division by zero")
			}
			return IntVal(l.Int / r.Int), nil
		}
	case "%":
		if l.Kind == VInt && r.Kind == VInt {
			if r.Int == 0 {
				return Null(), i.errorf(n, "division by zero")
			}
			return IntVal(l.Int % r.Int), nil
		}
	case "==":
		return BoolVal(equal(l, r)), nil
	case "!=":
		return BoolVal(!equal(l, r)), nil
	case "<", "<=", ">", ">=":
		return cmp(n.Op, l, r), nil
	}
	return Null(), i.errorf(n, "operator %q does not support %s and %s", n.Op, l.TypeName(), r.TypeName())
}

func toFloat(v Value) float64 {
	if v.Kind == VFloat {
		return v.Float
	}
	if v.Kind == VInt {
		return float64(v.Int)
	}
	return 0
}

func equal(a, b Value) bool {
	if a.Kind != b.Kind {
		if (a.Kind == VInt || a.Kind == VFloat) && (b.Kind == VInt || b.Kind == VFloat) {
			return toFloat(a) == toFloat(b)
		}
		return false
	}
	switch a.Kind {
	case VInt:
		return a.Int == b.Int
	case VFloat:
		return a.Float == b.Float
	case VBool:
		return a.Bool == b.Bool
	case VString:
		return a.Str == b.Str
	case VNull:
		return true
	}
	return false
}

func cmp(op string, l, r Value) Value {
	if l.Kind == VString && r.Kind == VString {
		switch op {
		case "<":
			return BoolVal(l.Str < r.Str)
		case "<=":
			return BoolVal(l.Str <= r.Str)
		case ">":
			return BoolVal(l.Str > r.Str)
		case ">=":
			return BoolVal(l.Str >= r.Str)
		}
	}
	a := toFloat(l)
	b := toFloat(r)
	switch op {
	case "<":
		return BoolVal(a < b)
	case "<=":
		return BoolVal(a <= b)
	case ">":
		return BoolVal(a > b)
	case ">=":
		return BoolVal(a >= b)
	}
	return BoolVal(false)
}

func (i *Interpreter) evalCall(env *Environment, n *ast.CallExpr) (Value, error) {
	callee, err := i.evalExpr(env, n.Callee)
	if err != nil {
		return Null(), err
	}
	args := make([]Value, 0, len(n.Args))
	for _, a := range n.Args {
		v, err := i.evalExpr(env, a)
		if err != nil {
			return Null(), err
		}
		args = append(args, v)
	}
	switch callee.Kind {
	case VBuiltin:
		return callee.Builtin.Fn(args)
	case VFunction:
		return i.callFunction(callee.Func, args, n)
	case VDataType:
		return i.constructData(callee.DataType, args, n)
	}
	return Null(), i.errorf(n, "value of type %s is not callable", callee.TypeName())
}

func (i *Interpreter) callFunction(fn *FunctionValue, args []Value, where ast.Node) (Value, error) {
	if len(args) != len(fn.Params) {
		return Null(), i.errorf(where, "function %q expects %d arguments, got %d", fn.Name, len(fn.Params), len(args))
	}
	callEnv := NewEnvironment(fn.Env)
	for idx, p := range fn.Params {
		callEnv.Define(p.Name, args[idx], false)
	}
	if fn.ExprBody != nil {
		return i.evalExpr(callEnv, fn.ExprBody)
	}
	if fn.Body != nil {
		var last Value = Null()
		for _, st := range fn.Body.Stmts {
			v, err := i.execStmt(callEnv, st)
			if err != nil {
				if rs, ok := err.(returnSignal); ok {
					return rs.value, nil
				}
				return Null(), err
			}
			last = v
		}
		return last, nil
	}
	return Null(), nil
}

func (i *Interpreter) constructData(dt *DataTypeValue, args []Value, where ast.Node) (Value, error) {
	if len(args) != len(dt.Fields) {
		return Null(), i.errorf(where, "data type %q expects %d arguments, got %d", dt.Name, len(dt.Fields), len(args))
	}
	inst := &DataInstance{Type: dt, Fields: map[string]Value{}}
	for idx, f := range dt.Fields {
		inst.Fields[f.Name] = args[idx]
	}
	return Value{Kind: VDataInstance, Instance: inst}, nil
}

func (i *Interpreter) evalMember(env *Environment, n *ast.MemberExpr) (Value, error) {
	target, err := i.evalExpr(env, n.Target)
	if err != nil {
		return Null(), err
	}
	if target.Kind == VDataInstance {
		if v, ok := target.Instance.Fields[n.Property]; ok {
			return v, nil
		}
		// Computed field?
		for _, cf := range target.Instance.Type.ComputedFields {
			if cf.Name == n.Property {
				instEnv := NewEnvironment(i.globals)
				for fname, fval := range target.Instance.Fields {
					instEnv.Define(fname, fval, false)
				}
				return i.evalExpr(instEnv, cf.Body)
			}
		}
		return Null(), i.errorf(n, "type %s has no field %q", target.Instance.Type.Name, n.Property)
	}
	if target.Kind == VList {
		switch n.Property {
		case "length", "size":
			return IntVal(int64(len(target.List))), nil
		}
	}
	if target.Kind == VString {
		switch n.Property {
		case "length", "size":
			return IntVal(int64(len([]rune(target.Str)))), nil
		}
	}
	return Null(), i.errorf(n, "cannot access %q on %s", n.Property, target.TypeName())
}

func (i *Interpreter) interpolateString(env *Environment, n *ast.StringLiteral) (Value, error) {
	src := n.Value
	if !strings.ContainsRune(src, '$') {
		return StringVal(src), nil
	}
	var sb strings.Builder
	runes := []rune(src)
	for j := 0; j < len(runes); j++ {
		r := runes[j]
		if r != '$' {
			sb.WriteRune(r)
			continue
		}
		// $ at end of string?
		if j+1 >= len(runes) {
			sb.WriteRune('$')
			continue
		}
		next := runes[j+1]
		if next == '{' {
			// find matching }
			end := j + 2
			depth := 1
			for end < len(runes) && depth > 0 {
				if runes[end] == '{' {
					depth++
				} else if runes[end] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				end++
			}
			if end >= len(runes) {
				return Null(), i.errorf(n, "unterminated ${} interpolation")
			}
			exprSrc := string(runes[j+2 : end])
			val, err := i.evalEmbeddedExpr(env, exprSrc, n)
			if err != nil {
				return Null(), err
			}
			sb.WriteString(val.String())
			j = end
			continue
		}
		if isIdentStart(next) {
			end := j + 1
			for end < len(runes) && isIdentCont(runes[end]) {
				end++
			}
			name := string(runes[j+1 : end])
			v, ok := env.Get(name)
			if !ok {
				return Null(), i.errorf(n, "unknown identifier %q in string interpolation", name)
			}
			sb.WriteString(v.String())
			j = end - 1
			continue
		}
		sb.WriteRune('$')
	}
	return StringVal(sb.String()), nil
}

func (i *Interpreter) evalEmbeddedExpr(env *Environment, src string, where ast.Node) (Value, error) {
	// Parse the expression-only fragment by feeding through a fresh parser.
	// To avoid an import cycle we do a tiny wrapper invocation through
	// the parser package via a helper exposed there. We use tokens directly.
	expr, err := parseEmbeddedExpr(src)
	if err != nil {
		return Null(), i.errorf(where, "interpolation parse error: %v", err)
	}
	return i.evalExpr(env, expr)
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}
func isIdentCont(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func parseEmbeddedExpr(src string) (ast.Expr, error) {
	return parser.ParseExpression(src)
}

func (i *Interpreter) errorf(node ast.Node, format string, args ...interface{}) error {
	l, c := node.Pos()
	return fmt.Errorf("[%d:%d] %s", l, c, fmt.Sprintf(format, args...))
}
