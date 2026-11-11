#!/usr/bin/env bash
# ============================================================
# deploy-yulin-remote.sh — YuLing 服务器端部署（Gitea Actions 调用）
# 位置: natvps /root/deploy-yulin-remote.sh
# 用法: deploy-yulin-remote.sh <源码目录>
#
# 链路: 静态检查+测试 → 编译 → 镜像(sha tag) → 导入 containerd
#       → 数据库备份 → 更新镜像 → 滚动发布 → 公网验证（失败自动回滚）
#
# 与 deploy-blog-remote.sh 的关键差异（都是踩过坑改的）：
#   1. 镜像 tag 用 git 短 sha 而不是 latest。
#      latest + imagePullPolicy:IfNotPresent 时，containerd 里 tag 指向新 digest，
#      但 kubelet 重启容器仍用缓存的旧 digest，表现为"部署成功但代码没变"。
#      不可变 tag 从根上消除这个问题，顺带让回滚和追溯变简单。
#   2. 发布失败自动 rollback，不需要人盯着。
#   3. 数据库备份用项目自己的账号，不用 root。
# ============================================================
set -euo pipefail

SRC="${1:?用法: $0 <源码目录>}"
NAMESPACE=apps
APP=yulin-server
PUBLIC=http://121.40.235.227:8000
BUILD=/tmp/yulin-build
IMAGE_REPO=yulin-server

# 数据库凭据从集群 Secret 现取，不硬编码在脚本里。
DB_USER=yulin
DB_PASS="$(k3s kubectl get secret yulin-secret -n "$NAMESPACE" -o jsonpath='{.data.MYSQL_PASSWORD}' | base64 -d)"
DB_NAME=yulin

step() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
ok()   { printf '\033[1;32m[OK]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[FAIL]\033[0m %s\n' "$*"; }

cd "$SRC"
[ -f go.mod ] || { echo "FATAL: $SRC 没有 go.mod"; exit 1; }

SHA="$(git rev-parse --short HEAD)"
TAG="${IMAGE_REPO}:${SHA}"
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
echo "分支=$BRANCH 提交=$SHA"

# ---------- 1/7 静态检查 + 测试 ----------
step "1/7 静态检查 + 单元测试"
export GOPROXY=https://goproxy.cn,direct
export GOFLAGS=-mod=mod

go vet ./...
ok "go vet 通过"

# 这里现在是真正的门禁：conf / dao / logic 都有单测，
# 其中 dao 层用 sqlmock 校验 GORM 生成的 SQL 与事务边界，不需要真实数据库。
# 任何一条失败都会让脚本在 set -e 下直接退出，不会带着坏代码往下走。
go test ./... 2>&1 | tail -20
ok "go test 通过"

# ---------- 2/7 编译 ----------
step "2/7 编译（linux/amd64 静态二进制）"
rm -rf "$BUILD"
mkdir -p "$BUILD/bin" "$BUILD/build"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$BUILD/bin/server" ./cmd/server
# migrate 二进制单独编译：服务启动时会检查业务表是否就绪，缺表直接拒绝启动，
# 所以迁移必须先于滚动发布执行，且用同一份代码编译，保证迁移逻辑和运行逻辑同源。
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$BUILD/bin/migrate" ./cmd/migrate
ok "编译完成 server=$(du -h "$BUILD/bin/server" | cut -f1) migrate=$(du -h "$BUILD/bin/migrate" | cut -f1)"

# ---------- 3/7 构建镜像 ----------
step "3/7 构建镜像 $TAG"
# 复用仓库里的离线 Dockerfile：COPY 预编译二进制 + CA 证书，零网络依赖。
# 宿主机的 docker 拉不到 docker.io（DNS 被污染），多阶段 Dockerfile 在这里不可用。
cp deploy/Dockerfile.prebuilt "$BUILD/Dockerfile"
cp /etc/ssl/certs/ca-certificates.crt "$BUILD/build/ca-certificates.crt"

DOCKER_BUILDKIT=0 docker build -q -t "$TAG" -t "${IMAGE_REPO}:latest" "$BUILD" > /dev/null
ok "镜像构建完成 $(docker images --format '{{.Size}}' "$TAG" | head -1)"

# ---------- 4/7 导入 containerd ----------
step "4/7 导入 k3s containerd"
TAR=/tmp/yulin-${SHA}.tar
docker save "$TAG" "${IMAGE_REPO}:latest" -o "$TAR"
# 先删同 tag 再导入：避免 containerd 里同名 tag 残留导致 kubelet 解析到旧 digest。
# 对不可变 sha tag 影响不大，但重复流水线（重跑同 commit）时会踩到。
k3s ctr -n k8s.io images rm "docker.io/library/${TAG}" > /dev/null 2>&1 || true
k3s ctr -n k8s.io images import "$TAR" > /dev/null 2>&1 || k3s ctr -n k8s.io images import "$TAR"
rm -f "$TAR"
k3s crictl images | grep "${IMAGE_REPO}" | head -3
ok "镜像已导入"

# ---------- 5/7 数据库备份 ----------
step "5/7 部署前数据库备份（$DB_NAME）"
BK=/root/yulin-pre-upgrade-$(date +%Y%m%d-%H%M%S).sql.gz
k3s kubectl exec -n infra deploy/mysql -- sh -c \
  "MYSQL_PWD='${DB_PASS}' mysqldump -u${DB_USER} --single-transaction --databases ${DB_NAME} 2>/dev/null | gzip > /tmp/pre-upgrade.sql.gz"
