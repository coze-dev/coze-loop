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
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "model_config.yaml"), []byte(config), 0o600))

	manage, err := NewManage(context.Background(), confviper.NewFileConfigLoaderFactory(
		confviper.WithFactoryConfigPath(configDir),
	))
	require.NoError(t, err)

	model, err := manage.GetModel(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "0.7", model.ParamConfig.ParamSchemas[0].DefaultValue)
}
