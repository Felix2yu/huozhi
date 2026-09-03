package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"huozhi/internal/database"
	"huozhi/internal/middleware"
	"huozhi/internal/models"

	"github.com/gin-gonic/gin"
)

// ==============================================
// AI 智能分类 & 智能记账
// 配置通过环境变量读取：
//   AI_PROVIDER    - 预留，默认 openai
//   AI_API_KEY     - 必填，API Key
//   AI_MODEL       - 模型名，默认 gpt-4o-mini
//   AI_ENDPOINT    - API Endpoint，默认 https://api.openai.com/v1/chat/completions
//   AI_ENABLED     - true/false，默认自动检测（有 key 即启用）
// ==============================================

type aiClassifyRequest struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type,omitempty"` // expense/income/transfer
	BookID      uint    `json:"book_id,omitempty"`
}

type aiClassifyResponse struct {
	CategoryID  uint    `json:"category_id"`
	Category    string  `json:"category"`
	Type        string  `json:"type"`
	Confidence  float64 `json:"confidence"`
	Explanation string  `json:"explanation,omitempty"`
}

type aiSmartRecordRequest struct {
	Text   string `json:"text"`
	BookID uint   `json:"book_id,omitempty"`
}

type aiSmartRecordResponse struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"`
	CategoryID  uint    `json:"category_id"`
	AccountID   uint    `json:"account_id,omitempty"`
	TxDate      string  `json:"tx_date"` // YYYY-MM-DD
	Tags        []string `json:"tags,omitempty"`
	Raw         string  `json:"raw,omitempty"`
}

type aiStatusResponse struct {
	Enabled    bool   `json:"enabled"`
	Model      string `json:"model,omitempty"`
	Configured bool   `json:"configured"`
}

func aiEnabled() bool {
	v := strings.ToLower(os.Getenv("AI_ENABLED"))
	if v == "true" {
		return true
	}
	if v == "false" {
		return false
	}
	// 自动检测
	return os.Getenv("AI_API_KEY") != ""
}

func aiModel() string {
	if m := os.Getenv("AI_MODEL"); m != "" {
		return m
	}
	return "gpt-4o-mini"
}

func aiEndpoint() string {
	if e := os.Getenv("AI_ENDPOINT"); e != "" {
		return e
	}
	return "https://api.openai.com/v1/chat/completions"
}

// AIStatus 检查 AI 是否可用
func AIStatus(c *gin.Context) {
	OK(c, aiStatusResponse{
		Enabled:    aiEnabled(),
		Model:      aiModel(),
		Configured: os.Getenv("AI_API_KEY") != "",
	})
}

// AIClassify 智能分类：根据描述推荐分类
func AIClassify(c *gin.Context) {
	if !aiEnabled() {
		Fail(c, 503, "AI 未启用，请在服务器配置 AI_API_KEY")
		return
	}
	uid := middleware.GetUID(c)
	var req aiClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		Bad(c, "请输入描述")
		return
	}

	// 加载账本分类
	bookID := req.BookID
	if bookID == 0 {
		// 用用户默认账本
		var book models.Book
		if err := database.DB.Where("user_id = ? AND is_default = ?", uid, true).First(&book).Error; err != nil {
			database.DB.Where("user_id = ?", uid).First(&book)
		}
		bookID = book.ID
	}

	var categories []models.Category
	database.DB.Where("user_id = ? AND book_id = ?", uid, bookID).Find(&categories)

	result, err := doAIClassify(req.Description, req.Amount, req.Type, categories)
	if err != nil {
		Fail(c, 500, "AI 调用失败: "+err.Error())
		return
	}
	OK(c, result)
}

// AISmartRecord 智能记账：自然语言转结构化交易
func AISmartRecord(c *gin.Context) {
	if !aiEnabled() {
		Fail(c, 503, "AI 未启用，请在服务器配置 AI_API_KEY")
		return
	}
	uid := middleware.GetUID(c)
	var req aiSmartRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Bad(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		Bad(c, "请输入记账内容")
		return
	}

	bookID := req.BookID
	if bookID == 0 {
		var book models.Book
		if err := database.DB.Where("user_id = ? AND is_default = ?", uid, true).First(&book).Error; err != nil {
			database.DB.Where("user_id = ?", uid).First(&book)
		}
		bookID = book.ID
	}

	var (
		categories []models.Category
		accounts   []models.Account
	)
	database.DB.Where("user_id = ? AND book_id = ?", uid, bookID).Find(&categories)
	database.DB.Where("user_id = ? AND book_id = ? AND archived = ?", uid, bookID, false).Find(&accounts)

	result, err := doAISmartRecord(req.Text, categories, accounts)
	if err != nil {
		Fail(c, 500, "AI 调用失败: "+err.Error())
		return
	}
	OK(c, result)
}

// ============ LLM 调用 ============

