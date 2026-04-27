package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"purelang/internal/ast"
	"purelang/internal/bytecode"
	"purelang/internal/checker"
	"purelang/internal/deps"
	"purelang/internal/fmtter"
	"purelang/internal/lsp"
	"purelang/internal/modules"
	"purelang/internal/native"
	"purelang/internal/parser"
	"purelang/internal/project"
	"purelang/internal/runtime"
	"purelang/internal/std"
	"purelang/internal/vm"
)

const Version = "0.1.0"

// Run dispatches CLI args. Returns process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage())
		return 1
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "PureLang %s\n", Version)
		return 0
	case "help", "--help", "-h":
		fmt.Fprintln(stdout, usage())
		return 0
	case "new":
		return cmdNew(rest, stdout, stderr)
	case "run":
		return cmdRun(rest, stdout, stderr)
	case "check":
		return cmdCheck(rest, stdout, stderr)
	case "build":
		return cmdBuild(rest, stdout, stderr)
	case "fmt":
		return cmdFmt(rest, stdout, stderr)
	case "test":
		return cmdTest(rest, stdout, stderr)
	case "deps":
		return cmdDeps(rest, stdout, stderr)
	case "lsp":
		return cmdLSP(rest, stdout, stderr)
	case "pkg", "publish":
		return cmdPkg(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s\n", cmd, usage())
		return 1
	}
}

func usage() string {
	return strings.TrimSpace(`
PureLang ` + Version + `

USAGE:
    pr <command> [arguments]

COMMANDS:
    version                   Print PureLang version
    new <name>                Create a new project
    run [path]                Run a file or project
    check [path]              Type-check a file or project
    build                     Build the current project
    test                      Run *_test.pure files in the current project
    fmt [path]                Format source files (not implemented)
    deps                      Install project dependencies
    deps add <name> <url>     Add a GitHub dependency
    deps update               Update branch dependencies
    deps clean                Remove .pure/deps
`)
}

func cmdNew(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: pr new <name>")
		return 1
	}
	name := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := project.CreateProject(cwd, name); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "created project %s\n", name)
	return 0
}

func cmdRun(args []string, stdout, stderr io.Writer) int {
	useVM := false
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "--vm":
			useVM = true
			args = args[1:]
		case "--tree":
			useVM = false
			args = args[1:]
		default:
			fmt.Fprintf(stderr, "unknown flag %q\n", args[0])
			return 1
		}
	}
	if useVM {
		if len(args) == 0 {
			fmt.Fprintln(stderr, "pr run --vm <file.pure>")
			return 1
		}
		return runFileVM(args[0], stdout, stderr)
	}
	if len(args) == 0 {
		// Run the current project.
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		root, err := project.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return runProject(root, stdout, stderr)
	}
	target := args[0]
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if info.IsDir() {
		root, err := filepath.Abs(target)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if _, err := os.Stat(filepath.Join(root, "pure.toml")); err != nil {
			fmt.Fprintf(stderr, "error: no pure.toml in %s\n", root)
			return 1
		}
		return runProject(root, stdout, stderr)
	}
	return runFile(target, stdout, stderr)
}

func runFile(path string, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prog, err := parser.Parse(string(data))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return 1
	}
	interp := runtime.NewWithWriter(stdout)
	// Single-file mode: load any std imports the file requested.
	for _, stmt := range prog.Stmts {
		if u, ok := stmt.(*ast.UseDecl); ok && std.IsStdPath(u.Path) {
			if !std.IsStdModule(u.Path) {
				fmt.Fprintf(stderr, "%s: unknown std module %q\n", path, strings.Join(u.Path, "."))
				return 1
			}
			interp.LoadStdModuleInto(u.Path[1], interp.Globals())
		}
	}
	if err := interp.Run(prog); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return 1
	}
	return 0
}

func runFileVM(path string, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prog, err := parser.Parse(string(data))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return 1
	}
	chunk, err := bytecode.CompileProgram(prog)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return 1
	}
	v := vm.NewWithWriter(stdout)
	if err := v.Run(chunk); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return 1
	}
	return 0
}

