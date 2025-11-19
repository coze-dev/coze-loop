package taskexe

import (
	"context"
	"os"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	CtxKeyEnv = "K_ENV"
	TceEnv    = "TCE_ENV"
	AppIDKey  = "LANE_C_FORNAX_APPID"
)

// todo 看看有没有更好的写法
func FillCtx(ctx context.Context, aid int64) context.Context {
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	ctx = metainfo.WithPersistentValue(ctx, AppIDKey, strconv.FormatInt(int64(aid), 10))
	if env := os.Getenv(TceEnv); env != "" {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck,SA1029
	}
	return ctx
}
