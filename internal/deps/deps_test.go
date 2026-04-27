package deps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"purelang/internal/project"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, string(out))
	}
}

func makeFakeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo := filepath.Join(dir, "pure-math")
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `name = "math"
version = "0.1.0"
entry = "src/math.pure"
`
	if err := os.WriteFile(filepath.Join(repo, "pure.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `square(x: Int) => x * x
cube(x: Int) => x * x * x
`
	if err := os.WriteFile(filepath.Join(repo, "src", "math.pure"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")
	return repo
}

func makeProject(t *testing.T, deps map[string]project.Dependency) *project.Project {
	t.Helper()
	dir := t.TempDir()
	if err := project.CreateProject(dir, "app"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "app")
	parsed := &project.TOMLData{
		Name:         "app",
		Version:      "0.1.0",
		Entry:        "src/main.pure",
		Dependencies: deps,
	}
	if err := os.WriteFile(filepath.Join(root, "pure.toml"), []byte(project.EncodeTOML(parsed)), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := project.LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseDependencyDeclarations(t *testing.T) {
	src := `name = "x"
[dependencies]
math = { git = "https://example.com/m.git", version = "v1" }
`
	d, err := project.ParseTOML(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Dependencies["math"].Version != "v1" {
		t.Errorf("got %+v", d.Dependencies["math"])
	}
}

func TestInstallFromLocalGit(t *testing.T) {
	if !GitInstalled() {
		t.Skip("git not installed")
	}
	repo := makeFakeRepo(t)
	p := makeProject(t, map[string]project.Dependency{
		"math": {Name: "math", Git: repo, Branch: "main"},
	})
	if err := Install(p); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.DepsDir(), "math", "pure.toml")); err != nil {
		t.Errorf("dep not installed: %v", err)
	}
	locks, err := LoadLock(p.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 1 {
		t.Fatalf("expected 1 lock entry, got %d", len(locks))
	}
	if locks[0].Name != "math" || locks[0].Source != "branch" || locks[0].Resolved == "" {
		t.Errorf("lock: %+v", locks[0])
	}
}

func TestWriteAndReadLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pure.lock")
	want := []LockedDependency{
		{Name: "math", Git: "https://example.com/m.git", Source: "version", Requested: "v0.1.0", Resolved: "abc123"},
	}
	if err := WriteLock(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "math" || got[0].Resolved != "abc123" {
		t.Errorf("got %+v", got)
	}
}

func TestAddDependency(t *testing.T) {
	if !GitInstalled() {
		t.Skip("git not installed")
	}
	repo := makeFakeRepo(t)
	dir := t.TempDir()
	if err := project.CreateProject(dir, "app"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "app")
	if err := Add(root, "math", repo); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "pure.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `math = { git`) {
		t.Errorf("toml does not contain math: %s", string(raw))
	}
	if _, err := os.Stat(filepath.Join(root, ".pure", "deps", "math", "pure.toml")); err != nil {
		t.Errorf("expected installed dep: %v", err)
	}
}

func TestClean(t *testing.T) {
	if !GitInstalled() {
		t.Skip("git not installed")
	}
	repo := makeFakeRepo(t)
	p := makeProject(t, map[string]project.Dependency{
		"math": {Name: "math", Git: repo, Branch: "main"},
	})
	if err := Install(p); err != nil {
		t.Fatal(err)
	}
	if err := Clean(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.DepsDir()); !os.IsNotExist(err) {
		t.Errorf("deps dir still exists: %v", err)
	}
}

func TestValidateDep(t *testing.T) {
	if err := validateDep(project.Dependency{Name: "x"}); err == nil {
		t.Error("expected error for missing git")
	}
	if err := validateDep(project.Dependency{Name: "x", Git: "g", Version: "v1", Branch: "main"}); err == nil {
		t.Error("expected error for multiple sources")
	}
}
