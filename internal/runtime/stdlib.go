package runtime

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// stdModule returns the set of (name, value) pairs to inject for a std module.
func (i *Interpreter) stdModule(name string) map[string]Value {
	switch name {
	case "io":
		return i.stdIO()
	case "list":
		return i.stdList()
	case "string":
		return i.stdString()
	case "math":
		return i.stdMath()
	case "os":
		return i.stdOS()
	case "fs":
		return i.stdFS()
	}
	return nil
}

// LoadStdModuleInto injects all functions from std.<name> into env.
// Returns false if the module is not known.
func (i *Interpreter) LoadStdModuleInto(name string, env *Environment) bool {
	mod := i.stdModule(name)
	if mod == nil {
		return false
	}
	for k, v := range mod {
		env.Define(k, v, false)
	}
	return true
}

func builtin(name string, fn func(args []Value) (Value, error)) Value {
	return Value{
		Kind:    VBuiltin,
		Builtin: &BuiltinFunction{Name: name, Fn: fn},
	}
}

func arity(name string, want int, args []Value) error {
	if len(args) != want {
		return fmt.Errorf("%s expects %d arguments, got %d", name, want, len(args))
	}
	return nil
}

func (i *Interpreter) stdIO() map[string]Value {
	return map[string]Value{
		"print":   i.globals.values["print"],
		"println": i.globals.values["println"],
		"input": builtin("input", func(args []Value) (Value, error) {
			if len(args) > 1 {
				return Null(), fmt.Errorf("input expects 0 or 1 arguments")
			}
			if len(args) == 1 {
				fmt.Fprint(i.out, args[0].String())
			}
			r := bufio.NewReader(os.Stdin)
			line, err := r.ReadString('\n')
			if err != nil {
				return Null(), err
			}
			return StringVal(strings.TrimRight(line, "\r\n")), nil
		}),
	}
}

