package mysqldump

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"testing"
)

func Test_mergeInsert(t *testing.T) {
	type args struct {
		insertSQLs []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			args: args{
				insertSQLs: []string{
					"INSERT INTO `test` VALUES (1, 'a');",
					"INSERT INTO `test` VALUES (2, 'b');",
				},
			},
			want:    "INSERT INTO `test` VALUES (1, 'a'), (2, 'b');",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mergeInsert(tt.args.insertSQLs)
			if (err != nil) != tt.wantErr {
				t.Errorf("mergeInsert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("mergeInsert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSource(t *testing.T) {
	dn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Asia%%2FShanghai",
		"root", "Unitech@1998", "192.168.44.154", 3307, "n9e_v6")
	f, err := os.Open("C:\\Users\\raolejia1\\Desktop\\n9e_v6_20240821_085237.dump")
	if err != nil {
		log.Printf("[ERROR] Open dump file failed: %v", err)
	}

	// 读取文件内容
	content, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	// 去除注释
	cleanSQL := removeSQLComments(string(content))

	// 将去除注释后的SQL转换为io.Reader
	reader := strings.NewReader(cleanSQL)

	err = Source(dn, reader, WithDebug())
	if err != nil {
		log.Printf("[ERROR] Source failed: %v", err)
	}
}

// 去除SQL中的注释
func removeSQLComments(sql string) string {
	// 匹配单行注释（-- 注释）
	singleLineCommentRegex := regexp.MustCompile(`(?m)--.*$`)
	sql = singleLineCommentRegex.ReplaceAllString(sql, "")

	// 匹配多行注释（/* 注释 */）
	multiLineCommentRegex := regexp.MustCompile(`(?s)/\*.*?\*/`)
	sql = multiLineCommentRegex.ReplaceAllString(sql, "")

	return sql
}
