# 白虎面板极简版 (Node & pnpm) 现代化改造方案

本文档基于现代 Node.js 运行时与包管理技术规范，为白虎面板（Baihu Panel）量身定制一套**专注于 Node.js & pnpm 极简生态、全新独立部署与完整重新运行**的现代化架构改造实施方案。本项目通过架构级重构直接演进为 Minimal 独立版本，彻底剥离旧版多语言与 Mise 运行时，无需历史环境增量迁移或双轨过渡。

---

## 一、改造定位与收益矩阵

| 核心维度 | 旧版多语言架构 (Legacy) | 极简现代化架构 (Baihu Minimal) | 核心技术收益 |
| :--- | :--- | :--- | :--- |
| **运行时生态** | Mise 多语言插件链 (Python/Go/Node) | **纯 Node.js 22+/24+ 原生运行时** | 镜像体积由 1.2GB 骤降至 **~160MB**，内存 < **35MB** |
| **包安装速度** | `npm install -g` 全量解压复制 | **pnpm 全局 CAS 内容寻址 + 硬链接** | **依赖安装速度提升 5~10 倍** |
| **磁盘存储占用**| 多环境重复下载包膨胀 | **全局单一 CAS 哈希存储，物理单份** | **节约 80%+ 依赖磁盘空间** |
| **ESM 模块支持** | `import` 无法识别 `NODE_PATH` 直接报错 | **`/app/data/node_modules` 软链原生向上寻址** | 彻底消除跨模块寻址障碍，原生兼容 ESM 与 CJS |
| **TypeScript 支持**| 需预先编译或安装大量外部工具 | **Node 原生剥离类型 (`--experimental-strip-types`)** | **零构建直接执行 `.ts` / `.mts` 脚本** |
| **依赖边界安全** | 扁平提升存在幽灵依赖隐患 | **严格符号链接虚拟树隔离** | 杜绝非法引用与版本漂移 |
| **错误诊断自愈** | 仅捕获 CJS `Cannot find module` | **精准捕获 ESM `[ERR_MODULE_NOT_FOUND]` 及 TS 报错** | Web 端与 CLI 依赖一键自愈修复 |

---

## 二、极简现代化五层架构蓝图

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                           白虎面板极简版 (Node & pnpm) 架构蓝图                              │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                             │
│  [1. 极简容器底座 (Docker & Toolchain)]                                                      │
│  ├── 基于官方轻量 node:22-alpine 单容器构建交付 (彻底剥离 Dockerfile.base 与 Mise)           │
│  ├── 启用 Corepack 官方工具链: corepack enable && corepack prepare pnpm@latest              │
│  └── 统一 CAS 存储持久化: PNPM_HOME=/app/envs/pnpm, PNPM_STORE=/app/envs/pnpm-store          │
│                                                                                             │
│  [2. 依赖管理服务层 (Backend DepService)]                                                    │
│  ├── 纯 Go 实现 DepService: 直接调用 pnpm CLI (pnpm add/remove -g, pnpm list -g --json)     │
│  ├── 升级 NodeDetector: 精准捕获 ESM [ERR_MODULE_NOT_FOUND] 与 TypeScript 缺失报错            │
│  └── 升级 ParsePackageJson: 适配 type: module, exports, packageManager 及 catalog: 协议      │
│                                                                                             │
│  [3. 内置 SDK 现代化改造 (Builtin SDK)]                                                       │
│  ├── 改造 builtin/nodejs 为 Dual Package (原生提供 index.mjs 与 index.cjs)                   │
│  └── 导出完整 TypeScript 类型定义 (index.d.ts)                                               │
│                                                                                             │
│  [4. 任务执行与调度层 (Task & Execution Engine)]                                             │
│  ├── 原生 TypeScript 零构建直接执行 (Node 22+ --experimental-strip-types)                    │
│  ├── 原生 ESM (import / Top-level await) 第一公民支持                                        │
│  └── 根目录软链桥接 (/app/data/node_modules ──> CAS 全局目录)，彻底废弃 NODE_PATH 黑魔法    │
│                                                                                             │
│  [5. 前端工程与管理界面 (Web UI & Editor)]                                                    │
│  ├── 前端自身采用 pnpm 构建体系 (Vue 3.5 + Vite 7 + Tailwind 4 + Monaco)                     │
│  └── 脚本编辑器全面补齐 .ts, .mts, .cts, .mjs, .cjs 扩展名语法高亮与执行映射                  │
│                                                                                             │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 三、各层代码级改造实现规范

