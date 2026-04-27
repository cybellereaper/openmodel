package fmtter

import (
	"strings"
	"testing"
)

func TestFormatIdempotent(t *testing.T) {
	src := `use std.io

User(name: String, age: Int) {
    adult => age >= 18
}
x = User("Alex", 21)
print(x.name)
`
	out, err := Format(src)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := Format(out)
	if err != nil {
		t.Fatal(err)
	}
	if out != out2 {
		t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, out2)
	}
}

func TestFormatNormalizes(t *testing.T) {
	src := "use std.io\n\nx=1+2\nprint(x)\n"
	out, err := Format(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "x = 1 + 2") {
		t.Errorf("normalization failed: %s", out)
	}
}

func TestFormatWhen(t *testing.T) {
	src := `result = when x { 1 => "a"
2,3 => "b"
else => "c" }`
	out, err := Format(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "when x {") {
		t.Errorf("got %s", out)
	}
}
