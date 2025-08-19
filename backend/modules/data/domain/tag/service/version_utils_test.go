// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"
)

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name       string
		preVersion string
		newVersion string
		wantErr    bool
	}{
		{
			name:       "new version is invalid",
			newVersion: "asdfasdf",
			wantErr:    true,
		},
		{
			name:       "seg is more than 999",
			newVersion: "1000.1.1",
			wantErr:    true,
		},
		{
			name:       "seg is less than 0",
			newVersion: "1.1.-1",
			wantErr:    true,
		},
		{
			name:       "new version",
			newVersion: "1.1.1",
			wantErr:    false,
		},
		{
			name:       "old version is illegal",
			newVersion: "1.1.1",
			preVersion: "asdfasdf",
			wantErr:    true,
		},
		{
			name:       "old is greater than new",
			newVersion: "1.1.1",
			preVersion: "1.1.2",
			wantErr:    true,
		},
		{
			name:       "normal case",
			newVersion: "1.1.5",
			preVersion: "1.1.2",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.preVersion, tt.newVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
