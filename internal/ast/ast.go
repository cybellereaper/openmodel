package ast

type Node interface {
	Pos() (int, int)
}

type Stmt interface {
	Node
	stmtNode()
}

type Expr interface {
	Node
	exprNode()
}

type Position struct {
	Line   int
	Column int
}

func (p Position) Pos() (int, int) { return p.Line, p.Column }

type Program struct {
	Position
	Stmts []Stmt
}

type UseDecl struct {
	Position
	Path []string // e.g. ["std", "io"]
}

func (*UseDecl) stmtNode() {}

type VarDecl struct {
	Position
	Name    string
	Mutable bool
	Type    string // optional explicit type
	Value   Expr
}

func (*VarDecl) stmtNode() {}

type AssignStmt struct {
	Position
	Target Expr
	Value  Expr
}

func (*AssignStmt) stmtNode() {}

type Param struct {
	Name string
	Type string
}

type FunctionDecl struct {
	Position
	Name       string
	Params     []Param
	ReturnType string
	Body       *BlockStmt
	ExprBody   Expr // for `=>` shorthand
}

func (*FunctionDecl) stmtNode() {}

type ComputedField struct {
	Position
	Name string
	Body Expr
}

type DataField struct {
	Name string
	Type string
}

type DataDecl struct {
	Position
	Name           string
	Fields         []DataField
	ComputedFields []*ComputedField
}

func (*DataDecl) stmtNode() {}

type BlockStmt struct {
	Position
	Stmts []Stmt
}

func (*BlockStmt) stmtNode() {}
func (*BlockStmt) exprNode() {}

type ExpressionStmt struct {
	Position
	Expr Expr
}

func (*ExpressionStmt) stmtNode() {}

type ReturnStmt struct {
	Position
	Value Expr // may be nil
}

func (*ReturnStmt) stmtNode() {}

type IfExpr struct {
	Position
	Cond Expr
	Then *BlockStmt
	Else Stmt // *BlockStmt or *IfExpr or nil
}

func (*IfExpr) stmtNode() {}
func (*IfExpr) exprNode() {}

type ForStmt struct {
	Position
	Var      string
	Iterable Expr
	Body     *BlockStmt
}

func (*ForStmt) stmtNode() {}

type CallExpr struct {
	Position
	Callee Expr
	Args   []Expr
}

func (*CallExpr) exprNode() {}

type MemberExpr struct {
	Position
	Target   Expr
	Property string
}

func (*MemberExpr) exprNode() {}

type IndexExpr struct {
	Position
	Target Expr
	Index  Expr
}

func (*IndexExpr) exprNode() {}

type BinaryExpr struct {
	Position
	Op    string
	Left  Expr
	Right Expr
}

func (*BinaryExpr) exprNode() {}

type UnaryExpr struct {
	Position
	Op      string
	Operand Expr
}

func (*UnaryExpr) exprNode() {}

type StringLiteral struct {
	Position
	Value string
}

func (*StringLiteral) exprNode() {}

type IntLiteral struct {
	Position
	Value int64
}

func (*IntLiteral) exprNode() {}

type FloatLiteral struct {
	Position
	Value float64
}

func (*FloatLiteral) exprNode() {}

type BoolLiteral struct {
	Position
	Value bool
}

func (*BoolLiteral) exprNode() {}

type NullLiteral struct {
	Position
}

func (*NullLiteral) exprNode() {}

type Identifier struct {
	Position
	Name string
}

func (*Identifier) exprNode() {}

type ListLiteral struct {
	Position
	Elements []Expr
}

func (*ListLiteral) exprNode() {}
