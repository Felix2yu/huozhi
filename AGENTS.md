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

### 前端目前没有静态类型覆盖（已知缺陷，别把绿灯当通过）

三层都是空的，2026-10-16 实测：

1. `pnpm check` 走 `svelte-check --tsgo`，它**不产生任何类型诊断**。往 `src/lib/utils/cn.ts`
   加 `export const x: number = "str"`、往某个 `.svelte` 的 `<script lang="ts">` 加未声明标识符，
   `--tsgo` 依旧 `found 0 errors and 10 warnings`；同一条命令去掉 `--tsgo` 报 290 errors，
   两处注入都被抓到。原因：`#lib/*` 声明在 `package.json` 的 `imports` 里，`--tsgo` 的虚拟文件
   布局解析不到，类型程序是空的 —— 既不报错，也不检查。
2. CI 从不跑 `check`：`ci.yml` 没设 `web-build-command`，reusable workflow 默认执行
   `pnpm run build`，而 build = `vite build && node scripts/build-sw.mjs`，不含类型检查。
3. `pnpm test` / `test:coverage` 走 Vitest + esbuild，只转译不查类型，所以它绿也不能补这一层。

去掉 `--tsgo` 也不是现成的解法：`#lib/stores/app` 指向 `app.svelte.ts`，TS 的扩展名解析映射不到，
光这一类就 288 条。在 tsconfig 补 `"paths": {"#lib/*": ["./src/lib/*.svelte.ts", "./src/lib/*.svelte.js", "./src/lib/*"]}`
加 `"types": ["$app/types", "vitest/globals", "node"]` 能把它压到 **66 条**（`plain` 与
`--tsgo-experimental-api` 两条路径数目一致），但还没法直接采用：

- 子配置的 `paths` 是**整体覆盖**而不是合并，`$app/tsconfig` 继承来的 `$components`/`$api`/`$stores`
  等映射会被顶掉，实测反而多出 2 条 `$app/environment` 解析失败；要用就得把这些条目一并抄全。
- **不要加 `baseUrl`**，TS7 已移除该选项，加了 `--tsgo` 会立刻报
  `Option 'baseUrl' has been removed`。
- 其中 37 条 `'"disabled"' does not exist in ...` 是**真实缺陷**，已定位并实测：`src/lib/components/ui/Button.svelte`
  的 props 写成 `} & HTMLAttributes<HTMLButtonElement> = $props()`，而 `svelte/elements` 的泛型
  `HTMLAttributes<T>` 只含全局属性、不含元素专属属性，`disabled` 不在其中；组件内部把 `...rest` 透传给
  `<button>`，运行时是好的，类型上是漏的。改成 `& HTMLButtonAttributes` 后这 37 条全消（66 → 29）。
  全仓只有 Button 用这个写法，其余 8 条 `"for"` 来自别的组件，要按同样思路逐个看。
- 剩下的 29 条里混着真实问题（`'previewTx' is possibly 'null'`、`Property 'role' does not exist on type 'Book'`、
  `Type 'number' is not assignable to type 'string'`、`Object is possibly 'undefined'`）和上面 `paths`
  覆盖带来的 2 条 `$app/environment`，需要逐条判：前者该修代码，后者该把继承来的映射抄全。

恢复覆盖时优先走 `--tsgo-experimental-api`（它与 plain 数目一致、用的是 TS7），把上面几条收干净后，
再把 `check` 挂进 CI 的 `web-build-command`。

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
