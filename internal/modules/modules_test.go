package modules

import (
	"path/filepath"
	"testing"
)

func TestFileToModuleName(t *testing.T) {
	cases := []struct {
		root, srcDir, file, want string
	}{
		{"app", "/p/src", "/p/src/main.pure", "app.main"},
		{"app", "/p/src", "/p/src/models/user.pure", "app.models.user"},
		{"math", "/m/src", "/m/src/math.pure", "math"},
		{"math", "/m/src", "/m/src/geometry/area.pure", "math.geometry.area"},
	}
	for _, c := range cases {
		got, err := FileToModuleName(c.root, c.srcDir, c.file)
		if err != nil {
			t.Errorf("err=%v", err)
			continue
		}
		want := filepath.ToSlash(c.want)
		if got != want {
			t.Errorf("FileToModuleName(%q,%q,%q)=%q want %q", c.root, c.srcDir, c.file, got, want)
		}
	}
}
