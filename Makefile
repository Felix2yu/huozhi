# huozhi —— 目标约定见 Felix2yu/.github/docs/makefile.md
#
# 七个必备目标：build / dev / test / lint / fmt / docker / clean
# 其余目标按仓库自身需要添加。
.PHONY: build dev test lint lint-strict fmt docker clean

BIN  := bin/huozhi-server
MAIN := ./cmd/huozhi-server

## 产出 $(BIN)。前端 SvelteKit 构建到 web/build，再摆进 internal/web/dist 供 go:embed
build:
	cd web && pnpm install --frozen-lockfile && pnpm run build
	rm -rf internal/web/dist
	mkdir -p internal/web/dist
	cp -R web/build/. internal/web/dist/
	@touch internal/web/dist/.gitkeep
	mkdir -p bin
	go build -trimpath -ldflags "-s -w" -o $(BIN) $(MAIN)

## 前端热更新另开终端：cd web && pnpm run dev
dev:
	go run $(MAIN)

test:
	go test ./...

lint:
	go vet ./...
	cd web && pnpm run check

lint-strict: lint
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else echo "  （未安装 golangci-lint，跳过）"; fi

fmt:
	gofmt -w $$(git ls-files '*.go')

docker: build
	docker build -t huozhi:dev .

clean:
	rm -rf bin web/build web/.svelte-kit internal/web/dist/*
