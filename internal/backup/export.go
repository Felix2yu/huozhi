// Package backup 是 `huozhi-server backup` 子命令的导出路径：把一份 SQLite 一致性
// 快照（zip 形态再带上库外的账单图片）流式写到调用方给的 writer 上。
//
// 外部备份工具（docker-db-auto-backup）经 `docker exec` 调起本进程并把 stdout 当产物，
// 所以这里必须走临时目录 + 流式输出，不能在数据目录里留任何中间文件。
//
// 与 internal/handlers 那两条备份路径的分工：
//   - 界面「全量导出 / 自动备份」（handlers.ExportBackup、generateAutoBackup）按**单个用户**
//     导业务数据，包内是 backup.json + images/<key>，能被 POST /api/io/restore 读回去；
//   - 本包导的是**整库**快照（含全部用户、用户账号本身、库外文件），面向冷备份：
//     产物经界面导入会被判「ZIP中未找到backup.json」，恢复方式是把文件放回数据目录
//     （见 README「数据备份与恢复」）。两种布局刻意不通用——一份整库快照塞进按用户
//     合并导入的通道里，语义上根本对不上。
package backup

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	// 导出用 database/sql 直开，不走 gorm：gorm.Open 会挂上 logger.Default（它写 stdout，
	// 而 stdout 只能装产物字节），AutoMigrate 更会改写库结构。
	// 但驱动必须是同一个纯 Go 实现——github.com/glebarez/sqlite 注册 database/sql 的
	// "sqlite" 驱动（内部就是它包着的那份 SQLite），取它的 DriverName 常量既保证驱动
	// 已注册，也把「与服务用同一个 SQLite 实现」这件事写死在代码里。
	sqlite "github.com/glebarez/sqlite"
)

// 压缩包内部的固定布局。回档时按同样的名字把两样东西放回原处，改这里要同步改 README。
const (
	// ZipDBEntry 是放在包根的库快照文件名（与镜像里的二进制同名，好认）。
	ZipDBEntry = "huozhi.db"
	// ZipUploadsDir 是账单图片在包内的目录前缀（含结尾斜杠，便于直接拼接相对路径）。
	// 包内路径与 upload.path 下的相对路径一一对应，解包即可原样放回。
	ZipUploadsDir = "uploads/"
)

// OpenForExport 以「只做导出」的方式打开既有数据库：不建目录、不跑迁移、不写种子数据。
//
// 导出进程是在应用主进程正在读写这个库的时候被外部工具调起来的，它只读数据、不拥有这个库：
//   - 文件不存在直接报错。那是路径配错，不是「该新建一个空库」——SQLite 驱动会顺手把空库
//     建出来，而工具会把这份空快照当成一次成功备份存进档案。
//   - DSN 只带 busy_timeout，刻意不设 journal_mode / foreign_keys / auto_vacuum：这些 pragma
//     是写操作，会改库头，把「导出」变成一次 schema 层面的改动（WAL 会持久写进库头）。
//     已提交在 -wal 里的数据照常读得到，SQLite 自己会读它。
//   - 不创建任何父目录：main 里那句 os.MkdirAll(Upload.Path) 属于服务路径，导出路径绕开它。
func OpenForExport(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, errors.New("数据库路径为空（database.file / HZ_DB_DSN）")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("数据文件不可用: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath)
	conn, err := sql.Open(sqlite.DriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	// 导出是顺序读，一条连接足够，也避免多余连接各自开 -shm 句柄。
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	return conn, nil
}

// WriteSnapshot 把数据库以 SQLite 自身的一致性快照写到 w。
//
// 用 VACUUM INTO 而不是复制文件：热拷贝正在被写入的 db/-wal/-shm 不是原子操作，
// 可能拷出损坏的副本或漏掉 WAL 里已提交的数据；VACUUM INTO 由 SQLite 在事务里生成
// 一份自洽的快照，WAL 模式下也不阻塞写入。
//
// 快照先落在临时目录再流式输出：VACUUM INTO 要求目标不存在，而 stdout 不能倒回去重写。
// defer 删临时目录，调用方读到一半断开也不会在数据目录里留下半成品。
func WriteSnapshot(db *sql.DB, w io.Writer) error {
	dir, err := os.MkdirTemp("", "huozhi-snapshot-")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(dir)

	target := filepath.Join(dir, "snapshot.db")
	if _, err := db.Exec("VACUUM INTO ?", target); err != nil {
		return fmt.Errorf("导出数据库快照失败: %w", err)
	}
	f, err := os.Open(target)
	if err != nil {
		return fmt.Errorf("读取快照失败: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("写出快照失败: %w", err)
	}
	return nil
}

// WriteZIP 把「库快照 + uploads 目录」打成 zip 流式写到 w，返回打包进去的图片文件数。
//
// 账单图片存在库外（upload.path），只备份 .db 会丢图片、只拷图片会丢账本，
// 所以 zip 形态才是「一份产物能还原全部」。zip.Writer 本身就是流式的：
// 逐个文件拷进去，整包不在内存里攒。
//
// uploadsDir 传空串表示「这个部署没有库外文件可带」：S3 后端下图片在对象存储里，
// 本地目录根本没有内容（此时 zip 里只有库快照，调用方负责把这件事说清楚）。
func WriteZIP(db *sql.DB, uploadsDir string, w io.Writer) (int, error) {
	dir, err := os.MkdirTemp("", "huozhi-zip-")
	if err != nil {
		return 0, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(dir)

	snap := filepath.Join(dir, "snapshot.db")
	if _, err := db.Exec("VACUUM INTO ?", snap); err != nil {
		return 0, fmt.Errorf("导出数据库快照失败: %w", err)
	}

	zw := zip.NewWriter(w)
	if err := zipOne(zw, ZipDBEntry, snap); err != nil {
		_ = zw.Close() // 尽力写完中央目录，让半截包在解包时还能报出前缀
		return 0, err
	}
	count, err := zipDir(zw, uploadsDir)
	if err != nil {
		_ = zw.Close()
		return count, err
	}
	if err := zw.Close(); err != nil {
		return count, fmt.Errorf("完成压缩包失败: %w", err)
	}
	return count, nil
}

// zipOne 把磁盘上的一个文件作为指定条目写进压缩包。
func zipOne(zw *zip.Writer, name, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", name, err)
	}
	defer f.Close()
	hw, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("创建压缩包条目 %s 失败: %w", name, err)
	}
	if _, err := io.Copy(hw, f); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", name, err)
	}
	return nil
}

// zipDir 把 dir 下的普通文件按相对路径写进 uploads/ 前缀之下，返回打包个数。
//
// 目录不存在返回 0 而不是报错——一个还没上传过账单图片的库是正常的。
// 非常规文件（符号链接、套接字之类）跳过：备份包里塞一条指向数据目录之外的链接，
// 回档时就成了越界写。
func zipDir(zw *zip.Writer, dir string) (int, error) {
	if dir == "" {
		return 0, nil
	}
	count := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			log.Printf("[backup] 跳过 uploads 里的非常规文件 %s", p)
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		// 包内路径必须用正斜杠：zip 规范如此，filepath 的分隔符在别的平台上读不出来
		if err := zipOne(zw, ZipUploadsDir+filepath.ToSlash(rel), p); err != nil {
			return err
		}
		count++
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return count, err
}
