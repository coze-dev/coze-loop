// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject
// +build wireinject

package application

import (
	"github.com/google/wire"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	tag2 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/service"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
	service2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/service"
	dataset_config "github.com/coze-dev/coze-loop/backend/modules/data/infra/conf"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/mq/producer"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/item_dao"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/mysql"
	oss_dao "github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/oss"
	redis2 "github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/redis"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/rpc/foundation"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/vfs/oss"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/vfs/unionfs"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

var (
	datasetSet = wire.NewSet(
		NewDatasetApplicationImpl,
		service.NewDatasetServiceImpl,
		dataset.NewDatasetRepo,
		mysql.NewDatasetDAO,
		mysql.NewDatasetItemDAO,
		mysql.NewDatasetVersionDAO,
		mysql.NewDatasetItemSnapshotDAO,
		mysql.NewDatasetSchemaDAO,
		mysql.NewDatasetIOJobDAO,
		redis2.NewOperationDAO,
		redis2.NewDatasetDAO,
		redis2.NewVersionDAO,
		dataset_config.NewConfiger,
		producer.NewDatasetJobPublisher,
		foundation.NewAuthRPCProvider,
		oss.NewClient,
		unionfs.NewUnionFS,
		lock.NewRedisLocker,
		NewItemProviderDAO,
	)

	tagSet = wire.NewSet(
		NewTagApplicationImpl,
		service2.NewTagServiceImpl,
		tag.NewTagRepoImpl,
		dataset_config.NewConfiger,
		userinfo.NewUserInfoServiceImpl,
		foundation.NewUserRPCProvider,
		lock.NewRedisLocker,
	)
)

func NewItemProviderDAO(batchObjectStorage fileserver.BatchObjectStorage) map[entity.Provider]item_dao.ItemDAO {
	return map[entity.Provider]item_dao.ItemDAO{
		entity.ProviderS3: oss_dao.NewDatasetItemDAO(batchObjectStorage),
	}
}

func InitDatasetApplication(
	idgen idgen.IIDGenerator,
	db db.Provider,
	cmdable redis.Cmdable,
	configFactory conf.IConfigLoaderFactory,
	configLoader conf.IConfigLoader,
	mqFactory mq.IFactory,
	objectStorage fileserver.ObjectStorage,
	batchObjectStorage fileserver.BatchObjectStorage,
	auditClient audit.IAuditService,
	authClient authservice.Client,
) (IDatasetApplication, error) {
	wire.Build(
		datasetSet,
	)
	return nil, nil
}

func InitTagApplication(idgen idgen.IIDGenerator,
	db db.Provider,
	cmdable redis.Cmdable,
	configLoader conf.IConfigLoader,
	userClient userservice.Client,
	authAdapter rpc.IAuthProvider) (tag2.TagService, error) {
	wire.Build(tagSet)
	return nil, nil
}
