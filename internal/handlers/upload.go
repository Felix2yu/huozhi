package handlers

import (
	"io"
	"net/http"
	"strings"

	"huozhi/internal/middleware"
	"huozhi/internal/storage"

	"github.com/gin-gonic/gin"
)

// UploadImage 上传账单图片等附件到服务器本地或 S3 存储，返回可公开访问的路径。
// 表单字段：file（multipart）。返回 { url: "/api/uploads/<key>" }。
func UploadImage(c *gin.Context) {
	uid := middleware.GetUID(c)
	file, err := c.FormFile("file")
	if err != nil {
		Bad(c, "缺少文件: "+err.Error())
		return
	}
	url, err := storage.Save(file, uid)
	if err != nil {
		Bad(c, "上传失败: "+err.Error())
		return
	}
	OK(c, gin.H{"url": url})
}

// ServeUpload 按 key 取回存储的附件（本地或 S3），供前端 <img> 直接加载。
// 路由：GET /api/uploads/*filepath
func ServeUpload(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("filepath"), "/")
	if key == "" || strings.Contains(key, "..") {
		c.Status(http.StatusBadRequest)
		return
	}
	rc, ct, err := storage.Open(key)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer rc.Close()
	if ct != "" {
		c.Header("Content-Type", ct)
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = io.Copy(c.Writer, rc)
}
