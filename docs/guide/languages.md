# 依赖管理

白虎面板采用极简架构，深度集成 **Node.js 24 LTS** 与 **pnpm 11.24** 依赖管理生态。

## 脚本运行环境

白虎面板原生支持以下脚本的定时执行：
- **JavaScript**：`.js`, `.mjs`, `.cjs`
- **TypeScript**：`.ts`, `.mts`, `.cts`（Node.js 24 原生直接执行，零编译构建）
- **Bash / Shell**：`.sh`, `.bash`

---

## 依赖管理架构

系统基于标准 Node.js 项目工程管理所有第三方依赖包：

1. **项目自包含**：所有依赖直接维护在 `/app/data/package.json` 与 `/app/data/node_modules/` 目录下；
2. **纯净脚本目录**：`/app/data/scripts/` 仅保留用户脚本文件，Node.js 执行任务时原生向上寻址加载 `node_modules`；
3. **秒级安装**：由 pnpm 驱动，支持官方源与镜像源快速切换；
4. **内置 SDK**：系统默认通过 `pnpm add` 在项目根目录下注入本地 `@baihu` 助手库，用户脚本直接 `import { ... } from 'baihu'` 即可使用。

---

## 使用方法

### 1. 添加依赖
在白虎面板的「依赖管理」页面，点击「添加依赖」，输入 npm 包名（如 `axios`、`lodash-es`、`dayjs` 等）与可选版本号，系统会自动调用 `pnpm add <package>` 完成安装。

### 2. 批量导入
在「依赖管理」中支持粘贴 `package.json` 或包名列表批量导入并一次性完成安装。

### 3. 智能补全
当脚本运行报错提示缺少模块（如 `Cannot find package 'xxx'`）时，可在面板中点击一键自动补全，或通过命令行执行 `baihu depinstall <log_id>` 自动解析并补装缺失依赖。