func runProject(root string, stdout, stderr io.Writer) int {
	p, err := project.LoadProject(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := ensureDeps(p, stderr); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	graph, entryName, err := buildModuleGraph(p)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if entryName == "" {
		fmt.Fprintf(stderr, "error: entry file %s not found\n", p.Entry)
		return 1
	}
	interp := runtime.NewWithWriter(stdout)
	// Register declarations into per-module scopes.
	for _, mod := range graph.Modules {
		scope := interp.ModuleScope(mod.Name)
		interp.RegisterDeclarationsInto(scope, mod.Program)
	}
	// Resolve imports for every module.
	for _, mod := range graph.Modules {
		dst := interp.ModuleScope(mod.Name)
		for _, imp := range mod.Imports {
			parts := strings.Split(imp, ".")
			if std.IsStdPath(parts) {
				if !std.IsStdModule(parts) {
					fmt.Fprintf(stderr, "error: %s: unknown std module %q\n", mod.Path, imp)
					return 1
				}
				interp.LoadStdModuleInto(parts[1], dst)
				continue
			}
			src, ok := graph.Modules[imp]
			if !ok {
				fmt.Fprintf(stderr, "error: %s: cannot resolve module %q\n", mod.Path, imp)
				return 1
			}
			interp.ImportInto(dst, interp.ModuleScope(src.Name))
		}
	}
	// Execute the entry module top-level statements.
	entry := graph.Modules[entryName]
	if err := interp.RunInScope(interp.ModuleScope(entry.Name), entry.Program); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// buildModuleGraph parses every .pure file in the project's src/ and in
// every dependency under .pure/deps/<name>/src/, then computes module names.
func buildModuleGraph(p *project.Project) (*modules.Graph, string, error) {
	g := modules.NewGraph()
	srcFiles, err := project.ListSourceFiles(p)
	if err != nil {
		return nil, "", err
	}
	entryAbs, _ := filepath.Abs(p.EntryPath())
	entryName := ""
	for _, f := range srcFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, "", err
		}
		prog, err := parser.Parse(string(data))
		if err != nil {
			return nil, "", fmt.Errorf("%s: %v", f, err)
		}
		name, err := modules.FileToModuleName(p.Name, p.SourceDir, f)
		if err != nil {
			return nil, "", err
		}
		mod := &modules.Module{
			Name:    name,
			Path:    f,
			Program: prog,
			Imports: modules.CollectImports(prog),
		}
		g.Add(mod)
		if abs, _ := filepath.Abs(f); abs == entryAbs {
			entryName = name
		}
	}
	// Dependencies
	if entries, err := os.ReadDir(p.DepsDir()); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			depRoot := filepath.Join(p.DepsDir(), e.Name())
			depProj, err := project.LoadProject(depRoot)
			if err != nil {
				continue
			}
			depFiles, err := project.ListSourceFiles(depProj)
			if err != nil {
				continue
			}
			for _, f := range depFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					return nil, "", err
				}
				prog, err := parser.Parse(string(data))
				if err != nil {
					return nil, "", fmt.Errorf("%s: %v", f, err)
				}
				name, err := modules.FileToModuleName(depProj.Name, depProj.SourceDir, f)
				if err != nil {
					return nil, "", err
				}
				mod := &modules.Module{
					Name:    name,
					Path:    f,
					Program: prog,
					Imports: modules.CollectImports(prog),
				}
				g.Add(mod)
			}
		}
	}
	return g, entryName, nil
}

type namedProgram struct {
	path string
	prog *ast.Program
}

