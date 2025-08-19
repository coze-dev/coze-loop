// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dkms

import "context"

//go:generate mockgen -destination=mocks/dkms.go -package=mocks . IDKMS
type IDKMS interface {
	Encrypt(ctx context.Context, dataKey, plaintext string) (string, error)
	Decrypt(ctx context.Context, dataKey, ciphertext string) (string, error)
}
