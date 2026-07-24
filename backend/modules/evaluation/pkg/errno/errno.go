package errno

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

type ErrImpl struct {
	Code  int
	Msg   string
	Cause error
}

func (e *ErrImpl) Error() string {
	if e == nil {
		return ""
	}
	var sb strings.Builder
	if len(e.Msg) > 0 {
		fmt.Fprintf(&sb, "ErrMsg=%v\n", e.Msg)
	}
	if e.Cause != nil {
		cm := e.CauseMsg()
		if len(cm) > 0 {
			fmt.Fprintf(&sb, "Cause=%v\n", e.Cause)
		}
	}
	return sb.String()
}

func (e *ErrImpl) CauseMsg() string {
	if e == nil {
		return ""
	}
	if errors.Is(e.Cause, e) {
		return ""
	}
	return e.Cause.Error()
}

func (e *ErrImpl) ErrMsg() string {
	if e != nil {
		return e.Msg
	}
	return ""
}

func (e *ErrImpl) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *ErrImpl) Serialize() []byte {
	if e == nil {
		return nil
	}
	persisted := e.toPersisted()
	bytes, _ := json.Marshal(persisted)
	return bytes
}

func (e *ErrImpl) toPersisted() *persistedErrImpl {
	if e == nil {
		return nil
	}
	p := &persistedErrImpl{
		Code: e.Code,
		Msg:  e.Msg,
	}
	if e.Cause != nil {
		p.Cause = e.Cause.Error()
	}
	return p
}

func (e *ErrImpl) SetErrMsg(msg string) *ErrImpl {
	if e != nil {
		e.Msg = msg
	}
	return e
}

func (e *ErrImpl) SetCause(err error) *ErrImpl {
	if errors.Is(e, err) {
		return e
	}
	if e != nil {
		e.Cause = err
	}
	return e
}

type persistedErrImpl struct {
	Code  int
	Msg   string
	Cause string
}

func (e *persistedErrImpl) toErrImpl() *ErrImpl {
	if e == nil {
		return nil
	}
	ei := &ErrImpl{
		Code: e.Code,
		Msg:  e.Msg,
	}
	if len(e.Cause) > 0 {
		ei.Cause = errors.New(e.Cause)
	}
	return ei
}

func ParseErrImpl(err error) (*ErrImpl, bool) {
	if err == nil {
		return nil, false
	}

	var ei *ErrImpl

	ok := errors.As(err, &ei)

	return ei, ok
}

func SerializeErr(err error) string {
	if err == nil {
		return ""
	}
	ei, ok := ParseErrImpl(err)
	if ok {
		return conv.UnsafeBytesToString(ei.Serialize())
	}
	return err.Error()
}

func DeserializeErr(b []byte) error {
	persisted := &persistedErrImpl{}
	if err := json.Unmarshal(b, persisted); err == nil {
		return persisted.toErrImpl()
	}
	return errors.New(string(b))
}

func CloneErr(err error) error {
	ei, ok := ParseErrImpl(err)
	if ok {
		return &ErrImpl{
			Code:  ei.Code,
			Msg:   ei.Msg,
			Cause: ei.Cause,
		}
	}
	return errors.New(err.Error())
}
