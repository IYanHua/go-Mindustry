# mdt-server Makefile
# 支持 Windows / Linux / macOS

BIN_DIR    := bin
BIN_NAME   := mdt-server
MODULE_DIR := ./cmd/mdt-server
PLUGIN_DIR := ./custom_plugin
MODS_DIR   := mods

ifeq ($(GOOS),windows)
  BIN_EXT := .exe
else
  BIN_EXT :=
endif

.PHONY: all build build-plugin clean test test-cover run dev lint fmt tidy \
        build-windows build-linux build-macos

# 默认目标
all: build

# 构建服务器
build:
	@echo "正在构建 mdt-server ($(GOOS))..."
	@go build -o $(BIN_DIR)/$(BIN_NAME)$(BIN_EXT) $(MODULE_DIR)
	@echo "构建完成: $(BIN_DIR)/$(BIN_NAME)$(BIN_EXT)"

# 构建插件
build-plugin:
	@echo "正在构建插件..."
	@go build -buildmode=plugin -o $(MODS_DIR)/main.plugin $(PLUGIN_DIR)
	@echo "插件构建完成: $(MODS_DIR)/main.plugin"

# 清理
clean:
	@echo "正在清理..."
	@-rm -rf $(BIN_DIR)/*
	@-rm -f *.txt *.py
	@-rm -rf temp-*
	@echo "清理完成"

# 测试
test:
	@echo "运行测试..."
	@go test ./...

# 测试 + 覆盖率
test-cover:
	@echo "运行测试（覆盖率）..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

# 运行服务器
run: build
	@echo "启动服务器..."
	@$(BIN_DIR)/$(BIN_NAME)$(BIN_EXT)

# 开发模式（带调试信息）
dev:
	@echo "开发模式构建..."
	@go build -gcflags="all=-N -l" -o $(BIN_DIR)/$(BIN_NAME)-dev$(BIN_EXT) $(MODULE_DIR)

# Lint（需要安装 golangci-lint）
lint:
	@echo "运行 golangci-lint..."
	@golangci-lint run ./...

# 格式化
fmt:
	@echo "格式化代码..."
	@go fmt ./...

# 整理依赖
tidy:
	@echo "整理 go.mod..."
	@go mod tidy
	@cd $(PLUGIN_DIR) && go mod tidy

# 交叉编译 Windows
build-windows:
	@echo "交叉编译 Windows amd64..."
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(BIN_NAME)-win64.exe $(MODULE_DIR)

# 交叉编译 Linux
build-linux:
	@echo "交叉编译 Linux amd64..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(BIN_NAME)-linux64 $(MODULE_DIR)

# 交叉编译 macOS
build-macos:
	@echo "交叉编译 macOS amd64..."
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(BIN_NAME)-macos64 $(MODULE_DIR)
