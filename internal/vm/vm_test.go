package vm

import (
	"bytes"
	"strings"
	"testing"

	"purelang/internal/bytecode"
	"purelang/internal/parser"
)

func runVM(t *testing.T, src string) string {
	t.Helper()
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	chunk, err := bytecode.CompileProgram(prog)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var buf bytes.Buffer
	v := NewWithWriter(&buf)
	if err := v.Run(chunk); err != nil {
		t.Fatalf("run: %v", err)
	}
	return buf.String()
}

func TestVMArithmetic(t *testing.T) {
	got := runVM(t, "print 1 + 2 * 3")
	if strings.TrimSpace(got) != "7" {
		t.Errorf("got %q", got)
	}
}

func TestVMFunction(t *testing.T) {
	got := runVM(t, `add(a: Int, b: Int) => a + b
print add(10, 20)`)
	if strings.TrimSpace(got) != "30" {
		t.Errorf("got %q", got)
	}
}

func TestVMIfExpr(t *testing.T) {
	got := runVM(t, `x = 10
print if x > 5 { "big" } else { "small" }`)
	if strings.TrimSpace(got) != "big" {
		t.Errorf("got %q", got)
	}
}

func TestVMElvis(t *testing.T) {
	got := runVM(t, `x = null
print x ?: "fallback"`)
	if strings.TrimSpace(got) != "fallback" {
		t.Errorf("got %q", got)
	}
}

func TestVMList(t *testing.T) {
	got := runVM(t, `xs = [1, 2, 3]
print xs[2]
print xs.length`)
	if strings.TrimSpace(got) != "3\n3" {
		t.Errorf("got %q", got)
	}
}
