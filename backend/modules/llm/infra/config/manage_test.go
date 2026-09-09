// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confviper "github.com/coze-dev/coze-loop/backend/pkg/conf/viper"
)

type changingModelConfigLoader struct {
	calls int
}

func (l *changingModelConfigLoader) Get(context.Context, string) any {
	return nil
}

func (l *changingModelConfigLoader) Unmarshal(context.Context, any, ...conf.DecodeOptionFn) error {
	return nil
}

func (l *changingModelConfigLoader) UnmarshalKey(_ context.Context, _ string, value any, _ ...conf.DecodeOptionFn) error {
	l.calls++
	switch target := value.(type) {
	case *[]*entity.Model:
		*target = []*entity.Model{
			{
				ID: 1,
				ParamConfig: &entity.ParamConfig{ParamSchemas: []*entity.ParamSchema{
					{Name: "temperature", DefaultValue: "legacy-temperature"},
					{Name: "top_p", DefaultValue: "legacy-top-p"},
				}},
			},
			{
				ID: 2,
				ParamConfig: &entity.ParamConfig{ParamSchemas: []*entity.ParamSchema{
					{Name: "max_tokens", DefaultValue: "legacy-max-tokens"},
				}},
			},
		}
	case *[]*modelConfigAliases:
		maxTokens := "canonical-max-tokens"
		topP := "canonical-top-p"
		temperature := "canonical-temperature"
		*target = []*modelConfigAliases{
			{
				ID: 2,
				ParamConfig: &paramConfigAliases{ParamSchemas: []*paramSchemaAliases{
					{Name: "max_tokens", DefaultVal: &maxTokens},
				}},
			},
			{
				ID: 1,
				ParamConfig: &paramConfigAliases{ParamSchemas: []*paramSchemaAliases{
					{Name: "top_p", DefaultVal: &topP},
					{Name: "temperature", DefaultVal: &temperature},
				}},
			},
		}
	default:
		return fmt.Errorf("unexpected unmarshal target %T", value)
	}
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
}

func TestManageImplMatchesAliasesByStableIdentity(t *testing.T) {
	manage := &ManageImpl{loader: &changingModelConfigLoader{}}

	models, err := manage.readConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "canonical-temperature", models[0].ParamConfig.ParamSchemas[0].DefaultValue)
	require.Equal(t, "canonical-top-p", models[0].ParamConfig.ParamSchemas[1].DefaultValue)
	require.Equal(t, "canonical-max-tokens", models[1].ParamConfig.ParamSchemas[0].DefaultValue)
}
