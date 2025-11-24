package mysql

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	genquery "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/query"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"gorm.io/gorm"
)

//go:generate mockgen -destination=mocks/trajectory_config.go -package=mocks . ITrajectoryConfigDao
type ITrajectoryConfigDao interface {
	GetTrajectoryConfig(ctx context.Context, workspaceID int64) (*model.ObservabilityTrajectoryConfig, error)
	UpdateTrajectoryConfig(ctx context.Context, po *model.ObservabilityTrajectoryConfig) error
	CreateTrajectoryConfig(ctx context.Context, po *model.ObservabilityTrajectoryConfig) error
}

func NewTrajectoryConfigDaoImpl(db db.Provider) ITrajectoryConfigDao {
	return &TrajectoryConfigDaoImpl{
		dbMgr: db,
	}
}

type TrajectoryConfigDaoImpl struct {
	dbMgr db.Provider
}

func (t TrajectoryConfigDaoImpl) UpdateTrajectoryConfig(ctx context.Context, po *model.ObservabilityTrajectoryConfig) error {
	//TODO implement me
	panic("implement me")
}

func (t TrajectoryConfigDaoImpl) CreateTrajectoryConfig(ctx context.Context, po *model.ObservabilityTrajectoryConfig) error {
	//TODO implement me
	panic("implement me")
}

func (t TrajectoryConfigDaoImpl) GetTrajectoryConfig(ctx context.Context, workspaceID int64) (*model.ObservabilityTrajectoryConfig, error) {
	q := genquery.Use(t.dbMgr.NewSession(ctx, db.WithMaster())).ObservabilityTrajectoryConfig
	qd := q.WithContext(ctx).Where(q.WorkspaceID.Eq(workspaceID))
	trajectoryConfigPo, err := qd.First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	}

	return trajectoryConfigPo, nil
}
