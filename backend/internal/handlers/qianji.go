package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"huozhi/internal/database"
	"huozhi/internal/models"
	"huozhi/internal/storage"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ========== 钱迹 (QianJi) 账单导入 ==========
// 钱迹导出的 xlsx 为单工作表，表头（示例）：
// ID / 时间 / 账本 / 分类 / 二级分类 / 类型 / 金额 / 币种 / 账户1 / 账户2 /
// 备注 / 已报销 / 手续费 / 优惠券 / 记账者 / 账单标记 / 标签 / 账单图片 / 关联账单
// 类型取值：支出 / 收入 / 转账 / 退款 等。

// parseQianJi 解析钱迹 xlsx 账单
func parseQianJi(r io.Reader, uid, bookID uint) ([]models.Transaction, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	rows, err := readXLSXBytes(data)
	if err != nil {
		return nil, err
	}
	// 钱迹「账单图片」是内嵌在 xlsx 里的图片文件（并非 URL 外链），
	// 从工作表绘图锚点中提取并按行号转存到本系统存储。best-effort：失败不影响文本解析。
	rowImgs, _ := extractQianJiEmbeddedImages(data, uid)
	return parseQianJiRows(rows, rowImgs, uid, bookID)
}

// parseQianJiRows 核心解析逻辑（按表头名映射，便于单测）
// rowImgs 为「工作表行号(0 基) -> 内嵌图片公开路径」，由 parseQianJi 提取后传入。
func parseQianJiRows(rows [][]string, rowImgs map[int][]string, uid, bookID uint) ([]models.Transaction, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("空文件或缺少表头")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(h)] = i
	}
	get := func(row []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	// 账本名 -> bookID 缓存：每行的「账本」列决定归属账本，缺省回退到导入目标账本
	bookCache := map[string]uint{}
	resolveBook := func(name string) uint {
		if name == "" {
			return bookID
		}
		if id, ok := bookCache[name]; ok {
			return id
		}
		id := findBookByNameOrCreate(uid, name).ID
		bookCache[name] = id
		return id
	}
	// 账本 -> 兜底分类(其他支出/其他收入/转账) 缓存
	fallbackCache := map[uint][3]models.Category{}
	getFallbacks := func(bid uint) (exp, inc, tr models.Category) {
		if c, ok := fallbackCache[bid]; ok {
			return c[0], c[1], c[2]
		}
		e := findCategoryOrCreate(uid, bid, "其他支出", models.KindExpense, "📦")
		i := findCategoryOrCreate(uid, bid, "其他收入", models.KindIncome, "💰")
		t := findCategoryOrCreate(uid, bid, "转账", models.KindExpense, "🔁")
		fallbackCache[bid] = [3]models.Category{e, i, t}
		return e, i, t
	}

	var out []models.Transaction
	for ri := 1; ri < len(rows); ri++ {
		row := rows[ri]
		if len(row) == 0 {
			continue
		}
		typeStr := get(row, "类型")
		amountStr := get(row, "金额")
		if amountStr == "" {
			continue
		}
		amt := parseFloat(amountStr)
		if amt <= 0 {
			continue
		}

		// 归属账本：优先取「账本」列，否则回退到导入目标账本
		rowBookID := resolveBook(get(row, "账本"))
		expenseCat, incomeCat, transferCat := getFallbacks(rowBookID)

		// 日期：钱迹为 "2006-01-02 15:04:05"，兼容多种格式
		d := parseQianJiDate(get(row, "时间"))

		tx := models.Transaction{
			BookID:      rowBookID,
			Amount:      amt,
			Currency:    firstNonEmpty(get(row, "币种"), "CNY"),
			TxDate:      d,
			Description: get(row, "备注"),
			Remark:      get(row, "备注"),
			// 钱迹原始交易 ID：用于导入幂等去重与「关联账单」解析
			ExternalID: get(row, "ID"),
			// 记账者（协作账本中记录是谁记的账）
			RecordedBy: get(row, "记账者"),
			// 账单标记（信用卡账单归属标记）
			BillMarker: get(row, "账单标记"),
		}

		// 账户（先解析，便于按「账户1+账户2 同时存在」兜底识别转账）
		a1 := get(row, "账户1")
		a2 := get(row, "账户2")

		subName := get(row, "二级分类")
		switch typeStr {
		case "收入":
			tx.Type = models.TxIncome
			parentID := matchCategory(uid, rowBookID, get(row, "分类"), models.KindIncome, incomeCat)
			tx.CategoryID = resolveSubCategory(uid, parentID, subName)
		case "退款":
			tx.Type = models.TxRefund
			parentID := matchCategoryAny(uid, bookID, get(row, "分类"), incomeCat)
			tx.CategoryID = resolveSubCategory(uid, parentID, subName)
		case "转账":
			tx.Type = models.TxTransfer
			tx.CategoryID = resolveSubCategory(uid, transferCat.ID, subName)
			if fee := parseFloat(get(row, "手续费")); fee > 0 {
				tx.TransferFee = fee
			}
			if disc := parseFloat(get(row, "优惠券")); disc > 0 {
				tx.TransferDiscount = disc
			}
		case "支出":
			tx.Type = models.TxExpense
			parentID := matchCategory(uid, rowBookID, get(row, "分类"), models.KindExpense, expenseCat)
			tx.CategoryID = resolveSubCategory(uid, parentID, subName)
		default:
			// 其他 / 未知类型：钱迹中「账户1、账户2 同时存在」即视为转账（含信用卡还款等）；
			// 否则保守视为支出。
			if a1 != "" && a2 != "" {
				tx.Type = models.TxTransfer
				tx.CategoryID = resolveSubCategory(uid, transferCat.ID, subName)
				if fee := parseFloat(get(row, "手续费")); fee > 0 {
					tx.TransferFee = fee
				}
				if disc := parseFloat(get(row, "优惠券")); disc > 0 {
					tx.TransferDiscount = disc
				}
			} else {
				tx.Type = models.TxExpense
				parentID := matchCategory(uid, rowBookID, get(row, "分类"), models.KindExpense, expenseCat)
				tx.CategoryID = resolveSubCategory(uid, parentID, subName)
			}
		}

		// 账户
		if a1 != "" {
			tx.AccountID = findAccountByNameOrCreate(uid, bookID, a1).ID
		}
		if a2 != "" {
			tx.ToAccountID = findAccountByNameOrCreate(uid, bookID, a2).ID
		}

		// 报销状态：钱迹「已报销」列一般为「是/否」、空、日期或报销金额。
		// 若是非空数字则视为报销金额（同时置 done）；否则按否定词判定；其余非空视为已报销。
		if rb := get(row, "已报销"); rb != "" {
			if num := parseFloat(rb); num > 0 {
				tx.ReimburseStatus = "done"
				tx.ReimburseAmount = num
			} else if isReimbursed(rb) {
				tx.ReimburseStatus = "done"
			} else {
				tx.ReimburseStatus = "none"
			}
		}

		// 标签
		if tags := get(row, "标签"); tags != "" {
			for _, name := range splitTags(tags) {
				t := findTagOrCreate(uid, rowBookID, name)
				tx.Tags = append(tx.Tags, &t)
			}
		}

		// 账单图片：钱迹导出为内嵌于 xlsx 的图片文件（非 URL 外链），
		// 已在 parseQianJi 中按工作表行号提取并转存，此处直接挂载（rowImgs 键即工作表行号）。
		if len(rowImgs[ri]) > 0 {
			tx.Images = rowImgs[ri]
		}

		// 关联账单：记录原始交易外部 ID，导入入库后再解析为 RefundOfID
		if rel := get(row, "关联账单"); rel != "" {
			tx.RefundOfExternalID = rel
		}

		tx.IncludeInBalance = true
		tx.IncludeInBudget = true
		out = append(out, tx)
	}
	return out, nil
}

