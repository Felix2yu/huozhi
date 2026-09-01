package router

import (
	"huozhi/internal/handlers"
	"huozhi/internal/middleware"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func New(mode string) *gin.Engine {
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

		// 需要鉴权
		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			// 用户
			auth.GET("/auth/me", handlers.GetMe)
			auth.PUT("/auth/me", handlers.UpdateMe)
			auth.POST("/auth/password", handlers.ChangePassword)
			auth.POST("/auth/logout", handlers.Logout)

			// 账本
			books := auth.Group("/books")
			{
				books.GET("", handlers.ListBooks)
				books.GET("/:id", handlers.GetBook)
				books.POST("", handlers.CreateBook)
				books.PUT("/:id", handlers.UpdateBook)
				books.DELETE("/:id", handlers.DeleteBook)
				books.GET("/:id/members", handlers.ListBookMembers)
				books.POST("/:id/members", handlers.InviteBookMember)
			}

			// 账户/资产
			accounts := auth.Group("/accounts")
			{
				accounts.GET("", handlers.ListAccounts)
				accounts.GET("/:id", handlers.GetAccount)
				accounts.POST("", handlers.CreateAccount)
				accounts.PUT("/:id", handlers.UpdateAccount)
				accounts.DELETE("/:id", handlers.DeleteAccount)
				accounts.POST("/:id/adjust", handlers.AdjustAccountBalance)
				accounts.GET("/groups", handlers.ListAccountGroups)
				accounts.POST("/groups", handlers.CreateAccountGroup)
				accounts.DELETE("/groups/:id", handlers.DeleteAccountGroup)
			}

			// 分类
			categories := auth.Group("/categories")
			{
				categories.GET("", handlers.ListCategories)
				categories.POST("", handlers.CreateCategory)
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
				txs.GET("/:id", handlers.GetTransaction)
				txs.POST("", handlers.CreateTransaction)
				txs.PUT("/:id", handlers.UpdateTransaction)
				txs.DELETE("/:id", handlers.DeleteTransaction)
				txs.POST("/batch-delete", handlers.BatchDeleteTransactions)
			}

			// 预算
			budgets := auth.Group("/budgets")
			{
				budgets.GET("", handlers.ListBudgets)
				budgets.POST("", handlers.CreateBudget)
				budgets.PUT("/:id", handlers.UpdateBudget)
				budgets.DELETE("/:id", handlers.DeleteBudget)
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

			// 导入导出
			io := auth.Group("/io")
			{
				io.GET("/export", handlers.ExportTransactions)
				io.POST("/import", handlers.ImportTransactions)
				io.GET("/template", handlers.DownloadImportTemplate)
			}
		}
	}

	return r
}