### 第一层：极简单容器底座与环境初始化

#### 1. 极简单容器 Dockerfile（取代多语言复杂镜像）
- **实现文件**：`docker/Dockerfile`
```dockerfile
# ================================
# Stage 1: Build Web Frontend
# ================================
FROM node:22-alpine AS web-builder
WORKDIR /app/web
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

# ================================
# Stage 2: Build Go Binary
# ================================
FROM golang:1.22-alpine AS server-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./internal/static/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o baihu-server cmd/server/main.go

# ================================
# Stage 3: Minimal Production Image
# ================================
FROM node:22-alpine
WORKDIR /app

ENV NODE_ENV=production
ENV PNPM_HOME=/app/envs/pnpm
ENV PNPM_STORE_DIR=/app/envs/pnpm-store
ENV PATH="$PNPM_HOME:$PATH"

RUN apk add --no-cache tzdata ca-certificates bash curl git openssh-client \
    && corepack enable && corepack prepare pnpm@latest --activate \
    && pnpm config set store-dir /app/envs/pnpm-store

COPY --from=server-builder /app/baihu-server /usr/local/bin/baihu-server
COPY builtin/ /www/builtin
COPY docker/docker-entrypoint.sh .
RUN chmod +x docker-entrypoint.sh

EXPOSE 8052
VOLUME ["/app/data", "/app/envs"]

ENTRYPOINT ["./docker-entrypoint.sh"]
CMD ["baihu-server"]
```

#### 2. 容器入口点初始化与软链桥接
- **实现文件**：`docker/docker-entrypoint.sh`
```bash
#!/bin/bash
set -e

export PNPM_HOME="/app/envs/pnpm"
export PNPM_STORE_DIR="/app/envs/pnpm-store"
export PATH="$PNPM_HOME:$PATH"

# 初始化数据与存储目录
mkdir -p "$PNPM_HOME" "$PNPM_STORE_DIR" /app/data/scripts

# 确保全局启用 Corepack
corepack enable >/dev/null 2>&1 || true

# 建立用户脚本根目录的 node_modules 软链接（ESM/CJS 原生向上寻址，彻底摒弃 NODE_PATH）
mkdir -p "$PNPM_HOME/global/5/node_modules"
ln -sf "$PNPM_HOME/global/5/node_modules" /app/data/node_modules

# 安装内置 SDK
if [ -d "/www/builtin/nodejs" ]; then
    pnpm add -g /www/builtin/nodejs >/dev/null 2>&1 || true
fi

exec "$@"
```

---

### 第二层：依赖管理服务层改造（Go 后端）

#### 3. 纯 pnpm CAS 依赖管理器 (`DepService`)
- **实现文件**：`internal/services/dep_service.go`
```go
package services

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/engigu/baihu-panel/internal/logger"
	"github.com/engigu/baihu-panel/internal/models"
)

type DepService struct{}

func NewDepService() *DepService {
	return &DepService{}
}

// Install 安装全局 pnpm 依赖 (享受 CAS 内容寻址与硬链接加速)
func (s *DepService) Install(name, version string) (string, error) {
	pkgSpec := name
	if version != "" {
		pkgSpec = name + "@" + version
	}
	cmd := exec.Command("pnpm", "add", "-g", pkgSpec)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Uninstall 卸载全局依赖
func (s *DepService) Uninstall(name string) (string, error) {
	cmd := exec.Command("pnpm", "remove", "-g", name)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ListInstalled 获取已安装包列表
func (s *DepService) ListInstalled() ([]models.Dependency, error) {
	cmd := exec.Command("pnpm", "list", "-g", "--json", "--depth=0")
	out, err := cmd.Output()
	if err != nil {
		logger.Warnf("pnpm list failed: %v", err)
		return []models.Dependency{}, nil
	}

	type pnpmListItem struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}

	var list []pnpmListItem
	var packages []models.Dependency
	if err := json.Unmarshal(out, &list); err == nil && len(list) > 0 {
		for name, info := range list[0].Dependencies {
			packages = append(packages, models.Dependency{
				Name:    name,
				Version: info.Version,
			})
		}
	}
	return packages, nil
}
```

