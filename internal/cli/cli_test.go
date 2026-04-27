package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// chdir helper
func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	fn()
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	rc := Run(args, &out, &errBuf)
	return rc, out.String(), errBuf.String()
}

func examplesDir(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	abs, err := filepath.Abs(filepath.Join(wd, "..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func projectsDir(t *testing.T, name string) string {
	t.Helper()
	wd, _ := os.Getwd()
	abs, err := filepath.Abs(filepath.Join(wd, "..", "..", name))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestRunHelloExample(t *testing.T) {
	rc, out, errStr := runCLI(t, "run", filepath.Join(examplesDir(t), "hello.pure"))
	if rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errStr)
	}
	if strings.TrimSpace(out) != "Hello, PureLang" {
		t.Errorf("got %q", out)
	}
}

func TestRunUserExample(t *testing.T) {
	rc, out, errStr := runCLI(t, "run", filepath.Join(examplesDir(t), "user.pure"))
	if rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errStr)
	}
	want := "Alex\n21\ntrue\nHello, Alex\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestRunFunctionsExample(t *testing.T) {
	rc, out, errStr := runCLI(t, "run", filepath.Join(examplesDir(t), "functions.pure"))
	if rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errStr)
	}
	if out != "5\n16\n" {
		t.Errorf("got %q", out)
	}
}

func TestRunExampleProject(t *testing.T) {
	rc, out, errStr := runCLI(t, "run", projectsDir(t, "examples_project"))
	if rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errStr)
	}
	want := "Alex\ntrue\nHello, Alex\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestNewProjectAndRun(t *testing.T) {
	tmp := t.TempDir()
	withCwd(t, tmp, func() {
		rc, _, errStr := runCLI(t, "new", "demo")
		if rc != 0 {
			t.Fatalf("new failed: %s", errStr)
		}
		demo := filepath.Join(tmp, "demo")
		withCwd(t, demo, func() {
			rc, out, errStr := runCLI(t, "run")
			if rc != 0 {
				t.Fatalf("run failed: %s", errStr)
			}
			if strings.TrimSpace(out) != "Hello, PureLang" {
				t.Errorf("got %q", out)
			}
		})
	})
}

func TestVersion(t *testing.T) {
	rc, out, _ := runCLI(t, "version")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.HasPrefix(out, "PureLang ") {
		t.Errorf("got %q", out)
	}
}

func TestCheckSingleFile(t *testing.T) {
	rc, _, errStr := runCLI(t, "check", filepath.Join(examplesDir(t), "hello.pure"))
	if rc != 0 {
		t.Fatalf("check failed: %s", errStr)
	}
}

func TestProjectWithLocalDependency(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git missing")
	}
	// Set up local repo to be cloned.
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "pure-math")
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "pure.toml"), []byte(`name = "math"
version = "0.1.0"
entry = "src/math.pure"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "math.pure"), []byte("square(x: Int) => x * x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"add", "."},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// Create app project.
	app := filepath.Join(tmp, "app")
	if err := os.MkdirAll(filepath.Join(app, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `name = "app"
version = "0.1.0"
entry = "src/main.pure"

[dependencies]
math = { git = "` + repo + `", branch = "main" }
`
	if err := os.WriteFile(filepath.Join(app, "pure.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "src", "main.pure"), []byte(`use std.io
use math

print square(5)
`), 0o644); err != nil {
		t.Fatal(err)
	}
	withCwd(t, app, func() {
		rc, _, errStr := runCLI(t, "deps")
		if rc != 0 {
			t.Fatalf("deps failed: %s", errStr)
		}
		rc, out, errStr := runCLI(t, "run")
		if rc != 0 {
			t.Fatalf("run failed: %s", errStr)
		}
		if strings.TrimSpace(out) != "25" {
			t.Errorf("got %q", out)
		}
	})
}
