# Playwright 示例

这组示例用于演示如何在 Baihu 中通过远程 Browserless / Chromium 服务执行浏览器自动化，并完成一次最小可用的页面截图。

当前目录包含两个脚本：

```text
example/
└── playwright/
    ├── playwright.js
    ├── playwright.py
    └── readme.md
```

- `playwright.js`：Node.js 示例，使用 `puppeteer-core` 连接 Browserless
- `playwright.py`：Python 示例，使用 `playwright` 连接 Browserless

其中 Python 方案是最主流、也最接近 Puppeteer 使用体验的一种。
同时建议在 Baihu 中为 Python 示例安装并使用 Python `3.11` 版本，因为过高版本的 Python 可能会出现依赖安装失败。

## 先安装语言依赖

运行脚本前，请先到 Baihu 的“语言依赖”页面安装对应的包，否则任务会因为缺少模块而失败。

- 运行 `playwright.js` 前，请在 Node.js 语言依赖中安装：`puppeteer-core`
- 运行 `playwright.py` 前，请在 Python 语言依赖中安装：`playwright`
- Python 运行环境建议选择：`3.11`，避免因为版本过高导致依赖安装失败

安装完成后，再创建任务执行对应脚本。

如果你当前是通过 Browserless 连接远程浏览器：

- 不要执行 `playwright install`
- 不需要额外下载 Chromium / Firefox / WebKit
- 直接使用 `connect_over_cdp` 连接远程浏览器即可

## 适用场景

- 验证 Baihu 到 Browserless 的网络连通性
- 验证远程浏览器地址和 Token 是否配置正确
- 快速测试浏览器自动化脚本是否能正常运行

## 推荐部署方式

建议配合 `browserless/chromium` 一起使用，再由 Baihu 中的脚本连接远程浏览器服务。

参考 `docker-compose.yml`：

```yaml
version: "3.8"

services:
  browser:
    image: ghcr.io/browserless/chromium:latest
    container_name: browser
    restart: unless-stopped
    environment:
      MAX_CONCURRENT_SESSIONS: 5
      MAX_QUEUE_LENGTH: 20
      CONNECTION_TIMEOUT: 300000
      DEFAULT_LAUNCH_ARGS: '["--no-sandbox","--disable-setuid-sandbox","--disable-dev-shm-usage"]'
      TOKEN: your-secret-token
      ENABLE_DEBUGGER: "false"
    shm_size: "1gb"
    mem_limit: 2g
    cpus: 2
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3

  baihu:
    image: ghcr.io/uyloal/baihu:latest
    container_name: baihu
    restart: unless-stopped
    ports:
      - "8052:8052"
    volumes:
      - ./data:/app/data
      - ./envs:/app/envs
    environment:
      - TZ=Asia/Shanghai
      - BH_SERVER_PORT=8052
      - BH_SERVER_HOST=0.0.0.0
      - BH_DB_TYPE=sqlite
      - BH_DB_PATH=/app/data/baihu.db
      - BH_DB_TABLE_PREFIX=baihu_
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    depends_on:
      - browser
```

## 使用步骤

1. 启动 `browser` 和 `baihu` 服务。
2. 在 Baihu 的“语言依赖”中安装对应包。
3. 按实际环境修改脚本中的 Browserless 地址和 Token。
4. 在 Baihu 中创建任务并运行对应脚本。

Python 示例的核心写法如下：

```python
from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(
        "http://browser:3000?token=your-secret-token"
    )
    page = browser.new_page()
    page.goto("https://www.baidu.com")
    page.screenshot(path="baidu.png")
    browser.close()
```

## 运行前检查

- Browserless 服务已经正常启动
- Baihu 可以访问 Browserless 地址
- `TOKEN` 与 Browserless 配置保持一致
- 脚本中的地址、端口和协议填写正确

## 常见问题

### 1. 报错提示找不到模块

通常是还没有在“语言依赖”中安装对应包：

- Node.js 示例需要 `puppeteer-core`
- Python 示例需要 `playwright`

### 2. 为什么没有执行 `playwright install`

这是预期行为。

如果你使用的是 Browserless 这类远程浏览器服务，Playwright 只是作为客户端发起连接，不需要在白虎容器里再下载本地浏览器，所以通常不要执行 `playwright install`。

### 3. 连接不上 Browserless

请优先检查：

- Browserless 服务是否正常启动
- Token 是否正确
- Baihu 与 Browserless 是否在同一网络中
- 地址是否写成了当前运行环境可访问的地址

### 4. 页面打开超时

可以尝试：

- 换一个更稳定的目标站点
- 调大超时时间
- 增加 Browserless 容器的 `shm_size`
- 检查容器 CPU / 内存是否不足

### 5. 没有看到截图文件

请确认：

- 脚本已经执行成功
- 截图保存路径是否正确
- 任务工作目录是否符合预期

## 说明

这组示例主要用于快速验证 Baihu 与远程浏览器服务之间的连通性，以及对应语言环境是否已经配置完成。
