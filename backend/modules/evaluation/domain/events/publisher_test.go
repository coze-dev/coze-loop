// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestPublishNotSentWrapping(t *testing.T) {
	cause := errors.New("invalid execution scope")
	marked := fmt.Errorf("%w: %w", ErrPublishNotSent, cause)
	wrapped := errorx.Wrapf(fmt.Errorf("schedule: %w", marked), "dispatch failed")
	require.ErrorIs(t, wrapped, ErrPublishNotSent)
	require.ErrorIs(t, wrapped, cause)
	for _, err := range []error{nil, cause, context.Canceled, context.DeadlineExceeded, errors.New(ErrPublishNotSent.Error())} {
		require.False(t, errors.Is(err, ErrPublishNotSent))
	}
}
