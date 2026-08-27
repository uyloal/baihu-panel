# 白虎面板

> **本仓库 Fork 自 [engigu/baihu-panel](https://github.com/engigu/baihu-panel)**，专为自建私有化单一镜像服务定制与维护，开箱即用。

## 特色

- **极简单一镜像：** 基于 Node.js 24 LTS + Alpine，内置 Go 核心与完整 WebUI，支持 `linux/amd64` 与 `linux/arm64` 双架构
- **开箱即用：** 单一 Compose 文件或一行 `docker run` 即可启动，数据与依赖自包含持久化在 `./data`
- **任务调度：** 支持标准 Cron 表达式，常用时间规则快捷选择，内存实时日志流
- **脚本管理：** 在线 Monaco 代码编辑器，支持文件上传、压缩包解压与多文件批量管理
- **在线终端：** WebSocket 实时终端，命令执行结果实时输出
- **消息推送：** 原生内置主流通知渠道（钉钉、企业微信、Telegram、Server酱等），支持系统级事件告警
- **机密管理：** 类似 GitHub Secrets 的安全存储，支持 AES-GCM 加密，日志自动打码
- **现代运行时：** 原生支持 JavaScript / TypeScript（Node.js 24 零编译直接执行）与 Shell 脚本
- **内建助手库：** 提供零配置 `@baihu` 助手库，简单 import 即可实现一键推信与任务交互


## 功能特性 

<details>
<summary><b>点击展开查看详细功能</b></summary>

### 定时任务管理
- 支持标准 Cron 表达式调度
- 常用时间规则快捷选择
- 任务启用/禁用状态切换
- 手动触发执行
- 任务超时控制

### 脚本文件管理
- 在线代码编辑器
- 文件树形结构展示
- 支持创建、重命名、删除文件/文件夹
- 支持压缩包上传解压
- 支持多文件批量上传

### 在线终端
- WebSocket 实时终端
- 支持常用 Shell 命令
- 命令执行结果实时输出

### 执行日志
- 任务执行历史记录
- 执行状态追踪（成功/失败/超时）
- 执行耗时统计
- 日志内容压缩存储
- 日志自动清理

### 消息推送与系统通知
- 原生内置各大主流平台渠道（钉钉、企业微信、Telegram、Server酱等）
- 支持系统级事件条件触发通知（例如任务失败报警、服务下线提醒）
- 自动生成跨语言调用 API 示例代码供脚本集成

### 变量与机密
- 支持普通环境变量与安全机密（Secret）分类管理
- 机密使用 **AES-GCM** 加密存储，数据库不存明文
- 秘钥仅在内存中留存，启动读取后立即销毁（Unset）
- 执行日志自动搜索并**屏蔽(********)**敏感机密内容
- 严格权限隔离：机密仅在定时任务调度时注入，终端/测试环境不可见

### 仓库任务同步 (New)
- 支持 青龙 仓库命令格式快捷导入
- 自动解析脚本注释中的 Cron 表达式和环境变量名
- 支持基于正则表达式的白名单、黑名单文件筛选
- 支持脚本依赖文件的识别与保留
- 自动同步远程 Git 仓库变更，增量更新面板任务

### 系统设置
- 站点标题、标语、图标自定义
- 分页大小、Cookie 有效期配置
- 调度参数热重载
- 数据备份与恢复

</details>

## 脚本运行与依赖管理

<details>
<summary><b>点击展开查看脚本与依赖管理详情</b></summary>

### 脚本运行环境
白虎面板原生内置以下脚本的定时执行：
- **JavaScript**：`.js`, `.mjs`, `.cjs`
- **TypeScript**：`.ts`, `.mts`, `.cts`（Node.js 24 LTS 原生直接执行，无需构建编译）
- **Bash / Shell**：`.sh`, `.bash`

### 依赖管理支持
系统内置了基于 **pnpm 11.24** 的依赖管理体系：
- 所有依赖统一维护在 `data/package.json` 与 `data/node_modules` 中；
- 任务执行时由 Node.js 从 `data/scripts` 原生向上寻址解析依赖；
- 支持在面板「依赖管理」页面一键安装、批量导入与自动补全缺失依赖。

</details>

## 效果图 

![baihu-display.gif](https://raw.githubusercontent.com/engigu/resources/refs/heads/images/baihu-display.gif)
<!-- TODO: 添加效果图 -->

## 快速部署 

| 标签 (Tag) | 基础镜像 | 说明 |
| :--- | :--- | :--- |
| `latest` | Node.js 24 Alpine | **极简专属**：集成 Node.js 24 LTS 与 pnpm 11.24，极致轻量，开箱即用 |


<details>
<summary><b>方式一：环境变量部署（推荐）</b></summary>

通过环境变量指定配置，简单灵活，适合容器编排场景。

**使用 SQLite（默认）：**

```bash
docker run -d \
  --name baihu \
  -p 8052:8052 \
  -v $(pwd)/data:/app/data \
  -e TZ=Asia/Shanghai \
  -e BH_SERVER_PORT=8052 \
  -e BH_SERVER_HOST=0.0.0.0 \
  -e BH_DB_TYPE=sqlite \
  -e BH_DB_PATH=/app/data/baihu.db \
  -e BH_DB_TABLE_PREFIX=baihu_ \
  -e BAIHU_SECRET_KEY=your_secret_key_here \
  --restart unless-stopped \
  ghcr.io/uyloal/baihu:latest
```

> **提示**：如需通过反向代理部署在子路径（如 `/baihu`），添加环境变量：
> ```bash
> -e BH_SERVER_URL_PREFIX=/baihu
> ```
> 配置后访问地址为 `http://your-domain.com/baihu/`，详见下方「URL 前缀配置」说明。

**Docker Compose（SQLite）：**

```yaml
services:
  baihu:
    image: ghcr.io/uyloal/baihu:latest
    container_name: baihu
    ports:
      - "8052:8052"
    volumes:
      - ./data:/app/data
      - ./configs:/app/configs
    environment:
      - TZ=Asia/Shanghai
      - BH_SERVER_PORT=8052
      - BH_SERVER_HOST=0.0.0.0
      - BH_DB_TYPE=sqlite
      - BH_DB_PATH=/app/data/baihu.db
      - BH_DB_TABLE_PREFIX=baihu_
      - BAIHU_SECRET_KEY=your_secret_key_here
      # - BH_SERVER_URL_PREFIX=/baihu  # 可选：配置 URL 前缀用于反向代理
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
```

**使用 MySQL：**

```bash
docker run -d \
  --name baihu \
  -p 8052:8052 \
  -v $(pwd)/data:/app/data \
  -e TZ=Asia/Shanghai \
  -e BH_SERVER_PORT=8052 \
  -e BH_SERVER_HOST=0.0.0.0 \
  -e BH_DB_TYPE=mysql \
  -e BH_DB_HOST=mysql-server \
  -e BH_DB_PORT=3306 \
  -e BH_DB_USER=root \
  -e BH_DB_PASSWORD=your_password \
  -e BH_DB_NAME=baihu \
  -e BH_DB_TABLE_PREFIX=baihu_ \
  -e BAIHU_SECRET_KEY=your_secret_key_here \
  --restart unless-stopped \
  ghcr.io/uyloal/baihu:latest
```

> **提示**：如需配置 URL 前缀，添加 `-e BH_SERVER_URL_PREFIX=/baihu`

**Docker Compose（MySQL）：**

```yaml
services:
  baihu:
    image: ghcr.io/uyloal/baihu:latest
    container_name: baihu
    ports:
      - "8052:8052"
    volumes:
      - ./data:/app/data
      - ./configs:/app/configs
    environment:
      - TZ=Asia/Shanghai
      - BH_SERVER_PORT=8052
      - BH_SERVER_HOST=0.0.0.0
      # - BH_SERVER_URL_PREFIX=/baihu  # 可选：配置 URL 前缀
      - BH_DB_TYPE=mysql
      - BH_DB_HOST=mysql-server
      - BH_DB_PORT=3306
      - BH_DB_USER=root
      - BH_DB_PASSWORD=your_password
      - BH_DB_NAME=baihu
      - BH_DB_TABLE_PREFIX=baihu_
      - BAIHU_SECRET_KEY=your_secret_key_here
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
```

</details>

<details>
<summary><b>方式二：配置文件部署</b></summary>

通过挂载 `config.ini` 配置文件来管理配置，适合需要持久化配置的场景。

**Docker 命令：**

```bash
docker run -d \
  --name baihu \
  -p 8052:8052 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  -e TZ=Asia/Shanghai \
  -e BAIHU_SECRET_KEY=your_secret_key_here \
  --restart unless-stopped \
  ghcr.io/uyloal/baihu:latest
```

**Docker Compose：**

```yaml
services:
  baihu:
    image: ghcr.io/uyloal/baihu:latest
    container_name: baihu
    ports:
      - "8052:8052"
    volumes:
      - ./data:/app/data
      - ./configs:/app/configs
    environment:
      - TZ=Asia/Shanghai
      - BAIHU_SECRET_KEY=your_secret_key_here
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
```

首次使用需要复制 `configs/config.example.ini` 为 `configs/config.ini`，然后根据需要修改配置。

**配置文件示例（`configs/config.ini`）：**

```ini
[server]
port = 8052
host = 0.0.0.0
# 可选：配置 URL 前缀用于反向代理，例如 /baihu
url_prefix = 
# 全局会话 Cookie 名称
cookie_name = BHToken

[database]
type = sqlite
path = ./data/baihu.db
table_prefix = baihu_
```

</details>

<details>
<summary><b>方式三：配合独立中心化消息服务部署（非必需/仅供参考）</b></summary>

> 🎉 **好消息**：自白虎面板最新版本起，系统已**原生内置**了完整强大的消息推送功能！您可直接在面板「消息推送」菜单内绑定十余种主流渠道和系统通知事件，原配合外置的 `Message-Push-Nest` 部署方式已不再是使用面板的基础要求。您可随时直接使用上方的第一种简单命令开箱即用体验。
>  
> 以下「白虎 + 消息聚合服务」的联合部署内容被予以保留，专为仍然需要「中心化通知网关」的重度企业解耦用户作为参考：

白虎面板通过系统集成也可轻松连接独立的消息聚合服务。这里推荐使用 [Message-Push-Nest](https://github.com/engigu/Message-Push-Nest) 作为分布式的统一消息推送中心。

**使用 SQLite**

创建 `docker-compose.yml` 文件：

```yaml
services:
  # 白虎面板
  baihu:
    image: ghcr.io/uyloal/baihu:latest
    container_name: baihu
    ports:
      - "8052:8052"
    volumes:
      - ./data:/app/data
      - ./configs:/app/configs
    environment:
      - TZ=Asia/Shanghai
      - BH_SERVER_PORT=8052
      - BH_SERVER_HOST=0.0.0.0
      - BH_DB_TYPE=sqlite
      - BH_DB_PATH=/app/data/baihu.db
      - BH_DB_TABLE_PREFIX=baihu_
      - BAIHU_SECRET_KEY=your_secret_key_here
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
    depends_on:
      - message-nest

  # 消息推送服务
  message-nest:
    image: ghcr.io/engigu/message-nest:latest
    # 或使用 Docker Hub 镜像
    # image: engigu/message-nest:latest
    container_name: message-nest
    ports:
      - "8053:8000"
    environment:
      - TZ=Asia/Shanghai
      - DB_TYPE=sqlite
    volumes:
      - ./message-nest-data:/app/data
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
```

**使用 MySQL（适合生产环境，需要已有 MySQL 服务）**

创建 `docker-compose.yml` 文件：

```yaml
services:
  # 白虎面板
  baihu:
    image: ghcr.io/uyloal/baihu:latest
    container_name: baihu
    ports:
      - "8052:8052"
    volumes:
      - ./data:/app/data
      - ./configs:/app/configs
    environment:
      - TZ=Asia/Shanghai
      - BH_SERVER_PORT=8052
      - BH_SERVER_HOST=0.0.0.0
      - BH_DB_TYPE=mysql
      - BH_DB_HOST=192.168.1.100  # 修改为你的 MySQL 地址
      - BH_DB_PORT=3306
      - BH_DB_USER=root
      - BH_DB_PASSWORD=your_password  # 修改为你的 MySQL 密码
      - BH_DB_NAME=baihu
      - BH_DB_TABLE_PREFIX=baihu_
      - BAIHU_SECRET_KEY=your_secret_key_here
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
    depends_on:
      - message-nest

  # 消息推送服务
  message-nest:
    image: ghcr.io/engigu/message-nest:latest
    # 或使用 Docker Hub 镜像
    # image: engigu/message-nest:latest
    container_name: message-nest
    ports:
      - "8053:8000"
    environment:
      - TZ=Asia/Shanghai
      - DB_TYPE=mysql
      - MYSQL_HOST=192.168.1.100  # 修改为你的 MySQL 地址
      - MYSQL_PORT=3306
      - MYSQL_USER=root
      - MYSQL_PASSWORD=your_password  # 修改为你的 MySQL 密码
      - MYSQL_DB=message_nest
      - MYSQL_TABLE_PREFIX=message_
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
```

启动服务：

```bash
docker-compose up -d
```

**访问地址：**
- 白虎面板：http://localhost:8052
- 消息推送服务：http://localhost:8053

> 注意：使用 MySQL 方式时，请先在 MySQL 中创建 `baihu` 和 `message_nest` 两个数据库，并修改配置中的 MySQL 地址和密码。也可以使用同一个数据库。

**在任务中使用推送**

如今你无需依赖任何外部服务，可以通过面板自身完成：
1. 进入白虎面板「消息推送」模块，新建你需要通知的渠道。
2. 可以在「事件绑定」中直接设定**自动化系统通知**（如监控任务失败或脚本异常中止时进行自动提醒）。
3. 如需在自己的脚本逻辑内动态推送，可以点击「脚本调用」页面，立刻获取一键调用的 Shell / API 代码，内嵌进你的业务逻辑中即可完成推信！

*(如继续使用 `Message-Push-Nest`，你可以通过其管理界面中「消息模板」的「复制推送代码」提取旧有版集成样例。)*

![1768143124572.png](https://f.pz.al/pzal/2026/01/11/1360cd334ff20.png)

> 提示：在 Docker Compose 部署的环境中，推送服务地址使用 `http://message-nest:8000`（容器内部通信）。如果是独立部署，请使用实际的服务地址。

</details>

> 环境变量优先级高于配置文件，两种方式可以混合使用。

<details>
<summary><b>方式四：Nginx 反向代理部署（HTTPS）</b></summary>

如果需要通过域名和 HTTPS 访问白虎面板，可以使用 Nginx 作为反向代理。

**Nginx 配置示例：**

```nginx
# 在 http 块中添加 WebSocket 升级配置
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}

server {
    listen 443 ssl http2;
    server_name example.com;
    
    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;
    
    access_log /var/log/nginx/example.access.log;
    error_log  /var/log/nginx/example.error.log warn;
    
    location / {
        proxy_pass http://172.17.0.1:8052;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        
        # WebSocket 支持（终端功能需要）
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        
        proxy_buffering off;
        proxy_read_timeout 60s;
    }
}

# HTTP 自动跳转 HTTPS（可选）
server {
    listen 80;
    server_name example.com;
    return 301 https://$server_name$request_uri;
}
```

**配置说明：**

1. 将 `example.com` 替换为你的域名
2. 修改 SSL 证书路径为你的实际路径
3. `172.17.0.1:8052` 是 Docker 容器的宿主机地址和端口，根据实际情况修改
4. WebSocket 配置是必需的，否则在线终端功能无法使用


**重载 Nginx 配置：**

```bash
nginx -t && nginx -s reload
```

</details>


### 访问面板

启动后访问：http://localhost:8052

**默认账号：** 用户名 `admin`，密码见启动日志（首次启动会自动生成 12 位随机密码并打印在日志中）

> **注意**：出于安全性考虑，系统不再使用固定默认密码。请在容器启动日志中搜索 `管理员账号创建成功` 找到您的随机密码，并登录后及时修改。

<details>
<summary><b>命令行工具 (CLI)</b></summary>

白虎面板在环境内内置了同名的 `baihu` 命令行工具。如果您在终端内需要执行系统级别的操作，可以使用以下命令：

```bash
baihu server             # 以前台方式启动面板的后台进程服务（面板启动指令）
baihu reposync           # 供定时任务调用，将远程 Git 仓库的高级特性同步到本地目录中
baihu resetpwd           # 交互式重置系统 admin 账号密码（密码丢失时可通过进入终端重置）
baihu restore <file>     # 使用本地的 .zip 备份压缩包文件，一条命令直接全量恢复系统数据
```

终端执行 `baihu` 会直接打印内置支持的高级命令帮助列表。

</details>

### 数据目录

```
./
├── baihu                 # 可执行文件
├── data/                 # 数据目录（Node 项目根目录，统一持久化所有数据与依赖）
│   ├── baihu.db          # SQLite 数据库
│   ├── package.json      # Node.js 项目配置 (管理所有全局与任务依赖)
│   ├── node_modules/     # 安装的 Node.js 依赖包 (Node 向上寻址)
│   └── scripts/          # 用户脚本工作区 (保持纯净的脚本文件)
└── configs/
    └── config.ini        # 配置文件（自动创建）
```

### Docker 启动流程

容器启动时 `docker-entrypoint.sh` 会执行以下操作：

1. **目录就绪**：检查并创建 `/app/data`、`/app/configs`、`/app/data/scripts` 目录。
2. **项目与 SDK 初始化**：自动初始化 `/app/data/package.json` 并通过 `pnpm add` 添加内建 `baihu` SDK 助手库。
3. **运行时验证**：校验 Node.js 24 与 pnpm 11.24 环境。
4. **命令行补全**：自动配置 bash 下的 `baihu` 命令行补全。
5. **启动应用**：运行 `baihu server` 后台服务进程。

> **提示**：通过挂载 `./data:/app/data`，您的所有计划任务脚本、已安装的 Node.js 依赖包以及 SQLite 数据库都会永久保存在该目录下，容器升级或重启后零配置即启即用。

## 配置说明

<details>
<summary><b>点击展开查看配置详情</b></summary>

### 配置文件

配置文件路径：`configs/config.ini`

```ini
[server]
port = 8052
host = 0.0.0.0
url_prefix =

[database]
type = sqlite
host = localhost
port = 3306
user = root
password = 
dbname = bh_panel
table_prefix = baihu_
```

### 环境变量

所有配置项都支持通过环境变量覆盖，环境变量优先级高于配置文件：

| 环境变量 | 对应配置 | 说明 | 默认值 |
|----------|----------|------|--------|
| `BH_SERVER_PORT` | server.port | 服务端口 | 8052 |
| `BH_SERVER_HOST` | server.host | 监听地址 | 0.0.0.0 |
| `BH_SERVER_URL_PREFIX` | server.url_prefix | URL 前缀，用于反向代理子路径部署 | - |
| `BH_COOKIE_NAME` | server.cookie_name | 全局会话 Cookie 名称 | BHToken |
| `BH_DB_TYPE` | database.type | 数据库类型 (sqlite/mysql) | sqlite |
| `BH_DB_HOST` | database.host | 数据库地址 | localhost |
| `BH_DB_PORT` | database.port | 数据库端口 | 3306 |
| `BH_DB_USER` | database.user | 数据库用户 | root |
| `BH_DB_PASSWORD` | database.password | 数据库密码 | - |
| `BH_DB_NAME` | database.dbname | 数据库名称 | bh_panel |
| `BH_DB_PATH` | database.path | SQLite 文件路径 | ./data/baihu.db |
| `BH_DB_TABLE_PREFIX` | database.table_prefix | 表前缀 | baihu_ |
| `BAIHU_SECRET_KEY` | - | 系统加密秘钥，用于机密功能（**注：仅支持环境变量设置，不支持配置文件**） | - |

### URL 前缀配置

如果需要通过反向代理（如 Nginx）将白虎面板部署在子路径下，可以配置 URL 前缀。

**配置方式：**

```bash
# 方式一：配置文件
[server]
url_prefix = /baihu

# 方式二：环境变量
-e BH_SERVER_URL_PREFIX=/baihu
```

**配置效果：**

配置 `url_prefix = /baihu` 后，访问路径变为：

| 类型 | 路径示例 |
|------|---------|
| 前端页面 | `http://your-domain.com/baihu/` |
| 登录页面 | `http://your-domain.com/baihu/login` |
| 任务管理 | `http://your-domain.com/baihu/tasks` |
| API 接口 | `http://your-domain.com/baihu/api/v1/*` |
| WebSocket | `ws://your-domain.com/baihu/api/v1/terminal/ws` |

**Nginx 反向代理配置示例：**

```nginx
location /baihu/ {
    proxy_pass http://localhost:8052/baihu/;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

**MySQL 示例：**

参考上方「方式一：环境变量部署」中的 MySQL 配置示例。

### 调度设置

系统采用 Worker Pool + 任务队列的架构来控制任务执行，可在「系统设置 > 调度设置」中配置：

| 设置项 | 说明 | 默认值 |
|--------|------|--------|
| Worker 数量 | 并发执行任务的 worker 数量 | 4 |
| 队列大小 | 任务队列缓冲区大小 | 100 |
| 速率间隔 | 任务启动间隔（毫秒） | 200 |

修改调度设置后立即生效，无需重启服务。

</details>

## 免责声明 ⚠️

白虎面板（Baihu Panel）仅作为一个轻量级的任务托管与调度平台，本项目及相关代码**不提供、不内置任何具有实际业务逻辑的第三方脚本**。

在使用本项目时，请您务必知悉并同意以下条款：

1. **脚本来源审核**：请勿轻易执行任何来源不明或不可信的外部脚本。所有在平台上运行的脚本及代码均需由用户自行添加或配置，用户必须在执行前仔细阅读并审核其源代码，确保其安全性。
2. **安全责任自负**：本项目作为基础调度工具，**无法且不保证任何被执行任务的安全性**。因运行不安全、违规脚本带来的一切数据泄露、系统损坏、财产损失及法律责任等后果，均由使用者自行承担，与本项目及开发者无关。
3. **软件按“原样”提供**：本项目为业余开源开发，按“原样”提供，**不保证不存在 Bug 或漏洞**。开发者不对因使用本项目而引起的任何直接或间接损失负责。

## 鸣谢

- 感谢原项目 [engigu/baihu-panel](https://github.com/engigu/baihu-panel) 及其贡献者们的开源付出。

## 许可证 

本项目采用 [Apache License 2.0](LICENSE) 协议发布，并包含额外的 [NOTICE](NOTICE) 说明。

**强制要求：** 在任何分发、修改或二次开发中，**必须完整保留原作者署名及项目名称**（详见 NOTICE 文件）。

