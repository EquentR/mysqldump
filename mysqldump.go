package mysqldump

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func init() {
	// 打印 日志 行数
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

var (
	noAnno      = false
	withDbName  = ""
	needColName = false
	useBinary   = false
)

type dumpOption struct {
	// 导出表数据
	isData bool

	// 导出指定表, 与 isAllTables 互斥, isAllTables 优先级高
	tables []string
	// 导出全部表
	isAllTable bool
	// 是否删除表
	isDropTable bool

	noAnno bool

	// writer 默认为 os.Stdout
	writer io.Writer
}

type DumpOption func(*dumpOption)

// 删除表
func WithDropTable() DumpOption {
	return func(option *dumpOption) {
		option.isDropTable = true
	}
}

// 导出表数据
func WithData() DumpOption {
	return func(option *dumpOption) {
		option.isData = true
	}
}

// 导出指定表, 与 WithAllTables 互斥, WithAllTables 优先级高
func WithTables(tables ...string) DumpOption {
	return func(option *dumpOption) {
		option.tables = tables
	}
}

// 导出全部表
func WithAllTable() DumpOption {
	return func(option *dumpOption) {
		option.isAllTable = true
	}
}

// 导出到指定 writer
func WithWriter(writer io.Writer) DumpOption {
	return func(option *dumpOption) {
		option.writer = writer
	}
}

func WithNoAnnotation() DumpOption {
	return func(option *dumpOption) {
		option.noAnno = true
		noAnno = true
	}
}

func WithDbName(dbName string) DumpOption {
	return func(option *dumpOption) {
		withDbName = dbName
	}
}

func WithColName() DumpOption {
	return func(option *dumpOption) {
		needColName = true
	}
}

func WithBinary() DumpOption {
	return func(option *dumpOption) {
		useBinary = true
	}
}

func Dump(dsn string, opts ...DumpOption) error {
	// 打印开始
	start := time.Now()
	log.Printf("[info] [dump] start at %s\n", start.Format("2006-01-02 15:04:05"))
	// 打印结束
	defer func() {
		end := time.Now()
		log.Printf("[info] [dump] end at %s, cost %s\n", end.Format("2006-01-02 15:04:05"), end.Sub(start))
	}()

	var err error

	var o dumpOption

	for _, opt := range opts {
		opt(&o)
	}

	if len(o.tables) == 0 {
		// 默认包含全部表
		o.isAllTable = true
	}

	if o.writer == nil {
		// 默认输出到 os.Stdout
		o.writer = os.Stdout
	}

	buf := bufio.NewWriter(o.writer)
	defer buf.Flush()

	// 打印 Header
	if !noAnno && !useBinary {
		_, _ = buf.WriteString("-- ----------------------------\n")
		_, _ = buf.WriteString("-- MySQL Database Dump\n")
		_, _ = buf.WriteString("-- Start Time: " + start.Format("2006-01-02 15:04:05") + "\n")
		_, _ = buf.WriteString("-- ----------------------------\n")
		_, _ = buf.WriteString("\n\n")
	}

	// 连接数据库
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("[error] %v \n", err)
		return err
	}
	defer db.Close()

	// 1. 获取数据库
	dbName, err := GetDBNameFromDSN(dsn)
	if err != nil {
		log.Printf("[error] %v \n", err)
		return err
	}
	_, err = db.Exec(fmt.Sprintf("USE `%s`", dbName))
	if err != nil {
		log.Printf("[error] %v \n", err)
		return err
	}

	// 2. 获取表
	var tables []string
	if o.isAllTable {
		tmp, err := getAllTables(db)
		if err != nil {
			log.Printf("[error] %v \n", err)
			return err
		}
		tables = tmp
	} else {
		tables = o.tables
	}

	// 3. 导出表
	for _, table := range tables {
		// 删除表
		if o.isDropTable {
			_, _ = buf.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", table))
		}

		// 导出表结构
		err = writeTableStruct(db, table, buf)
		if err != nil {
			log.Printf("[error] %v \n", err)
			return err
		}

		// 导出表数据
		if o.isData {
			err = writeTableData(db, table, buf)
			if err != nil {
				log.Printf("[error] %v \n", err)
				return err
			}
		}
	}

	// 导出每个表的结构和数据
	if !o.noAnno && !useBinary {
		_, _ = buf.WriteString("-- ----------------------------\n")
		_, _ = buf.WriteString("-- Dumped by mysqldump\n")
		_, _ = buf.WriteString("-- Cost Time: " + time.Since(start).String() + "\n")
		_, _ = buf.WriteString("-- ----------------------------\n")
	}
	buf.Flush()
	return nil
}

func getCreateTableSQL(db *sql.DB, table string) (string, error) {
	var createTableSQL string
	err := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", table)).Scan(&table, &createTableSQL)
	if err != nil {
		return "", err
	}
	// IF NOT EXISTS
	if withDbName != "" {
		createTableSQL = strings.Replace(createTableSQL,
			"CREATE TABLE ",
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.", withDbName),
			1)
	} else {
		createTableSQL = strings.Replace(createTableSQL,
			"CREATE TABLE",
			"CREATE TABLE IF NOT EXISTS",
			1)
	}
	return createTableSQL, nil
}

