# 货殖 —— 个人记账应用

一个使用 Go + Svelte/SvelteKit/TailwindCSS 构建的现代个人记账系统，
支持 Docker 一键部署，响应式界面 + PWA 支持，手机/桌面皆可良好使用。

---

## 功能模块

| 模块 | 说明 |
|-----|------|
| 用户与认证 | 注册/登录、JWT、个人资料、安全设置 |
| 多账本 | 账本创建/切换/归档、共享账本与成员角色 |
| 账户/资产管理 | 现金、储蓄、信用卡、储值卡、投资、负债，资产分组、余额调整、信用额度、账单日/还款日 |
| 分类管理 | 收入/支出两级分类，系统分类，自定义图标/颜色/排序，归档 |
| 交易记账 | 收入/支出/转账/退款/报销/调整，标签、图片、商家、地点、记账日期任意选 |
| 标签中心 | 标签云、使用次数统计、标签筛选流水 |
| 预算管理 | 总预算/分类预算，月/年/自定义周期，进度条、日均、超支预警 |
| 统计分析 | 饼图/折线图/柱状图，分类/账户/标签/日/月维度，Top支出，资产净值曲线，收支趋势 |
| 存钱计划 | 目标金额+目标日期，进度条、存钱记录、日均需要存 |
| 周期记账 | 日/周/两周/月/年/自定义间隔到期自动入账 |
| 分期管理 | 消费分期，自动生成每期还款日历 |
| 报销管理 | 报销单管理，关联账单并自动标记 |
| 导入/导出 | CSV 模板下载，微信/支付宝账单自动解析并入库 |
| **AI 助手（MCP）** | 内置 MCP 服务端，AI 可用自然语言搜索/分析/增删改账单，详见 [docs/MCP.md](docs/MCP.md) |
| **多币种与汇率** | 22 种货币，自动拉取免密钥公开汇率源并定时刷新，外币账单自动折算为基准货币，录入/列表实时显示汇率与折算金额 |
| 其他 | 资产快照（每日），自定义月起始日，多设备云同步（按用户） |

## 多币种与汇率

记账时可选择任意币种；非基准货币的账单会**自动代入当前汇率**并实时显示折算后的基准币金额，
账单列表与详情同样给出「原币金额 × 汇率 = 基准币金额」，统计口径一律以基准币为准。

- **基准货币**：设置页 →「基准货币与汇率」，可配置基准货币、自动刷新开关与刷新间隔（6/12/24 小时）。
- **数据源**：默认 `open.er-api.com`，失败自动回退到 `@fawazahmed0/currency-api` 公共镜像，均无需 API Key。
  全部失败时沿用上一次成功拉取的快照（界面标记「已过期」），不会让外币记账变成 0 折算。
- **离线/离网部署**：`config.yaml` 里设 `fx.disabled: true`（或环境变量 `HZ_FX_DISABLED=true`）即可完全关闭拉取，
  此时仍可在记账表单手工填写汇率。
- 相关配置：`fx.provider` / `fx.endpoint`（自建镜像）/ `fx.timeout_seconds` / `fx.refresh_interval_hours` /
  `fx.default_base` / `fx.symbols`。

## MCP：让 AI 直接操作账单

货殖内置 MCP（Model Context Protocol）服务端，接入后 AI 助手（Claude Desktop / Cursor /
Cherry Studio 等）可以直接用自然语言记账、查账、做消费分析，无需手动调用接口。

```bash
# 1) 设置页生成 API 密钥 → 2) 在 AI 客户端里加一条 MCP 配置
{
  "mcpServers": {
    "huozhi": {
      "url": "https://your-domain.com/api/mcp",
      "headers": { "X-API-Key": "你的API密钥" }
    }
  }
}
```

然后就可以直接问：

```
「上个月我在餐饮上花了多少？比前一个月涨了还是降了？」
「查一下最近 30 天超过 200 元的支出」
「记一笔：今天午饭 42.5 元，餐饮，现金」
「把昨天那笔地铁改成 5 元」
```

内置 13 个工具：搜索账单、消费分析（趋势 + 分类占比 + 环比）、预算状态、字典查询、
增删改与回收站恢复。支持 `今天 / 上月 / 最近30天` 等自然语言时间，分类账户可直接给名称；
删除为软删除可恢复，批量删除需显式确认，写操作支持 `dry_run` 预演。

完整说明见 **[docs/MCP.md](docs/MCP.md)**。

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.27 + Gin + GORM + JWT (golang-jwt/v5) + bcrypt |
| 数据库 | SQLite（本地/零依赖）/ PostgreSQL 16（生产） |
| 前端 | Svelte 5 + SvelteKit 2 + TypeScript 7 + Vite 8 + TailwindCSS 4 |
| 状态 | Svelte stores（响应式） |
| 图表 | Chart.js 4 + svelte-chartjs |
| UI 组件 | 自研轻量 + vaul-svelte 抽屉 + svelte-sonner Toast + @lucide/svelte 图标 |
| 虚拟滚动 | @tanstack/svelte-virtual（大数据量列表优化） |
| PWA | vite-plugin-pwa（可安装离线访问） |
| 部署 | Docker 多阶段，单进程 Go 托管前后端（可前置反向代理） |
| 周期调度器 | Go 内置 Tick（每日资产快照 / 周期记账） |

