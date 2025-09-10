// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package errno

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		// Common error codes
		{"CommonNoPermissionCode", CommonNoPermissionCode, 600900101},
		{"CommonBadRequestCode", CommonBadRequestCode, 600900201},
		{"CommonInvalidParamCode", CommonInvalidParamCode, 600900202},
		{"CommonBizInvalidCode", CommonBizInvalidCode, 600900203},
		{"CommonResourceDuplicatedCode", CommonResourceDuplicatedCode, 600900204},
		{"CommonRequestRateLimitCode", CommonRequestRateLimitCode, 600900205},
		{"CommonNetworkTimeOutCode", CommonNetworkTimeOutCode, 600900701},
		{"CommonInternalErrorCode", CommonInternalErrorCode, 600900702},
		{"CommonRPCErrorCode", CommonRPCErrorCode, 600900703},
		{"CommonMySqlErrorCode", CommonMySqlErrorCode, 600900801},
		{"CommonRedisErrorCode", CommonRedisErrorCode, 600900803},

		// Resource error codes
		{"ResourceNotFoundCode", ResourceNotFoundCode, 600903001},
		{"JSONErrorCode", JSONErrorCode, 600903002},
		{"InvalidRPCResponseCode", InvalidRPCResponseCode, 600903003},
		{"InvalidFieldFilterParamCode", InvalidFieldFilterParamCode, 600903004},
		{"MetaInfoBuildErrorCode", MetaInfoBuildErrorCode, 600903005},
		{"HttpCallErrorCode", HttpCallErrorCode, 600903006},
		{"QueryOfflineErrorCode", QueryOfflineErrorCode, 600903007},
		{"QueryOfflineAuthErrorCode", QueryOfflineAuthErrorCode, 600903008},

		// User error codes
		{"UserParseFailedCode", UserParseFailedCode, 600903100},
		{"ManagerAllowedOnlyCode", ManagerAllowedOnlyCode, 600903101},
		{"SearchTraceNotAllowedCode", SearchTraceNotAllowedCode, 600903102},

		// Trace parsing error codes
		{"ParseTagErrorCode", ParseTagErrorCode, 600903201},
		{"ParseArgosSpanErrorCode", ParseArgosSpanErrorCode, 600903202},
		{"TraceNotInSpaceErrorCode", TraceNotInSpaceErrorCode, 600903203},
		{"BotNotRegisteredInSpaceErrorCode", BotNotRegisteredInSpaceErrorCode, 600903204},
		{"NotInSpaceCommonErrorCode", NotInSpaceCommonErrorCode, 600903205},
		{"NoRegisteredBotInSpaceErrorCode", NoRegisteredBotInSpaceErrorCode, 600903206},
		{"InvalidTraceErrorCode", InvalidTraceErrorCode, 600903207},
		{"ExpiredTraceErrorCode", ExpiredTraceErrorCode, 600903208},

		// Capacity error codes
		{"TraceNoCapacityAvailableErrorCode", TraceNoCapacityAvailableErrorCode, 600903230},
		{"AccountNotAvailableErrorCode", AccountNotAvailableErrorCode, 600903231},

		// Sampling error codes
		{"UnsupportedDownSampleIntervalTypeCode", UnsupportedDownSampleIntervalTypeCode, 600903301},

		// Commercial error codes
		{"CommercialUnsupportedMethodCodeCode", CommercialUnsupportedMethodCodeCode, 600904001},
		{"CommercialCommonInvalidParamCodeCode", CommercialCommonInvalidParamCodeCode, 600904002},
		{"CommercialCommonBadRequestCodeCode", CommercialCommonBadRequestCodeCode, 600904003},
		{"CommercialCommonInternalErrorCodeCode", CommercialCommonInternalErrorCodeCode, 600904004},
		{"CommercialUserParseFailedCodeCode", CommercialUserParseFailedCodeCode, 600904005},
		{"CommercialCommonRPCErrorCodeCode", CommercialCommonRPCErrorCodeCode, 600904006},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code)
		})
	}
}

