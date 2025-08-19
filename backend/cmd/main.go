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

	"github.com/bytedance/gg/gptr"

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

	if err := registry.NewConsumerRegistry(c.mqFactory).Register(mustInitConsumerWorkers(c.cfgFactory, handler, handler, handler)).StartAll(ctx); err != nil {
		panic(err)
	}

	api.Start(handler)
}

type ComponentConfig struct {
	Redis struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Password string `mapstructure:"password"`
	} `mapstructure:"redis"`
	RDS struct {
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		DB       string `mapstructure:"db"`
	} `mapstructure:"rds"`
	S3Config struct {
		Region          string `mapstructure:"region"`
		Endpoint        string `mapstructure:"endpoint"`
		Bucket          string `mapstructure:"bucket"`
		AccessKey       string `mapstructure:"access_key"`
		SecretAccessKey string `mapstructure:"secret_access_key"`
		ForcePathStyle  *bool  `mapstructure:"force_path_style"`
	} `mapstructure:"s3_config"`
	CKConfig struct {
		Host        string `mapstructure:"host"`
		Database    string `mapstructure:"database"`
		UserName    string `mapstructure:"username"`
		Password    string `mapstructure:"password"`
		DialTimeout int    `mapstructure:"dial_timeout"`
		ReadTimeout int    `mapstructure:"read_timeout"`
	} `mapstructure:"ck_config"`
	IDGen struct {
		ServerIDs []int64 `mapstructure:"server_ids"`
	} `mapstructure:"idgen"`
	LogLevel string `mapstructure:"log_level"`
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
	c := new(component)
	cfgFactory := viper.NewFileConfigLoaderFactory(viper.WithFactoryConfigPath("conf"))
	componentConfig, err := getComponentConfig(cfgFactory)
	if err != nil {
		return c, err
	}
	switch componentConfig.LogLevel {
	case "debug":
		logs.SetLogLevel(logs.DebugLevel)
	case "info":
		logs.SetLogLevel(logs.InfoLevel)
	case "warn":
		logs.SetLogLevel(logs.WarnLevel)
	case "error":
		logs.SetLogLevel(logs.ErrorLevel)
	case "fatal":
		logs.SetLogLevel(logs.FatalLevel)
	}
	cmdable, err := redis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%s", getRedisDomain(), getRedisPort()),
		Password: getRedisPassword(),
	})
	if err != nil {
		return nil, err
	}

	redisCli, ok := redis.Unwrap(cmdable)
	if !ok {
		return c, errors.New("unwrap redis cli fail")
	}

	db, err := db.NewDBFromConfig(&db.Config{
		DBHostname:   getMysqlDomain(),
		DBPort:       getMysqlPort(),
		User:         getMysqlUser(),
		Password:     getMysqlPassword(),
		DBName:       getMysqlDatabase(),
		Loc:          "Local",
		DBCharset:    "utf8mb4",
		Timeout:      time.Minute,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
		DSNParams:    url.Values{"clientFoundRows": []string{"true"}},
	})
	if err != nil {
		return nil, err
	}

	s3Config := fileserver.NewS3Config(func(cfg *fileserver.S3Config) {
		cfg.Endpoint = func() string {
			if getOssPort() == "" {
				return fmt.Sprintf("%s://%s", getOssProtocol(), getOssDomain())
			}
			return fmt.Sprintf("%s://%s:%s", getOssProtocol(), getOssDomain(), getOssPort())
		}()
		cfg.Region = getOssRegion()
		cfg.AccessKeyID = getOssUser()
		cfg.SecretAccessKey = getOssPassword()
		cfg.Bucket = getOssBucket()
		cfg.ForcePathStyle = getOssForcePathStyle()
	})
	objectStorage, err := fileserver.NewS3Client(s3Config)
	if err != nil {
		return nil, err
	}

	ckDb, err := ck.NewCKFromConfig(&ck.Config{
		Host:              fmt.Sprintf("%s:%s", getClickhouseDomain(), getClickhousePort()),
		Username:          getClickhouseUser(),
		Password:          getClickhousePassword(),
		Database:          getClickhouseDatabase(),
		CompressionMethod: ck.CompressionMethodLZ4,
		CompressionLevel:  3,
		Protocol:          ck.ProtocolNative,
		DialTimeout:       time.Duration(componentConfig.CKConfig.DialTimeout) * time.Second,
		ReadTimeout:       time.Duration(componentConfig.CKConfig.ReadTimeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	idgenerator, err := redis_gen.NewIDGenerator(redisCli, componentConfig.IDGen.ServerIDs)
	if err != nil {
		return nil, err
	}

	localeDir, err := file.FindSubDir(os.Getenv("PWD"), "conf/locales")
	if err != nil {
		return nil, err
	}
	translater, err := goi18n.NewTranslater(localeDir)
	if err != nil {
		return nil, err
	}

	return &component{
		idgen:              idgenerator,
		db:                 db,
		redis:              cmdable,
		cfgFactory:         cfgFactory,
		mqFactory:          rocketmq.NewFactory(),
		objectStorage:      objectStorage,
		batchObjectStorage: objectStorage,
		benefitSvc:         benefit.NewNoopBenefitService(),
		auditClient:        audit.NewNoopAuditService(),
		metric:             metrics.GetMeter(),
		limiterFactory:     dist.NewRateLimiterFactory(cmdable),
		ckDb:               ckDb,
		translater:         translater,
	}, nil
}

func getRedisDomain() string {
	return os.Getenv("COZE_LOOP_REDIS_DOMAIN")
}

func getRedisPort() string {
	return os.Getenv("COZE_LOOP_REDIS_PORT")
}

func getRedisPassword() string {
	return os.Getenv("COZE_LOOP_REDIS_PASSWORD")
}

func getMysqlDomain() string {
	return os.Getenv("COZE_LOOP_MYSQL_DOMAIN")
}

func getMysqlPort() string {
	return os.Getenv("COZE_LOOP_MYSQL_PORT")
}

func getMysqlUser() string {
	return os.Getenv("COZE_LOOP_MYSQL_USER")
}

func getMysqlPassword() string {
	return os.Getenv("COZE_LOOP_MYSQL_PASSWORD")
}

func getMysqlDatabase() string {
	return os.Getenv("COZE_LOOP_MYSQL_DATABASE")
}

func getClickhouseDomain() string {
	return os.Getenv("COZE_LOOP_CLICKHOUSE_DOMAIN")
}

func getClickhousePort() string {
	return os.Getenv("COZE_LOOP_CLICKHOUSE_PORT")
}

func getClickhouseUser() string {
	return os.Getenv("COZE_LOOP_CLICKHOUSE_USER")
}

func getClickhousePassword() string {
	return os.Getenv("COZE_LOOP_CLICKHOUSE_PASSWORD")
}

func getClickhouseDatabase() string {
	return os.Getenv("COZE_LOOP_CLICKHOUSE_DATABASE")
}

func getOssProtocol() string {
	return os.Getenv("COZE_LOOP_OSS_PROTOCOL")
}

func getOssDomain() string {
	return os.Getenv("COZE_LOOP_OSS_DOMAIN")
}

func getOssPort() string {
	return os.Getenv("COZE_LOOP_OSS_PORT")
}

func getOssUser() string {
	return os.Getenv("COZE_LOOP_OSS_USER")
}

func getOssPassword() string {
	return os.Getenv("COZE_LOOP_OSS_PASSWORD")
}

func getOssRegion() string {
	return os.Getenv("COZE_LOOP_OSS_REGION")
}

func getOssBucket() string {
	return os.Getenv("COZE_LOOP_OSS_BUCKET")
}

func getOssForcePathStyle() *bool {
	if getOssDomain() == "coze-loop-minio" {
		return gptr.Of(true)
	}
	return gptr.Of(false)
}
