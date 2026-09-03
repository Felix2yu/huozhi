package handlers

import (
	"huozhi/internal/dto"
	"huozhi/internal/middleware"
	"huozhi/internal/ws"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Broadcast 向当前请求用户的所有连接推送变更通知
// table: transactions / accounts / categories / tags / budgets / books ...
// action: create / update / delete
func Broadcast(c *gin.Context, table, action string, id uint) {
	uid := middleware.GetUID(c)
	if uid > 0 {
		ws.DefaultHub.Broadcast(uid, table, action, id)
	}
}

// OK 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, dto.Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// Created 创建成功
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, dto.Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// Fail 业务失败
func Fail(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, dto.Response{
		Code:    code,
		Message: message,
	})
}

// Bad 请求参数错误
func Bad(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, dto.Response{
		Code:    400,
		Message: message,
	})
}

// Unauthorized 未授权
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, dto.Response{
		Code:    401,
		Message: message,
	})
}

// Forbidden 无权限
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, dto.Response{
		Code:    403,
		Message: message,
	})
}

// NotFound 未找到
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, dto.Response{
		Code:    404,
		Message: message,
	})
}

// InternalErr 服务器内部错误
func InternalErr(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, dto.Response{
		Code:    500,
		Message: message,
	})
}

// PagedOK 分页响应
func PagedOK(c *gin.Context, list interface{}, page, pageSize int, total int64) {
	OK(c, dto.PagedResponse{
		List: list,
		Pagination: dto.Pagination{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	})
}

// GetPageParams 获取分页参数
func GetPageParams(c *gin.Context) (page int, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 20
	}
	return
}
