# BENZHI_README

基于 Go 实现的频率变更启用审查台 HTTP API 项目，一款后端服务，已完整实现频率变更启用审查台：以 SQLite 原子保存版本化基线、核验、冲突、审计链和不可变许可，并通过版本化 JSON HTTP API 完成登记、送审、整改、复核、冻结、审批与验证流程。

## 项目说明
- 项目：benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875
- 项目用途：已完整实现频率变更启用审查台：以 SQLite 原子保存版本化基线、核验、冲突、审计链和不可变许可，并通过版本化 JSON HTTP API 完成登记、送审、整改、复核、冻结、审批与验证流程。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875-arm64 linux/arm64
docker run -it benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