func doAIClassify(desc string, amount float64, txType string, categories []models.Category) (*aiClassifyResponse, error) {
	typeMap := map[string]string{
		"expense": "支出", "income": "收入", "transfer": "转账",
	}
	typeLabel := typeMap[txType]
	if typeLabel == "" {
		typeLabel = "自动判断"
	}

	// 构建分类列表文本
	var catList strings.Builder
	for i, c := range categories {
		fmt.Fprintf(&catList, "%d. [%s] %s", i+1, c.Kind, c.Name)
		if c.Icon != "" {
			fmt.Fprintf(&catList, " (icon: %s)", c.Icon)
		}
		if c.IsSystem {
			catList.WriteString(" [系统]")
		}
		catList.WriteString("\n")
	}

	systemPrompt := `你是一个专业的记账助手。根据用户的交易描述，从给定分类列表中选择最合适的分类。
只返回 JSON，不要任何解释文字。格式：
{"category_index": 数字, "type": "expense|income|transfer", "confidence": 0-1, "explanation": "简短说明"}`

	userPrompt := fmt.Sprintf("交易描述：%s\n金额：%.2f\n交易类型：%s\n\n可选分类：\n%s",
		desc, amount, typeLabel, catList.String())

	var resp struct {
		CategoryIndex int     `json:"category_index"`
		Type          string  `json:"type"`
		Confidence    float64 `json:"confidence"`
		Explanation   string  `json:"explanation"`
	}
	if err := callLLM(systemPrompt, userPrompt, &resp); err != nil {
		return nil, err
	}

	if resp.CategoryIndex < 1 || resp.CategoryIndex > len(categories) {
		return nil, fmt.Errorf("AI 返回的分类索引无效")
	}
	cat := categories[resp.CategoryIndex-1]
	return &aiClassifyResponse{
		CategoryID:  cat.ID,
		Category:    cat.Name,
		Type:        string(cat.Kind),
		Confidence:  resp.Confidence,
		Explanation: resp.Explanation,
	}, nil
}

func doAISmartRecord(text string, categories []models.Category, accounts []models.Account) (*aiSmartRecordResponse, error) {
	var catList strings.Builder
	for i, c := range categories {
		fmt.Fprintf(&catList, "%d. [%s] %s\n", i+1, c.Kind, c.Name)
	}
	var accList strings.Builder
	for i, a := range accounts {
		fmt.Fprintf(&accList, "%d. %s (余额: %.2f %s)\n", i+1, a.Name, a.Balance, a.Currency)
	}

	today := time.Now().Format("2006-01-02")

	systemPrompt := fmt.Sprintf(`你是一个专业的中文记账助手。把用户输入的自然语言转换成结构化交易记录。
理解口语化表达，如"午饭35"="午餐花费35元"，"昨天打车20"。
日期默认为今天 (%s)，如果提到"昨天"则减一天，"前天"减两天。
只返回 JSON，格式：
{
  "description": "简洁描述",
  "amount": 数字,
  "type": "expense|income|transfer",
  "category_index": 数字,
  "account_index": 数字,
  "tx_date": "YYYY-MM-DD",
  "tags": ["标签1","标签2"]
}`, today)

	userPrompt := fmt.Sprintf("记账内容：%s\n\n可选分类：\n%s\n可选账户：\n%s", text, catList.String(), accList.String())

	var resp struct {
		Description   string   `json:"description"`
		Amount        float64  `json:"amount"`
		Type          string   `json:"type"`
		CategoryIndex int      `json:"category_index"`
		AccountIndex  int      `json:"account_index"`
		TxDate        string   `json:"tx_date"`
		Tags          []string `json:"tags"`
	}
	if err := callLLM(systemPrompt, userPrompt, &resp); err != nil {
		return nil, err
	}

	if resp.TxDate == "" {
		resp.TxDate = today
	}
	if resp.Amount <= 0 {
		return nil, fmt.Errorf("无法解析金额")
	}

	out := &aiSmartRecordResponse{
		Description: resp.Description,
		Amount:      resp.Amount,
		Type:        resp.Type,
		TxDate:      resp.TxDate,
		Tags:        resp.Tags,
		Raw:         text,
	}
	if resp.CategoryIndex >= 1 && resp.CategoryIndex <= len(categories) {
		out.CategoryID = categories[resp.CategoryIndex-1].ID
	}
	if resp.AccountIndex >= 1 && resp.AccountIndex <= len(accounts) {
		out.AccountID = accounts[resp.AccountIndex-1].ID
	}
	return out, nil
}

// callLLM 调用 OpenAI 兼容接口
func callLLM(system, user string, out interface{}) error {
	apiKey := os.Getenv("AI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("AI_API_KEY 未配置")
	}

	body := map[string]interface{}{
		"model": aiModel(),
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.2,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", aiEndpoint(), bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("API 错误 %d: %s", resp.StatusCode, string(data))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &chatResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return fmt.Errorf("AI 未返回内容")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	// 去除可能的 markdown code fence
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if err := json.Unmarshal([]byte(content), out); err != nil {
		return fmt.Errorf("解析 AI JSON 失败: %w\n原始: %s", err, content)
	}
	return nil
}
