// Package native turns a PureLang Program into a Go source file that uses
// a small embedded runtime (also generated). Compiling that Go file with
// `go build` yields a native executable.
//
// The translation supported here is intentionally a useful subset of
// PureLang sufficient to demonstrate native compilation:
//   - top-level immutable and mutable variable bindings
//   - function declarations (expression body or block body, single return)
//   - calls to user functions and to a small native standard library
//     (`print`, `println`)
//   - arithmetic, comparison, and logical expressions
//   - if-expressions, lists, list indexing
//   - string interpolation with $ident and ${expr}
//
// More advanced features (data types, when, ?., closures, etc.) fall back
// to a clear compilation error so that users know to either use the
// tree-walking interpreter or extend this package.
package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"purelang/internal/ast"
)

// Build compiles a PureLang source file to a native binary at outBin.
// It writes Go source into a temp directory and invokes the local `go`
// toolchain to produce the executable.
func Build(progSrc string, prog *ast.Program, outBin string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("native build requires the go toolchain in PATH")
	}
	src, err := Generate(prog)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "purelang-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module purelang_native\n\ngo 1.21\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-o", outBin, ".")
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %v\n%s\n--- generated source ---\n%s", err, out, src)
	}
	return nil
}

// Generate converts a Program to Go source code.
func Generate(prog *ast.Program) (string, error) {
	g := &generator{}
	g.writeHeader()
	// Hoist function declarations to package-level functions.
	var topStmts []ast.Stmt
	for _, s := range prog.Stmts {
		if fn, ok := s.(*ast.FunctionDecl); ok {
			if err := g.writeFunction(fn); err != nil {
				return "", err
			}
			continue
		}
		if _, ok := s.(*ast.UseDecl); ok {
			continue
		}
		topStmts = append(topStmts, s)
	}
	g.writeMainStart()
	for _, s := range topStmts {
		if err := g.writeStmt(s, 1); err != nil {
			return "", err
		}
	}
	g.writeMainEnd()
	return g.out.String(), nil
}

type generator struct {
	out strings.Builder
}

const runtimeBlock = `package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// pureValue is the universal runtime value used by the generated program.
type pureValue struct {
	kind int // 0 int, 1 float, 2 bool, 3 string, 4 null, 5 list
	i    int64
	f    float64
	b    bool
	s    string
	l    []pureValue
}

func vInt(v int64) pureValue       { return pureValue{kind: 0, i: v} }
func vFloat(v float64) pureValue   { return pureValue{kind: 1, f: v} }
func vBool(v bool) pureValue       { return pureValue{kind: 2, b: v} }
func vString(v string) pureValue   { return pureValue{kind: 3, s: v} }
func vNull() pureValue             { return pureValue{kind: 4} }
func vList(v []pureValue) pureValue { return pureValue{kind: 5, l: v} }

func (v pureValue) String() string {
	switch v.kind {
	case 0:
		return strconv.FormatInt(v.i, 10)
	case 1:
		return strconv.FormatFloat(v.f, 'f', -1, 64)
	case 2:
		if v.b {
			return "true"
		}
		return "false"
	case 3:
		return v.s
	case 4:
		return "null"
	case 5:
		parts := make([]string, len(v.l))
		for i, e := range v.l {
			if e.kind == 3 {
				parts[i] = strconv.Quote(e.s)
			} else {
				parts[i] = e.String()
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return ""
}

func (v pureValue) Truthy() bool {
	switch v.kind {
	case 4:
		return false
	case 2:
		return v.b
	case 0:
		return v.i != 0
	case 1:
		return v.f != 0
	case 3:
		return v.s != ""
	case 5:
		return len(v.l) > 0
	}
	return true
}

func toFloat(v pureValue) float64 {
	if v.kind == 1 {
		return v.f
	}
	return float64(v.i)
}

func add(a, b pureValue) pureValue {
	if a.kind == 3 || b.kind == 3 {
		return vString(a.String() + b.String())
	}
	if a.kind == 1 || b.kind == 1 {
		return vFloat(toFloat(a) + toFloat(b))
	}
	return vInt(a.i + b.i)
}
func sub(a, b pureValue) pureValue {
	if a.kind == 1 || b.kind == 1 {
		return vFloat(toFloat(a) - toFloat(b))
	}
	return vInt(a.i - b.i)
}
func mul(a, b pureValue) pureValue {
	if a.kind == 1 || b.kind == 1 {
		return vFloat(toFloat(a) * toFloat(b))
	}
	return vInt(a.i * b.i)
}
func div(a, b pureValue) pureValue {
	if a.kind == 1 || b.kind == 1 {
		return vFloat(toFloat(a) / toFloat(b))
	}
	return vInt(a.i / b.i)
}
func mod(a, b pureValue) pureValue { return vInt(a.i % b.i) }

func cmpLT(a, b pureValue) pureValue { return vBool(toFloat(a) < toFloat(b)) }
func cmpLTE(a, b pureValue) pureValue { return vBool(toFloat(a) <= toFloat(b)) }
func cmpGT(a, b pureValue) pureValue { return vBool(toFloat(a) > toFloat(b)) }
func cmpGTE(a, b pureValue) pureValue { return vBool(toFloat(a) >= toFloat(b)) }

func eq(a, b pureValue) pureValue {
	if (a.kind == 0 || a.kind == 1) && (b.kind == 0 || b.kind == 1) {
		return vBool(toFloat(a) == toFloat(b))
	}
	if a.kind != b.kind {
		return vBool(false)
	}
	switch a.kind {
	case 0:
		return vBool(a.i == b.i)
	case 1:
		return vBool(a.f == b.f)
	case 2:
		return vBool(a.b == b.b)
	case 3:
		return vBool(a.s == b.s)
	case 4:
		return vBool(true)
	}
	return vBool(false)
}
func neq(a, b pureValue) pureValue { e := eq(a, b); return vBool(!e.b) }

func elvis(a, b pureValue) pureValue {
	if a.kind == 4 {
		return b
	}
	return a
}

func index(target, idx pureValue) pureValue {
	if target.kind != 5 {
		fmt.Fprintln(os.Stderr, "runtime: cannot index non-list")
		os.Exit(1)
	}
	return target.l[idx.i]
}

func memberLength(v pureValue) pureValue {
	switch v.kind {
	case 3:
		return vInt(int64(len([]rune(v.s))))
	case 5:
		return vInt(int64(len(v.l)))
	}
	fmt.Fprintln(os.Stderr, "runtime: no length on this value")
	os.Exit(1)
	return vNull()
}

func ppPrint(args ...pureValue) pureValue {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.String()
	}
	fmt.Println(strings.Join(parts, " "))
	return vNull()
}
`