func parseQianJiDate(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
	} {
		if d, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return d
		}
	}
	return time.Now()
}

// matchCategoryAny 按名称匹配分类（不限 kind），用于退款等需保留原分类名的场景
func matchCategoryAny(uid, bookID uint, name string, fallback models.Category) uint {
	if name == "" {
		return fallback.ID
	}
	var c models.Category
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ?", uid, bookID, name).First(&c).Error; err == nil {
		return c.ID
	}
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name LIKE ?", uid, bookID, "%"+name+"%").First(&c).Error; err == nil {
		return c.ID
	}
	return fallback.ID
}

// resolveSubCategory 在一级分类 parentID 下查找/创建名为 subName 的二级分类，
// 返回该二级分类的 ID；若 subName 为空则返回 parentID 本身（仅挂一级分类）。
// 二级分类继承父级的 book_id / kind / 图标 / 颜色，与 app 既有的「叶子分类」约定一致。
func resolveSubCategory(uid, parentID uint, subName string) uint {
	subName = strings.TrimSpace(subName)
	if subName == "" {
		return parentID
	}
	var parent models.Category
	if err := database.DB.First(&parent, parentID).Error; err != nil {
		return parentID
	}
	kind := parent.Kind
	var sub models.Category
	if err := database.DB.Where("user_id = ? AND parent_id = ? AND name = ? AND kind = ?",
		uid, parentID, subName, kind).First(&sub).Error; err == nil {
		return sub.ID
	}
	sub = models.Category{
		UserID:   uid,
		BookID:   parent.BookID,
		ParentID: parentID,
		Name:     subName,
		Kind:     kind,
		Icon:     parent.Icon,
		Color:    parent.Color,
	}
	database.DB.Create(&sub)
	return sub.ID
}