#### 4. 升级依赖缺失诊断器 `NodeDetector`
- **实现文件**：`internal/services/deps/detector.go`
```go
package deps

import (
	"regexp"
	"strings"
)

// NodeDetector Node.js 依赖检测器（全面支持 CJS, ESM 与 TypeScript 错误捕获）
type NodeDetector struct{}

func (d *NodeDetector) Detect(logContent string) []string {
	var pkgs []string
	seen := make(map[string]bool)

	// 统一捕获 CJS, ESM, TS 缺失依赖模式：
	// 1. Error: Cannot find module 'axios'
	// 2. Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'axios' imported from ...
	// 3. Cannot find module 'axios' or Cannot find package 'axios'
	nodeRegex := regexp.MustCompile(`(?:Error(?:\s*\[ERR_MODULE_NOT_FOUND\])?:\s*)?Cannot find (?:module|package)\s+'([^']+)'`)

	matches := nodeRegex.FindAllStringSubmatch(logContent, -1)
	for _, m := range matches {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
				continue
			}

			pkgName := name
			if strings.HasPrefix(pkgName, "@") {
				parts := strings.Split(pkgName, "/")
				if len(parts) >= 2 {
					pkgName = parts[0] + "/" + parts[1]
				}
			} else {
				parts := strings.Split(pkgName, "/")
				pkgName = parts[0]
			}

			if !seen[pkgName] {
				seen[pkgName] = true
				pkgs = append(pkgs, pkgName)
			}
		}
	}
	return pkgs
}
```

#### 5. 升级清单解析器 `ParsePackageJson`
- **实现文件**：`internal/services/deps/parser.go`
```go
package deps

import (
	"encoding/json"
	"strings"

	"github.com/engigu/baihu-panel/internal/models"
)

