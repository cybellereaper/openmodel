package project

import (
	"fmt"
	"sort"
	"strings"
)

// Dependency describes a PureLang dependency in pure.toml.
//
// A dependency is either:
//   - Git-backed: Git is set; one of Version, Branch, Commit is set.
//   - Registry-backed (purepkg): Pkg is set with a package name; Version
//     gives the desired version. The dependency manager resolves Pkg to
//     a git URL and ref by talking to the registry.
type Dependency struct {
	Name    string
	Git     string
	Pkg     string
	Version string
	Branch  string
	Commit  string
}

// Source returns "version", "branch", "commit", or "" if none specified.
func (d Dependency) Source() string {
	if d.Version != "" {
		return "version"
	}
	if d.Branch != "" {
		return "branch"
	}
	if d.Commit != "" {
		return "commit"
	}
	return ""
}

// Requested returns the requested ref.
func (d Dependency) Requested() string {
	switch d.Source() {
	case "version":
		return d.Version
	case "branch":
		return d.Branch
	case "commit":
		return d.Commit
	}
	return ""
}

// TOMLData is the parsed result of pure.toml.
type TOMLData struct {
	Name         string
	Version      string
	Entry        string
	Dependencies map[string]Dependency
}

// ParseTOML parses a small subset of TOML used for pure.toml.
func ParseTOML(input string) (*TOMLData, error) {
	out := &TOMLData{Dependencies: map[string]Dependency{}}
	section := ""
	lines := strings.Split(input, "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		// strip trailing comment after #, but not inside strings.
		line = stripTrailingComment(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, fmt.Errorf("line %d: unterminated section header", lineNo)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			return nil, fmt.Errorf("line %d: expected key = value, got %q", lineNo, line)
		}
		key := strings.TrimSpace(line[:eq])
		valStr := strings.TrimSpace(line[eq+1:])
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNo)
		}
		switch section {
		case "":
			s, err := parseStringValue(valStr, lineNo)
			if err != nil {
				return nil, err
			}
			switch key {
			case "name":
				out.Name = s
			case "version":
				out.Version = s
			case "entry":
				out.Entry = s
			default:
				// allow other top-level scalars but ignore
			}
		case "dependencies":
			dep, err := parseDependencyValue(key, valStr, lineNo)
			if err != nil {
				return nil, err
			}
			out.Dependencies[key] = dep
		default:
			// unknown section, ignore for MVP
		}
	}
	return out, nil
}

func stripTrailingComment(s string) string {
	inStr := false
	var prev rune
	for i, r := range s {
		if r == '"' && prev != '\\' {
			inStr = !inStr
		}
		if r == '#' && !inStr {
			return s[:i]
		}
		prev = r
	}
	return s
}

func parseStringValue(v string, lineNo int) (string, error) {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return unescape(v[1 : len(v)-1]), nil
	}
	return "", fmt.Errorf("line %d: expected string value, got %q", lineNo, v)
}

func unescape(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			n := s[i+1]
			switch n {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteByte(c)
				sb.WriteByte(n)
			}
			i++
			continue
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

func parseDependencyValue(name, v string, lineNo int) (Dependency, error) {
	dep := Dependency{Name: name}
	v = strings.TrimSpace(v)
	if len(v) == 0 {
		return dep, fmt.Errorf("line %d: empty dependency value", lineNo)
	}
	if v[0] == '"' {
		ver, err := parseStringValue(v, lineNo)
		if err != nil {
			return dep, err
		}
		dep.Version = ver
		return dep, nil
	}
	if v[0] != '{' || v[len(v)-1] != '}' {
		return dep, fmt.Errorf("line %d: dependency %q must be a string or inline table", lineNo, name)
	}
	body := v[1 : len(v)-1]
	pairs, err := splitInlineTable(body, lineNo)
	if err != nil {
		return dep, err
	}
	for _, p := range pairs {
		eq := strings.Index(p, "=")
		if eq < 0 {
			return dep, fmt.Errorf("line %d: expected key = value in dependency", lineNo)
		}
		key := strings.TrimSpace(p[:eq])
		val := strings.TrimSpace(p[eq+1:])
		s, err := parseStringValue(val, lineNo)
		if err != nil {
			return dep, fmt.Errorf("line %d: dependency %q: %v", lineNo, name, err)
		}
		switch key {
		case "git":
			dep.Git = s
		case "pkg":
			dep.Pkg = s
		case "version":
			dep.Version = s
		case "branch":
			dep.Branch = s
		case "commit":
			dep.Commit = s
		default:
			return dep, fmt.Errorf("line %d: unknown dependency field %q", lineNo, key)
		}
	}
	return dep, nil
}

func splitInlineTable(body string, lineNo int) ([]string, error) {
	var out []string
	var cur strings.Builder
	inStr := false
	var prev rune
	for _, r := range body {
		if r == '"' && prev != '\\' {
			inStr = !inStr
			cur.WriteRune(r)
			prev = r
			continue
		}
		if r == ',' && !inStr {
			s := strings.TrimSpace(cur.String())
			if s != "" {
				out = append(out, s)
			}
			cur.Reset()
			prev = r
			continue
		}
		cur.WriteRune(r)
		prev = r
	}
	if inStr {
		return nil, fmt.Errorf("line %d: unterminated string in dependency", lineNo)
	}
	last := strings.TrimSpace(cur.String())
	if last != "" {
		out = append(out, last)
	}
	return out, nil
}

// EncodeTOML re-encodes a TOMLData back into pure.toml content.
// This is used to add dependencies while preserving a deterministic format.
func EncodeTOML(data *TOMLData) string {
	var sb strings.Builder
	if data.Name != "" {
		fmt.Fprintf(&sb, "name = %q\n", data.Name)
	}
	if data.Version != "" {
		fmt.Fprintf(&sb, "version = %q\n", data.Version)
	}
	if data.Entry != "" {
		fmt.Fprintf(&sb, "entry = %q\n", data.Entry)
	}
	sb.WriteString("\n[dependencies]\n")
	names := make([]string, 0, len(data.Dependencies))
	for n := range data.Dependencies {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		dep := data.Dependencies[n]
		var parts []string
		if dep.Git != "" {
			parts = append(parts, fmt.Sprintf("git = %q", dep.Git))
		}
		if dep.Pkg != "" {
			parts = append(parts, fmt.Sprintf("pkg = %q", dep.Pkg))
		}
		if dep.Version != "" {
			parts = append(parts, fmt.Sprintf("version = %q", dep.Version))
		}
		if dep.Branch != "" {
			parts = append(parts, fmt.Sprintf("branch = %q", dep.Branch))
		}
		if dep.Commit != "" {
			parts = append(parts, fmt.Sprintf("commit = %q", dep.Commit))
		}
		fmt.Fprintf(&sb, "%s = { %s }\n", n, strings.Join(parts, ", "))
	}
	return sb.String()
}
