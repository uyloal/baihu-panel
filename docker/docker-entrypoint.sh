#!/bin/sh
set -e
export LANG=C.UTF-8
export LC_ALL=C.UTF-8
export TZ=${TZ:-Asia/Shanghai}

# 日志输出格式
COLOR_PREFIX="\033[1;36m[Entrypoint]\033[0m"
log() {
  printf "${COLOR_PREFIX} %s\n" "$1"
}

log "Starting Baihu environment initialization..."

# ============================
# 创建基础目录
# ============================
mkdir -p \
  /app/data \
  /app/data/scripts \
  /app/configs \
  /root/.pnpm/bin

if [ -d "/app/example" ]; then
  mkdir -p /app/data/scripts/example
  rsync -a --ignore-existing /app/example/ /app/data/scripts/example/ || true
  log "Example scripts synced to /app/data/scripts/example"
fi

# ============================
# 初始化 /app/data 标准 Node.js 项目
# ============================
DATA_DIR="/app/data"
cd "$DATA_DIR"

if [ ! -f "$DATA_DIR/package.json" ]; then
  log "Initializing package.json in $DATA_DIR..."
  cat << 'EOF' > "$DATA_DIR/package.json"
{
  "name": "baihu-data",
  "version": "1.0.0",
  "type": "module",
  "private": true,
  "description": "Node.js environment for Baihu Panel"
}
EOF
fi

# 确保 packages/baihu 依赖就绪
if [ -d "/app/packages/baihu" ]; then
  log "Adding local baihu SDK to data workspace via pnpm add..."
  pnpm add /app/packages/baihu --prefer-offline 2>&1 | tail -n 5 || pnpm add /app/packages/baihu 2>&1 | tail -n 5 || true
fi

# ============================
# 运行时环境验证
# ============================
log "Checking Node.js & pnpm runtime..."
log "  - Node: $(node --version 2>&1 || echo "not found")"
log "  - pnpm: $(pnpm --version 2>&1 || echo "not found")"
log "  - git:  $(git --version 2>&1 || echo "not found")"

# ============================
# 将 baihu 注册到全局命令并配置自动补全
# ============================
ln -sf /app/baihu /usr/local/bin/baihu 2>/dev/null || true

for rcfile in /etc/bash.bashrc /etc/bashrc /root/.bashrc /root/.profile; do
  if [ -f "$rcfile" ] || [ "$rcfile" = "/root/.bashrc" ]; then
    if ! grep -q "baihu completion" "$rcfile" 2>/dev/null; then
      echo 'eval "$(baihu completion bash 2>/dev/null)"' >> "$rcfile" 2>/dev/null || true
    fi
  fi
done

# ============================
# 启动应用
# ============================
printf "\n\033[1;32m>>> Environment setup complete. Starting Baihu Server on :8052 ...\033[0m\n\n"

cd /app
exec /app/baihu server
