package deps

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/uyloal/baihu-panel/internal/models"
)

// PackageJson 代表 package.json 的依赖结构
type PackageJson struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// ParseManifest 解析依赖清单文件内容（支持 package.json 或换行包列表）
func ParseManifest(content string) ([]models.Dependency, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return ParsePackageJson(trimmed)
	}
	return ParsePackageList(trimmed), nil
}

// ParsePackageJson 解析 Node.js package.json
func ParsePackageJson(content string) ([]models.Dependency, error) {
	var pkg PackageJson
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil, err
	}

	var deps []models.Dependency
	collect := func(m map[string]string, isDev bool) {
		for name, versionRange := range m {
			version := strings.TrimLeft(versionRange, "^~>=<* ")
			remark := "dependencies"
			if isDev {
				remark = "devDependencies"
			}
			deps = append(deps, models.Dependency{
				Name:     name,
				Version:  version,
				Language: "node",
				Remark:   remark,
			})
		}
	}

	collect(pkg.Dependencies, false)
	collect(pkg.DevDependencies, true)
	return deps, nil
}

// ParsePackageList 解析纯文本按行或空格分隔的包名列表（例如: lodash@^4.17.21 axios@1.6.0）
func ParsePackageList(content string) []models.Dependency {
	var deps []models.Dependency
	lines := regexp.MustCompile(`\r?\n`).Split(content, -1)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.Fields(line)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" || strings.HasPrefix(part, "-") {
				continue
			}

			// 处理 @scope/pkg@1.0.0 与 pkg@1.0.0
			name := part
			version := ""

			lastAt := strings.LastIndex(part, "@")
			if lastAt > 0 { // 排除以 @ 开头但没有指定版本的 scope 包如 @types/node
				name = part[:lastAt]
				version = strings.TrimLeft(part[lastAt+1:], "^~>=<* ")
			}

			if name != "" {
				deps = append(deps, models.Dependency{
					Name:     name,
					Version:  version,
					Language: "node",
				})
			}
		}
	}
	return deps
}