func (g *generator) writeHeader() {
	g.out.WriteString(runtimeBlock)
	g.out.WriteString("\n")
}

func (g *generator) writeMainStart() {
	g.out.WriteString("\nfunc main() {\n")
}

func (g *generator) writeMainEnd() {
	g.out.WriteString("}\n")
}

func (g *generator) writeFunction(fn *ast.FunctionDecl) error {
	fmt.Fprintf(&g.out, "\nfunc %s(", goIdent(fn.Name))
	for i, p := range fn.Params {
		if i > 0 {
			g.out.WriteString(", ")
		}
		fmt.Fprintf(&g.out, "%s pureValue", goIdent(p.Name))
	}
	g.out.WriteString(") pureValue {\n")
	if fn.ExprBody != nil {
		expr, err := g.exprStr(fn.ExprBody)
		if err != nil {
			return err
		}
		fmt.Fprintf(&g.out, "\treturn %s\n", expr)
	} else if fn.Body != nil {
		hadReturn := false
		for i, s := range fn.Body.Stmts {
			isLast := i == len(fn.Body.Stmts)-1
			if isLast {
				if es, ok := s.(*ast.ExpressionStmt); ok {
					expr, err := g.exprStr(es.Expr)
					if err != nil {
						return err
					}
					fmt.Fprintf(&g.out, "\treturn %s\n", expr)
					hadReturn = true
					continue
				}
			}
			if err := g.writeStmt(s, 1); err != nil {
				return err
			}
		}
		if !hadReturn {
			g.out.WriteString("\treturn vNull()\n")
		}
	}
	g.out.WriteString("}\n")
	return nil
}

func (g *generator) writeStmt(s ast.Stmt, depth int) error {
	indent := strings.Repeat("\t", depth)
	switch n := s.(type) {
	case *ast.UseDecl:
		return nil
	case *ast.VarDecl:
		expr, err := g.exprStr(n.Value)
		if err != nil {
			return err
		}
		fmt.Fprintf(&g.out, "%s%s := %s\n", indent, goIdent(n.Name), expr)
		fmt.Fprintf(&g.out, "%s_ = %s\n", indent, goIdent(n.Name))
		return nil
	case *ast.AssignStmt:
		id, ok := n.Target.(*ast.Identifier)
		if !ok {
			return fmt.Errorf("native: only identifier assignment is supported")
		}
		expr, err := g.exprStr(n.Value)
		if err != nil {
			return err
		}
		fmt.Fprintf(&g.out, "%s%s = %s\n", indent, goIdent(id.Name), expr)
		return nil
	case *ast.ExpressionStmt:
		expr, err := g.exprStr(n.Expr)
		if err != nil {
			return err
		}
		fmt.Fprintf(&g.out, "%s_ = %s\n", indent, expr)
		return nil
	case *ast.ReturnStmt:
		if n.Value == nil {
			fmt.Fprintf(&g.out, "%sreturn vNull()\n", indent)
			return nil
		}
		expr, err := g.exprStr(n.Value)
		if err != nil {
			return err
		}
		fmt.Fprintf(&g.out, "%sreturn %s\n", indent, expr)
		return nil
	case *ast.IfExpr:
		return g.writeIf(n, depth)
	}
	return fmt.Errorf("native: unsupported statement %T", s)
}

