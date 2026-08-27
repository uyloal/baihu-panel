package deps

import (
	"testing"
)

func TestDetectMissingDependencies(t *testing.T) {
	tests := []struct {
		name       string
		logContent string
		wantPkgs   []string
		wantFound  bool
	}{
		{
			name:       "Node Error Cannot find module",
			logContent: "Error: Cannot find module 'axios'\nRequire stack:\n- /app/index.js",
			wantPkgs:   []string{"axios"},
			wantFound:  true,
		},
		{
			name:       "Node Error Cannot find package",
			logContent: "Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'cheerio' imported from /app/scripts/task.mjs",
			wantPkgs:   []string{"cheerio"},
			wantFound:  true,
		},
		{
			name:       "No match",
			logContent: "Success running script",
			wantPkgs:   nil,
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPkgs, gotFound := DetectMissingDependencies(tt.logContent)
			if gotFound != tt.wantFound {
				t.Errorf("DetectMissingDependencies() gotFound = %v, want %v", gotFound, tt.wantFound)
			}
			if len(gotPkgs) != len(tt.wantPkgs) {
				t.Errorf("DetectMissingDependencies() gotPkgs = %v, want %v", gotPkgs, tt.wantPkgs)
				return
			}
			for i, p := range gotPkgs {
				if p != tt.wantPkgs[i] {
					t.Errorf("DetectMissingDependencies() gotPkgs[%d] = %v, want %v", i, p, tt.wantPkgs[i])
				}
			}
		})
	}
}
