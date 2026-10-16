# 货殖（huozhi）项目指令

前端位于 `web/`：SvelteKit 2 + Svelte 5（runes）+ Vite 8 + Tailwind CSS v4，后端为 Go + GORM，纯 Go SQLite 驱动（CGO-free）。

## 包管理器（强制）

本项目前端统一使用 **pnpm 11.22.0**，不使用 npm / yarn：

- `web/package.json` 已声明 `"packageManager": "pnpm@11.22.0"`，锁文件为 `web/pnpm-lock.yaml`
- 安装依赖：`pnpm install`
- 添加 / 移除依赖：`pnpm add <pkg>` / `pnpm remove <pkg>`
- 运行脚本：`pnpm run <script>`
- **禁止使用 `npm install` / `npm ci` / `npm run` / `npx` 安装依赖或运行脚本**——锁文件是 pnpm 格式，
  npm 会生成 `package-lock.json` 并破坏 pnpm 的依赖树结构，还会让 CI 与本地构建结果不一致。

## 开发命令（均在 `web/` 下执行）

| 命令 | 说明 |
|------|------|
| `pnpm dev` | 启动 SvelteKit 开发服务器 |
| `pnpm build` | 构建前端产物 |
| `pnpm preview` | 预览构建产物 |
| `pnpm check` | SvelteKit sync + svelte-check 类型检查 |
| `pnpm lint` | Prettier 检查 + ESLint |
| `pnpm format` | Prettier 格式化 |
| `pnpm test` | 运行 Vitest |

### 前端类型检查：已恢复真实覆盖（`check` 绿灯 = 类型真的过）

`pnpm check` = `svelte-kit sync && svelte-check --tsconfig ./tsconfig.json`（**plain 路径，不带 `--tsgo`**），
基线 `0 errors and 10 warnings`（10 条是 `<slot>` 弃用 + 自闭合非 void 标签的既有告警）。

历史背景（2026-10-16 实测）：`--tsgo` 的虚拟文件布局解析不到只声明在 `package.json` imports 里的
`#lib/*`，类型程序是空的——注入 `export const x: number = "str"` 仍报 `0 errors`，是假绿灯。
TS7 的 checker 本体对 svelte-check 仍是 experimental，所以改用 TS6 plain 路径，
`@typescript/native`（npm:typescript@^7）依赖已移除，`typescript` 留在 `~6.0.0`。

现在必须守住的几条约束（改 tsconfig 前先看）：

- `web/tsconfig.json` 里的 `paths` 是**必需**的：镜像 `#lib` / `#lib/*`（含
  `./src/lib/*.svelte.ts`、`*.svelte.js` 候选），并且**把 `$app/tsconfig` 继承来的 10 条一并抄全**
  ——子配置 `paths` 是整体覆盖不是合并，漏抄会集体解析失败。
- **不要加 `baseUrl`**：TS7 已移除该选项，加了其它检查模式会直接崩。
- **不要走 `--tsgo` / `--tsgo-experimental-api`**：plain 路径的类型程序覆盖 4787 个文件
  （`svelte-check --output machine` 的 `COMPLETED` 行），别名撤掉后这两个模式会被版本守卫直接拒掉
  （`svelte-check failed`）。更要紧的是它们此前的行为：别名还在时，因为解析不到 `#lib/*`，
  类型程序里只装进 54 个转译产物并**静默报 0 errors** —— 注入硬错误也不涨，是假绿灯。
- `exclude` 里的 `build` / `dist` / `coverage` 不能省：本仓没有 `include`，默认 `**/*` 会把
  vite 产物灌进类型程序，`checkJs: true` 下压缩后的 JS 会贡献上千条诊断。
- `.svelte.ts` 模块用 **`from './x.svelte'`** 形式导入（先例：`#lib/stores/rates.svelte`）；
  写成裸 `./x` 只有 Vite 能解析，TS 侧会报 Cannot find module。
- Kit 3 里 `$app/environment` 已废弃且无类型，用 `$app/env`（`browser` / `dev` / `building` / `version`）。
- `virtual:pwa-register` 的声明放在 `src/pwa-register.d.ts`（全局脚本文件）：写进带 `export {}` 的
  `app.d.ts` 只会成为模块增强，照样报 Cannot find module。

验证覆盖是真的而不是换了个标签：往 `src/lib/utils/cn.ts` 加 `export const __probe: number = "str"`、
往某个 `.svelte` 的 `<script lang="ts">` 加未声明标识符，`pnpm run check` 必须**失败**（两处都抓到，
exit code 1）；`--tsgo` 那套在这两处都是假通过。

CI 侧：`ci.yml` 的 `test` job 已设 `web-build-command`（先 `pnpm run check` 再 `pnpm run build`），
`build` 本身不含类型检查，所以 check 不接进 CI 就等于没有静态类型闸门。

## 后端开发（仓库根）

- 构建：`go build ./...`
- 测试：`go test ./...`

## 前端内嵌（go:embed）

后端通过 `//go:embed all:dist` 把前端产物内嵌进二进制（`internal/web`），
运行时默认直接托管内嵌产物，无需 nginx、镜像里也不必再放一份静态文件。

- 内嵌目录：`internal/web/dist`（产物只在本地/CI 生成，不入库；
  目录里常驻 `.gitkeep` 占位，保证未构建前端时 `go build` / `go test` 仍能编译）
- 构建前先把产物摆到位（CI 与本地同一条命令）：

  ```sh
  mkdir -p internal/web/dist && cp -R web/build/. internal/web/dist/
  ```

- 覆盖：配置 `server.static_dir`（或 `HZ_STATIC_DIR`）指向磁盘目录时以磁盘目录优先，
  可不重新编译就替换前端；目录不存在则自动回退内嵌产物。
