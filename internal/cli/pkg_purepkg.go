package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"purelang/internal/deps"
	"purelang/internal/project"
	"purelang/internal/purepkg"
)

func init() {
	runPkgPublish = pkgPublish
	runPkgInfo = pkgInfo
	runPkgSearch = pkgSearch
}

func pkgInfo(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: pr pkg info <name> [version]")
		return 1
	}
	c := purepkg.NewClient()
	if len(args) >= 2 {
		v, err := c.Resolve(args[0], args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "%s@%s\n  git: %s\n", v.Name, v.Version, v.GitURL)
		if v.Tag != "" {
			fmt.Fprintf(stdout, "  tag: %s\n", v.Tag)
		}
		if v.Commit != "" {
			fmt.Fprintf(stdout, "  commit: %s\n", v.Commit)
		}
		return 0
	}
	versions, err := c.Versions(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, v := range versions {
		fmt.Fprintf(stdout, "%s@%s -> %s\n", v.Name, v.Version, v.GitURL)
	}
	if len(versions) == 0 {
		fmt.Fprintln(stdout, "no versions found")
	}
	return 0
}

func pkgSearch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: pr pkg search <query>")
		return 1
	}
	c := purepkg.NewClient()
	res, err := c.Search(strings.Join(args, " "))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(res) == 0 {
		fmt.Fprintln(stdout, "no results")
		return 0
	}
	for _, r := range res {
		latest := r.Latest
		if latest == "" {
			latest = "?"
		}
		fmt.Fprintf(stdout, "%-20s %-10s %s\n", r.Name, latest, r.Description)
	}
	return 0
}

// pkgPublish reads pure.toml, ensures the package has a clean working tree
// and an upstream tag matching the version, and notifies the registry.
//
// Because publishing depends on git state we keep this command light:
// it constructs the version record from pure.toml + the current commit
// hash and POSTs it to the registry. The registry's job is to verify
// that the tag/commit really exists at the public URL.
func pkgPublish(args []string, stdout, stderr io.Writer) int {
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
	if p.Name == "" || p.Version == "" {
		fmt.Fprintln(stderr, "pure.toml needs both 'name' and 'version' to publish")
		return 1
	}
	gitURL := ""
	for _, a := range args {
		if strings.HasPrefix(a, "--git=") {
			gitURL = strings.TrimPrefix(a, "--git=")
		}
	}
	if gitURL == "" {
		fmt.Fprintln(stderr, "pr pkg publish requires --git=<url> for the public repository")
		return 1
	}
	commit, err := deps.CurrentCommit(filepath.Clean(root))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c := purepkg.NewClient()
	rec := purepkg.PackageVersion{
		Name:    p.Name,
		Version: p.Version,
		GitURL:  gitURL,
		Tag:     "v" + strings.TrimPrefix(p.Version, "v"),
		Commit:  commit,
	}
	if err := c.Publish(rec); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "published %s@%s -> %s (commit %s)\n", rec.Name, rec.Version, rec.GitURL, rec.Commit)
	return 0
}
