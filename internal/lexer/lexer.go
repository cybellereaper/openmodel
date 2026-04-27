package lexer

import (
	"fmt"
	"strings"
	"unicode"

	"purelang/internal/token"
)

type Lexer struct {
	src    []rune
	pos    int
	line   int
	col    int
	tokens []token.Token
	errs   []string
}

func New(source string) *Lexer {
	return &Lexer{
		src:  []rune(source),
		pos:  0,
		line: 1,
		col:  1,
	}
}

func Tokenize(source string) ([]token.Token, error) {
	l := New(source)
	return l.Lex()
}

func (l *Lexer) Lex() ([]token.Token, error) {
	for l.pos < len(l.src) {
		l.scan()
	}
	l.add(token.EOF, "")
	if len(l.errs) > 0 {
		return l.tokens, fmt.Errorf("%s", strings.Join(l.errs, "; "))
	}
	return l.tokens, nil
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekAt(off int) rune {
	if l.pos+off >= len(l.src) {
		return 0
	}
	return l.src[l.pos+off]
}

func (l *Lexer) advance() rune {
	r := l.src[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) addAt(t token.Type, v string, line, col int) {
	l.tokens = append(l.tokens, token.Token{Type: t, Value: v, Line: line, Column: col})
}

func (l *Lexer) add(t token.Type, v string) {
	l.tokens = append(l.tokens, token.Token{Type: t, Value: v, Line: l.line, Column: l.col})
}

func (l *Lexer) errorf(line, col int, format string, args ...interface{}) {
	l.errs = append(l.errs, fmt.Sprintf("[%d:%d] %s", line, col, fmt.Sprintf(format, args...)))
}

func (l *Lexer) scan() {
	r := l.peek()

	if r == ' ' || r == '\t' || r == '\r' {
		l.advance()
		return
	}

	if r == '\n' {
		line, col := l.line, l.col
		l.advance()
		l.addAt(token.NEWLINE, "\n", line, col)
		return
	}

	if r == '/' && l.peekAt(1) == '/' {
		for l.pos < len(l.src) && l.peek() != '\n' {
			l.advance()
		}
		return
	}

	line, col := l.line, l.col

	if isLetter(r) || r == '_' {
		l.scanIdent(line, col)
		return
	}

	if unicode.IsDigit(r) {
		l.scanNumber(line, col)
		return
	}

	if r == '"' {
		l.scanString(line, col)
		return
	}

	l.scanSymbol(line, col)
}

func isLetter(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func (l *Lexer) scanIdent(line, col int) {
	start := l.pos
	for l.pos < len(l.src) {
		r := l.peek()
		if isLetter(r) || unicode.IsDigit(r) {
			l.advance()
		} else {
			break
		}
	}
	val := string(l.src[start:l.pos])
	if kw, ok := token.Keywords[val]; ok {
		l.addAt(kw, val, line, col)
		return
	}
	l.addAt(token.IDENT, val, line, col)
}

func (l *Lexer) scanNumber(line, col int) {
	start := l.pos
	isFloat := false
	for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
		l.advance()
	}
	if l.peek() == '.' && unicode.IsDigit(l.peekAt(1)) {
		isFloat = true
		l.advance()
		for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	val := string(l.src[start:l.pos])
	if isFloat {
		l.addAt(token.FLOAT, val, line, col)
	} else {
		l.addAt(token.INT, val, line, col)
	}
}

func (l *Lexer) scanString(line, col int) {
	l.advance() // consume opening "
	var sb strings.Builder
	for l.pos < len(l.src) {
		r := l.peek()
		if r == '"' {
			l.advance()
			l.addAt(token.STRING, sb.String(), line, col)
			return
		}
		if r == '\\' {
			l.advance()
			esc := l.peek()
			l.advance()
			switch esc {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '\\':
				sb.WriteRune('\\')
			case '"':
				sb.WriteRune('"')
			case '$':
				sb.WriteRune('$')
			case '0':
				sb.WriteRune(0)
			default:
				sb.WriteRune(esc)
			}
			continue
		}
		if r == '\n' {
			l.errorf(line, col, "unterminated string literal")
			l.addAt(token.STRING, sb.String(), line, col)
			return
		}
		sb.WriteRune(r)
		l.advance()
	}
	l.errorf(line, col, "unterminated string literal")
	l.addAt(token.STRING, sb.String(), line, col)
}

func (l *Lexer) scanSymbol(line, col int) {
	r := l.advance()
	switch r {
	case '+':
		l.addAt(token.PLUS, "+", line, col)
	case '-':
		if l.peek() == '>' {
			l.advance()
			l.addAt(token.ARROW, "->", line, col)
		} else {
			l.addAt(token.MINUS, "-", line, col)
		}
	case '*':
		l.addAt(token.STAR, "*", line, col)
	case '/':
		l.addAt(token.SLASH, "/", line, col)
	case '%':
		l.addAt(token.PERCENT, "%", line, col)
	case '=':
		if l.peek() == '=' {
			l.advance()
			l.addAt(token.EQ, "==", line, col)
		} else if l.peek() == '>' {
			l.advance()
			l.addAt(token.FATARROW, "=>", line, col)
		} else {
			l.addAt(token.ASSIGN, "=", line, col)
		}
	case '!':
		if l.peek() == '=' {
			l.advance()
			l.addAt(token.NEQ, "!=", line, col)
		} else {
			l.addAt(token.BANG, "!", line, col)
		}
	case '<':
		if l.peek() == '=' {
			l.advance()
			l.addAt(token.LTE, "<=", line, col)
		} else {
			l.addAt(token.LT, "<", line, col)
		}
	case '>':
		if l.peek() == '=' {
			l.advance()
			l.addAt(token.GTE, ">=", line, col)
		} else {
			l.addAt(token.GT, ">", line, col)
		}
	case '&':
		if l.peek() == '&' {
			l.advance()
			l.addAt(token.AND, "&&", line, col)
		} else {
			l.errorf(line, col, "unexpected character '&'")
		}
	case '|':
		if l.peek() == '|' {
			l.advance()
			l.addAt(token.OR, "||", line, col)
		} else {
			l.errorf(line, col, "unexpected character '|'")
		}
	case '?':
		if l.peek() == ':' {
			l.advance()
			l.addAt(token.ELVIS, "?:", line, col)
		} else if l.peek() == '.' {
			l.advance()
			l.addAt(token.QDOT, "?.", line, col)
		} else {
			l.addAt(token.QUESTION, "?", line, col)
		}
	case '.':
		l.addAt(token.DOT, ".", line, col)
	case ',':
		l.addAt(token.COMMA, ",", line, col)
	case ':':
		l.addAt(token.COLON, ":", line, col)
	case ';':
		l.addAt(token.SEMI, ";", line, col)
	case '(':
		l.addAt(token.LPAREN, "(", line, col)
	case ')':
		l.addAt(token.RPAREN, ")", line, col)
	case '{':
		l.addAt(token.LBRACE, "{", line, col)
	case '}':
		l.addAt(token.RBRACE, "}", line, col)
	case '[':
		l.addAt(token.LBRACK, "[", line, col)
	case ']':
		l.addAt(token.RBRACK, "]", line, col)
	default:
		l.errorf(line, col, "unexpected character %q", r)
	}
}
