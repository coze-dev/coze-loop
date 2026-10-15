// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confviper "github.com/coze-dev/coze-loop/backend/pkg/conf/viper"
)

type singleSnapshotModelConfigLoader struct {
	calls int
}

func (l *singleSnapshotModelConfigLoader) Get(context.Context, string) any {
	return nil
}

func (l *singleSnapshotModelConfigLoader) Unmarshal(context.Context, any, ...conf.DecodeOptionFn) error {
	return nil
}

func (l *singleSnapshotModelConfigLoader) UnmarshalKey(_ context.Context, _ string, value any, _ ...conf.DecodeOptionFn) error {
	l.calls++
	if l.calls > 1 {
		return fmt.Errorf("unexpected second config read")
	}
	target, ok := value.(*[]*entity.Model)
	if !ok {
		return fmt.Errorf("unexpected unmarshal target %T", value)
	}
	canonical := "canonical"
	*target = []*entity.Model{{
		ID: 1,
		ParamConfig: &entity.ParamConfig{ParamSchemas: []*entity.ParamSchema{{
			Name:         "temperature",
			DefaultValue: "legacy",
			DefaultVal:   &canonical,
		}}},
	}}
	return nil
}

func TestManageImplReadsParamSchemaDefaultVal(t *testing.T) {
	configDir := t.TempDir()
	config := `models:
  - id: 1
    param_config:
      param_schemas:
        - name: temperature
          default_val: "0.7"
  - id: 2
    param_config:
      param_schemas:
        - name: temperature
          default_value: "0.8"
  - id: 3
    param_config:
      param_schemas:
        - name: response_format
          properties:
            - name: type
              default_value: legacy
              default_val: canonical
  - id: 4
    param_config:
      param_schemas:
        - name: stop
          default_value: legacy
          default_val: ""
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "model_config.yaml"), []byte(config), 0o600))

	manage, err := NewManage(context.Background(), confviper.NewFileConfigLoaderFactory(
		confviper.WithFactoryConfigPath(configDir),
	))
	require.NoError(t, err)

	shippedConfigModel, err := manage.GetModel(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "0.7", shippedConfigModel.ParamConfig.ParamSchemas[0].DefaultValue)

	legacyConfigModel, err := manage.GetModel(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, "0.8", legacyConfigModel.ParamConfig.ParamSchemas[0].DefaultValue)

	bothKeysModel, err := manage.GetModel(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, "canonical", bothKeysModel.ParamConfig.ParamSchemas[0].Properties[0].DefaultValue)

	emptyCanonicalModel, err := manage.GetModel(context.Background(), 4)
	require.NoError(t, err)
	require.Empty(t, emptyCanonicalModel.ParamConfig.ParamSchemas[0].DefaultValue)

	schema := bothKeysModel.ParamConfig.ParamSchemas[0].Properties[0]
	jsonSchema, err := json.Marshal(schema)
	require.NoError(t, err)
	require.NotContains(t, string(jsonSchema), `"default_val":`)
	require.Contains(t, string(jsonSchema), `"default_value":"canonical"`)
	yamlSchema, err := yaml.Marshal(schema)
	require.NoError(t, err)
	require.NotContains(t, string(yamlSchema), "default_val:")
	require.Contains(t, string(yamlSchema), "default_value: canonical")
}

func TestManageImplNormalizesAliasFromSingleSnapshot(t *testing.T) {
	loader := &singleSnapshotModelConfigLoader{}
	manage := &ManageImpl{loader: loader}

	models, err := manage.readConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, loader.calls)
	require.Equal(t, "canonical", models[0].ParamConfig.ParamSchemas[0].DefaultValue)
	require.Nil(t, models[0].ParamConfig.ParamSchemas[0].DefaultVal)
}
