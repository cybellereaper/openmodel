// Package vm implements a stack-based virtual machine for PureLang
// bytecode produced by internal/bytecode.
package vm

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"purelang/internal/bytecode"
)

type ValueKind int

const (
	VInt ValueKind = iota
	VFloat
	VBool
	VString
	VNull
	VList
	VFunction
	VBuiltin
)

type Value struct {
	Kind    ValueKind
	Int     int64
	Float   float64
	Bool    bool
	Str     string
	List    []Value
	Func    *bytecode.FunctionProto
	Builtin func(args []Value) (Value, error)
	BName   string
}

func (v Value) String() string {
	switch v.Kind {
	case VInt:
		return strconv.FormatInt(v.Int, 10)
	case VFloat:
		return strconv.FormatFloat(v.Float, 'f', -1, 64)
	case VBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case VString:
		return v.Str
	case VNull:
		return "null"
	case VList:
		parts := make([]string, len(v.List))
		for i, e := range v.List {
			if e.Kind == VString {
				parts[i] = strconv.Quote(e.Str)
			} else {
				parts[i] = e.String()
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case VFunction:
		return "<function " + v.Func.Name + ">"
	case VBuiltin:
		return "<builtin " + v.BName + ">"
	}
	return "<value>"
}

func (v Value) Truthy() bool {
	switch v.Kind {
	case VNull:
		return false
	case VBool:
		return v.Bool
	case VInt:
		return v.Int != 0
	case VFloat:
		return v.Float != 0
	case VString:
		return v.Str != ""
	case VList:
		return len(v.List) > 0
	}
	return true
}

type frame struct {
	chunk  *bytecode.Chunk
	ip     int
	locals []Value
	base   int // stack base for this frame's locals (function args start here)
	fn     *bytecode.FunctionProto
}

// VM is a stack-based bytecode interpreter.
type VM struct {
	frames  []*frame
	stack   []Value
	globals map[string]Value
	out     io.Writer
}

func New() *VM {
	return NewWithWriter(os.Stdout)
}

func NewWithWriter(w io.Writer) *VM {
	v := &VM{
		stack:   make([]Value, 0, 256),
		globals: map[string]Value{},
		out:     w,
	}
	v.installBuiltins()
	return v
}

func (v *VM) installBuiltins() {
	v.globals["print"] = Value{Kind: VBuiltin, BName: "print", Builtin: func(args []Value) (Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		fmt.Fprintln(v.out, strings.Join(parts, " "))
		return Value{Kind: VNull}, nil
	}}
	v.globals["println"] = v.globals["print"]
}

// Run executes a top-level chunk to completion.
func (v *VM) Run(chunk *bytecode.Chunk) error {
	v.frames = append(v.frames, &frame{chunk: chunk, ip: 0, base: 0})
	for len(v.frames) > 0 {
		f := v.frames[len(v.frames)-1]
		if f.ip >= len(f.chunk.Code) {
			break
		}
		ins := f.chunk.Code[f.ip]
		f.ip++
		if err := v.exec(ins, f); err != nil {
			return err
		}
		if ins.Op == bytecode.OpHalt {
			return nil
		}
	}
	return nil
}

func (v *VM) push(val Value) { v.stack = append(v.stack, val) }
func (v *VM) pop() Value {
	val := v.stack[len(v.stack)-1]
	v.stack = v.stack[:len(v.stack)-1]
	return val
}
func (v *VM) peek() Value { return v.stack[len(v.stack)-1] }

func (v *VM) constToValue(c bytecode.Constant) Value {
	switch c.Kind {
	case bytecode.ConstInt:
		return Value{Kind: VInt, Int: c.Int}
	case bytecode.ConstFloat:
		return Value{Kind: VFloat, Float: c.Float}
	case bytecode.ConstBool:
		return Value{Kind: VBool, Bool: c.Bool}
	case bytecode.ConstString:
		return Value{Kind: VString, Str: c.Str}
	case bytecode.ConstNull:
		return Value{Kind: VNull}
	case bytecode.ConstFunction:
		return Value{Kind: VFunction, Func: c.Func}
	}
	return Value{Kind: VNull}
}

func (v *VM) exec(ins bytecode.Instruction, f *frame) error {
	switch ins.Op {
	case bytecode.OpNop, bytecode.OpHalt:
		return nil
	case bytecode.OpConst:
		v.push(v.constToValue(f.chunk.Constants[ins.Arg]))
	case bytecode.OpTrue:
		v.push(Value{Kind: VBool, Bool: true})
	case bytecode.OpFalse:
		v.push(Value{Kind: VBool, Bool: false})
	case bytecode.OpNull:
		v.push(Value{Kind: VNull})
	case bytecode.OpPop:
		v.pop()
	case bytecode.OpDefGlobal:
		name := f.chunk.Constants[ins.Arg].Str
		val := v.pop()
		v.globals[name] = val
	case bytecode.OpSetGlobal:
		name := f.chunk.Constants[ins.Arg].Str
		val := v.peek()
		if _, ok := v.globals[name]; !ok {
			return fmt.Errorf("vm: unknown variable %q", name)
		}
		v.globals[name] = val
	case bytecode.OpGetGlobal:
		name := f.chunk.Constants[ins.Arg].Str
		val, ok := v.globals[name]
		if !ok {
			return fmt.Errorf("vm: unknown identifier %q", name)
		}
		v.push(val)
	case bytecode.OpGetLocal:
		v.push(v.stack[f.base+ins.Arg])
	case bytecode.OpSetLocal:
		v.stack[f.base+ins.Arg] = v.peek()
	case bytecode.OpAdd:
		return v.binArith("+")
	case bytecode.OpSub:
		return v.binArith("-")
	case bytecode.OpMul:
		return v.binArith("*")
	case bytecode.OpDiv:
		return v.binArith("/")
	case bytecode.OpMod:
		return v.binArith("%")
	case bytecode.OpNeg:
		x := v.pop()
		if x.Kind == VInt {
			v.push(Value{Kind: VInt, Int: -x.Int})
		} else if x.Kind == VFloat {
			v.push(Value{Kind: VFloat, Float: -x.Float})
		} else {
			return fmt.Errorf("vm: cannot negate %v", x.Kind)
		}
	case bytecode.OpNot:
		x := v.pop()
		v.push(Value{Kind: VBool, Bool: !x.Truthy()})
	case bytecode.OpEq:
		b := v.pop()
		a := v.pop()
		v.push(Value{Kind: VBool, Bool: equal(a, b)})
	case bytecode.OpNeq:
		b := v.pop()
		a := v.pop()
		v.push(Value{Kind: VBool, Bool: !equal(a, b)})
	case bytecode.OpLt:
		return v.cmp("<")
	case bytecode.OpLte:
		return v.cmp("<=")
	case bytecode.OpGt:
		return v.cmp(">")
	case bytecode.OpGte:
		return v.cmp(">=")
	case bytecode.OpJump:
		f.ip = ins.Arg
	case bytecode.OpJumpIfFalse:
		x := v.pop()
		if !x.Truthy() {
			f.ip = ins.Arg
		}
	case bytecode.OpJumpIfFalseNoPop:
		if !v.peek().Truthy() {
			f.ip = ins.Arg
		}
	case bytecode.OpJumpIfTrueNoPop:
		if v.peek().Truthy() {
			f.ip = ins.Arg
		}
	case bytecode.OpList:
		n := ins.Arg
		out := make([]Value, n)
		for i := n - 1; i >= 0; i-- {
			out[i] = v.pop()
		}
		v.push(Value{Kind: VList, List: out})
	case bytecode.OpMember:
		// Members on data instances aren't supported by the VM. For builtin
		// types support a few useful properties.
		name := f.chunk.Constants[ins.Arg].Str
		x := v.pop()
		switch x.Kind {
		case VString:
			if name == "length" || name == "size" {
				v.push(Value{Kind: VInt, Int: int64(len([]rune(x.Str)))})
				return nil
			}
		case VList:
			if name == "length" || name == "size" {
				v.push(Value{Kind: VInt, Int: int64(len(x.List))})
				return nil
			}
		}
		return fmt.Errorf("vm: cannot access %q on %v", name, x.Kind)
	case bytecode.OpSafeMember:
		name := f.chunk.Constants[ins.Arg].Str
		x := v.pop()
		if x.Kind == VNull {
			v.push(Value{Kind: VNull})
			return nil
		}
		switch x.Kind {
		case VString:
			if name == "length" || name == "size" {
				v.push(Value{Kind: VInt, Int: int64(len([]rune(x.Str)))})
				return nil
			}
		case VList:
			if name == "length" || name == "size" {
				v.push(Value{Kind: VInt, Int: int64(len(x.List))})
				return nil
			}
		}
		return fmt.Errorf("vm: cannot access %q on %v", name, x.Kind)
	case bytecode.OpIndex:
		idx := v.pop()
		tgt := v.pop()
		if tgt.Kind != VList || idx.Kind != VInt {
			return fmt.Errorf("vm: invalid index")
		}
		if idx.Int < 0 || int(idx.Int) >= len(tgt.List) {
			return fmt.Errorf("vm: index out of range")
		}
		v.push(tgt.List[idx.Int])
	case bytecode.OpElvis:
		right := v.pop()
		left := v.pop()
		if left.Kind == VNull {
			v.push(right)
		} else {
			v.push(left)
		}
	case bytecode.OpCall:
		return v.doCall(ins.Arg)
	case bytecode.OpReturn:
		ret := v.pop()
		// drop locals
		v.stack = v.stack[:f.base]
		v.frames = v.frames[:len(v.frames)-1]
		v.push(ret)
	default:
		return fmt.Errorf("vm: unknown op %v", ins.Op)
	}
	return nil
}

func (v *VM) doCall(argc int) error {
	args := make([]Value, argc)
	for i := argc - 1; i >= 0; i-- {
		args[i] = v.pop()
	}
	callee := v.pop()
	switch callee.Kind {
	case VBuiltin:
		out, err := callee.Builtin(args)
		if err != nil {
			return err
		}
		v.push(out)
		return nil
	case VFunction:
		fn := callee.Func
		if len(args) != fn.Arity {
			return fmt.Errorf("vm: %s: expected %d args, got %d", fn.Name, fn.Arity, len(args))
		}
		base := len(v.stack)
		v.stack = append(v.stack, args...)
		v.frames = append(v.frames, &frame{chunk: fn.Chunk, ip: 0, base: base, fn: fn})
		return nil
	}
	return fmt.Errorf("vm: not callable")
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
	if (a.Kind == VInt || a.Kind == VFloat) && (b.Kind == VInt || b.Kind == VFloat) {
		return toFloat(a) == toFloat(b)
	}
	if a.Kind != b.Kind {
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

func (v *VM) binArith(op string) error {
	b := v.pop()
	a := v.pop()
	if op == "+" && (a.Kind == VString || b.Kind == VString) {
		v.push(Value{Kind: VString, Str: a.String() + b.String()})
		return nil
	}
	if a.Kind == VFloat || b.Kind == VFloat {
		af := toFloat(a)
		bf := toFloat(b)
		var r float64
		switch op {
		case "+":
			r = af + bf
		case "-":
			r = af - bf
		case "*":
			r = af * bf
		case "/":
			if bf == 0 {
				return fmt.Errorf("vm: division by zero")
			}
			r = af / bf
		case "%":
			return fmt.Errorf("vm: %% requires Int operands")
		}
		v.push(Value{Kind: VFloat, Float: r})
		return nil
	}
	if a.Kind == VInt && b.Kind == VInt {
		var r int64
		switch op {
		case "+":
			r = a.Int + b.Int
		case "-":
			r = a.Int - b.Int
		case "*":
			r = a.Int * b.Int
		case "/":
			if b.Int == 0 {
				return fmt.Errorf("vm: division by zero")
			}
			r = a.Int / b.Int
		case "%":
			if b.Int == 0 {
				return fmt.Errorf("vm: division by zero")
			}
			r = a.Int % b.Int
		}
		v.push(Value{Kind: VInt, Int: r})
		return nil
	}
	return fmt.Errorf("vm: arithmetic on %v and %v", a.Kind, b.Kind)
}

func (v *VM) cmp(op string) error {
	b := v.pop()
	a := v.pop()
	if a.Kind == VString && b.Kind == VString {
		var ok bool
		switch op {
		case "<":
			ok = a.Str < b.Str
		case "<=":
			ok = a.Str <= b.Str
		case ">":
			ok = a.Str > b.Str
		case ">=":
			ok = a.Str >= b.Str
		}
		v.push(Value{Kind: VBool, Bool: ok})
		return nil
	}
	af := toFloat(a)
	bf := toFloat(b)
	var ok bool
	switch op {
	case "<":
		ok = af < bf
	case "<=":
		ok = af <= bf
	case ">":
		ok = af > bf
	case ">=":
		ok = af >= bf
	}
	v.push(Value{Kind: VBool, Bool: ok})
	return nil
}
