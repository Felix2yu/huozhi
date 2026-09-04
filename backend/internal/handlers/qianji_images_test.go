package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"huozhi/internal/storage"
)

// 1x1 透明 PNG（base64）
const tinyPNGb64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

// buildQianJiXLSXWithImage 构造一个含内嵌图片的最小 xlsx：
// 图片锚定在工作表第 1 行（0 基，即第 2 行数据行），通过 drawing rels 关联到 xl/media/image1.png。
func buildQianJiXLSXWithImage(t *testing.T) []byte {
	t.Helper()
	png, err := base64.StdEncoding.DecodeString(tinyPNGb64)
	if err != nil {
		t.Fatalf("解码 PNG 失败: %v", err)
	}

	const sheetXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`

	const sheetRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rIdD1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>
</Relationships>`

	const drawingXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<xdr:twoCellAnchor><xdr:from><xdr:col>17</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>
<xdr:to><xdr:col>17</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>2</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>
<xdr:pic><xdr:blipFill><a:blip r:embed="rId1"/></xdr:blipFill></xdr:pic>
</xdr:twoCellAnchor></xdr:wsDr>`

	const drawingRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/>
</Relationships>`

	parts := []struct {
		name string
		data []byte
	}{
		{"xl/worksheets/sheet1.xml", []byte(sheetXML)},
		{"xl/worksheets/_rels/sheet1.xml.rels", []byte(sheetRels)},
		{"xl/drawings/drawing1.xml", []byte(drawingXML)},
		{"xl/drawings/_rels/drawing1.xml.rels", []byte(drawingRels)},
		{"xl/media/image1.png", png},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			t.Fatalf("创建 zip 条目失败: %v", err)
		}
		if _, err := w.Write(p.data); err != nil {
			t.Fatalf("写入 zip 条目失败: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	return buf.Bytes()
}

// TestExtractQianJiEmbeddedImages 验证：内嵌图片按锚点行号提取并转存到本系统存储。
func TestExtractQianJiEmbeddedImages(t *testing.T) {
	dir := t.TempDir()
	storage.SetLocalDir(dir)
	defer os.RemoveAll(dir)

	data := buildQianJiXLSXWithImage(t)
	rowImgs, err := extractQianJiEmbeddedImages(data, 7)
	if err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	if len(rowImgs) != 1 {
		t.Fatalf("期望仅 row1 有图片，实际: %#v", rowImgs)
	}
	urls := rowImgs[1]
	if len(urls) != 1 {
		t.Fatalf("期望 row1 有 1 张图，实际 %d: %#v", len(urls), urls)
	}
	if !strings.HasPrefix(urls[0], "/api/uploads/") {
		t.Fatalf("图片未被转存为本系统路径: %q", urls[0])
	}
	// 校验文件确实落盘且内容一致
	key := strings.TrimPrefix(urls[0], "/api/uploads/")
	onDisk := filepath.Join(dir, key)
	if _, err := os.Stat(onDisk); err != nil {
		t.Fatalf("转存后的文件未落盘: %v", err)
	}
	got, err := os.ReadFile(onDisk)
	if err != nil {
		t.Fatalf("读取落盘文件失败: %v", err)
	}
	want, _ := base64.StdEncoding.DecodeString(tinyPNGb64)
	if !bytes.Equal(got, want) {
		t.Fatalf("落盘内容不一致: len(got)=%d len(want)=%d", len(got), len(want))
	}
}

// TestParseQianJi_RowsWithEmbeddedImages 验证：提取出的图片按行号正确挂到对应交易上。
func TestParseQianJi_RowsWithEmbeddedImages(t *testing.T) {
	dir := t.TempDir()
	storage.SetLocalDir(dir)
	defer os.RemoveAll(dir)

	data := buildQianJiXLSXWithImage(t)
	rowImgs, err := extractQianJiEmbeddedImages(data, 7)
	if err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	if len(rowImgs) != 1 || len(rowImgs[1]) != 1 {
		t.Fatalf("前置条件失败: %#v", rowImgs)
	}

	rows := qianjiSampleRows()
	txs, err := parseQianJiRows(rows, rowImgs, 7, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txs) == 0 {
		t.Fatalf("期望至少 1 笔交易")
	}
	// 锚点行号为 1（rows 索引 1 = 第 1 笔数据）
	if len(txs[0].Images) != 1 || txs[0].Images[0] != rowImgs[1][0] {
		t.Errorf("第 1 笔交易未正确挂载图片: %#v，期望 %v", txs[0].Images, rowImgs[1])
	}
	if len(txs) > 1 && len(txs[1].Images) != 0 {
		t.Errorf("第 2 笔交易不应有图片: %#v", txs[1].Images)
	}
}

// TestCleanupOrphanImageDeletesUnused 验证：交易删除后其图片成为孤儿，宽限期外被自动清理。
func TestCleanupOrphanImageDeletesUnused(t *testing.T) {
	dir := t.TempDir()
	storage.SetLocalDir(dir)
	defer os.RemoveAll(dir)

	url, err := storage.SaveBytes(pngBytes(), "receipt.png", 7)
	if err != nil {
		t.Fatalf("SaveBytes 失败: %v", err)
	}
	key, ok := storage.KeyFromURL(url)
	if !ok {
		t.Fatalf("KeyFromURL 解析失败: %q", url)
	}
	onDisk := filepath.Join(dir, key)
	if _, err := os.Stat(onDisk); err != nil {
		t.Fatalf("前置：文件未落盘: %v", err)
	}

	// 模拟交易被删除：key 不再被引用。宽限期 0 -> 立即可清理。
	deleted, err := storage.CleanupOrphans(map[string]bool{}, 0)
	if err != nil {
		t.Fatalf("CleanupOrphans 失败: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("期望删除 1 个孤儿文件，实际 %d", deleted)
	}
	if _, err := os.Stat(onDisk); !os.IsNotExist(err) {
		t.Fatalf("孤儿文件未被删除: %v", err)
	}

	// 再次清理应为 0
	if deleted, _ := storage.CleanupOrphans(map[string]bool{}, 0); deleted != 0 {
		t.Fatalf("二次清理期望 0，实际 %d", deleted)
	}
}

func pngBytes() []byte {
	b, _ := base64.StdEncoding.DecodeString(tinyPNGb64)
	return b
}
