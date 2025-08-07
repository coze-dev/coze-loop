// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coze-dev/cozeloop-go"
	goredis "github.com/redis/go-redis/v9"

	"github.com/coze-dev/coze-loop/backend/api"
	"github.com/coze-dev/coze-loop/backend/api/handler/coze/loop/apis"
	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	"github.com/coze-dev/coze-loop/backend/infra/i18n"
	"github.com/coze-dev/coze-loop/backend/infra/i18n/goi18n"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/idgen/redis_gen"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/limiter/dist"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer/rpc"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/infra/mq/registry"
	"github.com/coze-dev/coze-loop/backend/infra/mq/rocketmq"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/foundation/lofile"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/observability/lotrace"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/conf/viper"
	"github.com/coze-dev/coze-loop/backend/pkg/file"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func main() {
	ctx := context.Background()
	c, err := newComponent(ctx)
	if err != nil {
		panic(err)
	}

	handler, err := api.Init(ctx, c.idgen, c.db, c.redis, c.cfgFactory, c.mqFactory, c.objectStorage, c.batchObjectStorage, c.benefitSvc, c.auditClient, c.metric, c.limiterFactory, c.ckDb, c.translater)
	if err != nil {
		panic(err)
	}

	if err := initTracer(handler); err != nil {
		panic(err)
	}

	if err := registry.NewConsumerRegistry(c.mqFactory).Register(mustInitConsumerWorkers(c.cfgFactory, handler, handler)).StartAll(ctx); err != nil {
		panic(err)
	}

	api.Start(handler)
}

type ComponentConfig struct {
	Redis    redisConfig `mapstructure:"redis"`
	RDS      rdsConfig   `mapstructure:"rds"`
	S3Config s3Config    `mapstructure:"s3_config"`
	CKConfig ckConfig    `mapstructure:"ck_config"`
	IDGen    idGenConfig `mapstructure:"idgen"`
	LogLevel string      `mapstructure:"log_level"`
}

func getComponentConfig(configFactory conf.IConfigLoaderFactory) (*ComponentConfig, error) {
	ctx := context.Background()
	componentConfigLoader, err := configFactory.NewConfigLoader("infrastructure.yaml")
	if err != nil {
		return nil, err
	}
	componentConfig := &ComponentConfig{}
	err = componentConfigLoader.UnmarshalKey(ctx, "infra", componentConfig)
	if err != nil {
		return nil, err
	}
	return componentConfig, nil
}

type component struct {
	idgen              idgen.IIDGenerator
	db                 db.Provider
	redis              redis.Cmdable
	cfgFactory         conf.IConfigLoaderFactory
	mqFactory          mq.IFactory
	objectStorage      fileserver.ObjectStorage
	batchObjectStorage fileserver.BatchObjectStorage
	benefitSvc         benefit.IBenefitService
	auditClient        audit.IAuditService
	metric             metrics.Meter
	limiterFactory     limiter.IRateLimiterFactory
	ckDb               ck.Provider
	translater         i18n.ITranslater
}

func initTracer(handler *apis.APIHandler) error {
	rpc.SetLoopTracerHandler(
		lofile.NewLocalFileService(handler.FileService),
		lotrace.NewLocalTraceService(handler.ITraceApplication),
	)

	client, err := cozeloop.NewClient(
		cozeloop.WithWorkspaceID("0"),
		cozeloop.WithAPIToken("0"),
		cozeloop.WithExporter(&looptracer.MultiSpaceSpanExporter{}),
	)
	if err != nil {
		return err
	}
	looptracer.InitTracer(looptracer.NewTracer(client))

	return nil
}

