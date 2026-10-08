package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/rawsql"
)

func TestGenerateEvaluationHookModels(t *testing.T) {
	sqlDir, err := filepath.Abs("../../../release/deployment/docker-compose/bootstrap/mysql-init/init-sql")
	require.NoError(t, err)
	db, err := gorm.Open(rawsql.New(rawsql.Config{FilePath: []string{sqlDir}}))
	require.NoError(t, err)
	output := os.Getenv("GORM_MODEL_TEST_OUTPUT")
	if output == "" {
		output = t.TempDir()
	}
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(output))
	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
	generateForEvaluationExpt(db)
	for _, tc := range []struct {
		file, field, wantType, wantSQL, wantNull string
	}{
		{"expt_lifecycle_run.gen.go", "SnapshotCipher", "[]byte", "mediumblob binary", "nullable"},
		{"expt_lifecycle_run.gen.go", "SnapshotHash", "string", "varchar(64) character set ascii", "not null"},
		{"expt_lifecycle_run.gen.go", "PlanHash", "*string", "varchar(64) character set ascii", "nullable"},
		{"expt_lifecycle_hook_run.gen.go", "RequestHash", "*string", "varchar(64) character set ascii", "nullable"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filepath.Join("modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model", tc.file), nil, 0)
			require.NoError(t, err)
			var field *ast.Field
			ast.Inspect(file, func(n ast.Node) bool {
				if f, ok := n.(*ast.Field); ok && len(f.Names) == 1 && f.Names[0].Name == tc.field {
					field = f
				}
				return true
			})
			require.NotNil(t, field)
			var typ bytes.Buffer
			require.NoError(t, printer.Fprint(&typ, fset, field.Type))
			require.Equal(t, tc.wantType, typ.String())
			require.NotNil(t, field.Tag)
			tag, err := strconv.Unquote(field.Tag.Value)
			require.NoError(t, err)
			gormTag := strings.Split(reflect.StructTag(tag).Get("gorm"), ";")
			require.Contains(t, gormTag, "type:"+tc.wantSQL)
			if tc.wantNull == "nullable" {
				require.NotContains(t, gormTag, "not null")
			} else {
				require.Contains(t, gormTag, "not null")
			}
			if tc.field == "SnapshotCipher" {
				require.Equal(t, "-", reflect.StructTag(tag).Get("json"))
			}
		})
	}
}
