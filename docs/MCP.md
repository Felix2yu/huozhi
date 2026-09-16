# MCP 接入指南：让 AI 助手直接操作你的账单

货殖内置了一个 **MCP（Model Context Protocol）服务端**。接入后，任何支持 MCP 的 AI 客户端
（Claude Desktop、Cursor、Cherry Studio、WorkBuddy 等）都能用自然语言查询、分析和修改你的账单，
不需要你手动调接口。

数据全程只在你自托管的服务器上流转，不外发第三方。

---

## 1. 快速开始

### 1.1 拿到 API 密钥

登录货殖 → **设置 → API密钥** → 生成并启用。这个密钥就是 MCP 的鉴权凭据。

### 1.2 找到服务地址

MCP 端点与 REST API 同前缀：

```
https://<你的域名>/api/mcp        # 推荐，与现有 /api 共用一条反代规则
https://<你的域名>/mcp            # 等价别名
```

设置页的「AI 助手（MCP）」卡片会自动算出地址并给出各客户端的配置片段，可一键复制。

### 1.3 在客户端里配置

**通用 / Cursor**（`~/.cursor/mcp.json`）：

```json
{
  "mcpServers": {
    "huozhi": {
      "url": "https://your-domain.com/api/mcp",
      "headers": { "X-API-Key": "你的API密钥" }
    }
  }
}
```

**Claude Desktop**（`claude_desktop_config.json`）：Claude Desktop 走 stdio，用 `mcp-remote` 桥接到 HTTP 端点。

```json
{
  "mcpServers": {
    "huozhi": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://your-domain.com/api/mcp", "--header", "X-API-Key:你的API密钥"]
    }
  }
}
```

**Cherry Studio**：设置 → MCP 服务器 → 添加，类型选「可流式传输的 HTTP（Streamable HTTP）」，
填入 URL 与 `X-API-Key` 请求头。

重启客户端后，就能在工具列表里看到 `search_transactions` 等 13 个工具。

---

## 2. 鉴权

三种方式任选其一，由后端 `middleware.MCPAuth()` 统一处理：

| 方式 | 说明 |
|------|------|
| `X-API-Key: <key>` | 推荐。设置页生成的 64 位十六进制密钥 |
| `Authorization: Bearer <key>` | 给只允许配一个 Bearer 的客户端（多数桌面客户端） |
| `Authorization: Bearer <JWT>` | 登录 JWT 同样有效，前端可直接复用登录态 |
| `?api_key=<key>` | 仅调试用，**不要**在生产使用（会出现在访问日志里） |

未通过鉴权返回 401，并带 `WWW-Authenticate: Bearer realm="huozhi-mcp"`。
API 密钥被禁用（设置页关闭开关）时立即失效。

---

## 3. 工具清单

### 查询（只读）

| 工具 | 用途 |
|------|------|
| `search_transactions` | 按时间 / 类型 / 分类 / 账户 / 账本 / 标签 / 关键词 / 金额区间组合筛选流水 |
| `get_transaction` | 按 id 取单条详情 |
| `analyze_spending` | 汇总 + 维度占比排行 + 趋势 + Top 大额 + 环比对比 |
| `get_budget_status` | 预算执行进度、剩余额度、是否超支 |
| `list_books` / `list_categories` / `list_accounts` / `list_tags` | 字典，供 AI 查名称 |

### 写入

| 工具 | 用途 | 安全设计 |
|------|------|---------|
| `create_transaction` | 新增一笔账 | 支持 `dry_run` 预演 |
| `update_transaction` | 修改（补丁语义，只改传了的字段） | 支持 `dry_run`，返回改前改后对照 |
| `delete_transaction` | 删除（**软删除**，进回收站） | 支持 `dry_run`，可用 `recover_transaction` 恢复 |
| `recover_transaction` | 从回收站恢复 | — |
| `batch_delete_transactions` | 批量删除 | 必须显式 `confirm=true`，否则拒绝执行；支持 `dry_run` 预演 |

---

## 4. 自然语言能力

服务端承担了两件模型不擅长的事，使得「用户怎么说，AI 就怎么传」：

### 4.1 时间表达

`start_date` / `end_date` / `period` / `date` 都接受自然语言：

| 说法 | 解析结果 |
|------|---------|
| `今天` `昨天` `前天` `大前天` `明天` `后天` | 对应日期 |
| `本周` `上周` `下周` | 周一至周日 |
| `本月` `上月` `下月` | 自然月 1 号至月末 |
| `本季度` `上季度` | 自然季度 |
| `今年` `去年` `明年` | 1 月 1 日至 12 月 31 日 |
| `最近7天` `近3个月` `过去一年` | 含今天往前推 |
| `3天前` `十天后` | 相对偏移（支持中文数字） |
| `2026-01-31` / `2026-02` / `2026年3月5日` | 精确日期 / 整月 |
| `recent_7d` `last_month` `this_year` | `period` 快捷词 |

边界与夏令时由服务端用服务器本地时区裁决，不依赖模型的系统时间。

### 4.2 名称解析

分类、账户、账本、标签都可以直接给名称，服务端做精确 → 包含 → 被包含的模糊匹配：

- 命中 **1 个**：直接使用；
- 命中 **多个**：查询场景按 OR 全部纳入（说「餐饮」能同时命中「餐饮」「外出餐饮」），
  写入场景返回候选列表让模型改用更精确的名称或 id；
- 命中 **0 个**：报错并列出可用名称，模型据此重试。

