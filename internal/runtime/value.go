package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"purelang/internal/ast"
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
	VDataType
	VDataInstance
)

type Value struct {
	Kind ValueKind

	Int   int64
	Float float64
	Bool  bool
	Str   string

	List []Value

	Func    *FunctionValue
	Builtin *BuiltinFunction

	DataType *DataTypeValue
	Instance *DataInstance
}

type FunctionValue struct {
	Name     string
	Params   []ast.Param
	Body     *ast.BlockStmt
	ExprBody ast.Expr
	Env      *Environment
}

type BuiltinFunction struct {
	Name string
	Fn   func(args []Value) (Value, error)
}

type DataTypeValue struct {
	Name           string
	Fields         []ast.DataField
	ComputedFields []*ast.ComputedField
}

type DataInstance struct {
	Type   *DataTypeValue
	Fields map[string]Value
}

func Null() Value      { return Value{Kind: VNull} }
func IntVal(v int64) Value { return Value{Kind: VInt, Int: v} }
func FloatVal(v float64) Value { return Value{Kind: VFloat, Float: v} }
func BoolVal(v bool) Value { return Value{Kind: VBool, Bool: v} }
func StringVal(v string) Value { return Value{Kind: VString, Str: v} }
func ListVal(v []Value) Value { return Value{Kind: VList, List: v} }

func (v Value) String() string {
	switch v.Kind {
	case VInt:
		return strconv.FormatInt(v.Int, 10)
	case VFloat:
		s := strconv.FormatFloat(v.Float, 'f', -1, 64)
		return s
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
		return "<builtin " + v.Builtin.Name + ">"
	case VDataType:
		return "<data " + v.DataType.Name + ">"
	case VDataInstance:
		var sb strings.Builder
		sb.WriteString(v.Instance.Type.Name)
		sb.WriteString("(")
		first := true
		for _, f := range v.Instance.Type.Fields {
			if !first {
				sb.WriteString(", ")
			}
			first = false
			sb.WriteString(f.Name)
			sb.WriteString("=")
			val := v.Instance.Fields[f.Name]
			if val.Kind == VString {
				sb.WriteString(strconv.Quote(val.Str))
			} else {
				sb.WriteString(val.String())
			}
		}
		sb.WriteString(")")
		return sb.String()
	}
	return fmt.Sprintf("<value kind=%d>", v.Kind)
}

func (v Value) Truthy() bool {
	switch v.Kind {
	case VBool:
		return v.Bool
	case VNull:
		return false
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

func (v Value) TypeName() string {
	switch v.Kind {
	case VInt:
		return "Int"
	case VFloat:
		return "Float"
	case VBool:
		return "Bool"
	case VString:
		return "String"
	case VNull:
		return "Null"
	case VList:
		return "List"
	case VFunction, VBuiltin:
		return "Function"
	case VDataType:
		return "DataType"
	case VDataInstance:
		return v.Instance.Type.Name
	}
	return "Unknown"
}
