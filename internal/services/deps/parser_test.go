package deps

import (
	"testing"
)

func TestParsePackageJson(t *testing.T) {
	content := `{
  "dependencies": {
    "lodash": "^4.17.21",
    "express": "~4.18.2"
  },
  "devDependencies": {
    "typescript": "^5.0.4"
  }
}`
	deps, err := ParsePackageJson(content)
	if err != nil {
		t.Fatalf("failed to parse package.json: %v", err)
	}

	if len(deps) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(deps))
	}

	expectedMap := map[string]struct {
		version string
		remark  string
	}{
		"lodash":     {"4.17.21", "dependencies"},
		"express":    {"4.18.2", "dependencies"},
		"typescript": {"5.0.4", "devDependencies"},
	}

	for _, dep := range deps {
		exp, ok := expectedMap[dep.Name]
		if !ok {
			t.Errorf("unexpected dependency: %s", dep.Name)
			continue
		}
		if dep.Version != exp.version {
			t.Errorf("for %s, expected version %s, got %s", dep.Name, exp.version, dep.Version)
		}
		if dep.Remark != exp.remark {
			t.Errorf("for %s, expected remark %s, got %s", dep.Name, exp.remark, dep.Remark)
		}
		if dep.Language != "node" {
			t.Errorf("for %s, expected language node, got %s", dep.Name, dep.Language)
		}
	}
}

func TestParsePackageList(t *testing.T) {
	content := `
# some comment
axios@^1.6.0
lodash
@types/node@20.0.0
`
	deps := ParsePackageList(content)
	if len(deps) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(deps))
	}

	expectedMap := map[string]string{
		"axios":       "1.6.0",
		"lodash":      "",
		"@types/node": "20.0.0",
	}

	for _, dep := range deps {
		expVer, ok := expectedMap[dep.Name]
		if !ok {
			t.Errorf("unexpected dependency: %s", dep.Name)
			continue
		}
		if dep.Version != expVer {
			t.Errorf("for %s, expected version %s, got %s", dep.Name, expVer, dep.Version)
		}
		if dep.Language != "node" {
			t.Errorf("for %s, expected language node, got %s", dep.Name, dep.Language)
		}
	}
}
