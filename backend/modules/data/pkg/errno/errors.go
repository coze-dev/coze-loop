package errno

import (
	"fmt"
	"strings"

	"github.com/bytedance/gg/gslice"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

const (
	ExtraKeyAffectStability = "biz_err_affect_stability" // errno 中用于记录稳定性影响的 key.
)

type ErrorMsg string

const (
	DAOParamIsNilError        ErrorMsg = "dao param is nil"
	DAOParamWithoutIndexError ErrorMsg = "at least one of the query params using index must be set"
	DAOParamIsIllegalError    ErrorMsg = "dao param is illegal"
)

func (e ErrorMsg) Error() string {
	return string(e)
}

func DBErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, CommonMySqlErrorCode, msgAndArgs...)
}

func MaybeDBErr(cause error, msgAndArgs ...any) error {
	return MaybeWrapf(cause, CommonMySqlErrorCode, msgAndArgs...)
}

func JSONErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, JSONErrorCode, msgAndArgs...)
}

func BadReqErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, CommonBadRequestCode, msgAndArgs...)
}

func MaybeBadReqErr(cause error, msgAndArgs ...any) error {
	return MaybeWrapf(cause, CommonBadRequestCode, msgAndArgs...)
}

func BadReqErrorf(msgFormat string, args ...any) error {
	return Errorf(CommonBadRequestCode, msgFormat, args...)
}

func NotFoundErrorf(msgFormat string, args ...any) error {
	return Errorf(ResourceNotFoundCode, msgFormat, args...)
}

func UnauthorizedErrorf(msgFormat string, args ...any) error {
	return Errorf(UnauthorizedCode, msgFormat, args...)
}

func InternalErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, CommonInternalErrorCode, msgAndArgs...)
}

func InternalErrorf(msgFormat string, args ...any) error {
	return Errorf(CommonInternalErrorCode, msgFormat, args...)
}

func MaybeInternalErr(cause error, msgAndArgs ...any) error {
	return MaybeWrapf(cause, CommonInternalErrorCode, msgAndArgs...)
}

func InvalidParamErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, CommonInvalidParamCode, msgAndArgs...)
}

func InvalidParamErrorf(msgFormat string, args ...any) error {
	return Errorf(CommonInvalidParamCode, msgFormat, args...)
}

func MaybeInvalidParamErr(cause error, msgAndArgs ...any) error {
	return MaybeWrapf(cause, CommonInvalidParamCode, msgAndArgs...)
}

func RPCErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, CommonRPCErrorCode, msgAndArgs...)
}

func RPCErrorf(msgFormat string, args ...any) error {
	return Errorf(CommonRPCErrorCode, msgFormat, args...)
}

func UnauthorizedErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, UnauthorizedCode, msgAndArgs...)
}

func MaybeUnauthorizedErr(cause error, msgAndArgs ...any) error {
	return MaybeWrapf(cause, UnauthorizedCode, msgAndArgs...)
}

func RedisErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, CommonRedisErrorCode, msgAndArgs...)
}

func AIAnnotateTaskFieldExistsErr(msgFormat string, args ...any) error {
	return Errorf(AIAnnotateTaskColumnExistsCode, msgFormat, args...)
}

func AIAnnotateTaskRunStatusUpdatedErr(msgFormat string, args ...any) error {
	return Errorf(AIAnnotateTaskRunStatusUpdatedCode, msgFormat, args...)
}

func InvalidDatasetCodeErr(msgFormat string, args ...any) error {
	return Errorf(InvalidDatasetCode, msgFormat, args...)
}

func DatasetNotEditableCodeError(msgFormat string, args ...any) error {
	return Errorf(DatasetNotEditableCode, msgFormat, args...)
}

func ConcurrentDatasetOperationsErrorf(msgFormat string, args ...any) error {
	return Errorf(ConcurrentDatasetOperationsCode, msgFormat, args...)
}

func ImageXErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, ImageXErrorCode, msgAndArgs...)
}

func CozeModelNotExistErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, CozeModelNotExistCode, msgAndArgs...)
}

func CozeModelNotExistErrorf(msgFormat string, args ...any) error {
	return Errorf(CozeModelNotExistCode, msgFormat, args...)
}

func InterfaceNotAvailableInHouseErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, InterfaceNotAvailableInHouseCode, msgAndArgs...)
}

func InterfaceNotAvailableInHouseErrorf(msgFormat string, args ...any) error {
	return Errorf(InterfaceNotAvailableInHouseCode, msgFormat, args...)
}

func GetCozeModelListFailedErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, GetCozeModelListFailedCode, msgAndArgs...)
}

func GetCozeModelListFailedErrorf(msgFormat string, args ...any) error {
	return Errorf(GetCozeModelListFailedCode, msgFormat, args...)
}

func GetCozeModelFailedErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, GetCozeModelFailedCode, msgAndArgs...)
}

