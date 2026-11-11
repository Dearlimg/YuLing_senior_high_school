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

# ---------- 1/6 静态检查 + 测试 ----------
step "1/6 静态检查 + 单元测试"
export GOPROXY=https://goproxy.cn,direct
export GOFLAGS=-mod=mod

go vet ./...
ok "go vet 通过"

# 目前没有测试文件，go test 会输出 "no test files" 并返回 0；
# 一旦补上单测，这一步会自动变成真正的门禁，不需要改脚本。
go test ./... 2>&1 | tail -20
ok "go test 通过"

# ---------- 2/6 编译 ----------
step "2/6 编译（linux/amd64 静态二进制）"
rm -rf "$BUILD"
mkdir -p "$BUILD/bin" "$BUILD/build"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$BUILD/bin/server" ./cmd/server
ok "编译完成 $(du -h "$BUILD/bin/server" | cut -f1)"

# ---------- 3/6 构建镜像 ----------
step "3/6 构建镜像 $TAG"
# 复用仓库里的离线 Dockerfile：COPY 预编译二进制 + CA 证书，零网络依赖。
# 宿主机的 docker 拉不到 docker.io（DNS 被污染），多阶段 Dockerfile 在这里不可用。
cp deploy/Dockerfile.prebuilt "$BUILD/Dockerfile"
cp /etc/ssl/certs/ca-certificates.crt "$BUILD/build/ca-certificates.crt"

DOCKER_BUILDKIT=0 docker build -q -t "$TAG" -t "${IMAGE_REPO}:latest" "$BUILD" > /dev/null
ok "镜像构建完成 $(docker images --format '{{.Size}}' "$TAG" | head -1)"

# ---------- 4/6 导入 containerd ----------
step "4/6 导入 k3s containerd"
TAR=/tmp/yulin-${SHA}.tar
docker save "$TAG" "${IMAGE_REPO}:latest" -o "$TAR"
# 先删同 tag 再导入：避免 containerd 里同名 tag 残留导致 kubelet 解析到旧 digest。
# 对不可变 sha tag 影响不大，但重复流水线（重跑同 commit）时会踩到。
k3s ctr -n k8s.io images rm "docker.io/library/${TAG}" > /dev/null 2>&1 || true
k3s ctr -n k8s.io images import "$TAR" > /dev/null 2>&1 || k3s ctr -n k8s.io images import "$TAR"
rm -f "$TAR"
k3s crictl images | grep "${IMAGE_REPO}" | head -3
ok "镜像已导入"

# ---------- 5/6 数据库备份 ----------
step "5/6 部署前数据库备份（$DB_NAME）"
BK=/root/yulin-pre-upgrade-$(date +%Y%m%d-%H%M%S).sql.gz
k3s kubectl exec -n infra deploy/mysql -- sh -c \
  "MYSQL_PWD='${DB_PASS}' mysqldump -u${DB_USER} --single-transaction --databases ${DB_NAME} 2>/dev/null | gzip > /tmp/pre-upgrade.sql.gz"
k3s kubectl exec -n infra deploy/mysql -- cat /tmp/pre-upgrade.sql.gz > "$BK"
gzip -t "$BK" || { fail "备份文件损坏"; exit 1; }
ok "备份 $BK ($(du -h "$BK" | cut -f1))"

# ---------- 6/6 发布 + 验证 ----------
step "6/6 更新镜像并滚动发布"
PREV_IMAGE="$(k3s kubectl get deploy "$APP" -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].image}')"
echo "上一版本镜像: $PREV_IMAGE"

k3s kubectl set image "deploy/$APP" "server=$TAG" -n "$NAMESPACE" > /dev/null

if ! k3s kubectl rollout status "deploy/$APP" -n "$NAMESPACE" --timeout=180s; then
  fail "滚动发布超时，自动回滚到上一版本"
  k3s kubectl rollout undo "deploy/$APP" -n "$NAMESPACE" || true
  k3s kubectl logs -n "$NAMESPACE" -l "app=$APP" --tail=60 || true
  exit 1
fi
ok "Pod 已就绪"
k3s kubectl get pod -n "$NAMESPACE" -l "app=$APP" -o wide

# 公网验证：/ready 会真的 ping MySQL 和 Redis，
# 比只看 /health 更能证明这一版是"能干活"而不只是"进程起来了"。
step "公网验证"
HEALTH=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$PUBLIC/health" || echo 000)
READY=$(curl -s --max-time 10 "$PUBLIC/ready" || echo '{}')
echo "health HTTP=$HEALTH"
echo "ready=$READY"

if [ "$HEALTH" != "200" ] || ! echo "$READY" | grep -q '"mysql":"up"'; then
  fail "公网验证未通过，自动回滚"
  k3s kubectl rollout undo "deploy/$APP" -n "$NAMESPACE" || true
  exit 1
fi

echo
echo "DEPLOY_OK commit=$SHA image=$TAG http=$HEALTH"
