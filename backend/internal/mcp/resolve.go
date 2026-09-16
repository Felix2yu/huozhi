package mcp

import (
	"errors"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/handlers"
	"huozhi/internal/models"
	"strconv"
	"strings"
)

// 本文件做两件事：
//  1. 从工具入参里安全地取值（LLM 传来的 JSON 类型不稳定：数字可能是 string，
//     布尔可能是 "true"，金额可能带「元」或千分位逗号）；
//  2. 把「分类名 / 账户名 / 账本名 / 标签名」解析成内部 ID。
//
// 第 2 点是自然语言可用性的关键：用户说「查一下餐饮花了多少」，模型手上只有
// 「餐饮」这个词，没有 ID。由服务端做名称解析，模型不需要先调一次字典接口再
// 拼 ID，一次 tools/call 就能完成。

// ==================== 参数取值 ====================

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	switch v := args[key].(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	}
	return ""
}

// argFloat 解析金额/数值。兼容 number、字符串（"128.5" / "¥128.5" / "1,280" / "128元"）。
func argFloat(args map[string]any, key string) (float64, bool) {
	if args == nil {
		return 0, false
	}
	switch v := args[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		s = strings.NewReplacer(",", "", "，", "", "¥", "", "￥", "", "$", "", "元", "", "块", "").Replace(s)
		s = strings.TrimSpace(s)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func argInt(args map[string]any, key string) (int, bool) {
	if f, ok := argFloat(args, key); ok {
		return int(f), true
	}
	return 0, false
}

func argUint(args map[string]any, key string) (uint, bool) {
	if f, ok := argFloat(args, key); ok && f > 0 {
		return uint(f), true
	}
	return 0, false
}

// argBool 布尔取值。未传返回 (false, false)，与「传了 false」区分开。
func argBool(args map[string]any, key string) (bool, bool) {
	if args == nil {
		return false, false
	}
	switch v := args[key].(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y", "是", "需要", "确认":
			return true, true
		case "false", "0", "no", "n", "否", "不":
			return false, true
		}
	case float64:
		return v != 0, true
	}
	return false, false
}

func argStringSlice(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return v
	case string:
		// 兼容 "餐饮,交通" 这种逗号分隔写法
		parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '，' || r == '、' })
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ==================== 名称解析 ====================

// candidate 候选条目，用于歧义提示
type candidate struct {
	ID   uint
	Name string
}

// Resolver 以某个用户身份解析名称 → ID
type Resolver struct {
	UID     uint
	BookIDs []uint // 可见的共享账本
}

func newResolver(uid uint) *Resolver {
	return &Resolver{UID: uid, BookIDs: handlers.VisibleBookIDs(uid)}
}

// toolErrorf 构造带提示的工具错误
func toolErrorf(format string, a ...any) error { return errors.New(fmt.Sprintf(format, a...)) }

// Books 按名称/ID 解析账本，返回全部匹配项
func (r *Resolver) Books(name string) ([]candidate, error) {
	var list []models.Book
	q := handlers.ScopeByBooks(database.DB.Model(&models.Book{}), r.UID, r.BookIDs)
	q = q.Where("is_archived = ?", false)
	if id, ok := asID(name); ok {
		q = q.Where("id = ?", id)
	}
	q.Order("is_default DESC, sort ASC, id ASC").Find(&list)
	return matchCandidates(name, list, func(b models.Book) (uint, string) { return b.ID, b.Name })
}

// Categories 按名称/ID 解析分类。kind 为空表示不限。
func (r *Resolver) Categories(name string, kind string) ([]candidate, error) {
	var list []models.Category
	q := database.DB.Model(&models.Category{}).
		Where("(user_id = ? OR book_id IN ?)", r.UID, append(append([]uint{}, r.BookIDs...), 0))
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if id, ok := asID(name); ok {
		q = q.Where("id = ?", id)
	} else if kind != string(models.KindSystem) {
		// 隐蔽分类（转账手续费等后端自用）不参与名称匹配。
		// 但 system 类型例外：转账 / 余额调整这类后端预置分类本身是隐藏的，
		// 过滤掉会让「记一笔转账」永远找不到分类。
		q = q.Where("is_hidden = ?", false)
	}
	q.Order("sort ASC, id ASC").Find(&list)
	return matchCandidates(name, list, func(c models.Category) (uint, string) { return c.ID, c.Name })
}

// Accounts 按名称/ID 解析账户
func (r *Resolver) Accounts(name string) ([]candidate, error) {
	var list []models.Account
	q := handlers.ScopeByBooks(database.DB.Model(&models.Account{}), r.UID, r.BookIDs)
	if id, ok := asID(name); ok {
		q = q.Where("id = ?", id)
	} else {
		q = q.Where("is_hidden = ?", false)
	}
	q.Order("sort ASC, id DESC").Find(&list)
	return matchCandidates(name, list, func(a models.Account) (uint, string) { return a.ID, a.Name })
}

