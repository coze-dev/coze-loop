// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package localos

import (
	"fmt"
	"os"
)

func GetLocalOSHost() string {
	protocol := os.Getenv("COZE_LOOP_OSS_PROTOCOL")
	domain := os.Getenv("COZE_LOOP_OSS_DOMAIN")
	port := os.Getenv("COZE_LOOP_OSS_PORT")
	if port == "" {
		return fmt.Sprintf("%s://%s", protocol, domain)
	}
	return fmt.Sprintf("%s:%s", domain, port)
}