func GetCozeModelFailedErrorf(msgFormat string, args ...any) error {
	return Errorf(GetCozeModelFailedCode, msgFormat, args...)
}

func GetCozeModelListParamFailedErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, GetCozeModelListParamFailedCode, msgAndArgs...)
}

func GetCozeModelListParamFailedErrorf(msgFormat string, args ...any) error {
	return Errorf(GetCozeModelListParamFailedCode, msgFormat, args...)
}

func GetCozeModelUsageFailedErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, GetCozeModelUsageFailedCode, msgAndArgs...)
}

func GetCozeModelUsageFailedErrorf(msgFormat string, args ...any) error {
	return Errorf(GetCozeModelUsageFailedCode, msgFormat, args...)
}

func GetLLMGatewayModelConfigFailedErr(cause error, msgAndArgs ...any) error {
	return Wrapf(cause, GetLLMGatewayModelConfigFailedCode, msgAndArgs...)
}

func GetLLMGatewayModelConfigFailedErrorf(msgFormat string, args ...any) error {
	return Errorf(GetLLMGatewayModelConfigFailedCode, msgFormat, args...)
}

func GetContentAuditFailedErrorf(msgFormat string, args ...any) error {
	return Errorf(ContentAuditFailedCode, msgFormat, args...)
}

func Wrapf(cause error, code int32, msgAndArgs ...any) error {
	if cause == nil {
		return nil
	}

	msg := messageFromMsgAndArgs(msgAndArgs...)
	cause = errors.Wrap(cause, msg)

	var opt []errorx.Option
	ws := strings.SplitN(cause.Error(), "\n", 4)
	if len(ws) > 3 {
		ws = ws[:3]
	}
	opt = append(opt, errorx.WithExtraMsg(strings.Join(ws, "\n")))
	return errorx.WrapByCode(cause, code, opt...)
}

func Errorf(code int32, msgFormat string, args ...any) error {
	if code == 0 {
		return nil
	}
	msg := fmt.Sprintf(msgFormat, args...)
	return errorx.WrapByCode(errors.New(msg), code, errorx.WithExtraMsg(msg))
}

// MaybeWrapf 对 cause 进行错误码包装. 如果 cause 已经是一个错误码错误, 则忽略 code, 直接返回原错误.
func MaybeWrapf(cause error, code int32, msgAndArgs ...any) error {
	if cause == nil {
		return nil
	}

	var bizErr kerrors.BizStatusErrorIface
	if ok := errors.As(cause, &bizErr); ok {
		if len(msgAndArgs) == 0 {
			return cause
		}
		return errors.Wrapf(cause, "%s", messageFromMsgAndArgs(msgAndArgs...))
	}

	return Wrapf(cause, code, msgAndArgs...)
}

func messageFromMsgAndArgs(msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}

	if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0]
		if msgAsStr, ok := msg.(string); ok {
			return msgAsStr
		}
		return fmt.Sprintf("%+v", msg)
	}

	if len(msgAndArgs) > 1 {
		msg := msgAndArgs[0]
		if msgAsStr, ok := msg.(string); ok {
			return fmt.Sprintf(msgAsStr, msgAndArgs[1:]...)
		}

		msgs := gslice.Map(msgAndArgs, func(v any) string {
			return fmt.Sprintf("%+v", v)
		})
		return strings.Join(msgs, ",")
	}
	return ""
}

func CodeInErr(err error) (int32, bool) {
	e1, ok := kerrors.FromBizStatusError(err)
	if !ok {
		return 0, false
	}
	code := e1.BizStatusCode()
	if code == CommonRPCErrorCode { // 被 client middleware 进行过包装的 RPC error, 再进行一次解析.
		e2 := errors.Unwrap(e1)
		if e3, ok := kerrors.FromBizStatusError(e2); ok {
			return e3.BizStatusCode(), true
		}
	}
	return code, true
}

func BizErrorInErr(err error) (kerrors.BizStatusErrorIface, bool) {
	e1, ok := kerrors.FromBizStatusError(err)
	if ok && e1.BizStatusCode() == CommonRPCErrorCode {
		e2 := errors.Unwrap(e1)
		if e3, ok := kerrors.FromBizStatusError(e2); ok {
			return e3, true
		}
	}
	return e1, ok
}

// RetryableErr 可由 RMQ 重试的错误
type RetryableErr struct {
	err error
}

func (r *RetryableErr) Error() string {
	if r.err != nil {
		return r.err.Error()
	}

	return ""
}

func (r *RetryableErr) Unwrap() error {
	return r.err
}

func NewRetryableErr(err error) *RetryableErr {
	if err == nil {
		return nil
	}
	return &RetryableErr{err: err}
}

func IsRetryableErr(err error) bool {
	re := &RetryableErr{}
	return errors.As(err, &re)
}

func GetInternalErrorMsg(err error) string {
	err1, sucess := errorx.FromStatusError(err)
	if sucess {
		return err1.Error()
	}
	return err.Error()
}
