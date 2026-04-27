package native

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"purelang/internal/parser"
)

func TestNativeGenerateAndRun(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	src := `add(a: Int, b: Int) => a + b

print add(2, 3)
name = "Alex"
print "hi $name"
`
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	out := filepath.Join(tmp, "app")
	if err := Build(src, prog, out); err != nil {
		t.Fatalf("Build: %v", err)
	}
	cmd := exec.Command(out)
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, got)
	}
	want := "5"
	if !strings.Contains(string(got), want) {
		t.Errorf("output %q does not contain %q", got, want)
	}
}
