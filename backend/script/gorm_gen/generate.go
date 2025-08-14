package main

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/rawsql"
)

func main() {
	db := initDB()
	generateForPrompt(db)
	generateForData(db)
	generateForEvaluationTarget(db)
	generateForEvaluationEvaluator(db)
	generateForEvaluationExpt(db)
	generateForObservability(db)
	generateForFoundation(db)
}

func initDB() *gorm.DB {
	cli, err := gorm.Open(rawsql.New(rawsql.Config{
		FilePath: []string{"../conf/default/mysql/init-sql"},
	}))
	if err != nil {
		panic(err)
	}
	return cli
}

func getGenerateConfig(path string) gen.Config {
	config := gen.Config{
		// 最终package不能设置为model，在有数据库表同步的情况下会产生冲突，若一定要使用可以单独指定model package的新名字
		OutPath: fmt.Sprintf("./%s/query", path),
		// Mode: gen.WithoutContext,
		ModelPkgPath:      fmt.Sprintf("./%s/model", path), // 默认情况下会跟随OutPath参数，在同目录下生成model目录
		FieldNullable:     true,                            // 对于数据库中nullable的数据，在生成代码中自动对应为指针类型
		FieldWithIndexTag: true,                            // 从数据库同步的表结构代码包含gorm的index tag
		FieldWithTypeTag:  true,
	}
	config.WithImportPkgPath(fmt.Sprintf("github.com/coze-dev/coze-loop/backend/%s/model", path))
	return config
}

func generateForPrompt(db *gorm.DB) {
	path := "modules/prompt/infra/repo/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)

	var models []any
	for _, table := range []string{"prompt_basic", "prompt_user_draft", "prompt_debug_log", "prompt_debug_context"} {
		models = append(models, g.GenerateModel(table,
			// 添加软删除字段
			gen.FieldType("deleted_at", "soft_delete.DeletedAt"),
			gen.FieldGORMTag("deleted_at", func(tag field.GormTag) field.GormTag {
				return tag.Set("column:deleted_at;not null;default:0;softDelete:milli")
			}),
			gen.FieldGORMTag("*", func(tag field.GormTag) field.GormTag {
				return tag.Set("charset=utf8mb4")
			})))
	}

	for _, table := range []string{"prompt_commit"} {
		models = append(models, g.GenerateModel(table,
			gen.FieldGORMTag("*", func(tag field.GormTag) field.GormTag {
				return tag.Set("charset=utf8mb4")
			})))
	}
	g.ApplyBasic(models...)
	g.Execute()
}

func generateForDataset(db *gorm.DB) {
	path := "modules/data/infra/repo/dataset/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)

	dataset := g.GenerateModelAs("dataset", "Dataset",
		gen.FieldType("description", "*string"),
		gen.FieldType("spec", "datatypes.JSON"),
		gen.FieldType("features", "datatypes.JSON"),
		gen.FieldType("deleted_at", "soft_delete.DeletedAt"),
	)
	datasetSchema := g.GenerateModelAs("dataset_schema", "DatasetSchema",
		gen.FieldType("fields", "datatypes.JSON"),
	)
	datasetVersion := g.GenerateModelAs("dataset_version", "DatasetVersion",
		gen.FieldType("description", "*string"),
		gen.FieldType("dataset_brief", "datatypes.JSON"),
		gen.FieldType("snapshot_progress", "datatypes.JSON"),
	)
	datasetItem := g.GenerateModelAs("dataset_item", "DatasetItem",
		gen.FieldType("app_id", "int32"),
		gen.FieldType("data", "datatypes.JSON"),
		gen.FieldType("repeated_data", "datatypes.JSON"),
		gen.FieldType("data_properties", "datatypes.JSON"),
		gen.FieldType("source", "datatypes.JSON"),
		gen.FieldType("deleted_at", "soft_delete.DeletedAt"),
	)

	ioJob := g.GenerateModelAs("dataset_io_job", "DatasetIOJob",
		gen.FieldType("app_id", "int32"),
		gen.FieldType("source_file", "datatypes.JSON"),
		gen.FieldType("source_dataset", "datatypes.JSON"),
		gen.FieldType("target_file", "datatypes.JSON"),
		gen.FieldType("target_dataset", "datatypes.JSON"),
		gen.FieldType("field_mappings", "datatypes.JSON"),
		gen.FieldType("sub_progresses", "datatypes.JSON"),
		gen.FieldType("errors", "datatypes.JSON"),
		gen.FieldType("option", "datatypes.JSON"),
	)
	datasetItemSnapshot := g.GenerateModelAs("dataset_item_snapshot", "ItemSnapshot",
		gen.FieldType("app_id", "int32"),
		gen.FieldType("data", "datatypes.JSON"),
		gen.FieldType("repeated_data", "datatypes.JSON"),
		gen.FieldType("data_properties", "datatypes.JSON"),
		gen.FieldType("source", "datatypes.JSON"),
	)

	g.ApplyBasic(
		dataset,
		datasetSchema,
		datasetVersion,
		datasetItem,
		ioJob,
		datasetItemSnapshot,
	)
	g.Execute()
}

