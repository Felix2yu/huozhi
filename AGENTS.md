# 货殖（huozhi）项目指令

前端位于 `frontend/`：SvelteKit 2 + Svelte 5（runes）+ Vite 8 + Tailwind CSS v4，后端为 Go + GORM，纯 Go SQLite 驱动（CGO-free）。

## 包管理器（强制）

本项目前端统一使用 **pnpm 11.22.0**，不使用 npm / yarn：

- `frontend/package.json` 已声明 `"packageManager": "pnpm@11.22.0"`，锁文件为 `frontend/pnpm-lock.yaml`
- 安装依赖：`pnpm install`
- 添加 / 移除依赖：`pnpm add <pkg>` / `pnpm remove <pkg>`
- 运行脚本：`pnpm run <script>`
- **禁止使用 `npm install` / `npm ci` / `npm run` / `npx` 安装依赖或运行脚本**——锁文件是 pnpm 格式，
  npm 会生成 `package-lock.json` 并破坏 pnpm 的依赖树结构，还会让 CI 与本地构建结果不一致。

## 开发命令（均在 `frontend/` 下执行）

| 命令 | 说明 |
|------|------|
| `pnpm dev` | 启动 SvelteKit 开发服务器 |
| `pnpm build` | 构建前端产物 |
| `pnpm preview` | 预览构建产物 |
| `pnpm check` | SvelteKit sync + svelte-check 类型检查 |
| `pnpm lint` | Prettier 检查 + ESLint |
| `pnpm format` | Prettier 格式化 |
| `pnpm test` | 运行 Vitest |

## 后端开发（仓库根）

- 构建：`go build ./...`
- 测试：`go test ./...`

## 前端内嵌（go:embed）

后端通过 `//go:embed all:dist` 把前端产物内嵌进二进制（`backend/internal/web`），
运行时默认直接托管内嵌产物，无需 nginx、镜像里也不必再放一份静态文件。

- 内嵌目录：`backend/internal/web/dist`（产物只在本地/CI 生成，不入库；
  目录里常驻 `.gitkeep` 占位，保证未构建前端时 `go build` / `go test` 仍能编译）
- 构建前先把产物摆到位（CI 与本地同一条命令）：

  ```sh
  mkdir -p backend/internal/web/dist && cp -R frontend/build/. backend/internal/web/dist/
  ```

- 覆盖：配置 `server.static_dir`（或 `HZ_STATIC_DIR`）指向磁盘目录时以磁盘目录优先，
  可不重新编译就替换前端；目录不存在则自动回退内嵌产物。