func (g *generator) writeIf(n *ast.IfExpr, depth int) error {
	indent := strings.Repeat("\t", depth)
	cond, err := g.exprStr(n.Cond)
	if err != nil {
		return err
	}
	fmt.Fprintf(&g.out, "%sif (%s).Truthy() {\n", indent, cond)
	for _, s := range n.Then.Stmts {
		if err := g.writeStmt(s, depth+1); err != nil {
			return err
		}
	}
	g.out.WriteString(indent + "}")
	if n.Else != nil {
		switch e := n.Else.(type) {
		case *ast.BlockStmt:
			g.out.WriteString(" else {\n")
			for _, s := range e.Stmts {
				if err := g.writeStmt(s, depth+1); err != nil {
					return err
				}
			}
			g.out.WriteString(indent + "}")
		case *ast.IfExpr:
			g.out.WriteString(" else ")
			if err := g.writeIf(e, depth); err != nil {
				return err
			}
			return nil
		}
	}
	g.out.WriteString("\n")
	return nil
}

func (g *generator) exprStr(e ast.Expr) (string, error) {
	switch n := e.(type) {
	case *ast.IntLiteral:
		return fmt.Sprintf("vInt(%d)", n.Value), nil
	case *ast.FloatLiteral:
		return fmt.Sprintf("vFloat(%v)", n.Value), nil
	case *ast.StringLiteral:
		return interpolatedString(n.Value), nil
	case *ast.BoolLiteral:
		if n.Value {
			return "vBool(true)", nil
		}
		return "vBool(false)", nil
	case *ast.NullLiteral:
		return "vNull()", nil
	case *ast.Identifier:
		switch n.Name {
		case "print", "println":
			return "ppPrint", nil
		}
		return goIdent(n.Name), nil
	case *ast.ListLiteral:
		var parts []string
		for _, el := range n.Elements {
			s, err := g.exprStr(el)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return fmt.Sprintf("vList([]pureValue{%s})", strings.Join(parts, ", ")), nil
	case *ast.UnaryExpr:
		op, err := g.exprStr(n.Operand)
		if err != nil {
			return "", err
		}
		switch n.Op {
		case "-":
			return "sub(vInt(0), " + op + ")", nil
		case "!":
			return "vBool(!(" + op + ").Truthy())", nil
		}
	case *ast.BinaryExpr:
		l, err := g.exprStr(n.Left)
		if err != nil {
			return "", err
		}
		r, err := g.exprStr(n.Right)
		if err != nil {
			return "", err
		}
		switch n.Op {
		case "+":
			return "add(" + l + ", " + r + ")", nil
		case "-":
			return "sub(" + l + ", " + r + ")", nil
		case "*":
			return "mul(" + l + ", " + r + ")", nil
		case "/":
			return "div(" + l + ", " + r + ")", nil
		case "%":
			return "mod(" + l + ", " + r + ")", nil
		case "==":
			return "eq(" + l + ", " + r + ")", nil
		case "!=":
			return "neq(" + l + ", " + r + ")", nil
		case "<":
			return "cmpLT(" + l + ", " + r + ")", nil
		case "<=":
			return "cmpLTE(" + l + ", " + r + ")", nil
		case ">":
			return "cmpGT(" + l + ", " + r + ")", nil
		case ">=":
			return "cmpGTE(" + l + ", " + r + ")", nil
		case "&&":
			return "vBool((" + l + ").Truthy() && (" + r + ").Truthy())", nil
		case "||":
			return "vBool((" + l + ").Truthy() || (" + r + ").Truthy())", nil
		case "?:":
			return "elvis(" + l + ", " + r + ")", nil
		}
	case *ast.CallExpr:
		callee, err := g.exprStr(n.Callee)
		if err != nil {
			return "", err
		}
		var parts []string
		for _, a := range n.Args {
			s, err := g.exprStr(a)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return callee + "(" + strings.Join(parts, ", ") + ")", nil
	case *ast.MemberExpr:
		t, err := g.exprStr(n.Target)
		if err != nil {
			return "", err
		}
		if n.Property == "length" || n.Property == "size" {
			return "memberLength(" + t + ")", nil
		}
		return "", fmt.Errorf("native: unsupported member %q", n.Property)
	case *ast.IndexExpr:
		t, err := g.exprStr(n.Target)
		if err != nil {
			return "", err
		}
		i, err := g.exprStr(n.Index)
		if err != nil {
			return "", err
		}
		return "index(" + t + ", " + i + ")", nil
	case *ast.IfExpr:
		// Translate if-expr to an immediately-invoked closure.
		var b strings.Builder
		b.WriteString("func() pureValue {\n")
		cond, err := g.exprStr(n.Cond)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\tif (%s).Truthy() {\n", cond)
		thenExpr, err := g.lastExprOfBlock(n.Then)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\t\treturn %s\n\t}\n", thenExpr)
		if n.Else != nil {
			switch e := n.Else.(type) {
			case *ast.BlockStmt:
				elseExpr, err := g.lastExprOfBlock(e)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "\treturn %s\n", elseExpr)
			case *ast.IfExpr:
				elseExpr, err := g.exprStr(e)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "\treturn %s\n", elseExpr)
			}
		} else {
			b.WriteString("\treturn vNull()\n")
		}
		b.WriteString("}()")
		return b.String(), nil
	}
	return "", fmt.Errorf("native: unsupported expression %T", e)
}

