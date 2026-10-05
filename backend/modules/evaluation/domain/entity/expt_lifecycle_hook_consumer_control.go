// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type HookConsumerAction string

const (
	HookConsumerYield             HookConsumerAction = "yield"
	HookConsumerFail              HookConsumerAction = "fail"
	HookConsumerStartReserved     HookConsumerAction = "start_reserved"
	HookConsumerReservationAbsent HookConsumerAction = "reservation_absent"
)

type HookConsumerControlInput struct {
	Key          HookRunKey
	ItemID       int64
	Action       HookConsumerAction
	RetryTimes   int32
	ErrorMessage string
}

type HookConsumerControlResult struct {
	Handled           bool
	Changed           bool
	Proceed           bool
	ProjectionChanged bool
}