// findAccountByNameOrCreate 按名称匹配账户，不存在则按名称猜测类型后创建
func findAccountByNameOrCreate(uid, bookID uint, name string) models.Account {
	var a models.Account
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ?", uid, bookID, name).First(&a).Error; err == nil {
		return a
	}
	t := guessAccountType(name)
	a = models.Account{UserID: uid, BookID: bookID, Name: name, Type: t, Currency: "CNY"}
	switch t {
	case models.AccCredit:
		a.Icon = "💳"
	case models.AccBank:
		a.Icon = "🏦"
	case models.AccPrepaid:
		a.Icon = "🍱"
	case models.AccLiability:
		a.Icon = "💸"
	case models.AccVirtual:
		a.Icon = "📱"
	default:
		a.Icon = "💰"
	}
	database.DB.Create(&a)
	return a
}

// guessAccountType 依据账户名猜测账户类型（尽力而为）
func guessAccountType(name string) models.AccountType {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(name, "信用卡") || strings.Contains(name, "贷记") || strings.Contains(n, "credit"):
		return models.AccCredit
	case strings.Contains(name, "花呗") || strings.Contains(name, "借呗") || strings.Contains(name, "白条") ||
		strings.Contains(name, "负债") || strings.Contains(n, "loan"):
		return models.AccLiability
	case strings.Contains(name, "银行") || strings.Contains(name, "储蓄") || strings.Contains(name, "借记"):
		return models.AccBank
	case strings.Contains(name, "饭卡") || strings.Contains(name, "公交") || strings.Contains(name, "储值") ||
		strings.Contains(name, "加油") || strings.Contains(name, "预付"):
		return models.AccPrepaid
	case strings.Contains(name, "支付宝") || strings.Contains(name, "微信") || strings.Contains(name, "零钱") ||
		strings.Contains(name, "余额"):
		return models.AccVirtual
	default:
		return models.AccCash
	}
}

// findTagOrCreate 按名称匹配或创建标签
func findTagOrCreate(uid, bookID uint, name string) models.Tag {
	var t models.Tag
	if err := database.DB.Where("user_id = ? AND (book_id = 0 OR book_id = ?) AND name = ?", uid, bookID, name).First(&t).Error; err == nil {
		return t
	}
	t = models.Tag{UserID: uid, BookID: bookID, Name: name}
	database.DB.Create(&t)
	return t
}

func splitTags(s string) []string {
	parts := regexp.MustCompile(`[、,，;；\s/]+`).Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isReimbursed 判断钱迹「已报销」列是否表示已报销。
// 明确否定词（否/无/未/不/没/no/false/none 等）视为未报销，其余非空值视为已报销。
func isReimbursed(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false
	}
	negatives := []string{"否", "无", "未", "不", "没", "no", "false", "none", "n/a", "null"}
	for _, n := range negatives {
		if strings.Contains(s, n) {
			return false
		}
	}
	return true
}

// ========== 钱迹「账单图片」内嵌图片提取 ==========
// 钱迹导出的 xlsx 中，账单图片以「内嵌图片文件」形式存在（位于 xl/media/，
// 通过工作表绘图锚点 twoCellAnchor/oneCellAnchor 的 from.row 关联到具体行），并非 URL 外链。
// 因此导入时从 xlsx 压缩包里提取这些图片，按行号映射后转存到本系统存储（本地 / S3）。

// maxImportImageBytes 导入时单张内嵌图片体积上限（10MB）
const maxImportImageBytes = 10 * 1024 * 1024