func TestErrorCodeUniqueness(t *testing.T) {
	// Test that all error codes are unique
	codes := []int{
		CommonNoPermissionCode,
		CommonBadRequestCode,
		CommonInvalidParamCode,
		CommonBizInvalidCode,
		CommonResourceDuplicatedCode,
		CommonRequestRateLimitCode,
		CommonNetworkTimeOutCode,
		CommonInternalErrorCode,
		CommonRPCErrorCode,
		CommonMySqlErrorCode,
		CommonRedisErrorCode,
		ResourceNotFoundCode,
		JSONErrorCode,
		InvalidRPCResponseCode,
		InvalidFieldFilterParamCode,
		MetaInfoBuildErrorCode,
		HttpCallErrorCode,
		QueryOfflineErrorCode,
		QueryOfflineAuthErrorCode,
		UserParseFailedCode,
		ManagerAllowedOnlyCode,
		SearchTraceNotAllowedCode,
		ParseTagErrorCode,
		ParseArgosSpanErrorCode,
		TraceNotInSpaceErrorCode,
		BotNotRegisteredInSpaceErrorCode,
		NotInSpaceCommonErrorCode,
		NoRegisteredBotInSpaceErrorCode,
		InvalidTraceErrorCode,
		ExpiredTraceErrorCode,
		TraceNoCapacityAvailableErrorCode,
		AccountNotAvailableErrorCode,
		UnsupportedDownSampleIntervalTypeCode,
		CommercialUnsupportedMethodCodeCode,
		CommercialCommonInvalidParamCodeCode,
		CommercialCommonBadRequestCodeCode,
		CommercialCommonInternalErrorCodeCode,
		CommercialUserParseFailedCodeCode,
		CommercialCommonRPCErrorCodeCode,
	}

	seen := make(map[int]bool)
	for _, code := range codes {
		assert.False(t, seen[code], "Duplicate error code found: %d", code)
		seen[code] = true
	}
}

func TestErrorCodeRanges(t *testing.T) {
	// Test that error codes are in expected ranges
	tests := []struct {
		name     string
		code     int
		minRange int
		maxRange int
		category string
	}{
		// Common errors: 600900xxx
		{"CommonNoPermissionCode", CommonNoPermissionCode, 600900000, 600900999, "common"},
		{"CommonBadRequestCode", CommonBadRequestCode, 600900000, 600900999, "common"},
		{"CommonInternalErrorCode", CommonInternalErrorCode, 600900000, 600900999, "common"},

		// Business errors: 600903xxx
		{"ResourceNotFoundCode", ResourceNotFoundCode, 600903000, 600903999, "business"},
		{"UserParseFailedCode", UserParseFailedCode, 600903000, 600903999, "business"},
		{"ParseTagErrorCode", ParseTagErrorCode, 600903000, 600903999, "business"},

		// Commercial errors: 600904xxx
		{"CommercialUnsupportedMethodCodeCode", CommercialUnsupportedMethodCodeCode, 600904000, 600904999, "commercial"},
		{"CommercialCommonInvalidParamCodeCode", CommercialCommonInvalidParamCodeCode, 600904000, 600904999, "commercial"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, tt.code, tt.minRange, "Error code %d should be >= %d for %s category", tt.code, tt.minRange, tt.category)
			assert.LessOrEqual(t, tt.code, tt.maxRange, "Error code %d should be <= %d for %s category", tt.code, tt.maxRange, tt.category)
		})
	}
}

func TestErrorCodeGrouping(t *testing.T) {
	// Test that error codes are properly grouped by functionality

	// Permission related codes
	permissionCodes := []int{
		CommonNoPermissionCode,
		ManagerAllowedOnlyCode,
		SearchTraceNotAllowedCode,
		QueryOfflineAuthErrorCode,
	}

	for _, code := range permissionCodes {
		assert.Greater(t, code, 0, "Permission error code should be positive")
	}

	// Validation related codes
	validationCodes := []int{
		CommonBadRequestCode,
		CommonInvalidParamCode,
		CommercialCommonInvalidParamCodeCode,
		CommercialCommonBadRequestCodeCode,
		InvalidFieldFilterParamCode,
	}

	for _, code := range validationCodes {
		assert.Greater(t, code, 0, "Validation error code should be positive")
	}

	// Internal error codes
	internalCodes := []int{
		CommonInternalErrorCode,
		CommonRPCErrorCode,
		CommonMySqlErrorCode,
		CommonRedisErrorCode,
		CommercialCommonInternalErrorCodeCode,
		CommercialCommonRPCErrorCodeCode,
	}

	for _, code := range internalCodes {
		assert.Greater(t, code, 0, "Internal error code should be positive")
	}
}

