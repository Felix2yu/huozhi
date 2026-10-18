package main

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
	"strings"
	"testing"
	"time"

	"huozhi/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 备份子命令的测试口径：走 runBackup（与 main 拦截后调的是同一个函数），
// 库路径一律通过 -c 指向临时目录里的 config.yaml，验证「沿用应用自己的配置」这条前提。
// 产物字节进 bytes.Buffer，失败时必须是零字节——外部工具把 stdout 当档案。

// backupDir 返回一个临时数据目录（绝对路径，供 config.yaml 引用）。
func backupDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// backupWriteConfig 写一份最小 config.yaml（sqlite + 指定库/上传目录），返回其路径。
func backupWriteConfig(t *testing.T, dir, driver, dbPath, uploadPath string) string {
	t.Helper()
	cfg := "database:\n  driver: \"" + driver + "\"\n  file: \"" + dbPath + "\"\n" +
		"upload:\n  path: \"" + uploadPath + "\"\n"
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatalf("写 config: %v", err)
	}
	return path
}

// backupAppDB 按服务口径（gorm + AutoMigrate）建一份已迁移的库并塞一行流水。
//
// 连接刻意保持打开：`huozhi-server backup` 正是在应用运行中被 docker exec 调起来的，
// 两条连接同时存在才是真实场景。
func backupAppDB(t *testing.T, dbPath string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 测试进程里也不要 gorm 往 stdout 灌水
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{}, &models.Book{}, &models.BookMember{}, &models.AccountGroup{},
		&models.Account{}, &models.Category{}, &models.Tag{}, &models.Transaction{},
		&models.TransactionTag{}, &models.Budget{}, &models.SavingPlan{}, &models.SavingRecord{},
		&models.Recurring{}, &models.Installment{}, &models.Reimbursement{}, &models.Loan{},
		&models.LoanRepayment{}, &models.AssetSnapshot{}, &models.SyncLog{}, &models.ExchangeRate{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// backupSeedTx 写一条合成流水并返回它的描述，供「快照里查得到这一行」作断言。
func backupSeedTx(t *testing.T, db *gorm.DB, desc string) string {
	t.Helper()
	tx := models.Transaction{
		UserID: 1, BookID: 1, Type: models.TxExpense, Amount: 1234, Currency: "CNY",
		CategoryID: 1, AccountID: 1, TxDate: time.Now(), Description: desc,
		IncludeInBalance: true, IncludeInBudget: true,
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("建流水: %v", err)
	}
	return tx.Description
}

// backupMustOpenExport 以导出口径打开一份产物，用于「快照到底还能不能读」这条断言。
func backupMustOpenExport(t *testing.T, path string) *sql.DB {
	t.Helper()
	conn, err := sql.Open(sqlite.DriverName, "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("打开产物 %s: %v", path, err)
	}
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		conn.Close()
		t.Fatalf("连接产物 %s: %v", path, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func backupCountTx(t *testing.T, conn *sql.DB, desc string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow("SELECT count(*) FROM transactions WHERE description = ?", desc).Scan(&n); err != nil {
		t.Fatalf("查 %s: %v", desc, err)
	}
	return n
}

func backupIntegrity(t *testing.T, conn *sql.DB) {
	t.Helper()
	var msg string
	if err := conn.QueryRow("PRAGMA integrity_check").Scan(&msg); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if msg != "ok" {
		t.Errorf("integrity_check = %q, want ok", msg)
	}
}

func backupFileSum(t *testing.T, path string) string {
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

// backupWriteFile 落一个文件（测试夹具）。
func backupWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录 %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写 %s: %v", path, err)
	}
}

const sqliteMagic = "SQLite format 3\x00"

// backupPNG 是一张最小合成位图：内容不参与导出逻辑，只为让 uploads 里有东西可打包。
var backupPNG = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 'h', 'u', 'o', 'z', 'h', 'i'}

// ===== (a) db：真迁移库 + 数据的快照，产物还能读回来 =====

func TestRunBackupDBSnapshotIsReadableSQLite(t *testing.T) {
	dir := backupDir(t)
	dbPath := filepath.Join(dir, "huozhi.db")
	cfgPath := backupWriteConfig(t, dir, "sqlite", dbPath, filepath.Join(dir, "uploads"))
	db := backupAppDB(t, dbPath)

	// 导出前一刻再写一行：它此刻还在应用的连接里（本应用用默认回滚日志，
	// WAL 部署下这类行只活在 -wal 里），快照必须带上——漏掉最近的记录等于漏掉备份。
	last := backupSeedTx(t, db, "合成流水-最新")
	backupSeedTx(t, db, "合成流水-较早")

	var buf bytes.Buffer
	if err := runBackup([]string{"-c", cfgPath}, &buf); err != nil {
		t.Fatalf("runBackup: %v", err)
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
	backupWriteFile(t, snap, data)
	conn := backupMustOpenExport(t, snap)
	backupIntegrity(t, conn)
	for _, desc := range []string{"合成流水-较早", last} {
		if got := backupCountTx(t, conn, desc); got != 1 {
			t.Errorf("快照里 %q 应有 1 行，得到 %d", desc, got)
		}
	}
}

// ===== (b) zip：库快照 + 账单图片，字节一致 =====

func TestRunBackupZIPContainsSnapshotAndUploads(t *testing.T) {
	dir := backupDir(t)
	dbPath := filepath.Join(dir, "huozhi.db")
	uploads := filepath.Join(dir, "uploads")
	cfgPath := backupWriteConfig(t, dir, "sqlite", dbPath, uploads)
	db := backupAppDB(t, dbPath)
	desc := backupSeedTx(t, db, "zip-快照行")

	// 账单图片在库外：zip 形态的意义就是把它们和库一起带走。
	// 目录层级按本地存储的键名口径 <uid>/<随机名>.png。
	backupWriteFile(t, filepath.Join(uploads, "1", "bill.png"), backupPNG)
	backupWriteFile(t, filepath.Join(uploads, "nested-bill.png"), []byte("nested-bytes"))

	var buf bytes.Buffer
	if err := runBackup([]string{"-c", cfgPath, "--format", "zip"}, &buf); err != nil {
		t.Fatalf("runBackup zip: %v", err)
	}
	data := buf.Bytes()
	if len(data) < 22 || string(data[:2]) != "PK" {
		t.Fatalf("产物不是 zip: %d 字节", len(data))
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip 读不出来（中央目录可能没写完）: %v", err)
	}
	entries := map[string][]byte{}
	var list []string
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
		entries[f.Name] = body
		list = append(list, f.Name)
	}
	t.Logf("zip 条目: %v", list)

	snap, ok := entries["huozhi.db"]
	if !ok {
		t.Fatalf("zip 里缺 huozhi.db，实际条目 %v", list)
	}
	if string(snap[:16]) != sqliteMagic {
		t.Errorf("包内快照头部不是 SQLite 魔数: %q", string(snap[:16]))
	}
	snapPath := filepath.Join(t.TempDir(), "from-zip.db")
	backupWriteFile(t, snapPath, snap)
	conn := backupMustOpenExport(t, snapPath)
	backupIntegrity(t, conn)
	if got := backupCountTx(t, conn, desc); got != 1 {
		t.Errorf("包内快照应能读到 %q，得到 %d", desc, got)
	}
	for name, want := range map[string][]byte{
		"uploads/1/bill.png":      backupPNG,
		"uploads/nested-bill.png": []byte("nested-bytes"),
	} {
		got, ok := entries[name]
		if !ok {
			t.Errorf("zip 里缺条目 %s（实际 %v）", name, list)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("条目 %s 内容不符: %q", name, got)
		}
	}
}

// ===== (c) 库不存在：报错、零字节、什么都不建 =====

func TestRunBackupMissingDBFailsAndCreatesNothing(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	dbPath := filepath.Join(dataDir, "huozhi.db")
	uploads := filepath.Join(dataDir, "uploads")
	cfgPath := backupWriteConfig(t, root, "sqlite", dbPath, uploads)

	var buf bytes.Buffer
	err := runBackup([]string{"-c", cfgPath}, &buf)
	if err == nil {
		t.Fatal("库不存在时应当失败")
	}
	t.Logf("报错文本: %v", err)
	if buf.Len() != 0 {
		t.Errorf("失败时 stdout 必须零字节，得到 %d 字节", buf.Len())
	}
	// 导出进程不许建库，也不许把上传目录顺手建出来（main 里的 MkdirAll 属于服务路径）
	if _, err := os.Stat(dbPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("不该创建数据库文件: %v", err)
	}
	if _, err := os.Stat(uploads); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("不该创建上传目录: %v", err)
	}
	// 配置文件读不到时必须直接报错，不能退回 config.Default() 去猜一个 ./huozhi.db：
	// HZ_DB_DSN 这类覆盖是 config.Load 读到文件之后才应用的，绕过文件就等于备份错库。
	var buf2 bytes.Buffer
	missing := filepath.Join(root, "elsewhere", "config.yaml")
	err = runBackup([]string{"-c", missing}, &buf2)
	if err == nil {
		t.Fatal("配置文件不存在时应当失败")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("报错要指出读不到哪个配置: %v", err)
	}
	if buf2.Len() != 0 {
		t.Errorf("失败时 stdout 必须零字节，得到 %d 字节", buf2.Len())
	}
}

// ===== (d) 导出路径不跑迁移、不改源库 =====

// TestRunBackupDoesNotMigrateOldSchema 造一份「旧 schema」的库：transactions 表缺
// 后来才加的描述列。AutoMigrate 会把它补齐——导出进程绝不能碰表结构。
func TestRunBackupDoesNotMigrateOldSchema(t *testing.T) {
	dir := backupDir(t)
	dbPath := filepath.Join(dir, "huozhi.db")
	cfgPath := backupWriteConfig(t, dir, "sqlite", dbPath, filepath.Join(dir, "uploads"))

	conn, err := sql.Open(sqlite.DriverName, dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE transactions (id INTEGER PRIMARY KEY, amount INTEGER)`); err != nil {
		t.Fatalf("建旧表: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO transactions(id, amount) VALUES(1, 100)`); err != nil {
		t.Fatalf("塞数据: %v", err)
	}
	conn.Close()
	before := backupFileSum(t, dbPath)

	var buf bytes.Buffer
	if err := runBackup([]string{"-c", cfgPath}, &buf); err != nil {
		t.Fatalf("runBackup: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("db 形态产物为空")
	}
	if after := backupFileSum(t, dbPath); after != before {
		t.Error("备份过程改写了源库文件（sha256 不一致）")
	}

	probe, err := sql.Open(sqlite.DriverName, "file:"+dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("复查打开失败: %v", err)
	}
	defer probe.Close()
	var cols int
	// gorm 的 AutoMigrate 会 ALTER TABLE 加回缺失列；列数仍是 2 才证明它没跑
	if err := probe.QueryRow(`SELECT count(*) FROM pragma_table_info('transactions')`).Scan(&cols); err != nil {
		t.Fatalf("查列数: %v", err)
	}
	if cols != 2 {
		t.Errorf("旧 schema 被改写了：transactions 有 %d 列，仍是 2 列才算没跑迁移", cols)
	}
	var sqlText string
	if err := probe.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'transactions'`).Scan(&sqlText); err != nil {
		t.Fatalf("查建表语句: %v", err)
	}
	if !strings.Contains(sqlText, "amount INTEGER") || strings.Contains(sqlText, "description") {
		t.Errorf("建表语句已不是原样: %q", sqlText)
	}
}

// ===== (e) Postgres 部署：拒绝导出，零字节 =====

func TestRunBackupRefusesNonSQLiteDriver(t *testing.T) {
	for _, tc := range []struct{ name, driver, env string }{
		{"配置文件里是 postgres", "postgres", ""},
		{"环境变量覆盖成 postgres", "sqlite", "postgres"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := backupDir(t)
			dbPath := filepath.Join(dir, "huozhi.db")
			// 先按 sqlite 建一份真库：拒绝必须发生在打开库之前，产物不该有任何字节
			backupAppDB(t, dbPath)
			cfgPath := backupWriteConfig(t, dir, tc.driver, dbPath, filepath.Join(dir, "uploads"))
			if tc.env != "" {
				t.Setenv("HZ_DB_DRIVER", tc.env)
			}
			before := backupFileSum(t, dbPath)

			var buf bytes.Buffer
			err := runBackup([]string{"-c", cfgPath}, &buf)
			if err == nil {
				t.Fatal("非 sqlite 驱动应当拒绝导出")
			}
			if buf.Len() != 0 {
				t.Errorf("拒绝时 stdout 必须零字节，得到 %d 字节", buf.Len())
			}
			if !strings.Contains(err.Error(), "postgres") {
				t.Errorf("报错要说清是 Postgres 部署: %v", err)
			}
			t.Logf("报错文本: %v", err)
			if after := backupFileSum(t, dbPath); after != before {
				t.Error("拒绝路径不该碰源库")
			}
		})
	}
}

// ===== (f) 默认格式是 db =====

func TestRunBackupDefaultsToDBFormat(t *testing.T) {
	dir := backupDir(t)
	dbPath := filepath.Join(dir, "huozhi.db")
	uploads := filepath.Join(dir, "uploads")
	cfgPath := backupWriteConfig(t, dir, "sqlite", dbPath, uploads)
	db := backupAppDB(t, dbPath)
	backupSeedTx(t, db, "默认格式行")
	// 图片在库里也有，但默认形态不该把它们顺手打包走
	backupWriteFile(t, filepath.Join(uploads, "1", "bill.png"), backupPNG)

	var buf bytes.Buffer
	if err := runBackup([]string{"-c", cfgPath}, &buf); err != nil {
		t.Fatalf("runBackup（省略 --format）: %v", err)
	}
	data := buf.Bytes()
	// 默认必须是 db：备份工具的默认扩展名就是 .db，输出 zip 会被按 SQLite 魔数校验判失败
	if string(data[:2]) == "PK" {
		t.Fatal("省略 --format 时不该默认输出 zip")
	}
	if string(data[:16]) != sqliteMagic {
		t.Errorf("默认产物应是 SQLite 快照，头部是 %q", string(data[:16]))
	}
}

// TestRunBackupUsesConfiguredDSN 库路径来自应用自己的配置口径：
// 配置文件里的 database.file 会被 HZ_DB_DSN 覆盖（driver=sqlite 时），导出必须跟着走。
func TestRunBackupUsesConfiguredDSN(t *testing.T) {
	dir := backupDir(t)
	other := filepath.Join(dir, "elsewhere", "hz.db")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	db := backupAppDB(t, other)
	desc := backupSeedTx(t, db, "DSN 覆盖行")

	// 配置文件里的 file 指向一个不存在的库，HZ_DB_DSN 指向真库
	cfgPath := backupWriteConfig(t, dir, "sqlite", filepath.Join(dir, "wrong.db"), filepath.Join(dir, "uploads"))
	t.Setenv("HZ_DB_DSN", other)

	var buf bytes.Buffer
	if err := runBackup([]string{"-c", cfgPath}, &buf); err != nil {
		t.Fatalf("runBackup: %v", err)
	}
	snap := filepath.Join(t.TempDir(), "from-dsn.db")
	backupWriteFile(t, snap, buf.Bytes())
	if got := backupCountTx(t, backupMustOpenExport(t, snap), desc); got != 1 {
		t.Errorf("导出的应是 HZ_DB_DSN 指向的库，得到 %d 行", got)
	}
}

func TestRunBackupRejectsUnknownFormat(t *testing.T) {
	dir := backupDir(t)
	dbPath := filepath.Join(dir, "huozhi.db")
	cfgPath := backupWriteConfig(t, dir, "sqlite", dbPath, filepath.Join(dir, "uploads"))
	db := backupAppDB(t, dbPath)
	backupSeedTx(t, db, "未知格式行")

	for _, args := range [][]string{{"-c", cfgPath, "--format", "tar"}, {"-c", cfgPath, "--format"}} {
		var buf bytes.Buffer
		err := runBackup(args, &buf)
		if err == nil {
			t.Fatalf("%v 应当报错", args)
		}
		if buf.Len() != 0 {
			t.Errorf("%v 失败时 stdout 必须零字节，得到 %d 字节", args, buf.Len())
		}
	}
}

// TestRunBackupHelpExitsQuietly -h 走 flag 的用法输出（已重定向到 stderr），
// 不当失败、也不往 stdout 写任何东西。
func TestRunBackupHelpExitsQuietly(t *testing.T) {
	var buf bytes.Buffer
	if err := runBackup([]string{"-h"}, &buf); err != nil {
		t.Fatalf("-h 不该算失败: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("-h 时 stdout 必须零字节，得到 %d 字节", buf.Len())
	}
}

// TestRunBackupS3ModeZipStillSucceeds S3 后端下本地没有图片可带：
// zip 仍然要成功（里面只有库快照），不能因为目录空着就判失败。
func TestRunBackupS3ModeZipStillSucceeds(t *testing.T) {
	dir := backupDir(t)
	dbPath := filepath.Join(dir, "huozhi.db")
	db := backupAppDB(t, dbPath)
	backupSeedTx(t, db, "S3 模式行")

	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := "database:\n  driver: \"sqlite\"\n  file: \"" + dbPath + "\"\n" +
		"upload:\n  path: \"" + filepath.Join(dir, "uploads") + "\"\n" +
		"s3:\n  enabled: true\n  bucket: \"huozhi-test\"\n  access_key: \"a\"\n  secret_key: \"b\"\n"
	backupWriteFile(t, cfgPath, []byte(cfg))

	var buf bytes.Buffer
	if err := runBackup([]string{"-c", cfgPath, "--format", "zip"}, &buf); err != nil {
		t.Fatalf("S3 模式下 zip 应照常成功: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip 读不出来: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "huozhi.db" {
		t.Errorf("S3 模式下包里只该有库快照，实际 %d 个条目", len(zr.File))
	}
	// 本地上传目录一个文件也没有（导出路径不建目录）
	if _, err := os.Stat(filepath.Join(dir, "uploads")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("不该创建上传目录: %v", err)
	}
}
