package backup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	sqlite "github.com/glebarez/sqlite"
)

// mustOpenCreate 用**服务同款驱动**建一份库并跑给定 DDL/写入，返回文件路径。
// 测试里只建一张自己的小表：本包不认业务 schema，只认「能不能拿出一致快照」。
func mustOpenCreate(t *testing.T, ddl []string, rows [][]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "huozhi.db")
	conn, err := sql.Open(sqlite.DriverName, path)
	if err != nil {
		t.Fatalf("建库失败: %v", err)
	}
	defer conn.Close()
	for _, stmt := range ddl {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("执行 DDL 失败: %v", err)
		}
	}
	for _, args := range rows {
		if _, err := conn.Exec("INSERT INTO items(id, name) VALUES(?, ?)", args[0], args[1]); err != nil {
			t.Fatalf("插入失败: %v", err)
		}
	}
	return path
}

func mustIntegrity(t *testing.T, conn *sql.DB) {
	t.Helper()
	var msg string
	if err := conn.QueryRow("PRAGMA integrity_check").Scan(&msg); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if msg != "ok" {
		t.Errorf("integrity_check = %q, want ok", msg)
	}
}

func countItems(t *testing.T, conn *sql.DB, name string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow("SELECT count(*) FROM items WHERE name = ?", name).Scan(&n); err != nil {
		t.Fatalf("查 %s: %v", name, err)
	}
	return n
}

func fileSum(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开 %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("摘要 %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// zipEntries 解开产物，返回条目名 -> 内容；顺带检查包内快照的魔数。
func zipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip 读不出来（中央目录可能没写完）: %v", err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("打开条目 %s: %v", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("读条目 %s: %v", f.Name, err)
		}
		if f.Name == ZipDBEntry && len(body) < 16 {
			t.Errorf("包内快照短得不像一份库: %d 字节", len(body))
		} else if f.Name == ZipDBEntry && string(body[:16]) != "SQLite format 3\x00" {
			t.Errorf("包内快照头部不是 SQLite 魔数: %q", string(body[:16]))
		}
		out[f.Name] = body
	}
	return out
}

const sqliteMagic = "SQLite format 3\x00"

// TestOpenForExportMissingFileFailsAndCreatesNothing 库不存在时必须失败：
// SQLite 驱动会顺手建出空库，而外部工具会把那份空快照当成一次成功备份存进档案。
func TestOpenForExportMissingFileFailsAndCreatesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope", "huozhi.db")
	if _, err := OpenForExport(path); err == nil {
		t.Fatal("库文件不存在时应当失败")
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("不该创建数据库文件: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("不该创建父目录: %v", err)
	}
	if _, err := OpenForExport(""); err == nil {
		t.Error("空路径应当失败")
	}
}

// TestOpenForExportLeavesSourceUntouched 导出连接不改写源库：
// 不跑迁移（schema 保持原样）、不设 journal_mode（那会改库头）。
func TestOpenForExportLeavesSourceUntouched(t *testing.T) {
	path := mustOpenCreate(t,
		[]string{"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"},
		[][]any{{1, "早餐"}})
	before := fileSum(t, path)

	conn, err := OpenForExport(path)
	if err != nil {
		t.Fatalf("OpenForExport: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteSnapshot(conn, &buf); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	conn.Close()

	if got := fileSum(t, path); got != before {
		t.Errorf("导出改写了源库（sha256 前后不一致）")
	}
	// 库头里的 journal_mode 仍是默认值：设成 WAL 是一次实打实的写操作
	reopened, err := OpenForExport(path)
	if err != nil {
		t.Fatalf("重新打开: %v", err)
	}
	defer reopened.Close()
	var mode string
	if err := reopened.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode == "wal" {
		t.Error("导出连接不该把源库切成 WAL")
	}
	if _, err := os.Stat(path + "-wal"); err == nil {
		t.Error("导出后不该留下 -wal 文件")
	}
}

func TestWriteSnapshotIsReadableSQLite(t *testing.T) {
	path := mustOpenCreate(t,
		[]string{"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"},
		[][]any{{1, "早餐"}})

	conn, err := OpenForExport(path)
	if err != nil {
		t.Fatalf("OpenForExport: %v", err)
	}
	defer conn.Close()

	var buf bytes.Buffer
	if err := WriteSnapshot(conn, &buf); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	data := buf.Bytes()
	if len(data) < 4096 {
		t.Fatalf("快照只有 %d 字节，不像一份库", len(data))
	}
	// 工具侧就是按这 16 字节判校验与命名的（docker-db-auto-backup 的 validate.go）
	if string(data[:16]) != sqliteMagic {
		t.Errorf("头部不是 SQLite 魔数: %q", string(data[:16]))
	}

	snap := filepath.Join(t.TempDir(), "snapshot.db")
	if err := os.WriteFile(snap, data, 0o644); err != nil {
		t.Fatal(err)
	}
	restored, err := OpenForExport(snap)
	if err != nil {
		t.Fatalf("打开产物失败: %v", err)
	}
	defer restored.Close()
	mustIntegrity(t, restored)
	if got := countItems(t, restored, "早餐"); got != 1 {
		t.Errorf("快照里「早餐」应有 1 行，得到 %d", got)
	}
}

// TestWriteSnapshotTempDirIsOutsideDataDir 临时快照落在 MkdirTemp，不进数据目录。
func TestWriteSnapshotCleansUpTempDir(t *testing.T) {
	path := mustOpenCreate(t, []string{"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"}, nil)
	conn, err := OpenForExport(path)
	if err != nil {
		t.Fatalf("OpenForExport: %v", err)
	}
	defer conn.Close()

	var buf bytes.Buffer
	if err := WriteSnapshot(conn, &buf); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "snapshot.db")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("快照不该落在数据目录里: %v", err)
	}
}