// Tags 按名称/ID 解析标签
func (r *Resolver) Tags(name string) ([]candidate, error) {
	var list []models.Tag
	q := handlers.ScopeByBooks(database.DB.Model(&models.Tag{}), r.UID, r.BookIDs)
	if id, ok := asID(name); ok {
		q = q.Where("id = ?", id)
	}
	q.Order("count DESC, id ASC").Find(&list)
	return matchCandidates(name, list, func(t models.Tag) (uint, string) { return t.ID, t.Name })
}

// DefaultBook 取默认账本：显式指定优先，其次 is_default，最后按排序取第一个。
func (r *Resolver) DefaultBook(name string) (models.Book, error) {
	var book models.Book
	if name != "" {
		cands, err := r.Books(name)
		if err != nil {
			return book, err
		}
		id, err := pickOne("账本", name, cands)
		if err != nil {
			return book, err
		}
		if err := database.DB.Where("id = ?", id).First(&book).Error; err != nil {
			return book, toolErrorf("账本不存在: %s", name)
		}
		return book, nil
	}
	q := handlers.ScopeByBooks(database.DB.Model(&models.Book{}), r.UID, r.BookIDs).Where("is_archived = ?", false)
	if err := q.Order("is_default DESC, sort ASC, id ASC").First(&book).Error; err != nil {
		return book, toolErrorf("当前用户还没有账本，请先在应用里创建一个账本")
	}
	return book, nil
}

// RequireCategory 解析出唯一的分类 ID（写入场景）
func (r *Resolver) RequireCategory(name string, kind string) (uint, error) {
	cands, err := r.Categories(name, kind)
	if err != nil {
		return 0, err
	}
	return pickOne("分类", name, cands)
}

// RequireAccount 解析出唯一的账户 ID（写入场景）
func (r *Resolver) RequireAccount(name string) (uint, error) {
	cands, err := r.Accounts(name)
	if err != nil {
		return 0, err
	}
	return pickOne("账户", name, cands)
}

// IDsOf 取候选列表的 ID 集合（搜索场景：多匹配即 OR 条件）
func IDsOf(cs []candidate) []uint {
	out := make([]uint, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

// ==================== 内部 ====================

// asID 判断输入是否为纯数字（视为 ID）
func asID(s string) (uint, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}

func matchCandidates[T any](input string, list []T, kv func(T) (uint, string)) ([]candidate, error) {
	key := strings.ToLower(strings.TrimSpace(input))
	all := make([]candidate, 0, len(list))
	var exact []candidate
	var partial []candidate
	for _, item := range list {
		id, name := kv(item)
		all = append(all, candidate{ID: id, Name: name})
		ln := strings.ToLower(strings.TrimSpace(name))
		if key == "" || ln == key {
			exact = append(exact, candidate{ID: id, Name: name})
			continue
		}
		if strings.Contains(ln, key) || strings.Contains(key, ln) {
			partial = append(partial, candidate{ID: id, Name: name})
		}
	}
	if key == "" {
		return all, nil
	}
	if len(exact) > 0 {
		return exact, nil
	}
	return partial, nil
}

// pickOne 从候选中选出唯一结果：多个匹配时优先完全相等，否则报错并列出候选，
// 让模型能带着正确名称重试（而不是由服务端瞎猜一个）。
func pickOne(kindLabel, input string, cands []candidate) (uint, error) {
	if len(cands) == 0 {
		return 0, toolErrorf("未找到%s「%s」。请先用 list_categories / list_accounts / list_books 查看可用名称。", kindLabel, input)
	}
	if len(cands) == 1 {
		return cands[0].ID, nil
	}
	key := strings.ToLower(strings.TrimSpace(input))
	for _, c := range cands {
		if strings.ToLower(strings.TrimSpace(c.Name)) == key {
			return c.ID, nil
		}
	}
	names := make([]string, 0, len(cands))
	for i, c := range cands {
		if i >= 20 {
			names = append(names, "...")
			break
		}
		names = append(names, fmt.Sprintf("%s(id=%d)", c.Name, c.ID))
	}
	return 0, toolErrorf("%s「%s」匹配到多个：%s。请改用更精确的名称，或直接传 id。", kindLabel, input, strings.Join(names, "、"))
}

// nameList 把候选列表渲染成名称字符串，用于错误提示
func nameList(cs []candidate, limit int) string {
	names := make([]string, 0, limit)
	for i, c := range cs {
		if i >= limit {
			names = append(names, "...")
			break
		}
		names = append(names, c.Name)
	}
	return strings.Join(names, "、")
}