func (i *Interpreter) stdList() map[string]Value {
	return map[string]Value{
		"len": builtin("len", func(args []Value) (Value, error) {
			if err := arity("len", 1, args); err != nil {
				return Null(), err
			}
			switch args[0].Kind {
			case VList:
				return IntVal(int64(len(args[0].List))), nil
			case VString:
				return IntVal(int64(len([]rune(args[0].Str)))), nil
			}
			return Null(), fmt.Errorf("len: unsupported type %s", args[0].TypeName())
		}),
		"first": builtin("first", func(args []Value) (Value, error) {
			if err := arity("first", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VList {
				return Null(), fmt.Errorf("first: expected List")
			}
			if len(args[0].List) == 0 {
				return Null(), nil
			}
			return args[0].List[0], nil
		}),
		"last": builtin("last", func(args []Value) (Value, error) {
			if err := arity("last", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VList {
				return Null(), fmt.Errorf("last: expected List")
			}
			n := len(args[0].List)
			if n == 0 {
				return Null(), nil
			}
			return args[0].List[n-1], nil
		}),
		"push": builtin("push", func(args []Value) (Value, error) {
			if err := arity("push", 2, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VList {
				return Null(), fmt.Errorf("push: expected List")
			}
			out := make([]Value, len(args[0].List)+1)
			copy(out, args[0].List)
			out[len(args[0].List)] = args[1]
			return ListVal(out), nil
		}),
		"range": builtin("range", func(args []Value) (Value, error) {
			start, end, step := int64(0), int64(0), int64(1)
			switch len(args) {
			case 1:
				if args[0].Kind != VInt {
					return Null(), fmt.Errorf("range: expected Int")
				}
				end = args[0].Int
			case 2:
				if args[0].Kind != VInt || args[1].Kind != VInt {
					return Null(), fmt.Errorf("range: expected Ints")
				}
				start, end = args[0].Int, args[1].Int
			case 3:
				if args[0].Kind != VInt || args[1].Kind != VInt || args[2].Kind != VInt {
					return Null(), fmt.Errorf("range: expected Ints")
				}
				start, end, step = args[0].Int, args[1].Int, args[2].Int
			default:
				return Null(), fmt.Errorf("range expects 1-3 arguments")
			}
			if step == 0 {
				return Null(), fmt.Errorf("range: step must not be 0")
			}
			var out []Value
			if step > 0 {
				for v := start; v < end; v += step {
					out = append(out, IntVal(v))
				}
			} else {
				for v := start; v > end; v += step {
					out = append(out, IntVal(v))
				}
			}
			return ListVal(out), nil
		}),
		"reverse": builtin("reverse", func(args []Value) (Value, error) {
			if err := arity("reverse", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VList {
				return Null(), fmt.Errorf("reverse: expected List")
			}
			n := len(args[0].List)
			out := make([]Value, n)
			for k, v := range args[0].List {
				out[n-1-k] = v
			}
			return ListVal(out), nil
		}),
		"sort_ints": builtin("sort_ints", func(args []Value) (Value, error) {
			if err := arity("sort_ints", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VList {
				return Null(), fmt.Errorf("sort_ints: expected List")
			}
			ints := make([]int64, 0, len(args[0].List))
			for _, v := range args[0].List {
				if v.Kind != VInt {
					return Null(), fmt.Errorf("sort_ints: list must contain only Ints")
				}
				ints = append(ints, v.Int)
			}
			sort.Slice(ints, func(a, b int) bool { return ints[a] < ints[b] })
			out := make([]Value, len(ints))
			for k, v := range ints {
				out[k] = IntVal(v)
			}
			return ListVal(out), nil
		}),
	}
}

func (i *Interpreter) stdString() map[string]Value {
	return map[string]Value{
		"upper": builtin("upper", func(args []Value) (Value, error) {
			if err := arity("upper", 1, args); err != nil {
				return Null(), err
			}
			return StringVal(strings.ToUpper(args[0].String())), nil
		}),
		"lower": builtin("lower", func(args []Value) (Value, error) {
			if err := arity("lower", 1, args); err != nil {
				return Null(), err
			}
			return StringVal(strings.ToLower(args[0].String())), nil
		}),
		"trim": builtin("trim", func(args []Value) (Value, error) {
			if err := arity("trim", 1, args); err != nil {
				return Null(), err
			}
			return StringVal(strings.TrimSpace(args[0].String())), nil
		}),
		"split": builtin("split", func(args []Value) (Value, error) {
			if err := arity("split", 2, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VString || args[1].Kind != VString {
				return Null(), fmt.Errorf("split: expected (String, String)")
			}
			parts := strings.Split(args[0].Str, args[1].Str)
			out := make([]Value, len(parts))
			for k, p := range parts {
				out[k] = StringVal(p)
			}
			return ListVal(out), nil
		}),
		"join": builtin("join", func(args []Value) (Value, error) {
			if err := arity("join", 2, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VList || args[1].Kind != VString {
				return Null(), fmt.Errorf("join: expected (List, String)")
			}
			parts := make([]string, len(args[0].List))
			for k, v := range args[0].List {
				parts[k] = v.String()
			}
			return StringVal(strings.Join(parts, args[1].Str)), nil
		}),
		"contains": builtin("contains", func(args []Value) (Value, error) {
			if err := arity("contains", 2, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VString || args[1].Kind != VString {
				return Null(), fmt.Errorf("contains: expected (String, String)")
			}
			return BoolVal(strings.Contains(args[0].Str, args[1].Str)), nil
		}),
		"to_int": builtin("to_int", func(args []Value) (Value, error) {
			if err := arity("to_int", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VString {
				return Null(), fmt.Errorf("to_int: expected String")
			}
			v, err := strconv.ParseInt(strings.TrimSpace(args[0].Str), 10, 64)
			if err != nil {
				return Null(), nil
			}
			return IntVal(v), nil
		}),
		"to_string": builtin("to_string", func(args []Value) (Value, error) {
			if err := arity("to_string", 1, args); err != nil {
				return Null(), err
			}
			return StringVal(args[0].String()), nil
		}),
	}
}

func (i *Interpreter) stdMath() map[string]Value {
	return map[string]Value{
		"abs": builtin("abs", func(args []Value) (Value, error) {
			if err := arity("abs", 1, args); err != nil {
				return Null(), err
			}
			switch args[0].Kind {
			case VInt:
				v := args[0].Int
				if v < 0 {
					v = -v
				}
				return IntVal(v), nil
			case VFloat:
				return FloatVal(math.Abs(args[0].Float)), nil
			}
			return Null(), fmt.Errorf("abs: expected number")
		}),
		"min": builtin("min", func(args []Value) (Value, error) {
			if len(args) == 0 {
				return Null(), fmt.Errorf("min: needs at least 1 argument")
			}
			cur := args[0]
			for _, v := range args[1:] {
				if cmpLess(v, cur) {
					cur = v
				}
			}
			return cur, nil
		}),
		"max": builtin("max", func(args []Value) (Value, error) {
			if len(args) == 0 {
				return Null(), fmt.Errorf("max: needs at least 1 argument")
			}
			cur := args[0]
			for _, v := range args[1:] {
				if cmpLess(cur, v) {
					cur = v
				}
			}
			return cur, nil
		}),
		"sqrt": builtin("sqrt", func(args []Value) (Value, error) {
			if err := arity("sqrt", 1, args); err != nil {
				return Null(), err
			}
			return FloatVal(math.Sqrt(toFloat(args[0]))), nil
		}),
		"pow": builtin("pow", func(args []Value) (Value, error) {
			if err := arity("pow", 2, args); err != nil {
				return Null(), err
			}
			return FloatVal(math.Pow(toFloat(args[0]), toFloat(args[1]))), nil
		}),
		"floor": builtin("floor", func(args []Value) (Value, error) {
			if err := arity("floor", 1, args); err != nil {
				return Null(), err
			}
			return IntVal(int64(math.Floor(toFloat(args[0])))), nil
		}),
		"ceil": builtin("ceil", func(args []Value) (Value, error) {
			if err := arity("ceil", 1, args); err != nil {
				return Null(), err
			}
			return IntVal(int64(math.Ceil(toFloat(args[0])))), nil
		}),
		"pi": FloatVal(math.Pi),
		"e":  FloatVal(math.E),
	}
}

func cmpLess(a, b Value) bool {
	if a.Kind == VString && b.Kind == VString {
		return a.Str < b.Str
	}
	return toFloat(a) < toFloat(b)
}

func (i *Interpreter) stdOS() map[string]Value {
	return map[string]Value{
		"args": builtin("args", func(args []Value) (Value, error) {
			if err := arity("args", 0, args); err != nil {
				return Null(), err
			}
			out := make([]Value, len(os.Args))
			for k, v := range os.Args {
				out[k] = StringVal(v)
			}
			return ListVal(out), nil
		}),
		"env": builtin("env", func(args []Value) (Value, error) {
			if err := arity("env", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VString {
				return Null(), fmt.Errorf("env: expected String key")
			}
			return StringVal(os.Getenv(args[0].Str)), nil
		}),
		"exit": builtin("exit", func(args []Value) (Value, error) {
			code := 0
			if len(args) == 1 && args[0].Kind == VInt {
				code = int(args[0].Int)
			}
			os.Exit(code)
			return Null(), nil
		}),
	}
}

func (i *Interpreter) stdFS() map[string]Value {
	return map[string]Value{
		"read_file": builtin("read_file", func(args []Value) (Value, error) {
			if err := arity("read_file", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VString {
				return Null(), fmt.Errorf("read_file: expected String path")
			}
			data, err := os.ReadFile(args[0].Str)
			if err != nil {
				return Null(), err
			}
			return StringVal(string(data)), nil
		}),
		"write_file": builtin("write_file", func(args []Value) (Value, error) {
			if err := arity("write_file", 2, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VString || args[1].Kind != VString {
				return Null(), fmt.Errorf("write_file: expected (String, String)")
			}
			if err := os.WriteFile(args[0].Str, []byte(args[1].Str), 0o644); err != nil {
				return Null(), err
			}
			return Null(), nil
		}),
		"exists": builtin("exists", func(args []Value) (Value, error) {
			if err := arity("exists", 1, args); err != nil {
				return Null(), err
			}
			if args[0].Kind != VString {
				return Null(), fmt.Errorf("exists: expected String path")
			}
			_, err := os.Stat(args[0].Str)
			return BoolVal(err == nil), nil
		}),
	}
}
