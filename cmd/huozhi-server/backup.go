package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"huozhi/internal/backup"
	"huozhi/internal/config"
)

// runBackup 实现 `huozhi-server backup`：把一份备份写到 w（由调用方给 os.Stdout）。
//
// db（默认）= SQLite 自身的一致性快照（VACUUM INTO），产物只有数据库；
// zip = 快照 + upload.path 目录下的账单图片，货殖的图片存在库外，只有 zip 形态
// 是「一份产物能还原全部」。
// 默认取 db 是为了对上备份工具的默认扩展名：改用 zip 必须同时告诉它 BACKUP_SQLITE_EXT=zip，
// 否则工具会拿 SQLite 魔数去校验 zip 产物，这一轮备份判失败（不静默存坏备份）。
//
// 库路径与图片路径都沿用应用自己的配置（-c 指向的 config.yaml + HZ_DB_DSN / HZ_UPLOAD_PATH），
// 外部工具不必再配一份路径——应用最清楚自己的库在哪。
//
// 日志一律走 stderr，stdout 只承载产物字节：所以这条路径不碰 gorm（logger.Default 写 stdout）、
// 不起 gin（DefaultWriter 同样是 stdout），也不执行 main 里那几步建目录的副作用。
func runBackup(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("huozhi-server backup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr) // 用法错误与 -h 只能进 stderr，stdout 留给产物
	cfgPath := fs.String("c", "./config.yaml", "配置文件路径（与服务启动口径一致）")
	format := fs.String("format", "db", "导出格式：db（SQLite 一致性快照）或 zip（快照 + 账单图片）")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // -h 已自行打印到 stderr，不算失败
		}
		return err
	}

	cfg, err := loadExportConfig(*cfgPath)
	if err != nil {
		return err
	}

	// 驱动闸门：货殖可以切 Postgres（HZ_DB_DRIVER / database.driver），
	// 而 VACUUM INTO 只对 SQLite 有意义。真对着 Postgres 部署产出一份「.db」，
	// 工具侧魔数校验会过、清单会记成功，实际是零数据——这是最坏的一种静默漏备份。
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if driver == "" {
		driver = "sqlite" // 与 database.Init 的分支一致：driver 留空按 sqlite 处理
	}
	if driver != "sqlite" {
		return fmt.Errorf("当前数据库驱动是 %q，backup 子命令只导出 SQLite 库：Postgres 部署不该走本应用，"+
			"请让备份工具的 postgres provider 直接对着 Postgres 容器执行 pg_dump（它不依赖应用侧导出）", cfg.Database.Driver)
	}

	conn, err := backup.OpenForExport(cfg.Database.File)
	if err != nil {
		return err
	}
	defer conn.Close()

	// S3 后端下账单图片在对象存储里，本地 upload.path 根本没有内容：
	// 这时 zip 只有库快照，必须把这件事说出来，不能让运维以为 zip 就一定是完整备份。
	uploads := cfg.Upload.Path
	if cfg.S3.Enabled {
		uploads = ""
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "db":
		if err := backup.WriteSnapshot(conn, w); err != nil {
			return err
		}
		switch {
		case uploads != "":
			log.Printf("[backup] db 快照完成（不含账单图片：图片在 %s 下，要一次带走全部请用 --format zip）", uploads)
		default:
			log.Printf("[backup] db 快照完成（S3 后端，账单图片不在本地目录）")
		}
		return nil
	case "zip":
		if uploads == "" {
			log.Printf("[backup] 存储后端是 S3 (bucket=%s)，账单图片不在本地目录，zip 里只有库快照；"+
				"图片请由 S3 侧单独备份", cfg.S3.Bucket)
		}
		n, err := backup.WriteZIP(conn, uploads, w)
		if err != nil {
			return err
		}
		if uploads == "" {
			log.Printf("[backup] zip 完成：库快照（S3 后端，无本地图片）")
		} else {
			log.Printf("[backup] zip 完成：库快照 + %d 个账单图片文件（来自 %s）", n, uploads)
		}
		return nil
	default:
		return fmt.Errorf("未知格式 %q（可选 db / zip）", *format)
	}
}

// loadExportConfig 按服务同样的口径解析配置（-c 指向的 config.yaml + 环境变量覆盖），
// 但**不做 main 里那几步目录副作用**（MkdirAll 上传目录）。
//
// 配置口径与启动路径完全一致：配置文件读不到就失败，不退回 config.Default()。环境变量覆盖
// （HZ_DB_DSN / HZ_UPLOAD_PATH）是 config.Load 读到文件之后才应用的——缺配置时退回默认值，
// 等于绕过运维配的 HZ_DB_DSN 去快照一个没人要求的 ./huozhi.db，那是一份「看起来合法但备错库」
// 的产物。启动路径同样不接受默认值（见 main 里加载配置那段），两边一条规则。
func loadExportConfig(cfgPath string) (*config.Config, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		// 用 errors.Is 而不是 os.IsNotExist：config.Load 把 os.ReadFile 的错误包了一层
		// （fmt.Errorf("read config file: %w")），os.IsNotExist 不认包装过的错误。
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s 不存在；导出用与启动相同的配置口径（-c / 缺省 ./config.yaml），"+
				"库路径取 database.file（可被 HZ_DB_DSN 覆盖），不接受缺省配置的猜测值", cfgPath)
		}
		return nil, err
	}
	// 与 main 一致：挂到全局，便于排查「导出用的到底是哪份配置」
	config.AppConfig = cfg
	return cfg, nil
}