func TestSpecificErrorCodes(t *testing.T) {
	// Test specific important error codes that are frequently used
	tests := []struct {
		name        string
		code        int
		description string
	}{
		{
			name:        "TraceNoCapacityAvailableErrorCode",
			code:        TraceNoCapacityAvailableErrorCode,
			description: "Should be used when trace capacity is insufficient",
		},
		{
			name:        "AccountNotAvailableErrorCode",
			code:        AccountNotAvailableErrorCode,
			description: "Should be used when account is not available for benefits",
		},
		{
			name:        "CommonRequestRateLimitCode",
			code:        CommonRequestRateLimitCode,
			description: "Should be used when request rate limit is exceeded",
		},
		{
			name:        "InvalidTraceErrorCode",
			code:        InvalidTraceErrorCode,
			description: "Should be used when trace has no root span",
		},
		{
			name:        "ExpiredTraceErrorCode",
			code:        ExpiredTraceErrorCode,
			description: "Should be used when trace has expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Greater(t, tt.code, 0, "Error code should be positive: %s", tt.description)
			assert.Greater(t, tt.code, 600000000, "Error code should be in observability range: %s", tt.description)
		})
	}
}

func TestPublicConstants(t *testing.T) {
	// Test that public constants (exported) have correct values
	assert.Equal(t, "request is limited", CommonRequestRateLimitMessage)
	assert.Equal(t, true, CommonRequestRateLimitNoAffectStability)
}

func TestErrorCodeConsistency(t *testing.T) {
	// Test that similar error codes follow consistent naming patterns

	// All commercial codes should start with "Commercial"
	commercialCodes := map[string]int{
		"CommercialUnsupportedMethodCodeCode":   CommercialUnsupportedMethodCodeCode,
		"CommercialCommonInvalidParamCodeCode":  CommercialCommonInvalidParamCodeCode,
		"CommercialCommonBadRequestCodeCode":    CommercialCommonBadRequestCodeCode,
		"CommercialCommonInternalErrorCodeCode": CommercialCommonInternalErrorCodeCode,
		"CommercialUserParseFailedCodeCode":     CommercialUserParseFailedCodeCode,
		"CommercialCommonRPCErrorCodeCode":      CommercialCommonRPCErrorCodeCode,
	}

	for name, code := range commercialCodes {
		assert.Contains(t, name, "Commercial", "Commercial error code name should contain 'Commercial': %s", name)
		assert.GreaterOrEqual(t, code, 600904000, "Commercial error code should be in 600904xxx range: %s", name)
		assert.LessOrEqual(t, code, 600904999, "Commercial error code should be in 600904xxx range: %s", name)
	}

	// All common codes should start with "Common"
	commonCodes := map[string]int{
		"CommonNoPermissionCode":       CommonNoPermissionCode,
		"CommonBadRequestCode":         CommonBadRequestCode,
		"CommonInvalidParamCode":       CommonInvalidParamCode,
		"CommonBizInvalidCode":         CommonBizInvalidCode,
		"CommonResourceDuplicatedCode": CommonResourceDuplicatedCode,
		"CommonRequestRateLimitCode":   CommonRequestRateLimitCode,
		"CommonNetworkTimeOutCode":     CommonNetworkTimeOutCode,
		"CommonInternalErrorCode":      CommonInternalErrorCode,
		"CommonRPCErrorCode":           CommonRPCErrorCode,
		"CommonMySqlErrorCode":         CommonMySqlErrorCode,
		"CommonRedisErrorCode":         CommonRedisErrorCode,
	}

	for name, code := range commonCodes {
		assert.Contains(t, name, "Common", "Common error code name should contain 'Common': %s", name)
		assert.GreaterOrEqual(t, code, 600900000, "Common error code should be in 600900xxx range: %s", name)
		assert.LessOrEqual(t, code, 600900999, "Common error code should be in 600900xxx range: %s", name)
	}
}