func newComponent(ctx context.Context) (*component, error) {
	cfgFactory := viper.NewFileConfigLoaderFactory(viper.WithFactoryConfigPath("conf"))
	cfg, err := getComponentConfig(cfgFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to get component config: %w", err)
	}

	if err := setupLogging(cfg.LogLevel); err != nil {
		return nil, fmt.Errorf("failed to setup logging: %w", err)
	}

	redisClient, err := newRedisClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	db, err := newDB(cfg.RDS)
	if err != nil {
		return nil, fmt.Errorf("failed to create database provider: %w", err)
	}

	objectStorage, err := newObjectStorage(cfg.S3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create object storage client: %w", err)
	}

	ckDb, err := newCKDB(cfg.CKConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clickhouse provider: %w", err)
	}

	idGenerator, err := newIDGenerator(redisClient, cfg.IDGen)
	if err != nil {
		return nil, fmt.Errorf("failed to create id generator: %w", err)
	}

	translater, err := newTranslater()
	if err != nil {
		return nil, fmt.Errorf("failed to create translater: %w", err)
	}

	return &component{
		idgen:              idGenerator,
		db:                 db,
		redis:              redisClient,
		cfgFactory:         cfgFactory,
		mqFactory:          rocketmq.NewFactory(),
		objectStorage:      objectStorage,
		batchObjectStorage: objectStorage,
		benefitSvc:         benefit.NewNoopBenefitService(),
		auditClient:        audit.NewNoopAuditService(),
		metric:             metrics.GetMeter(),
		limiterFactory:     dist.NewRateLimiterFactory(redisClient),
		ckDb:               ckDb,
		translater:         translater,
	}, nil
}

func setupLogging(logLevel string) error {
	level, err := logs.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %w", err)
	}
	logs.SetLogLevel(level)
	return nil
}

type redisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
}

type rdsConfig struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	DB       string `mapstructure:"db"`
}

type s3Config struct {
	Region          string `mapstructure:"region"`
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	AccessKey       string `mapstructure:"access_key"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
}

type ckConfig struct {
	Host        string `mapstructure:"host"`
	Database    string `mapstructure:"database"`
	UserName    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	DialTimeout int    `mapstructure:"dial_timeout"`
	ReadTimeout int    `mapstructure:"read_timeout"`
}

type idGenConfig struct {
	ServerIDs []int64 `mapstructure:"server_ids"`
}

func newRedisClient(cfg redisConfig) (redis.Cmdable, error) {
	cmdable, err := redis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}
	return cmdable, nil
}

func newDB(cfg rdsConfig) (db.Provider, error) {
	db, err := db.NewDBFromConfig(&db.Config{
		User:         cfg.User,
		Password:     cfg.Password,
		DBHostname:   cfg.Host,
		DBPort:       cfg.Port,
		DBName:       cfg.DB,
		Loc:          "Local",
		DBCharset:    "utf8mb4",
		Timeout:      time.Minute,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
		DSNParams:    url.Values{"clientFoundRows": []string{"true"}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create database provider: %w", err)
	}
	return db, nil
}

func newObjectStorage(cfg s3Config) (fileserver.ObjectStorage, error) {
	s3Config := fileserver.NewS3Config(func(c *fileserver.S3Config) {
		c.Endpoint = cfg.Endpoint
		c.Region = cfg.Region
		c.Bucket = cfg.Bucket
		c.AccessKeyID = cfg.AccessKey
		c.SecretAccessKey = cfg.SecretAccessKey
	})
	objectStorage, err := fileserver.NewS3Client(s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create object storage client: %w", err)
	}
	return objectStorage, nil
}

func newCKDB(cfg ckConfig) (ck.Provider, error) {
	ckDb, err := ck.NewCKFromConfig(&ck.Config{
		Host:              cfg.Host,
		Database:          cfg.Database,
		Username:          cfg.UserName,
		Password:          cfg.Password,
		CompressionMethod: ck.CompressionMethodZSTD,
		CompressionLevel:  3,
		Protocol:          ck.ProtocolNative,
		DialTimeout:       time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:       time.Duration(cfg.ReadTimeout) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create clickhouse provider: %w", err)
	}
	return ckDb, nil
}

func newIDGenerator(redisClient redis.Cmdable, cfg idGenConfig) (idgen.IIDGenerator, error) {
	redisCli, ok := redis.Unwrap(redisClient)
	if !ok {
		return nil, errors.New("unwrap redis cli fail")
	}
	idGenerator, err := redis_gen.NewIDGenerator(redisCli, cfg.ServerIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create id generator: %w", err)
	}
	return idGenerator, nil
}

func newTranslater() (i18n.ITranslater, error) {
	localeDir, err := file.FindSubDir(os.Getenv("PWD"), "runtime/locales")
	if err != nil {
		return nil, fmt.Errorf("failed to find locales directory: %w", err)
	}
	translater, err := goi18n.NewTranslater(localeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create translater: %w", err)
	}
	return translater, nil
}
