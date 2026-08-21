# BENZHI_README

## 项目说明
- 项目：benzhi-project-f14750f4-9030-439e-b7eb-42dd90da4935
- 项目用途：舞台吊挂演出就绪验证台已完整实现。Go 服务提供嵌入式浏览器工作台和同源 JSON API，使用追加式哈希链日志保存事实事件，支持设备与动作配置、干运行实测、违规反馈、整改重测、安全复核及不可变演出就绪证书签发。
- Go 工具链：`golang:1.26`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/stageready -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-f14750f4-9030-439e-b7eb-42dd90da4935-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-f14750f4-9030-439e-b7eb-42dd90da4935-arm64 linux/arm64
docker run -it benzhi-project-f14750f4-9030-439e-b7eb-42dd90da4935-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/stageready -selfcheck -addr=127.0.0.1:19081`
