# 快速部署

白虎面板采用极简架构，基于 **Node.js 24 LTS + Alpine**，原生内置 pnpm 11.24。所有数据和脚本均在 `data/` 目录下自包含持久化管理，开箱即用。

## 基础镜像

| 镜像地址 | 架构 | 说明 |
| :--- | :--- | :--- |
| `ghcr.io/uyloal/baihu:latest` | `linux/amd64`, `linux/arm64` | **单一镜像**：集成 Node.js 24 LTS 与 pnpm 11.24，极致轻量，开箱即用 |

---

## 方式一：Docker Compose 部署（推荐）

最简单、便于维护的部署方式。

### 核心部署模板 (`docker-compose.yml`)
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
      # 可选环境变量：
      # - BH_SERVER_PORT=8052
      # - BH_SERVER_HOST=0.0.0.0
      # - BH_SERVER_URL_PREFIX=/baihu  # 反向代理子路径
      # - BAIHU_SECRET_KEY=your_secret_key_here  # 机密数据加密秘钥
      # - BH_DB_TYPE=sqlite
      # - BH_DB_PATH=/app/data/baihu.db
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    restart: unless-stopped
```

启动命令：
```bash
docker compose pull
docker compose up -d
```

---

## 方式二：Docker CLI 运行

通过 `docker run` 命令直接启动：

### SQLite（默认，开箱即用）
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

### MySQL（可选外置数据库）
```bash
docker run -d \
  --name baihu \
  -p 8052:8052 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  -e TZ=Asia/Shanghai \
  -e BH_DB_TYPE=mysql \
  -e BH_DB_HOST=192.168.1.100 \
  -e BH_DB_PORT=3306 \
  -e BH_DB_USER=root \
  -e BH_DB_PASSWORD=your_password \
  -e BH_DB_NAME=baihu \
  -e BH_DB_TABLE_PREFIX=baihu_ \
  -e BAIHU_SECRET_KEY=your_secret_key_here \
  --restart unless-stopped \
  ghcr.io/uyloal/baihu:latest
```

---

## Docker 启动流程与数据持久化

容器启动时 `docker-entrypoint.sh` 会自动执行以下初始化：

1. **目录检查**：自动检查并就绪 `/app/data`、`/app/configs`、`/app/data/scripts` 目录。
2. **初始化项目与 SDK**：自动初始化 `/app/data/package.json` 并通过 `pnpm add` 注册内建 `@baihu` SDK 助手库。
3. **运行时验证**：验证 Node.js 24 与 pnpm 11.24 环境状态。
4. **命令行补全**：自动配置 bash 下的 `baihu` 命令行补全。
5. **主进程启动**：执行 `baihu server` 启动后台服务进程。

> **持久化说明**：通过挂载宿主机目录 `./data:/app/data` 和 `./configs:/app/configs`，您的定时任务脚本、安装的 Node 依赖模块以及 SQLite 数据库文件均持久化在宿主机，容器升级或销毁重建不会丢失数据。

---

## 自动更新 Docker 镜像 (Watchtower)

推荐使用 **Watchtower** 实现容器全自动无感更新：

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e TZ=Asia/Shanghai \
  -e WATCHTOWER_SCHEDULE="0 0 3 * * *" \
  -e WATCHTOWER_CLEANUP=true \
  --restart unless-stopped \
  containrrr/watchtower \
  baihu
```

### 访问面板

启动成功后，使用浏览器访问：`http://localhost:8052`
* **默认账号**：用户名 `admin`，初始随机密码会在控制台首次启动日志中打印（执行 `docker logs baihu` 查看），登录后请第一时间修改密码。



