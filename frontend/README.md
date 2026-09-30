# 前端（SvelteKit）

由 [`sv`](https://github.com/sveltejs/cli) 脚手架生成。**开发环境的完整搭建（含后端）见仓库根 [README](../README.md)**，
这里只留这个目录自己的命令。

依赖用 **pnpm**：版本来自 `package.json` 的 `packageManager` 字段，`package-lock.json` / `bun.lock`
一律不入库（多把锁并存会让 CI 认错包管理器，这是历史上踩过的坑）。

```sh
pnpm install            # 装依赖（--frozen-lockfile 由 CI 使用）
pnpm run dev            # http://localhost:5173，已代理 /api → :8080
pnpm run build          # 产物在 build/
pnpm run test:coverage  # Vitest + 覆盖率（CI 跑这条）
pnpm run check          # svelte-kit sync + svelte-check
```

## 重新生成脚手架

要用同样的配置重建本工程时（`--no-install` 保留依赖由 pnpm 管）：

```sh
pnpm dlx sv@latest create --template minimal --types ts \
  --add tailwindcss="plugins:none" prettier eslint sveltekit-adapter="adapter:static" \
  --no-install frontend
```