func generateForTag(db *gorm.DB) {
	path := "modules/data/infra/repo/tag/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)

	tagKey := g.GenerateModelAs("tag_key", "TagKey",
		gen.FieldType("app_id", "int32"),
		gen.FieldType("change_log", "datatypes.JSON"),
		gen.FieldType("version_num", "*int32"),
		gen.FieldType("spec", "datatypes.JSON"),
	)

	tagValue := g.GenerateModelAs("tag_value", "TagValue",
		gen.FieldType("app_id", "int32"),
		gen.FieldType("version_num", "*int32"),
	)

	g.ApplyBasic(
		tagKey,
		tagValue,
	)
	g.Execute()
}

func generateForData(db *gorm.DB) {
	generateForDataset(db)
	generateForTag(db)
}

func generateForEvaluationTarget(db *gorm.DB) {
	path := "modules/evaluation/infra/repo/target/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)

	evaluatorModel := g.GenerateModelAs("eval_target", "Target")
	evaluatorVersionModel := g.GenerateModelAs("eval_target_version", "TargetVersion")
	evaluatorRecordModel := g.GenerateModelAs("eval_target_record", "TargetRecord")

	g.ApplyBasic(evaluatorModel, evaluatorVersionModel, evaluatorRecordModel)
	g.Execute()
}

func generateForEvaluationExpt(db *gorm.DB) {
	path := "modules/evaluation/infra/repo/experiment/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)
	tables := []string{
		"experiment",
		"expt_evaluator_ref",
		"expt_stats",
		"expt_turn_result",
		"expt_turn_evaluator_result_ref",
		"expt_item_result",
		"expt_item_result_run_log",
		"expt_turn_result_run_log",
		"expt_run_log",
		"expt_aggr_result",
	}

	var models []any
	titleCaser := cases.Title(language.English)
	for _, tn := range tables {
		parts := strings.Split(tn, "_")
		for i := range parts {
			if len(parts[i]) > 0 {
				parts[i] = titleCaser.String(parts[i])
			}
		}
		name := strings.Join(parts, "")
		models = append(models, g.GenerateModelAs(tn, name))
	}
	g.ApplyBasic(models...)
	g.Execute()
}

func generateForEvaluationEvaluator(db *gorm.DB) {
	path := "modules/evaluation/infra/repo/evaluator/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)

	evaluatorModel := g.GenerateModelAs("evaluator", "Evaluator")
	evaluatorVersionModel := g.GenerateModelAs("evaluator_version", "EvaluatorVersion")
	evaluatorRecordModel := g.GenerateModelAs("evaluator_record", "EvaluatorRecord")

	g.ApplyBasic(evaluatorModel, evaluatorVersionModel, evaluatorRecordModel)
	g.Execute()
}

func generateForObservability(db *gorm.DB) {
	path := "modules/observability/infra/repo/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)

	// 为 observability_view 表添加软删除字段
	observabilityView := g.GenerateModelAs("observability_view", "ObservabilityView")

	g.ApplyBasic(observabilityView)
	g.Execute()
}

func generateForFoundation(db *gorm.DB) {
	path := "modules/foundation/infra/repo/mysql/gorm_gen"
	g := gen.NewGenerator(getGenerateConfig(path))
	g.UseDB(db)

	userModel := g.GenerateModelAs("user", "User")
	spaceModel := g.GenerateModelAs("space", "Space")
	spaceUserModel := g.GenerateModelAs("space_user", "SpaceUser")
	apikeyModel := g.GenerateModelAs("api_key", "APIKey")
	g.ApplyBasic(
		userModel,
		spaceModel,
		spaceUserModel,
		apikeyModel)
	g.Execute()
}
