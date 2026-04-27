package types

type Kind int

const (
	KindUnknown Kind = iota
	KindInt
	KindFloat
	KindBool
	KindString
	KindNull
	KindVoid
	KindAny
	KindList
	KindFunction
	KindData
)

type Type struct {
	Kind     Kind
	Name     string // for data types
	ElemType *Type  // for list
}

var (
	Int    = &Type{Kind: KindInt, Name: "Int"}
	Float  = &Type{Kind: KindFloat, Name: "Float"}
	Bool   = &Type{Kind: KindBool, Name: "Bool"}
	String = &Type{Kind: KindString, Name: "String"}
	Null   = &Type{Kind: KindNull, Name: "Null"}
	Void   = &Type{Kind: KindVoid, Name: "Void"}
	Any    = &Type{Kind: KindAny, Name: "Any"}
)

func ListOf(elem *Type) *Type {
	return &Type{Kind: KindList, Name: "List", ElemType: elem}
}

func Function() *Type {
	return &Type{Kind: KindFunction, Name: "Function"}
}

func Data(name string) *Type {
	return &Type{Kind: KindData, Name: name}
}

func (t *Type) String() string {
	if t == nil {
		return "?"
	}
	if t.Kind == KindList && t.ElemType != nil {
		return "List<" + t.ElemType.String() + ">"
	}
	return t.Name
}

func FromName(name string) *Type {
	switch name {
	case "Int":
		return Int
	case "Float":
		return Float
	case "Bool":
		return Bool
	case "String":
		return String
	case "Null":
		return Null
	case "Void":
		return Void
	case "Any":
		return Any
	case "":
		return Any
	default:
		return Data(name)
	}
}

func Equal(a, b *Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Kind == KindAny || b.Kind == KindAny {
		return true
	}
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == KindData {
		return a.Name == b.Name
	}
	if a.Kind == KindList {
		return Equal(a.ElemType, b.ElemType)
	}
	return true
}
