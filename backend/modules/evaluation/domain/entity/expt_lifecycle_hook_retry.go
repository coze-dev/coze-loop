// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HookOutcomeCode string

const (
	HookSucceeded         HookOutcomeCode = "SUCCEEDED"
	HookFailed            HookOutcomeCode = "HOOK_FAILED"
	HookSecurityError     HookOutcomeCode = "HOOK_SECURITY_ERROR"
	HookTransportError    HookOutcomeCode = "TRANSPORT_ERROR"
	HookTimeoutUncertain  HookOutcomeCode = "TIMEOUT_UNCERTAIN"
	HookHTTPError         HookOutcomeCode = "HTTP_ERROR"
	HookHTTPRedirectError HookOutcomeCode = "HTTP_REDIRECT_ERROR"
	HookResultNotFinal    HookOutcomeCode = "RESULT_NOT_FINAL"
	HookProtocolError     HookOutcomeCode = "PROTOCOL_ERROR"
	HookResponseTooLarge  HookOutcomeCode = "RESPONSE_TOO_LARGE"
)

type HookOutcome struct {
	Code       HookOutcomeCode
	HTTPStatus int
	Retryable  bool
}

type HookRetryInput struct {
	Outcome        HookOutcome
	Retry          *HookRetryConf
	Attempt        int32
	TimeoutSeconds int32
	Now            time.Time
	Deadline       time.Time
	RetryAfter     string
}

type HookRetryDecision struct {
	Retry       bool
	Delay       time.Duration
	NextAttempt int32
}

func ClassifyHookOutcome(local HookOutcomeCode, httpStatus int, status string, retryable bool) HookOutcome {
	result := HookOutcome{HTTPStatus: httpStatus}
	switch local {
	case HookSecurityError:
		result.Code = local
		return result
	case HookTransportError, HookTimeoutUncertain:
		result.Code, result.Retryable = local, true
		return result
	}
	switch {
	case httpStatus == 408 || httpStatus == 429 || httpStatus >= 500 && httpStatus <= 599:
		result.Code, result.Retryable = HookHTTPError, true
	case httpStatus >= 400 && httpStatus <= 499:
		result.Code = HookHTTPError
	case httpStatus >= 300 && httpStatus <= 399:
		result.Code = HookHTTPRedirectError
	case httpStatus == 202:
		result.Code = HookResultNotFinal
	case httpStatus != 200:
		result.Code = HookProtocolError
	case local == HookResponseTooLarge:
		result.Code = HookResponseTooLarge
	case local != "":
		result.Code = HookProtocolError
	case status == "succeeded":
		result.Code = HookSucceeded
	case status == "failed":
		result.Code, result.Retryable = HookFailed, retryable
	default:
		result.Code = HookProtocolError
	}
	return result
}

// firstStartedAt is the original persisted start, never a worker restart time.
func HookStageDeadline(firstStartedAt time.Time, timeoutSeconds int32, retry *HookRetryConf) (time.Time, error) {
	timeout, retries, err := hookRetryBudget(timeoutSeconds, retry)
	if err != nil {
		return time.Time{}, err
	}
	return firstStartedAt.Add(time.Duration(retries+1)*timeout + time.Duration(retries)*time.Minute), nil
}

func HookRetryDelay(attempt int32, httpStatus int, retryAfter string, now time.Time) time.Duration {
	base := time.Minute
	switch attempt {
	case 1:
		base = 5 * time.Second
	case 2:
		base = 15 * time.Second
	case 3:
		base = 30 * time.Second
	}
	if httpStatus != 408 && httpStatus != 429 && !(httpStatus >= 500 && httpStatus <= 599) {
		return base
	}
	header := strings.TrimSpace(retryAfter)
	if header == "" {
		return base
	}
	digits := true
	for _, c := range header {
		if c < '0' || c > '9' {
			digits = false
			break
		}
	}
	delay := time.Duration(0)
	if digits {
		seconds, err := strconv.ParseUint(header, 10, 64)
		// Saturate before converting to a signed duration, including numeric overflow.
		if err != nil || seconds >= 60 {
			delay = time.Minute
		} else {
			delay = time.Duration(seconds) * time.Second
		}
	} else if date, err := http.ParseTime(header); err == nil {
		delay = date.Sub(now)
	}
	if delay > time.Minute {
		delay = time.Minute
	}
	if delay > base {
		return delay
	}
	return base
}

func DecideHookRetry(input HookRetryInput) (HookRetryDecision, error) {
	stop := HookRetryDecision{}
	if input.Attempt < 1 || input.Attempt > 11 {
		return stop, invalidParam("hook attempt must be between 1 and 11")
	}
	timeout, retries, err := hookRetryBudget(input.TimeoutSeconds, input.Retry)
	if err != nil {
		return stop, err
	}
	if input.Outcome.Code == HookSucceeded || !input.Outcome.Retryable || input.Attempt >= retries+1 {
		return stop, nil
	}
	delay := HookRetryDelay(input.Attempt, input.Outcome.HTTPStatus, input.RetryAfter, input.Now)
	// Reserve a full next attempt against the original deadline before sending.
	if input.Now.Add(delay + timeout).After(input.Deadline) {
		return stop, nil
	}
	return HookRetryDecision{Retry: true, Delay: delay, NextAttempt: input.Attempt + 1}, nil
}

func hookRetryBudget(timeoutSeconds int32, retry *HookRetryConf) (time.Duration, int32, error) {
	if timeoutSeconds < 0 || timeoutSeconds > 1200 {
		return 0, 0, invalidParam("hook timeout must be between 0 and 1200 seconds")
	}
	if timeoutSeconds == 0 {
		timeoutSeconds = 180
	}
	if retry != nil && retry.MaxRetries != nil && (*retry.MaxRetries < 1 || *retry.MaxRetries > 10) {
		return 0, 0, invalidParam("hook max_retries must be between 1 and 10")
	}
	return time.Duration(timeoutSeconds) * time.Second, retry.EffectiveMaxRetries(), nil
}