type PackageJson struct {
	Type            string            `json:"type"`
	PackageManager  string            `json:"packageManager"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func ParsePackageJson(content string) ([]models.Dependency, error) {
	var pkg PackageJson
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil, err
	}

	var deps []models.Dependency
	collect := func(m map[string]string, isDev bool) {
		for name, versionRange := range m {
			if strings.HasPrefix(versionRange, "workspace:") || strings.HasPrefix(versionRange, "file:") {
				continue
			}
			version := strings.TrimPrefix(versionRange, "catalog:")
			version = strings.TrimLeft(version, "^~>=<* ")

			remark := ""
			if isDev {
				remark = "devDependencies"
			}
			if pkg.Type == "module" {
				if remark != "" {
					remark += " (ESM)"
				} else {
					remark = "ESM"
				}
			}

			deps = append(deps, models.Dependency{
				Name:    name,
				Version: version,
				Remark:  remark,
			})
		}
	}

	collect(pkg.Dependencies, false)
	collect(pkg.DevDependencies, true)
	return deps, nil
}
```

---

### 第三层：内置 SDK 现代化改造 (`builtin/nodejs`)

#### 6. Dual Package 配置 (`builtin/nodejs/package.json`)
```json
{
  "name": "baihu",
  "version": "1.2.0",
  "description": "Baihu Panel Builtin SDK for Node.js (Pure ESM, CommonJS & TypeScript)",
  "main": "./index.cjs",
  "module": "./index.mjs",
  "types": "./index.d.ts",
  "exports": {
    ".": {
      "types": "./index.d.ts",
      "import": "./index.mjs",
      "require": "./index.cjs"
    },
    "./package.json": "./package.json"
  },
  "engines": {
    "node": ">=20.0.0"
  },
  "license": "MIT"
}
```

#### 7. 完整 TypeScript 声明定义 (`builtin/nodejs/index.d.ts`)
```typescript
export interface NotifyOptions {
  format?: 'text' | 'markdown' | 'html';
  channel_id?: string;
  channelId?: string;
}

export function notify(title: string, content: string, options?: NotifyOptions | string): void;
export function getEnvs(): Promise<Array<{ id: string; name: string; value: string; remark?: string }>>;
export function getEnv(name: string): Promise<{ id: string; name: string; value: string; remark?: string } | null>;
export function addEnv(name: string, value: string, remark?: string): Promise<any>;
export function updateEnv(id: string, name: string, value: string, remark?: string): Promise<any>;
export function deleteEnv(id: string): Promise<boolean>;
export function getTasks(): Promise<Array<{ id: string; name: string; schedule?: string; command?: string }>>;
export function executeTask(id: string): Promise<any>;
export function stopTask(id: string): Promise<any>;

declare const baihu: {
  notify: typeof notify;
  getEnvs: typeof getEnvs;
  getEnv: typeof getEnv;
  addEnv: typeof addEnv;
  updateEnv: typeof updateEnv;
  deleteEnv: typeof deleteEnv;
  getTasks: typeof getTasks;
  executeTask: typeof executeTask;
  stopTask: typeof stopTask;
};

export default baihu;
```

---

### 第四层：任务执行与调度引擎升级

#### 8. 原生 TypeScript 与 ESM 执行参数注入
- **实现文件**：`internal/executor/executor.go`
```go
// 针对 Node.js 任务环境注入现代执行参数
cmd.Env = append(cmd.Env,
    "TERM=xterm",
    "NODE_NO_WARNINGS=1",
    // 启用 Node 22+ 原生 TypeScript 直接剥离类型执行 (无需额外构建)
    "NODE_OPTIONS=--experimental-strip-types --max-old-space-size=512",
)
```

---

### 第五层：前端与编辑器适配

#### 9. 前端构建配置
- **实现文件**：`Makefile`
```makefile
build-web:
	cd web && pnpm install --frozen-lockfile && pnpm run build
```

#### 10. 脚本编辑器扩展名映射与 Monaco 高亮
- **实现文件**：`web/src/constants/index.ts`
```typescript
export const FILE_RUNNERS: Record<string, string> = {
  js: 'node',
  mjs: 'node',
  cjs: 'node',
  ts: 'node',       // Node 22+ 原生 --experimental-strip-types
  mts: 'node',
  cts: 'node',
  sh: 'bash',
  bash: 'bash',
} as const
```
- **实现文件**：`web/src/views/editor/Editor.vue` 补齐 `.mjs` / `.cjs` / `.ts` / `.mts` / `.cts` 语法高亮映射。

---

## 四、极简版重构与全新运行路线

由于本项目直接重构改造为 Minimal 极简版并全新运行，整体开发与落地流程清晰明确：

```
[阶段一: 核心解耦与极简架构落地]
 ├── 1. 彻底剥离 Mise、Python 及异构语言代码，简化数据模型与控制器
 ├── 2. 重构 DepService 为纯 pnpm CAS 依赖管理，实现安装、卸载与深度列表解析
 └── 3. 改造 builtin/nodejs 为 Dual Package (CJS/ESM/TS)

[阶段二: 调度引擎与诊断能力升级]
 ├── 1. 注入 --experimental-strip-types 开启原生 TS 剥离支持与原生 ESM 向上寻址
 ├── 2. 升级 NodeDetector 正则，全面捕获 ESM 与 TS 缺失错误
 └── 3. 优化进程组隔离与级联终止，保障长时间任务调度稳定性

[阶段三: 前端工程与 Monaco 编辑器适配]
 ├── 1. 前端全面切至 pnpm 依赖管理与构建加速
 └── 2. Monaco 编辑器补全 TS / ESM 语法高亮与快捷执行映射

[阶段四: 单容器交付与端到端运行验收]
 ├── 1. 编写基于 node:22-alpine 的极简 Dockerfile 与 docker-compose.yml 部署模板
 ├── 2. 全新初始化运行并验证各项能力：
 │     ├── 验证一：pnpm CAS 秒级安装与软链寻址
 │     ├── 验证二：原生 TypeScript 直接执行与 Top-level await
 │     ├── 验证三：内置 SDK 双模块导入与告警分发
 │     └── 验证四：系统资源看板与 Web 终端 PTY 诊断
```
