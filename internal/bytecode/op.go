// Package bytecode defines the PureLang bytecode instruction set used by the
// compiler and the stack VM under internal/vm.
package bytecode

type Op byte

const (
	OpNop Op = iota
	OpConst       // push constants[arg]
	OpTrue
	OpFalse
	OpNull
	OpPop

	OpGetGlobal // push globals[name=constants[arg].(string)]
	OpSetGlobal // pop, globals[name] = top
	OpDefGlobal // define name=constants[arg], mutable=arg2 ignored

	OpGetLocal
	OpSetLocal

	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpNeg
	OpNot

	OpEq
	OpNeq
	OpLt
	OpLte
	OpGt
	OpGte

	OpAnd // logical and used by short-circuiting jumps in source; here is bitwise truth
	OpOr

	OpCall          // arg = arg count
	OpReturn
	OpJump          // arg = absolute address
	OpJumpIfFalse   // pops, arg = absolute address
	OpJumpIfFalseNoPop // peeks, arg = absolute address
	OpJumpIfTrueNoPop  // peeks, arg = absolute address

	OpList // arg = element count
	OpMember // member name in constants[arg]
	OpSafeMember
	OpIndex
	OpElvis // pops two, pushes left if !null else right (logical)

	OpHalt
)

func (o Op) String() string {
	if int(o) >= len(opNames) || opNames[o] == "" {
		return "Op(?)"
	}
	return opNames[o]
}

var opNames = [...]string{
	OpNop:              "Nop",
	OpConst:            "Const",
	OpTrue:             "True",
	OpFalse:            "False",
	OpNull:             "Null",
	OpPop:              "Pop",
	OpGetGlobal:        "GetGlobal",
	OpSetGlobal:        "SetGlobal",
	OpDefGlobal:        "DefGlobal",
	OpGetLocal:         "GetLocal",
	OpSetLocal:         "SetLocal",
	OpAdd:              "Add",
	OpSub:              "Sub",
	OpMul:              "Mul",
	OpDiv:              "Div",
	OpMod:              "Mod",
	OpNeg:              "Neg",
	OpNot:              "Not",
	OpEq:               "Eq",
	OpNeq:              "Neq",
	OpLt:               "Lt",
	OpLte:              "Lte",
	OpGt:               "Gt",
	OpGte:              "Gte",
	OpAnd:              "And",
	OpOr:               "Or",
	OpCall:             "Call",
	OpReturn:           "Return",
	OpJump:             "Jump",
	OpJumpIfFalse:      "JumpIfFalse",
	OpJumpIfFalseNoPop: "JumpIfFalseNoPop",
	OpJumpIfTrueNoPop:  "JumpIfTrueNoPop",
	OpList:             "List",
	OpMember:           "Member",
	OpSafeMember:       "SafeMember",
	OpIndex:            "Index",
	OpElvis:            "Elvis",
	OpHalt:             "Halt",
}

// Instruction is one Op + an int argument.
type Instruction struct {
	Op  Op
	Arg int
}

// Chunk holds compiled bytecode plus its constant table.
type Chunk struct {
	Code      []Instruction
	Constants []Constant
	Lines     []int // parallel to Code; line of source
}

// Constant kinds embedded in the constant table.
type ConstKind int

const (
	ConstInt ConstKind = iota
	ConstFloat
	ConstBool
	ConstString
	ConstNull
	ConstFunction
)

// Constant is a literal value referenced by OpConst.
type Constant struct {
	Kind   ConstKind
	Int    int64
	Float  float64
	Bool   bool
	Str    string
	Func   *FunctionProto
}

// FunctionProto is a compiled function reference.
type FunctionProto struct {
	Name       string
	Arity      int
	ParamNames []string
	Chunk      *Chunk
}