// loadProjectPrograms parses every .pure file in src/ and in .pure/deps/<dep>/src.
// Returns all parsed programs and the entry program separately.
//
// This is used by the type-checker (which checks all files together) and by
// commands that need a flat view of all parsed sources.
func loadProjectPrograms(p *project.Project) ([]namedProgram, *ast.Program, error) {
	var progs []namedProgram
	var entry *ast.Program
	srcFiles, err := project.ListSourceFiles(p)
	if err != nil {
		return nil, nil, err
	}
	entryAbs := p.EntryPath()
	for _, f := range srcFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		pg, err := parser.Parse(string(data))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", f, err)
		}
		progs = append(progs, namedProgram{path: f, prog: pg})
		fa, _ := filepath.Abs(f)
		ea, _ := filepath.Abs(entryAbs)
		if fa == ea {
			entry = pg
		}
	}
	// dependencies
	depDir := p.DepsDir()
	if entries, err := os.ReadDir(depDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			depRoot := filepath.Join(depDir, e.Name())
			depProj, err := project.LoadProject(depRoot)
			if err != nil {
				continue
			}
			depFiles, err := project.ListSourceFiles(depProj)
			if err != nil {
				continue
			}
			for _, f := range depFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					return nil, nil, err
				}
				pg, err := parser.Parse(string(data))
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %v", f, err)
				}
				progs = append(progs, namedProgram{path: f, prog: pg})
			}
		}
	}
	return progs, entry, nil
}

func ensureDeps(p *project.Project, stderr io.Writer) error {
	if len(p.Dependencies) == 0 {
		return nil
	}
	missing := false
	for name := range p.Dependencies {
		if _, err := os.Stat(filepath.Join(p.DepsDir(), name)); err != nil {
			missing = true
			break
		}
	}
	if !missing {
		return nil
	}
	if !deps.GitInstalled() {
		return fmt.Errorf("missing dependencies and %v", deps.ErrGitMissing)
	}
	fmt.Fprintln(stderr, "installing dependencies...")
	return deps.Install(p)
}

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		root, err := project.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return checkProject(root, stdout, stderr)
	}
	target := args[0]
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if info.IsDir() {
		root, err := filepath.Abs(target)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return checkProject(root, stdout, stderr)
	}
	return checkFile(target, stdout, stderr)
}

func checkFile(path string, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prog, err := parser.Parse(string(data))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return 1
	}
	c := checker.New()
	errs := c.Check(prog)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stderr, "%s: %s\n", path, e)
		}
		return 1
	}
	fmt.Fprintf(stdout, "%s: ok\n", path)
	return 0
}

func checkProject(root string, stdout, stderr io.Writer) int {
	p, err := project.LoadProject(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	progs, _, err := loadProjectPrograms(p)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c := checker.New()
	asts := make([]*ast.Program, 0, len(progs))
	for _, np := range progs {
		asts = append(asts, np.prog)
	}
	errs := c.Check(asts...)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stderr, "error: %s\n", e)
		}
		return 1
	}
	fmt.Fprintf(stdout, "%s: ok\n", p.Name)
	return 0
}

