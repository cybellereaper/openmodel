package deps

import (
	"fmt"
	"os"
	"strings"
)

// LockedDependency is a record in pure.lock.
type LockedDependency struct {
	Name      string
	Git       string
	Source    string // "version" | "branch" | "commit"
	Requested string
	Resolved  string
}

// LoadLock reads a pure.lock file.
// Format:
//
//	[[dependency]]
//	name = "math"
//	git = "..."
//	source = "version"
//	requested = "v0.1.0"
//	resolved = "<sha>"
func LoadLock(path string) ([]LockedDependency, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var locks []LockedDependency
	var cur *LockedDependency
	lines := strings.Split(string(data), "\n")
	flush := func() {
		if cur != nil {
			locks = append(locks, *cur)
		}
	}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[[dependency]]" {
			flush()
			cur = &LockedDependency{}
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("pure.lock line %d: unexpected key outside [[dependency]]", i+1)
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			return nil, fmt.Errorf("pure.lock line %d: expected key = value", i+1)
		}
		key := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		if !(strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`)) {
			return nil, fmt.Errorf("pure.lock line %d: expected string value", i+1)
		}
		val := v[1 : len(v)-1]
		switch key {
		case "name":
			cur.Name = val
		case "git":
			cur.Git = val
		case "source":
			cur.Source = val
		case "requested":
			cur.Requested = val
		case "resolved":
			cur.Resolved = val
		}
	}
	flush()
	return locks, nil
}

// WriteLock writes pure.lock atomically.
func WriteLock(path string, deps []LockedDependency) error {
	var sb strings.Builder
	for _, d := range deps {
		sb.WriteString("[[dependency]]\n")
		fmt.Fprintf(&sb, "name = %q\n", d.Name)
		fmt.Fprintf(&sb, "git = %q\n", d.Git)
		fmt.Fprintf(&sb, "source = %q\n", d.Source)
		fmt.Fprintf(&sb, "requested = %q\n", d.Requested)
		fmt.Fprintf(&sb, "resolved = %q\n", d.Resolved)
		sb.WriteString("\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}