// TestWriteZIPCarriesSnapshotAndUploads zip 形态的意义就是把库外图片一起带走。
func TestWriteZIPCarriesSnapshotAndUploads(t *testing.T) {
	path := mustOpenCreate(t,
		[]string{"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"},
		[][]any{{1, "午餐"}})

	// 本地存储的键名口径是 <uid>/<随机名>.<扩展名>，包内保持同样的相对层级
	uploads := t.TempDir()
	nested := filepath.Join(uploads, "7")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 'h', 'u', 'o', 'z', 'h', 'i'}
	if err := os.WriteFile(filepath.Join(nested, "bill.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}

	conn, err := OpenForExport(path)
	if err != nil {
		t.Fatalf("OpenForExport: %v", err)
	}
	defer conn.Close()

	var buf bytes.Buffer
	n, err := WriteZIP(conn, uploads, &buf)
	if err != nil {
		t.Fatalf("WriteZIP: %v", err)
	}
	if n != 1 {
		t.Errorf("打包图片数 = %d, want 1", n)
	}
	entries := zipEntries(t, buf.Bytes())
	snap, ok := entries[ZipDBEntry]
	if !ok {
		t.Fatalf("zip 里缺 %s，实际 %v", ZipDBEntry, names(entries))
	}
	if string(snap[:16]) != sqliteMagic {
		t.Errorf("包内快照头部不是 SQLite 魔数: %q", string(snap[:16]))
	}
	got, ok := entries[ZipUploadsDir+"7/bill.png"]
	if !ok {
		t.Fatalf("zip 里缺 uploads/7/bill.png，实际 %v", names(entries))
	}
	if !bytes.Equal(got, png) {
		t.Errorf("图片字节不符: %q", got)
	}
}

// TestWriteZIPWithoutUploadsDir 一个还没上传过账单图片的库是正常的；
// uploadsDir 传空串（S3 后端）同样只要求包里能有快照。
func TestWriteZIPWithoutUploadsDir(t *testing.T) {
	path := mustOpenCreate(t, []string{"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"}, nil)
	conn, err := OpenForExport(path)
	if err != nil {
		t.Fatalf("OpenForExport: %v", err)
	}
	defer conn.Close()

	for _, dir := range []string{filepath.Join(t.TempDir(), "uploads"), ""} {
		var buf bytes.Buffer
		n, err := WriteZIP(conn, dir, &buf)
		if err != nil {
			t.Fatalf("uploads=%q 时 zip 应照常成功: %v", dir, err)
		}
		if n != 0 {
			t.Errorf("uploads=%q 打包数 = %d, want 0", dir, n)
		}
		entries := zipEntries(t, buf.Bytes())
		if len(entries) != 1 {
			t.Errorf("uploads=%q 只应有快照一个条目，得到 %v", dir, names(entries))
		}
	}
}

// TestWriteZIPSkipsNonRegular 符号链接一类非常规文件不进包：
// 备份里塞一条指向数据目录之外的链接，回档时就是一次越界写。
func TestWriteZIPSkipsNonRegular(t *testing.T) {
	path := mustOpenCreate(t, []string{"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"}, nil)
	uploads := t.TempDir()
	if err := os.WriteFile(filepath.Join(uploads, "real.png"), []byte("png-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(uploads, "link.png")); err != nil {
		t.Skipf("该环境不支持符号链接: %v", err)
	}

	conn, err := OpenForExport(path)
	if err != nil {
		t.Fatalf("OpenForExport: %v", err)
	}
	defer conn.Close()

	var buf bytes.Buffer
	if _, err := WriteZIP(conn, uploads, &buf); err != nil {
		t.Fatalf("WriteZIP: %v", err)
	}
	entries := zipEntries(t, buf.Bytes())
	if _, ok := entries[ZipUploadsDir+"link.png"]; ok {
		t.Error("符号链接不该进备份包")
	}
	if _, ok := entries[ZipUploadsDir+"real.png"]; !ok {
		t.Errorf("普通文件应当进包，实际 %v", names(entries))
	}
}

func names(entries map[string][]byte) []string {
	out := make([]string, 0, len(entries))
	for k := range entries {
		out = append(out, k)
	}
	return out
}