// extractQianJiEmbeddedImages 从钱迹 xlsx 字节中提取内嵌的账单图片，
// 返回「工作表行号(0 基，与 readSheet 返回的 rows 索引一致) -> 图片公开路径列表」。
// 图片直接转存到本系统存储，实现「图片随账单一并导入」。
// best-effort：任何解析/存储失败都跳过该图片，不影响账单文本解析。
func extractQianJiEmbeddedImages(data []byte, uid uint) (map[int][]string, error) {
	out := map[int][]string{}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return out, err
	}
	sheet := chooseSheetFile(zr)
	if sheet == nil {
		return out, nil
	}
	// 1) 工作表 -> 绘图文件
	drawingPath, ok := sheetDrawingTarget(zr, sheet.Name)
	if !ok {
		return out, nil // 该表无内嵌图片
	}
	// 2) 绘图锚点 -> (行号, blip rId)
	anchors, err := drawingAnchors(zr, drawingPath)
	if err != nil {
		return out, err
	}
	if len(anchors) == 0 {
		return out, nil
	}
	// 3) 绘图关系 -> rId -> 媒体文件相对目标
	mediaTargets, err := drawingMediaTargets(zr, drawingPath)
	if err != nil {
		return out, err
	}
	// 4) 逐个提取媒体字节并转存
	for _, a := range anchors {
		target, ok := mediaTargets[a.blip]
		if !ok {
			continue
		}
		mediaPath := resolveRel(drawingPath, target)
		f := zipFile(zr, mediaPath)
		if f == nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		imgData, err := io.ReadAll(io.LimitReader(rc, maxImportImageBytes+1))
		rc.Close()
		if err != nil {
			continue
		}
		if len(imgData) > maxImportImageBytes {
			continue
		}
		if !isImageContent(imgData) {
			continue
		}
		url, err := storage.SaveBytes(imgData, f.Name, uid)
		if err != nil {
			continue
		}
		out[a.row] = append(out[a.row], url)
	}
	return out, nil
}

// anchor 记录绘图锚点关联的工作表行号与所引用图片的 rId。
type anchor struct {
	row  int
	blip string
}

// sheetDrawingTarget 读取工作表的关系文件，找到其引用的绘图部件路径。
func sheetDrawingTarget(zr *zip.Reader, sheetPath string) (string, bool) {
	rels, err := readRelEntries(zr, sheetPath)
	if err != nil {
		return "", false
	}
	for _, r := range rels {
		if strings.Contains(r.Type, "drawing") {
			return resolveRel(sheetPath, r.Target), true
		}
	}
	return "", false
}

// drawingMediaTargets 读取绘图部件的关系文件，返回 blip rId -> 媒体文件相对目标。
func drawingMediaTargets(zr *zip.Reader, drawingPath string) (map[string]string, error) {
	rels, err := readRelEntries(zr, drawingPath)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rels))
	for _, r := range rels {
		m[r.Id] = r.Target
	}
	return m, nil
}

// drawingAnchors 解析绘图 XML，提取每个图片锚点的工作表行号（from.row）与被引用图片的 rId。
func drawingAnchors(zr *zip.Reader, drawingPath string) ([]anchor, error) {
	var out []anchor
	f := zipFile(zr, drawingPath)
	if f == nil {
		return out, fmt.Errorf("绘图文件不存在: %s", drawingPath)
	}
	rc, err := f.Open()
	if err != nil {
		return out, err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var cur anchor
	inFrom, inRow := false, false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "twoCellAnchor", "oneCellAnchor", "absoluteAnchor":
				cur = anchor{}
				inFrom, inRow = false, false
			case "from":
				inFrom = true
			case "row":
				if inFrom {
					inRow = true
				}
			case "blip":
				for _, a := range e.Attr {
					if a.Name.Local == "embed" {
						cur.blip = a.Value
					}
				}
			}
		case xml.CharData:
			if inRow {
				cur.row, _ = strconv.Atoi(strings.TrimSpace(string(e)))
			}
		case xml.EndElement:
			switch e.Name.Local {
			case "row":
				inRow = false
			case "from":
				inFrom = false
			case "twoCellAnchor", "oneCellAnchor", "absoluteAnchor":
				if cur.blip != "" {
					out = append(out, cur)
				}
			}
		}
	}
	return out, nil
}

// relEntry 为 .rels 关系文件中的一条 Relationship。
type relEntry struct {
	Id     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
	Type   string `xml:"Type,attr"`
}

