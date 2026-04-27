package token

import "fmt"

type Type int

const (
	ILLEGAL Type = iota
	EOF
	NEWLINE

	IDENT
	INT
	FLOAT
	STRING

	TRUE
	FALSE
	NULL

	USE
	PUB
	VAR
	IF
	ELSE
	FOR
	IN
	WHEN
	RETURN
	OBJECT
	EXTEND
	SHAPE
	ASYNC
	AWAIT
	TRY
	THROW

	ASSIGN  // =
	FATARROW // =>
	ARROW   // ->
	PLUS
	MINUS
	STAR
	SLASH
	PERCENT
	EQ
	NEQ
	LT
	LTE
	GT
	GTE
	AND
	OR
	BANG
	DOT
	COMMA
	COLON
	SEMI
	QUESTION
	QDOT  // ?.
	ELVIS // ?:
	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACK
	RBRACK
)

var typeNames = map[Type]string{
	ILLEGAL:  "ILLEGAL",
	EOF:      "EOF",
	NEWLINE:  "NEWLINE",
	IDENT:    "IDENT",
	INT:      "INT",
	FLOAT:    "FLOAT",
	STRING:   "STRING",
	TRUE:     "TRUE",
	FALSE:    "FALSE",
	NULL:     "NULL",
	USE:      "USE",
	PUB:      "PUB",
	VAR:      "VAR",
	IF:       "IF",
	ELSE:     "ELSE",
	FOR:      "FOR",
	IN:       "IN",
	WHEN:     "WHEN",
	RETURN:   "RETURN",
	OBJECT:   "OBJECT",
	EXTEND:   "EXTEND",
	SHAPE:    "SHAPE",
	ASYNC:    "ASYNC",
	AWAIT:    "AWAIT",
	TRY:      "TRY",
	THROW:    "THROW",
	ASSIGN:   "=",
	FATARROW: "=>",
	ARROW:    "->",
	PLUS:     "+",
	MINUS:    "-",
	STAR:     "*",
	SLASH:    "/",
	PERCENT:  "%",
	EQ:       "==",
	NEQ:      "!=",
	LT:       "<",
	LTE:      "<=",
	GT:       ">",
	GTE:      ">=",
	AND:      "&&",
	OR:       "||",
	BANG:     "!",
	DOT:      ".",
	COMMA:    ",",
	COLON:    ":",
	SEMI:     ";",
	QUESTION: "?",
	QDOT:     "?.",
	ELVIS:    "?:",
	LPAREN:   "(",
	RPAREN:   ")",
	LBRACE:   "{",
	RBRACE:   "}",
	LBRACK:   "[",
	RBRACK:   "]",
}

func (t Type) String() string {
	if s, ok := typeNames[t]; ok {
		return s
	}
	return fmt.Sprintf("Token(%d)", int(t))
}

var Keywords = map[string]Type{
	"use":    USE,
	"pub":    PUB,
	"var":    VAR,
	"if":     IF,
	"else":   ELSE,
	"for":    FOR,
	"in":     IN,
	"when":   WHEN,
	"return": RETURN,
	"object": OBJECT,
	"extend": EXTEND,
	"shape":  SHAPE,
	"async":  ASYNC,
	"await":  AWAIT,
	"try":    TRY,
	"throw":  THROW,
	"true":   TRUE,
	"false":  FALSE,
	"null":   NULL,
}

type Token struct {
	Type    Type
	Value   string
	Line    int
	Column  int
}

func (t Token) String() string {
	return fmt.Sprintf("%s(%q) at %d:%d", t.Type, t.Value, t.Line, t.Column)
}
