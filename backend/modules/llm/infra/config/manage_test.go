// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	confviper "github.com/coze-dev/coze-loop/backend/pkg/conf/viper"
)

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
