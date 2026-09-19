package router

import (
	"huozhi/internal/config"
	"huozhi/internal/handlers"
	"huozhi/internal/mcp"
	"huozhi/internal/middleware"
	"huozhi/internal/ws"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// mcpServer 进程内唯一的 MCP 服务端实例（工具注册表无状态，可安全共享）
var mcpServer = func() *mcp.Server {
	s := mcp.New()
	if err := mcp.RegisterTools(s); err != nil {
		log.Printf("[MCP] 工具注册失败: %v", err)
	}
	return s
}()

// mountMCP 挂载 MCP 端点。
//
// 同时挂两个路径是有意为之：/api/mcp 与既有 REST API 同前缀，反向代理只需一条
// location 规则；/mcp 则是多数 MCP 客户端文档里的默认写法，少一次试错。
func mountMCP(r *gin.Engine) {
	if config.AppConfig != nil && !config.AppConfig.MCP.IsEnabled() {
		log.Println("[MCP] 已在配置中禁用（mcp.disabled=true 或 HZ_MCP_DISABLED），跳过挂载")
		return
	}
	path := "/mcp"
	if config.AppConfig != nil && config.AppConfig.MCP.Path != "" {
		path = config.AppConfig.MCP.Path
	}
	handler := func(c *gin.Context) { mcpServer.HandleHTTP(c) }

	r.GET(path, middleware.MCPAuth(), handler)
	r.POST(path, middleware.MCPAuth(), handler)
	r.DELETE(path, middleware.MCPAuth(), handler)
	r.GET("/api"+path, middleware.MCPAuth(), handler)
	r.POST("/api"+path, middleware.MCPAuth(), handler)
	r.DELETE("/api"+path, middleware.MCPAuth(), handler)

	log.Printf("[MCP] 端点已挂载: %s 与 /api%s（Streamable HTTP，需 API 密钥或 JWT 鉴权）", path, path)
}

func New(mode string, staticDir string) *gin.Engine {
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api")
	{
		// 公开
		api.GET("/health", handlers.HealthCheck)
		api.POST("/auth/register", handlers.Register)
		api.POST("/auth/login", handlers.Login)
		// 附件（账单图片等）读取：公开可读，key 为不可猜测的随机串
		api.GET("/uploads/*filepath", handlers.ServeUpload)

		// 需要鉴权
		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			// WebSocket 实时同步
			auth.GET("/ws", ws.ServeWS(ws.DefaultHub))

			// 用户
			auth.GET("/auth/me", handlers.GetMe)
			auth.PUT("/auth/me", handlers.UpdateMe)
			auth.POST("/auth/password", handlers.ChangePassword)
			auth.POST("/auth/logout", handlers.Logout)

			// 附件上传（账单图片等）：存储到本地或 S3
			auth.POST("/upload", handlers.UploadImage)
			// 手动触发孤儿附件清理（日常由后台定时任务自动执行）
			auth.POST("/uploads/cleanup", handlers.CleanupOrphanUploads)

		// AI 智能分类 & 智能记账
		auth.GET("/ai/status", handlers.AIStatus)
		auth.POST("/ai/classify", handlers.AIClassify)
		auth.POST("/ai/smart-record", handlers.AISmartRecord)

		// API密钥管理
		auth.POST("/api-key/generate", handlers.GenerateAPIKeyHandler)
		auth.GET("/api-key", handlers.GetAPIKeyInfo)
		auth.POST("/api-key/toggle", handlers.ToggleAPIKey)

			// 账本
			books := auth.Group("/books")
			{
				books.GET("", handlers.ListBooks)
				books.GET("/archived", handlers.ListArchivedBooks)
				books.GET("/:id", handlers.GetBook)
				books.POST("", handlers.CreateBook)
				books.PUT("/:id", handlers.UpdateBook)
				books.PUT("/:id/archive", handlers.ArchiveBook)
				books.DELETE("/:id", handlers.DeleteBook)
				books.GET("/:id/members", handlers.ListBookMembers)
				books.POST("/:id/members", handlers.InviteBookMember)
				books.DELETE("/:id/members/:memberId", handlers.RemoveBookMember)
			}

			// 账户/资产
			accounts := auth.Group("/accounts")
			{
				accounts.GET("", handlers.ListAccounts)
				// ⚠️ 静态路径必须在 /:id 之前注册，否则被参数路由劫持
				accounts.GET("/credit-summary", handlers.GetCreditSummary)
				accounts.GET("/audit", handlers.AuditAccounts) // 数据体检（B7）
				accounts.GET("/groups", handlers.ListAccountGroups)
				accounts.POST("/groups", handlers.CreateAccountGroup)
				accounts.DELETE("/groups/:id", handlers.DeleteAccountGroup)
				accounts.GET("/:id", handlers.GetAccount)
				accounts.POST("", handlers.CreateAccount)
				accounts.PUT("/:id", handlers.UpdateAccount)
				accounts.DELETE("/:id", handlers.DeleteAccount)
				accounts.POST("/:id/adjust", handlers.AdjustAccountBalance)
				accounts.POST("/:id/recalc", handlers.RecalcAccountBalance) // 按流水重算余额（B7）
				// C15：改用 POST 并在 Handler 内校验登录密码，
				// 避免密码出现在 URL / 访问日志中
				accounts.POST("/:id/full-card", handlers.GetFullCardNo)
			}

			// 分类
			categories := auth.Group("/categories")
			{
				categories.GET("", handlers.ListCategories)
				categories.POST("", handlers.CreateCategory)
				categories.PUT("/reorder", handlers.ReorderCategories)
				categories.PUT("/:id", handlers.UpdateCategory)
				categories.DELETE("/:id", handlers.DeleteCategory)
			}

			// 标签
			tags := auth.Group("/tags")
			{
				tags.GET("", handlers.ListTags)
				tags.POST("", handlers.CreateTag)
				tags.PUT("/:id", handlers.UpdateTag)
				tags.DELETE("/:id", handlers.DeleteTag)
			}

			// 交易
			txs := auth.Group("/transactions")
			{
				txs.GET("", handlers.ListTransactions)
				txs.GET("/deleted", handlers.ListDeletedTransactions) // 回收站（B3）
				txs.GET("/:id", handlers.GetTransaction)
				txs.POST("", handlers.CreateTransaction)
				txs.PUT("/:id", handlers.UpdateTransaction)
				txs.DELETE("/:id", handlers.DeleteTransaction)
				txs.POST("/:id/recover", handlers.RecoverTransaction) // 撤销删除（B3）
				txs.POST("/batch-delete", handlers.BatchDeleteTransactions)
			}

			// 预算
			budgets := auth.Group("/budgets")
			{
				budgets.GET("", handlers.ListBudgets)
				budgets.POST("", handlers.CreateBudget)
				budgets.POST("/recalc", handlers.RecalcAllBudgetsHandler) // 重算全部（B2）
				budgets.PUT("/:id", handlers.UpdateBudget)
				budgets.POST("/:id/recalc", handlers.RecalcBudgetHandler) // 重算单条（B2）
				budgets.DELETE("/:id", handlers.DeleteBudget)
			}

			// 汇率（基准货币折算）
			rates := auth.Group("/exchange-rates")
			{
				rates.GET("", handlers.ListExchangeRates)
				rates.GET("/convert", handlers.ConvertAmount)
				rates.POST("/refresh", handlers.RefreshExchangeRates)
			}

			// 统计
			stats := auth.Group("/statistics")
			{
				stats.GET("", handlers.GetStatistics)
				stats.GET("/assets", handlers.GetAssetOverview)
				stats.GET("/assets/timeline", handlers.GetAssetTimeline)
			}

			// 存钱计划
			savings := auth.Group("/saving-plans")
			{
				savings.GET("", handlers.ListSavingPlans)
				savings.POST("", handlers.CreateSavingPlan)
				savings.PUT("/:id", handlers.UpdateSavingPlan)
				savings.DELETE("/:id", handlers.DeleteSavingPlan)
				savings.POST("/:id/records", handlers.AddSavingRecord)
			}

			// 周期记账
			recurring := auth.Group("/recurring")
			{
				recurring.GET("", handlers.ListRecurrings)
				recurring.POST("", handlers.CreateRecurring)
				recurring.POST("/:id/toggle", handlers.ToggleRecurring)
				recurring.DELETE("/:id", handlers.DeleteRecurring)
			}

			// 分期
			installments := auth.Group("/installments")
			{
				installments.GET("", handlers.ListInstallments)
				installments.POST("", handlers.CreateInstallment)
				installments.DELETE("/:id", handlers.DeleteInstallment)
			}

			// 报销
			reimbs := auth.Group("/reimbursements")
			{
				reimbs.GET("", handlers.ListReimbursements)
				reimbs.POST("", handlers.CreateReimbursement)
				reimbs.PUT("/:id", handlers.UpdateReimbursement)
				reimbs.DELETE("/:id", handlers.DeleteReimbursement)
			}

			// 借贷
			loans := auth.Group("/loans")
			{
				loans.GET("", handlers.ListLoans)
				loans.POST("", handlers.CreateLoan)
				loans.POST("/:id/repay", handlers.RepayLoan)
				loans.PUT("/:id", handlers.UpdateLoan)
				loans.DELETE("/:id", handlers.DeleteLoan)
			}

			// 导入导出
			io := auth.Group("/io")
			{
				io.GET("/export", handlers.ExportTransactions)
				io.POST("/import", handlers.ImportTransactions)
				io.GET("/template", handlers.DownloadImportTemplate)
				io.GET("/bill", handlers.GetBill)
				io.GET("/backup", handlers.ExportBackup)         // 全量 JSON 快照（B8）
				io.POST("/restore", handlers.ImportBackup)       // 从快照恢复（B8）
				io.POST("/reset", handlers.ClearUserData)        // 清空全部业务数据（B9，需密码）
				io.GET("/auto-backups", handlers.ListAutoBackup) // 自动备份列表
				io.POST("/auto-backups", handlers.CreateAutoBackup)
				io.GET("/auto-backups/:name", handlers.DownloadAutoBackup)
			}
		}
	}

	// 公开API（需要API key认证）
	public := api.Group("/public")
	public.Use(middleware.APIKeyAuth())
	{
		public.GET("/bills/:id", handlers.GetPublicBill)
		public.GET("/bills", handlers.ListPublicBills)
	}

	// MCP（Model Context Protocol）：让 AI 助手用自然语言读写账单。
	// 内部实现见 internal/mcp，与 HTTP API 共用同一套业务核心层。
	mountMCP(r)

	// 前端静态托管（配置了 static_dir 时启用；未配置则仅提供 API，开发时由 vite dev server 承担）
	if staticDir != "" {
		if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
			mountFrontend(r, staticDir)
		} else {
			log.Printf("[router] static_dir 不存在，跳过前端托管: %s", staticDir)
		}
	}

	return r
}