未指定分类时，`create_transaction` 会依次尝试：备注里出现的分类名 → 同类目下的「其他」→
转账 / 余额调整自动取后端预置的系统分类。

### 4.3 金额与单位

所有金额输入输出的单位都是**元**（`42.5` = 42 元 5 角），不是分。
入参兼容 `"¥1,280.50"`、`"35元"`、数字、字符串等多种写法。

---

## 5. 典型对话

```
用户：上个月我在餐饮上花了多少？比前一个月是涨了还是降了？
AI  → analyze_spending(period="last_month", dimension="category")
    → 汇总 + 餐饮占比 + 环比 delta_percent

用户：查一下最近 30 天超过 200 元的支出
AI  → search_transactions(period="recent_30d", type="expense", min_amount=200)

用户：记一笔，今天午饭 42.5 元，餐饮，现金
AI  → create_transaction(amount=42.5, category="餐饮", account="现金", date="今天", description="午饭")

用户：把昨天那笔地铁改成 5 元
AI  → search_transactions(start_date="昨天", keyword="地铁")   # 先定位 id
    → update_transaction(id=..., amount=5)

用户：删掉刚才那笔
AI  → delete_transaction(id=..., dry_run=true)                 # 先预演
    → delete_transaction(id=...)                               # 确认后执行
```

---

## 6. 配置与运维

### 开关

MCP 默认启用。关闭方式（`config.yaml`）：

```yaml
mcp:
  disabled: true     # 注意是 disabled：缺失该段 = 启用
  path: "/mcp"       # 可选，默认 /mcp（同时挂 /api/mcp）
```

环境变量：`HZ_MCP_DISABLED=true` 关闭，`HZ_MCP_PATH=/custom` 改路径。

> 为什么用 `disabled` 而不是 `enabled`：大量既有部署的 `config.yaml` 里没有 `mcp` 段，
> 若用 `enabled bool`，YAML 零值会被解析成 `false`，用户升级后新功能被静默关掉且毫无线索。

### 传输协议

实现的是 **Streamable HTTP**（MCP 2025-06-18）：

- `POST /mcp` —— 客户端→服务端。`Accept` 含 `text/event-stream` 时以 SSE 回送，否则直接 JSON；
  通知（无 id）回 202 且无响应体；支持批量请求（JSON 数组）。
- `GET /mcp` —— 服务端→客户端的 SSE 长连接，25 秒一次心跳保活。
- `DELETE /mcp` —— 显式结束会话。

协议版本协商：客户端请求的 `2025-06-18` / `2025-03-26` / `2024-11-05` 原样采纳，
未知版本回落到服务端最新版（而不是报错拒绝连接）。

反向代理注意事项（Nginx）：

```nginx
location /api/mcp {
    proxy_pass http://huozhi:8080;
    proxy_http_version 1.1;
    proxy_set_header Connection '';      # SSE 不能被缓冲
    proxy_buffering off;
    proxy_read_timeout 3600s;
}
```

### 会话回收

会话对象常驻内存，空闲超过一定时间应回收。建议在自定义部署里定期调用
`Server.GCIdleSessions(maxIdle)`；当前版本会话表仅用于 SSE 流登记，
无状态请求（不建立 GET 流）不会累积。

---

## 7. 实现结构

```
backend/internal/mcp/
├── protocol.go      # JSON-RPC 2.0 消息、协议版本协商、工具/结果类型
├── server.go        # 方法分发（initialize / tools/list / tools/call / ping）、会话表
├── transport.go     # Streamable HTTP（POST/GET/DELETE，SSE 编解码）
├── tools.go         # 13 个工具的名称、描述、入参 JSON Schema、模型指令
├── bills.go         # 工具实现：搜索 / 分析 / 增删改 + 输出视图
├── nldate.go        # 自然语言时间解析（中英文、区间与单日）
├── resolve.go       # 参数取值 + 分类/账户/账本/标签「名称 → ID」解析
└── schema/          # JSON Schema 构造助手

backend/internal/handlers/tx_core.go   # 与 gin 解耦的交易核心层（余额/预算/派生流水）
backend/internal/middleware/mcp.go     # MCP 鉴权（API Key / JWT 三通道）
```

**关键设计：写操作不另起炉灶。** MCP 的建/改/删全部调用 `handlers.CreateTx / UpdateTx / DeleteTx /
RecoverTx / BatchDeleteTx`，与 Web 界面和定时任务共用同一份「更新账户余额 → 占用预算 →
生成转账手续费派生流水」逻辑。为此把交易业务逻辑从 gin handler 里抽成了不依赖
`*gin.Context` 的核心层，HTTP handler 退化为薄封装——避免出现「AI 记的账」和
「手工记的账」口径分叉。

---

## 8. 安全边界

1. **删除是软删除**，误删可用 `recover_transaction` 按 id 完整恢复（含余额与预算回滚）。
2. **批量删除需二次确认**：未传 `confirm=true` 一律拒绝，并提示先 `dry_run` 预演。
3. **写操作可预演**：`create/update/delete` 都支持 `dry_run=true`，只返回将要发生的变更。
4. **权限与 UI 一致**：共享账本的范围判定、写权限判定复用同一套函数
   （`handlers.VisibleBookIDs` / `ScopeByBooks` / `CanWriteBookFor` / `CanWriteTxFor`），
   AI 不能越过你在界面上的权限。
5. **失败信息可读**：工具执行失败返回 `isError=true` 的结果（而非中断会话的 JSON-RPC 错误），
   错误文本会带着「下一步怎么做」的提示进入模型上下文，便于自我纠正。
