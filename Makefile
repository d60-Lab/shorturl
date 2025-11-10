.PHONY: help generate run test benchmark clean

help:
	@echo "Fuxi 短URL系统 - 本地验证版"
	@echo ""
	@echo "使用方法:"
	@echo "  make generate    - 生成100万条短URL"
	@echo "  make run         - 启动API服务器"
	@echo "  make test        - 运行单元测试"
	@echo "  make benchmark   - 运行性能测试"
	@echo "  make stress      - 运行压力测试"
	@echo "  make clean       - 清理数据文件"
	@echo "  make quick       - 快速开始(生成+运行)"

generate:
	@echo "生成短URL数据文件..."
	@mkdir -p data
	@go run cmd/generator/main.go -count 1000000
	@echo "✓ 生成完成"

run:
	@echo "启动API服务器..."
	@go run cmd/api/main.go

test:
	@echo "运行测试..."
	@go test -v ./test/

benchmark:
	@echo "运行基准测试..."
	@go test -bench=. -benchmem ./test/

stress:
	@echo "运行压力测试..."
	@go run cmd/benchmark/main.go -c 100 -n 10000

clean:
	@echo "清理数据文件..."
	@rm -rf data/*.dat data/*.db
	@echo "✓ 清理完成"

quick: generate
	@echo ""
	@echo "启动API服务器..."
	@sleep 1
	@go run cmd/api/main.go

# 依赖管理
deps:
	@echo "安装依赖..."
	@go mod download
	@go mod tidy

# 构建二进制文件
build:
	@echo "构建二进制文件..."
	@mkdir -p bin
	@go build -o bin/fuxi-generator cmd/generator/main.go
	@go build -o bin/fuxi-api cmd/api/main.go
	@go build -o bin/fuxi-benchmark cmd/benchmark/main.go
	@echo "✓ 构建完成: bin/"

# 查看示例数据
inspect:
	@echo "=== 短URL文件信息 ==="
	@if [ -f data/shorturls.dat ]; then \
		ls -lh data/shorturls.dat; \
		echo ""; \
		echo "前10个短URL:"; \
		head -c 60 data/shorturls.dat | fold -w 6; \
	else \
		echo "文件不存在，请先运行: make generate"; \
	fi

# 查看统计
stats:
	@echo "=== 服务器统计 ==="
	@curl -s http://localhost:8080/api/stats | python3 -m json.tool