// readRelEntries 读取某个部件（工作表/绘图）的关系文件 _rels/<base>.rels。
func readRelEntries(zr *zip.Reader, ownerPath string) ([]relEntry, error) {
	relsPath := relsPathFor(ownerPath)
	f := zipFile(zr, relsPath)
	if f == nil {
		return nil, fmt.Errorf("关系文件不存在: %s", relsPath)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var rf struct {
		Relationships []relEntry `xml:"Relationship"`
	}
	if err := xml.NewDecoder(rc).Decode(&rf); err != nil {
		return nil, err
	}
	return rf.Relationships, nil
}

// relsPathFor 计算某部件对应的关系文件路径：<dir>/_rels/<base>.rels。
func relsPathFor(ownerPath string) string {
	dir := path.Dir(ownerPath)
	base := path.Base(ownerPath)
	return path.Join(dir, "_rels", base+".rels")
}

// resolveRel 将相对目标（可能含 ../）解析为 zip 内绝对路径。
func resolveRel(base, target string) string {
	return path.Clean(path.Join(path.Dir(base), target))
}

// isImageContent 依据字节内容判断是否为图片。
func isImageContent(data []byte) bool {
	ct := http.DetectContentType(data)
	return strings.HasPrefix(ct, "image/")
}

func parseFloat(s string) float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ========== 极简 xlsx 读取（无需第三方库，覆盖钱迹导出的简单结构） ==========

func readXLSX(r io.Reader) ([][]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return readXLSXBytes(data)
}

func readXLSXBytes(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	// 共享字符串表
	var shared []string
	if f := zipFile(zr, "xl/sharedStrings.xml"); f != nil {
		rc, _ := f.Open()
		shared = readSharedStrings(rc)
		rc.Close()
	}
	// 定位工作表（优先 sheet1.xml，否则取第一个 worksheets/sheetN.xml）
	sheetFile := chooseSheetFile(zr)
	if sheetFile == nil {
		return nil, fmt.Errorf("未找到工作表")
	}
	rc, err := sheetFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return readSheet(rc, shared)
}

// chooseSheetFile 选择用于解析数据的工作表文件（与内嵌图片提取使用同一张表）。
func chooseSheetFile(zr *zip.Reader) *zip.File {
	if f := zipFile(zr, "xl/worksheets/sheet1.xml"); f != nil {
		return f
	}
	re := regexp.MustCompile(`xl/worksheets/sheet\d+\.xml$`)
	for _, zf := range zr.File {
		if re.MatchString(zf.Name) {
			return zf
		}
	}
	return nil
}

func readSharedStrings(r io.Reader) []string {
	dec := xml.NewDecoder(r)
	var strs []string
	var cur strings.Builder
	inT := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			if e.Name.Local == "t" {
				inT = true
				cur.Reset()
			}
		case xml.CharData:
			if inT {
				cur.Write(e)
			}
		case xml.EndElement:
			if e.Name.Local == "t" {
				inT = false
			}
			if e.Name.Local == "si" {
				strs = append(strs, cur.String())
				cur.Reset()
			}
		}
	}
	return strs
}

func readSheet(r io.Reader, shared []string) ([][]string, error) {
	dec := xml.NewDecoder(r)
	var rows [][]string
	var curRow []string
	maxCol := -1
	curCol := 0
	cellType := ""
	inV := false
	var vbuf strings.Builder

	flushCell := func() {
		if !inV {
			return
		}
		val := vbuf.String()
		if cellType == "s" {
			if idx, err := strconv.Atoi(val); err == nil && idx >= 0 && idx < len(shared) {
				val = shared[idx]
			}
		}
		for len(curRow) <= curCol {
			curRow = append(curRow, "")
		}
		curRow[curCol] = val
		inV = false
		vbuf.Reset()
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "row":
				curRow = []string{}
				maxCol = -1
			case "c":
				cellType = ""
				ref := ""
				for _, attr := range e.Attr {
					switch attr.Name.Local {
					case "r":
						ref = attr.Value
					case "t":
						cellType = attr.Value
					}
				}
				curCol = colIndex(ref)
				if curCol > maxCol {
					maxCol = curCol
				}
				inV = false
				vbuf.Reset()
			case "v", "t":
				if cellType == "inlineStr" || e.Name.Local == "v" {
					inV = true
					vbuf.Reset()
				}
			}
		case xml.CharData:
			if inV {
				vbuf.Write(e)
			}
		case xml.EndElement:
			switch e.Name.Local {
			case "v", "t":
				flushCell()
			case "row":
				if maxCol >= 0 {
					for len(curRow) <= maxCol {
						curRow = append(curRow, "")
					}
				}
				rows = append(rows, curRow)
			}
		}
	}
	return rows, nil
}

// colIndex 将单元格列引用（如 "B"）转为 0 基索引
func colIndex(ref string) int {
	idx := 0
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' {
			idx = idx*26 + int(r-'A'+1)
		} else {
			break
		}
	}
	return idx - 1
}

// isXLSX 依据文件名后缀判断是否为 Excel 文件
func isXLSX(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".xlsx") || strings.HasSuffix(n, ".xls")
}

// zipFile 在 zip 读取器中按名称查找文件
func zipFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}
