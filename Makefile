# 本地开发 / 构建 / 部署的常用入口。
# 所有涉及密码的目标都不把密码写进命令行参数——命令行参数会出现在
# ps、shell history 和 CI 日志里。这里统一从环境变量读取。

APP        := yulin-server
IMAGE      := $(APP):latest
NS         := apps
BUILD_NODE := liuliango8j251gfjuap

.PHONY: help
help:
	@echo "make run          本地启动（需先 export 环境变量，见 .env.example）"
	@echo "make build        编译 linux/amd64 二进制"
	@echo "make vet          静态检查"
	@echo "make test         运行单测"
	@echo "make image        构建容器镜像（在构建机上执行）"
	@echo "make deploy       应用 K8s 清单（不包含 Secret）"
	@echo "make secret       交互式创建 Secret，不落地文件"
	@echo "make logs         查看 Pod 日志"
	@echo "make rollback     回滚到上一版本"

.PHONY: run
run:
	go run ./cmd/server

.PHONY: preview migrate
preview:
	DEMO_MODE=true SERVER_PORT=8090 go run ./cmd/server

migrate:
	go run ./cmd/migrate

# 目标平台写死 linux/amd64：本机是 Windows，
# 不指定的话产出 .exe 无法在容器里运行。
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./... -count=1

.PHONY: image
image:
	docker build -t $(IMAGE) .

.PHONY: deploy
deploy:
	kubectl apply -f deploy/k8s/00-configmap.yaml
	kubectl apply -f deploy/k8s/02-deployment.yaml
	kubectl rollout status deploy/$(APP) -n $(NS) --timeout=120s

# 用 read -s 避免密码回显，用 --dry-run 管道避免明文写入文件。
.PHONY: secret
secret:
	@read -s -p "MySQL 密码(yulin 账号): " MP; echo; \
	 read -s -p "Redis 密码: " RP; echo; \
	 kubectl create secret generic yulin-secret -n $(NS) \
	   --from-literal=MYSQL_PASSWORD="$$MP" \
	   --from-literal=REDIS_PASSWORD="$$RP" \
	   --dry-run=client -o yaml | kubectl apply -f -

.PHONY: logs
logs:
	kubectl logs -f deploy/$(APP) -n $(NS)

.PHONY: rollback
rollback:
	kubectl rollout undo deploy/$(APP) -n $(NS)
