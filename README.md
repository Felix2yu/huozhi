# Huozhi - 个人记账应用

一个参考钱迹（qianjiapp.com）功能、使用 Go + React/TypeScript/TailwindCSS 构建的现代个人记账系统，
支持 Docker 一键部署，响应式界面 + PWA 支持，手机/桌面皆可良好使用。

---

## 功能模块（对标钱迹）

| 模块 | 说明 |
|-----|------|
| 用户与认证 | 注册/登录、JWT、个人资料、安全设置 |
| 多账本 | 账本创建/切换/归档、共享账本与成员角色 |
| 账户/资产管理 | 现金、储蓄、信用卡、储值卡、投资、负债，资产分组、余额调整、信用额度、账单日/还款日 |
| 分类管理 | 收入/支出两级分类，系统分类，自定义图标/颜色/排序，归档 |
| 交易记账 | 收入/支出/转账/退款/报销/调整，标签、图片、商家、地点、记账日期任意选 |
| 标签中心 | 标签云、使用次数统计、标签筛选流水 |
| 预算管理 | 总预算/分类预算，月/年/自定义周期，进度条、日均、超支预警 |
| 统计分析 | 饼图/折线图/柱状图，分类/账户/标签/日/月维度，Top支出，资产净值曲线 |
| 存钱计划 | 目标金额+目标日期，进度条、存钱记录、日均需要存 |
| 周期记账 | 日/周/两周/月/年/自定义间隔到期自动入账 |
| 分期管理 | 消费分期，自动生成每期还款日历 |
| 报销管理 | 报销单管理，关联账单并自动标记 |
| 导入/导出 | CSV 模板下载，微信/支付宝账单自动解析并入库 |
| 其他 | 资产快照（每日），自定义月起始日，多币种，多设备云同步（按用户） |

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.22 + Gin + GORM + JWT (golang-jwt/v5) + bcrypt |
| 数据库 | SQLite（本地/零依赖）/ PostgreSQL 16（生产） |
| 前端 | React 18 + TypeScript 5 + Vite 5 + TailwindCSS 3 |
| 状态 | Zustand |
| 图表 | Recharts 2 |
| UI 组件 | 自研轻量 + lucide-react 图标 + Sonner Toast |
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
│   │   ├── router/                 # Gin 路由定义
│   │   ├── middleware/             # JWT 鉴权
│   │   └── dto/                    # 请求/响应 DTO
│   └── pkg/
│       ├── auth/                   # bcrypt 密码
│       └── jwt/                    # JWT 签发/解析
├── frontend/
│   ├── src/
│   │   ├── api/index.ts            # 模块式 HTTP API 封装
│   │   ├── stores/app.ts           # zustand: user/book/category/tag/account
│   │   ├── components/layout/      # AppLayout + 侧边栏/顶栏/移动Tab
│   │   ├── components/common/      # Modal/Drawer/Empty/Progress/TagChip 等
│   │   ├── pages/{auth,dashboard,transactions,accounts,categories,
│   │              budgets,statistics,tags,savings,shared,settings}/
│   │   ├── utils/                  # formatMoney/formatDate/pct/cn
│   │   └── types/                  # 全部类型定义
│   └── vite.config.ts + tailwind.config.js
├── scripts/entrypoint.sh           # Docker 入口脚本（自动生成JWT密钥）
├── Dockerfile                      # 多阶段镜像：Go build → Vite build → 单进程 Go 托管前后端
└── docker-compose.yaml             # 零依赖一键启动（SQLite）
```

---

## API 一览（略）

全部 REST 接口定义在 `backend/internal/router/router.go`：

```
POST /api/auth/register   POST /api/auth/login   GET /api/auth/me  ...

GET|POST|PUT|DELETE  /api/books        /api/accounts         /api/categories
                     /api/tags         /api/transactions     /api/budgets
                     /api/saving-plans /api/recurring        /api/installments
                     /api/reimbursements

GET  /api/statistics                   # 综合统计
GET  /api/statistics/assets            # 资产总览
GET  /api/statistics/assets/timeline   # 资产净值曲线

GET  /api/io/export                    # 导出CSV
POST /api/io/import                    # 导入CSV/微信/支付宝
GET  /api/io/template                  # 下载模板
```

---

## 数据安全

- 密码使用 bcrypt（`golang.org/x/crypto`）
- JWT 默认 7 天过期，`HZ_JWT_SECRET` Docker 首次启动会持久化生成随机值
- 生产部署建议：外层反向代理（Nginx/Traefik 等）反代到容器 8080，并在其上开启 HTTPS、`client_max_body_size` 按需调大（导入 xlsx 含图片）、WebSocket（/api/ws）需透传 Upgrade 头

## 下一步 RoadMap

- [x] 基础 12 大模块骨架
- [ ] 周期记账任务真正执行（目前有 runner，可进一步加强）
- [ ] 资产快照迁移 + 增量计算
- [ ] 真正的移动端 App（Capacitor 打包）
- [ ] WebSocket 多端实时同步
- [ ] AI 智能分类 / 智能记账

## License

MIT