func cmdBuild(args []string, stdout, stderr io.Writer) int {
	nativeMode := false
	outBin := ""
	var srcFile string
	for len(args) > 0 {
		switch args[0] {
		case "--native":
			nativeMode = true
			args = args[1:]
		case "-o":
			if len(args) < 2 {
				fmt.Fprintln(stderr, "-o requires a path")
				return 1
			}
			outBin = args[1]
			args = args[2:]
		default:
			if srcFile != "" {
				fmt.Fprintf(stderr, "unexpected argument %q\n", args[0])
				return 1
			}
			srcFile = args[0]
			args = args[1:]
		}
	}
	if nativeMode {
		if srcFile == "" {
			fmt.Fprintln(stderr, "pr build --native <file.pure> [-o out]")
			return 1
		}
		if outBin == "" {
			outBin = strings.TrimSuffix(filepath.Base(srcFile), ".pure")
			if outBin == "" {
				outBin = "purelang_app"
			}
		}
		absOut, err := filepath.Abs(outBin)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		data, err := os.ReadFile(srcFile)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		prog, err := parser.Parse(string(data))
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", srcFile, err)
			return 1
		}
		if err := native.Build(string(data), prog, absOut); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "built native binary %s\n", absOut)
		return 0
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	root, err := project.FindProjectRoot(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	p, err := project.LoadProject(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if rc := checkProject(root, stdout, stderr); rc != 0 {
		return rc
	}
	buildDir := filepath.Join(root, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	content := fmt.Sprintf("PureLang build artifact for %s\nentry = %s\n", p.Name, p.Entry)
	if err := os.WriteFile(filepath.Join(buildDir, "app.txt"), []byte(content), 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "built %s -> build/app.txt\n", p.Name)
	return 0
}

func cmdPkg(args []string, stdout, stderr io.Writer) int {
	// Forward to the package-registry CLI handler, which lives in pkg.go
	// to keep the imports/dependencies isolated.
	return runPkg(args, stdout, stderr)
}

func cmdLSP(args []string, stdout, stderr io.Writer) int {
	srv := lsp.NewServer(os.Stdin, stdout, stderr)
	if err := srv.Serve(); err != nil {
		fmt.Fprintf(stderr, "lsp: %v\n", err)
		return 1
	}
	return 0
}

func cmdFmt(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		// Format all files in the current project.
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		root, err := project.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		p, err := project.LoadProject(root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		files, err := project.ListSourceFiles(p)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		changed := 0
		for _, f := range files {
			ok, err := formatFile(f)
			if err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", f, err)
				return 1
			}
			if ok {
				fmt.Fprintf(stdout, "formatted %s\n", f)
				changed++
			}
		}
		if changed == 0 {
			fmt.Fprintln(stdout, "no changes needed")
		}
		return 0
	}
	target := args[0]
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if info.IsDir() {
		fmt.Fprintln(stderr, "pr fmt <dir> is not supported; cd into the project first")
		return 1
	}
	ok, err := formatFile(target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if ok {
		fmt.Fprintf(stdout, "formatted %s\n", target)
	} else {
		fmt.Fprintf(stdout, "%s: already formatted\n", target)
	}
	return 0
}

func formatFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	formatted, err := fmtter.Format(string(data))
	if err != nil {
		return false, err
	}
	if formatted == string(data) {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(formatted), 0o644)
}

func cmdTest(args []string, stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	root, err := project.FindProjectRoot(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	p, err := project.LoadProject(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	files, err := project.ListSourceFiles(p)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "running tests...")
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.pure") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		prog, err := parser.Parse(string(data))
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", f, err)
			return 1
		}
		interp := runtime.NewWithWriter(stdout)
		if err := interp.Run(prog); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", f, err)
			return 1
		}
	}
	fmt.Fprintln(stdout, "ok")
	return 0
}

func cmdDeps(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		cwd, _ := os.Getwd()
		root, err := project.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		p, err := project.LoadProject(root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := deps.Install(p); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "dependencies installed")
		return 0
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "usage: pr deps add <name> <github-url>\n       pr deps add --pkg <name>[@version]")
			return 1
		}
		cwd, _ := os.Getwd()
		root, err := project.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if args[1] == "--pkg" {
			spec := args[2]
			pkgName := spec
			version := ""
			if at := strings.Index(spec, "@"); at >= 0 {
				pkgName = spec[:at]
				version = spec[at+1:]
			}
			if err := deps.AddRegistry(root, pkgName, pkgName, version); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			fmt.Fprintf(stdout, "added registry dependency %s\n", pkgName)
			return 0
		}
		if err := deps.Add(root, args[1], args[2]); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "added dependency %s\n", args[1])
		return 0
	case "update":
		cwd, _ := os.Getwd()
		root, err := project.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		p, err := project.LoadProject(root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := deps.Update(p); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "dependencies updated")
		return 0
	case "clean":
		cwd, _ := os.Getwd()
		root, err := project.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		p, err := project.LoadProject(root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := deps.Clean(p); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "removed .pure/deps")
		return 0
	}
	fmt.Fprintf(stderr, "unknown deps subcommand %q\n", args[0])
	return 1
}