func (g *generator) lastExprOfBlock(b *ast.BlockStmt) (string, error) {
	if len(b.Stmts) == 0 {
		return "vNull()", nil
	}
	last := b.Stmts[len(b.Stmts)-1]
	if es, ok := last.(*ast.ExpressionStmt); ok {
		return g.exprStr(es.Expr)
	}
	return "", fmt.Errorf("native: complex blocks in if-expression are not supported yet")
}

// interpolatedString translates a PureLang string literal value (already
// unescaped) into a Go expression that evaluates to a pureValue string.
// Supports $ident and ${expr}.
func interpolatedString(s string) string {
	if !strings.ContainsRune(s, '$') {
		return "vString(" + strconv.Quote(s) + ")"
	}
	var parts []string
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() > 0 {
			parts = append(parts, "vString("+strconv.Quote(lit.String())+")")
			lit.Reset()
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '$' {
			lit.WriteRune(r)
			continue
		}
		if i+1 >= len(runes) {
			lit.WriteRune('$')
			continue
		}
		next := runes[i+1]
		if next == '{' {
			end := i + 2
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
				lit.WriteString(string(runes[i:]))
				break
			}
			expr := string(runes[i+2 : end])
			flushLit()
			// expr is parsed as identifier-only here for safety; fall back
			// to identifier substitution.
			parts = append(parts, fmt.Sprintf("vString((%s).String())", goExprForInterp(expr)))
			i = end
			continue
		}
		if isIdentStart(next) {
			end := i + 1
			for end < len(runes) && isIdentCont(runes[end]) {
				end++
			}
			name := string(runes[i+1 : end])
			flushLit()
			parts = append(parts, fmt.Sprintf("vString((%s).String())", goIdent(name)))
			i = end - 1
			continue
		}
		lit.WriteRune('$')
	}
	flushLit()
	if len(parts) == 0 {
		return "vString(\"\")"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	expr := parts[0]
	for _, p := range parts[1:] {
		expr = "add(" + expr + ", " + p + ")"
	}
	return expr
}

func isIdentStart(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}
func isIdentCont(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}

// goExprForInterp returns a Go expression representing a PureLang
// interpolation expression. For MVP we restrict ${...} to simple paths
// (a, a.b, a.b.c). Complex expressions fall back to a quoted error string.
func goExprForInterp(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return "vString(\"\")"
	}
	// only allow a chain of ident.ident.ident; anything else produces a
	// runtime placeholder. Member access on data instances isn't yet
	// supported in native, so chains > 1 will probably fail to compile.
	if !validIdentChain(src) {
		return "vString(" + strconv.Quote("${"+src+"}") + ")"
	}
	parts := strings.Split(src, ".")
	expr := goIdent(parts[0])
	for _, m := range parts[1:] {
		if m == "length" || m == "size" {
			expr = "memberLength(" + expr + ")"
		} else {
			// no support for arbitrary fields yet
			expr = "vString(" + strconv.Quote("."+m) + ")"
		}
	}
	return expr
}

func validIdentChain(s string) bool {
	parts := strings.Split(s, ".")
	for _, p := range parts {
		if p == "" {
			return false
		}
		for i, r := range p {
			if i == 0 {
				if !isIdentStart(r) {
					return false
				}
			} else if !isIdentCont(r) {
				return false
			}
		}
	}
	return true
}

// goIdent maps a PureLang identifier to a safe Go identifier. PureLang
// identifiers are already a subset of valid Go identifiers, but we prefix
// to avoid collisions with Go keywords.
func goIdent(name string) string {
	switch name {
	case "type", "func", "package", "import", "interface", "struct", "map",
		"chan", "select", "default", "switch", "case", "for", "range",
		"return", "go", "defer", "break", "continue", "goto", "var",
		"const", "true", "false", "nil":
		return "p_" + name
	}
	return "p_" + name
}