k3s kubectl exec -n infra deploy/mysql -- cat /tmp/pre-upgrade.sql.gz > "$BK"
gzip -t "$BK" || { fail "备份文件损坏"; exit 1; }
ok "备份 $BK ($(du -h "$BK" | cut -f1))"

# ---------- 6/7 同步配置 + 数据库迁移 ----------
#
# 迁移必须先于发布，顺序不可对调：
#   服务启动时会校验业务表是否就绪，缺表直接拒绝启动（见 cmd/server/main.go）。
#   如果先 set image 再迁移，新 Pod 会反复 CrashLoop，旧 Pod 顶在前面，
#   rollout 最终超时回滚，白跑一轮还留下一个坏版本在 ReplicaSet 里。
#
# 迁移实现是 GORM 的 additive schema 变更（只加表、加列），
# 所以可以放在流水线里自动执行。一旦将来出现破坏性变更
# （删列、改类型、重命名），必须改成需要人工确认的独立 job，不能继续自动跑。
step "6/7 同步 ConfigMap + 数据库迁移"

k3s kubectl apply -f "$SRC/deploy/k8s/00-configmap.yaml" > /dev/null
ok "ConfigMap 已同步"

APP_ENV=prod \
MYSQL_HOST=127.0.0.1 MYSQL_PORT=30306 \
MYSQL_DATABASE="$DB_NAME" MYSQL_USER="$DB_USER" MYSQL_PASSWORD="$DB_PASS" \
"$BUILD/bin/migrate"
ok "数据库迁移完成"

# ---------- 7/7 发布 + 验证 ----------
step "7/7 更新镜像并滚动发布"
PREV_IMAGE="$(k3s kubectl get deploy "$APP" -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].image}')"
echo "上一版本镜像: $PREV_IMAGE"

k3s kubectl set image "deploy/$APP" "server=$TAG" -n "$NAMESPACE" > /dev/null

# 300s 而不是 180s：首次拉 pause 镜像失败时 kubelet 会退避重试，
# 单是 sandbox 创建就可能耗掉 90s，180s 余量不够，会把正常发布判成超时。
if ! k3s kubectl rollout status "deploy/$APP" -n "$NAMESPACE" --timeout=300s; then
  fail "滚动发布超时，自动回滚到上一版本"
  k3s kubectl rollout undo "deploy/$APP" -n "$NAMESPACE" || true
  k3s kubectl logs -n "$NAMESPACE" -l "app=$APP" --tail=60 || true
  exit 1
fi
ok "Pod 已就绪"
k3s kubectl get pod -n "$NAMESPACE" -l "app=$APP" -o wide

# 公网验证分两级，这个划分是 2026-11-11 第一次流水线失败后改的：
#
#   内网（NodePort 回环）检验的是"这次发布的应用能不能干活"——不通过必须回滚。
#   公网（nginx + SSH 隧道）检验的是"整条入口链路通不通"——链路抖动与代码无关，
#   回滚到旧版本一样不通，所以只告警不回滚。
#
# 另一个坑：curl 用 -w 取状态码时，连接失败本身就输出 000。
# 之前写成 `... -w '%{http_code}' ... || echo 000`，两个 000 被拼成 "000000"，
# 判定必然失败，于是一次网络抖动就能让好好的发布被回滚掉。
check_http() {  # $1=url $2=期望状态码 $3=重试次数
  local url=$1 want=$2 tries=${3:-5} code='' i
  for ((i = 1; i <= tries; i++)); do
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url" 2>/dev/null) || true
    [ -z "$code" ] && code=000
    if [ "$code" = "$want" ]; then
      echo "$code"
      return 0
    fi
    echo "  第 $i/$tries 次 HTTP=$code" >&2
    if [ "$i" -lt "$tries" ]; then sleep 3; fi
  done
  echo "$code"
  return 1
}

step "内网验证（NodePort 30443 回环）"
if ! INTERNAL=$(check_http "http://127.0.0.1:30443/ready" 200 5); then
  fail "内网就绪检查失败（HTTP=$INTERNAL），回滚到上一版本"
  k3s kubectl logs -n "$NAMESPACE" -l "app=$APP" --tail=60 || true
  k3s kubectl rollout undo "deploy/$APP" -n "$NAMESPACE" || true
  exit 1
fi
ok "内网 /ready 正常（HTTP=$INTERNAL）"

step "公网验证（$PUBLIC）"
if PUBLIC_CODE=$(check_http "$PUBLIC/health" 200 5); then
  ok "公网 /health 正常（HTTP=$PUBLIC_CODE）"
else
  fail "公网链路异常（HTTP=$PUBLIC_CODE），但应用本身已就绪"
  echo "  这类问题回滚无效，保留本次部署。请检查入口链路："
  echo "    ssh aliyun 'systemctl status nginx; ss -lntp | grep 18043'"
  echo "    curl -s http://127.0.0.1:30443/ready   # 在部署机本地验"
fi

echo
echo "DEPLOY_OK commit=$SHA image=$TAG internal=$INTERNAL"
