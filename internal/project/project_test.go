package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTOMLBasic(t *testing.T) {
	src := `name = "my_app"
version = "0.1.0"
entry = "src/main.pure"

[dependencies]
http = "0.1.0"
`
	d, err := ParseTOML(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "my_app" || d.Version != "0.1.0" || d.Entry != "src/main.pure" {
		t.Errorf("got %+v", d)
	}
	if dep, ok := d.Dependencies["http"]; !ok || dep.Version != "0.1.0" {
		t.Errorf("http dep: %+v", dep)
	}
}

func TestParseTOMLInlineDep(t *testing.T) {
	src := `name = "x"
[dependencies]
math = { git = "https://example.com/pure-math.git", version = "v0.1.0" }
utils = { git = "https://example.com/pure-utils.git", branch = "main" }
core = { git = "https://example.com/pure-core.git", commit = "abc123" }
`
	d, err := ParseTOML(src)
	if err != nil {
		t.Fatal(err)
	}
	math := d.Dependencies["math"]
	if math.Git == "" || math.Version != "v0.1.0" {
		t.Errorf("math: %+v", math)
	}
	utils := d.Dependencies["utils"]
	if utils.Branch != "main" {
		t.Errorf("utils: %+v", utils)
	}
	core := d.Dependencies["core"]
	if core.Commit != "abc123" {
		t.Errorf("core: %+v", core)
	}
}

func TestParseTOMLInvalid(t *testing.T) {
	_, err := ParseTOML(`name`)
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("expected line-numbered error, got %v", err)
	}
}

func TestCreateAndLoadProject(t *testing.T) {
	dir := t.TempDir()
	if err := CreateProject(dir, "demo"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "demo")
	if _, err := os.Stat(filepath.Join(root, "pure.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "main.pure")); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "demo" {
		t.Errorf("name=%s", p.Name)
	}
	if p.Entry != "src/main.pure" {
		t.Errorf("entry=%s", p.Entry)
	}
}

func TestFindProjectRoot(t *testing.T) {
	dir := t.TempDir()
	if err := CreateProject(dir, "app"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "app")
	nested := filepath.Join(root, "src", "deep", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindProjectRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	gotAbs, _ := filepath.Abs(got)
	wantAbs, _ := filepath.Abs(root)
	if gotAbs != wantAbs {
		t.Errorf("got %q want %q", gotAbs, wantAbs)
	}
}

func TestListSourceFiles(t *testing.T) {
	dir := t.TempDir()
	if err := CreateProject(dir, "app"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "app")
	if err := os.MkdirAll(filepath.Join(root, "src", "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "models", "user.pure"), []byte("User(name: String)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := ListSourceFiles(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestEncodeTOML(t *testing.T) {
	d := &TOMLData{
		Name:    "app",
		Version: "0.1.0",
		Entry:   "src/main.pure",
		Dependencies: map[string]Dependency{
			"math": {Name: "math", Git: "https://example.com/pure-math.git", Branch: "main"},
		},
	}
	s := EncodeTOML(d)
	if !strings.Contains(s, `name = "app"`) || !strings.Contains(s, "[dependencies]") {
		t.Errorf("got %s", s)
	}
}