func getAllTables(db *sql.DB) ([]string, error) {
	var tables []string
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		err = rows.Scan(&table)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func writeTableStruct(db *sql.DB, table string, buf *bufio.Writer) error {
	// 导出表结构
	if !noAnno && !useBinary {
		_, _ = buf.WriteString("-- ----------------------------\n")
		_, _ = buf.WriteString(fmt.Sprintf("-- Table structure for %s\n", table))
		_, _ = buf.WriteString("-- ----------------------------\n")
	}
	createTableSQL, err := getCreateTableSQL(db, table)
	if err != nil {
		log.Printf("[error] %v \n", err)
		return err
	}
	if useBinary {
		pk := NewPackage([]byte(createTableSQL+";"), DDL)
		b, _ := pk.Bytes()
		_, _ = buf.Write(b)
	} else {
		_, _ = buf.WriteString(createTableSQL)
		_, _ = buf.WriteString(";")

		_, _ = buf.WriteString("\n\n")
		_, _ = buf.WriteString("\n\n")
	}
	return nil
}

// 禁止 golangci-lint 检查
// nolint: gocyclo
func writeTableData(db *sql.DB, table string, buf *bufio.Writer) error {

	// 导出表数据
	if !noAnno && !useBinary {
		_, _ = buf.WriteString("-- ----------------------------\n")
		_, _ = buf.WriteString(fmt.Sprintf("-- Records of %s\n", table))
		_, _ = buf.WriteString("-- ----------------------------\n")
	}

	lineRows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s`", table))
	if err != nil {
		log.Printf("[error] %v \n", err)
		return err
	}
	defer lineRows.Close()

	var columns []string
	columns, err = lineRows.Columns()
	if err != nil {
		log.Printf("[error] %v \n", err)
		return err
	}
	columnTypes, err := lineRows.ColumnTypes()
	if err != nil {
		log.Printf("[error] %v \n", err)
		return err
	}

	var values [][]interface{}
	for lineRows.Next() {
		row := make([]interface{}, len(columns))
		rowPointers := make([]interface{}, len(columns))
		for i := range columns {
			rowPointers[i] = &row[i]
		}
		err = lineRows.Scan(rowPointers...)
		if err != nil {
			log.Printf("[error] %v \n", err)
			return err
		}
		values = append(values, row)
	}

	for _, row := range values {
		var ssql string
		if withDbName != "" {
			ssql = fmt.Sprintf("INSERT INTO `%s`.`%s`", withDbName, table)
		} else {
			ssql = fmt.Sprintf("INSERT INTO `%s`", table)
		}

		if needColName {
			ssql = ssql + "("
			for _, col := range columns {
				ssql += fmt.Sprintf("`%s`,", col)
			}
			ssql = strings.TrimRight(ssql, ",") + ")"
		}

		ssql = ssql + " VALUES ("

		for i, col := range row {
			if col == nil {
				ssql += "NULL"
			} else {
				Type := columnTypes[i].DatabaseTypeName()
				// 去除 UNSIGNED 和空格
				Type = strings.Replace(Type, "UNSIGNED", "", -1)
				Type = strings.Replace(Type, " ", "", -1)
				switch Type {
				case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT":
					if bs, ok := col.([]byte); ok {
						ssql += string(bs)
					} else {
						ssql += fmt.Sprintf("%d", col)
					}
				case "FLOAT", "DOUBLE":
					if bs, ok := col.([]byte); ok {
						ssql += string(bs)
					} else {
						ssql += fmt.Sprintf("%f", col)
					}
				case "DECIMAL", "DEC":
					ssql += fmt.Sprintf("%s", col)

				case "DATE":
					t, ok := col.(time.Time)
					if !ok {
						log.Println("DATE 类型转换错误")
						return err
					}
					ssql += fmt.Sprintf("'%s'", t.Format("2006-01-02"))
				case "DATETIME":
					t, ok := col.(time.Time)
					if !ok {
						log.Println("DATETIME 类型转换错误")
						return err
					}
					ssql += fmt.Sprintf("'%s'", t.Format("2006-01-02 15:04:05"))
				case "TIMESTAMP":
					t, ok := col.(time.Time)
					if !ok {
						log.Println("TIMESTAMP 类型转换错误")
						return err
					}
					ssql += fmt.Sprintf("'%s'", t.Format("2006-01-02 15:04:05"))
				case "TIME":
					t, ok := col.([]byte)
					if !ok {
						log.Println("TIME 类型转换错误")
						return err
					}
					ssql += fmt.Sprintf("'%s'", string(t))
				case "YEAR":
					t, ok := col.([]byte)
					if !ok {
						log.Println("YEAR 类型转换错误")
						return err
					}
					ssql += string(t)
				case "CHAR", "VARCHAR", "TINYTEXT", "TEXT", "MEDIUMTEXT", "LONGTEXT":
					ssql += fmt.Sprintf("'%s'", strings.Replace(fmt.Sprintf("%s", col), "'", "''", -1))
				case "BIT", "BINARY", "VARBINARY", "TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB":
					ssql += fmt.Sprintf("0x%X", col)
				case "ENUM", "SET":
					ssql += fmt.Sprintf("'%s'", col)
				case "BOOL", "BOOLEAN":
					if col.(bool) {
						ssql += "true"
					} else {
						ssql += "false"
					}
				case "JSON":
					ssql += fmt.Sprintf("'%s'", col)
				default:
					// unsupported type
					log.Printf("unsupported type: %s", Type)
					return fmt.Errorf("unsupported type: %s", Type)
				}
			}
			if i < len(row)-1 {
				ssql += ","
			}
		}
		ssql += ");\n"
		if useBinary {
			pk := NewPackage([]byte(ssql), DML)
			b, _ := pk.Bytes()
			_, _ = buf.Write(b)
		} else {
			_, _ = buf.WriteString(ssql)
		}
	}

	if !useBinary {
		_, _ = buf.WriteString("\n\n")
	}
	return nil
}
