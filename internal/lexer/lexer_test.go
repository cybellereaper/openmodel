package lexer

import (
	"testing"

	"purelang/internal/token"
)

func collectTypes(toks []token.Token) []token.Type {
	types := make([]token.Type, 0, len(toks))
	for _, t := range toks {
		if t.Type == token.NEWLINE || t.Type == token.EOF {
			continue
		}
		types = append(types, t.Type)
	}
	return types
}

func TestLexIdentifiers(t *testing.T) {
	toks, err := Tokenize("foo bar_baz user1")
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{token.IDENT, token.IDENT, token.IDENT}
	got := collectTypes(toks)
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("token %d: got %v want %v", i, got[i], w)
		}
	}
	if toks[0].Value != "foo" {
		t.Errorf("expected 'foo', got %q", toks[0].Value)
	}
}

func TestLexStrings(t *testing.T) {
	toks, err := Tokenize(`"hello" "world\n"`)
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Type != token.STRING {
		t.Errorf("expected STRING got %v", toks[0].Type)
	}
	if toks[0].Value != "hello" {
		t.Errorf("expected 'hello' got %q", toks[0].Value)
	}
	if toks[1].Value != "world\n" {
		t.Errorf("expected 'world\\n' got %q", toks[1].Value)
	}
}

func TestLexNumbers(t *testing.T) {
	toks, err := Tokenize("42 3.14 100")
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Type != token.INT || toks[0].Value != "42" {
		t.Errorf("got %v", toks[0])
	}
	if toks[1].Type != token.FLOAT || toks[1].Value != "3.14" {
		t.Errorf("got %v", toks[1])
	}
	if toks[2].Type != token.INT || toks[2].Value != "100" {
		t.Errorf("got %v", toks[2])
	}
}

func TestLexKeywords(t *testing.T) {
	toks, err := Tokenize("var if else for in use return true false null")
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.VAR, token.IF, token.ELSE, token.FOR, token.IN,
		token.USE, token.RETURN, token.TRUE, token.FALSE, token.NULL,
	}
	got := collectTypes(toks)
	if len(got) != len(want) {
		t.Fatalf("len mismatch %v vs %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("at %d: got %v want %v", i, got[i], w)
		}
	}
}

func TestLexOperators(t *testing.T) {
	toks, err := Tokenize("= == != => -> + - * / % < > <= >= && || ! . , : ( ) { } [ ]")
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.ASSIGN, token.EQ, token.NEQ, token.FATARROW, token.ARROW,
		token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT,
		token.LT, token.GT, token.LTE, token.GTE,
		token.AND, token.OR, token.BANG,
		token.DOT, token.COMMA, token.COLON,
		token.LPAREN, token.RPAREN, token.LBRACE, token.RBRACE, token.LBRACK, token.RBRACK,
	}
	got := collectTypes(toks)
	if len(got) != len(want) {
		t.Fatalf("len mismatch got=%d want=%d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("op %d: got %v want %v", i, got[i], w)
		}
	}
}

func TestLexComments(t *testing.T) {
	src := `// this is a comment
foo = 1 // trailing comment
// another
bar`
	toks, err := Tokenize(src)
	if err != nil {
		t.Fatal(err)
	}
	got := collectTypes(toks)
	want := []token.Type{token.IDENT, token.ASSIGN, token.INT, token.IDENT}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestLexLineColumn(t *testing.T) {
	toks, err := Tokenize("foo\n  bar")
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Line != 1 || toks[0].Column != 1 {
		t.Errorf("foo at %d:%d", toks[0].Line, toks[0].Column)
	}
	var bar token.Token
	for _, tok := range toks {
		if tok.Value == "bar" {
			bar = tok
			break
		}
	}
	if bar.Line != 2 || bar.Column != 3 {
		t.Errorf("bar at %d:%d, want 2:3", bar.Line, bar.Column)
	}
}
