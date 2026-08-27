#!/bin/bash
set -e
export LANG=C.UTF-8
export LC_ALL=C.UTF-8
export TZ=${TZ:-Asia/Shanghai}

COLOR_PREFIX="\033[1;36m[Entrypoint]\033[0m"
log() {
    printf "${COLOR_PREFIX} %s\n" "$1"
}

log "Initializing Development Environment (Alpine / Root)..."

# ============================
# 基础目录创建
# ============================
mkdir -p /app/web/node_modules /app/data /app/data/scripts /app/configs /go /root/.pnpm/bin

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

if [ -d "/app/packages/baihu" ]; then
  log "Adding local baihu SDK to data workspace via pnpm add..."
  pnpm add /app/packages/baihu --prefer-offline 2>&1 | tail -n 5 || pnpm add /app/packages/baihu 2>&1 | tail -n 5 || true
fi

# ============================
# 运行时环境验证
# ============================
log "Checking Development Runtime..."
log "  - Node:    $(node --version 2>&1 || echo "not found")"
log "  - pnpm:    $(pnpm --version 2>&1 || echo "not found")"
log "  - Go:      $(go version 2>&1 || echo "not found")"
log "  - Git:     $(git --version 2>&1 || echo "not found")"

# ============================
# 配置 bash 命令行补全
# ============================
for rcfile in /etc/bash.bashrc /etc/bashrc /root/.bashrc /etc/profile; do
  if [ -f "$rcfile" ] || [ "$rcfile" = "/root/.bashrc" ]; then
    if ! grep -q "baihu completion" "$rcfile" 2>/dev/null; then
      echo 'eval "$(baihu completion bash 2>/dev/null)"' >> "$rcfile" 2>/dev/null || true
    fi
  fi
done

log "Environment ready! Starting command as root..."

cd /app
exec "$@"