---

## 快速开始

### 方式一：单机 Docker Compose（零依赖 · SQLite · 最快）

```bash
# 构建并启动（首次需要几分钟下载编译依赖）
docker compose up -d --build

# 打开
open http://localhost:8080

# 查看日志
docker compose logs -f huozhi

# 停掉（数据保留在 huozhi_data 卷中）
docker compose down
```

### 方式二：本地开发（前后端分开跑）

```bash
# 1) 后端
cd backend
cp config.example.yaml config.yaml   # 默认SQLite
go mod tidy
go run ./cmd/huozhi-server          # 监听 0.0.0.0:8080

# 2) 前端
cd ../frontend
npm install
npm run dev                         # 监听 http://localhost:5173（已代理 /api → :8080）

# 访问 http://localhost:5173 即可
```

---

## 主要目录

```
huozhi/
├── backend/                        # Go 后端
│   ├── cmd/huozhi-server/          # 入口 main.go
│   ├── internal/
│   │   ├── config/                 # 配置加载（YAML + 环境变量）
│   │   ├── database/               # GORM + SQLite/PostgreSQL
│   │   ├── models/                 # 全部数据模型（User,Book,Account,Category,Tx...）
│   │   ├── handlers/               # 每个 handler 对应 REST 资源
│   │   │   └── tx_core.go          # 与 gin 解耦的交易核心层（Web / MCP / 定时任务共用）
│   │   ├── mcp/                    # MCP 服务端（协议 + 传输 + 13 个账单工具）
│   │   ├── router/                 # Gin 路由定义
│   │   ├── middleware/             # JWT / API Key / MCP 鉴权
│   │   └── dto/                    # 请求/响应 DTO
│   └── pkg/
│       ├── auth/                   # bcrypt 密码
│       └── jwt/                    # JWT 签发/解析
├── frontend/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api/                # 模块式 HTTP API 封装
│   │   │   ├── stores/             # Svelte stores: user/book/category/tag/account
│   │   │   ├── components/
│   │   │   │   ├── ui/             # Button/Card/Dialog/Input/Select 等基础组件
│   │   │   │   ├── layout/         # AppLayout + 侧边栏/顶栏/移动Tab
│   │   │   │   ├── charts/         # CategoryPie/TrendLine/MonthlyBars 图表组件
│   │   │   │   └── business/       # 业务组件（交易表单、账户选择等）
│   │   │   ├── utils/              # formatMoney/formatDate/pct/cn
│   │   │   └── types.ts            # 全部类型定义
│   │   └── routes/
│   │       ├── (auth)/             # 需认证的页面（dashboard/transactions/accounts...）
│   │       ├── login/              # 登录页
│   │       └── register/           # 注册页
│   └── vite.config.ts + svelte.config.js
├── scripts/entrypoint.sh           # Docker 入口脚本（自动生成JWT密钥）
├── Dockerfile                      # 多阶段镜像：Go build → Vite build → 单进程 Go 托管前后端
└── docker-compose.yaml             # 零依赖一键启动（SQLite）
```

---

## API 一览

全部 REST 接口定义在 `backend/internal/router/router.go`：

```
POST /api/auth/register   POST /api/auth/login   GET /api/auth/me  ...

GET|POST|PUT|DELETE  /api/books        /api/accounts         /api/categories
                     /api/tags         /api/transactions     /api/budgets
                     /api/saving-plans /api/recurring        /api/installments
                     /api/reimbursements

GET  /api/statistics                   # 综合统计（支持 dimension=category|account|book|all）
GET  /api/statistics/assets            # 资产总览
GET  /api/statistics/assets/timeline   # 资产净值曲线

GET  /api/io/export                    # 导出CSV
POST /api/io/import                    # 导入CSV/微信/支付宝
GET  /api/io/template                  # 下载模板

POST|GET|DELETE /api/mcp               # MCP 端点（Streamable HTTP，供 AI 助手接入）
```

---

## 数据安全

- 密码使用 bcrypt（`golang.org/x/crypto`）
- JWT 默认 7 天过期，`HZ_JWT_SECRET` Docker 首次启动会持久化生成随机值
- 生产部署建议：外层反向代理（Nginx/Traefik 等）反代到容器 8080，并在其上开启 HTTPS、`client_max_body_size` 按需调大（导入 xlsx 含图片）、WebSocket（/api/ws）需透传 Upgrade 头

## 下一步 RoadMap

- [x] 基础 12 大模块骨架
- [x] 周期记账任务真正执行
- [x] 资产快照迁移 + 增量计算
- [ ] 真正的移动端 App（Capacitor 打包）
- [x] WebSocket 多端实时同步
- [x] AI 智能分类 / 智能记账
- [x] MCP 服务端：AI 通过自然语言搜索 / 分析 / 增删改账单

## License

MIT
