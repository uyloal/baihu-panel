package deps

import (
	"regexp"
	"strings"
)

// Detector 依赖检测器接口
type Detector interface {
	Detect(logContent string) []string
}

// NodeDetector Node.js 依赖检测器
type NodeDetector struct{}

var (
	nodeMissingPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:Error:\s*)?Cannot find (?:module|package)\s+'([^']+)'`),
		regexp.MustCompile(`\[ERR_MODULE_NOT_FOUND\]:\s*Cannot find package\s+'([^']+)'`),
		regexp.MustCompile(`\[ERR_PACKAGE_PATH_NOT_EXPORTED\]:\s*Package subpath\s+'.*'\s+is not defined by "exports" in\s+'.*/node_modules/([^/]+)/`),
	}
)

func (d *NodeDetector) Detect(logContent string) []string {
	var pkgs []string
	seen := make(map[string]bool)

	for _, re := range nodeMissingPatterns {
		matches := re.FindAllStringSubmatch(logContent, -1)
		for _, m := range matches {
			if len(m) > 1 {
				name := strings.TrimSpace(m[1])
				// 排除相对路径引用如 ./foo 或 ../bar
				if name != "" && !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "/") && !seen[name] {
					// 如果是 scoped 包如 @types/node，保留 scoped 名称；否则如果是子路径如 lodash/get，提取根包名
					if strings.HasPrefix(name, "@") {
						parts := strings.Split(name, "/")
						if len(parts) >= 2 {
							name = parts[0] + "/" + parts[1]
						}
					} else {
						parts := strings.Split(name, "/")
						name = parts[0]
					}
					if !seen[name] {
						seen[name] = true
						pkgs = append(pkgs, name)
					}
				}
			}
		}
	}

	return pkgs
}

var defaultDetector = &NodeDetector{}

// DetectMissingDependencies 从日志内容中检测缺失的依赖包名
func DetectMissingDependencies(logContent string) ([]string, bool) {
	pkgs := defaultDetector.Detect(logContent)
	return pkgs, len(pkgs) > 0
}
