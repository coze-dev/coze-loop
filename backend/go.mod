module github.com/coze-dev/coze-loop/backend

go 1.24.0

toolchain go1.24.1

replace github.com/apache/thrift => github.com/apache/thrift v0.13.0

require (
	github.com/ClickHouse/clickhouse-go/v2 v2.34.0
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/Masterminds/semver/v3 v3.3.1
	github.com/alicebob/miniredis/v2 v2.34.0
	github.com/alitto/pond/v2 v2.3.4
	github.com/apache/rocketmq-client-go/v2 v2.1.2
	github.com/apache/thrift v0.19.0
	github.com/aws/aws-sdk-go v1.55.7
	github.com/baidubce/bce-qianfan-sdk/go/qianfan v0.0.15
	github.com/bytedance/gg v1.0.0
	github.com/bytedance/gopkg v0.1.2
	github.com/bytedance/sonic v1.13.2
	github.com/cenk/backoff v2.2.1+incompatible
	github.com/cloudwego/eino v0.3.55
	github.com/cloudwego/eino-ext/components/model/ark v0.1.8
	github.com/cloudwego/eino-ext/components/model/arkbot v0.0.0-20250520101807-b2008771903a
	github.com/cloudwego/eino-ext/components/model/claude v0.0.0-20250513023651-7b19c6ffbf4a
	github.com/cloudwego/eino-ext/components/model/deepseek v0.0.0-20250514085234-473e80da5261
	github.com/cloudwego/eino-ext/components/model/gemini v0.0.0-20250520101807-b2008771903a
	github.com/cloudwego/eino-ext/components/model/ollama v0.1.0
	github.com/cloudwego/eino-ext/components/model/openai v0.0.0-20250513023651-7b19c6ffbf4a
	github.com/cloudwego/eino-ext/components/model/qianfan v0.0.0-20250520101807-b2008771903a
	github.com/cloudwego/eino-ext/components/model/qwen v0.0.0-20250520101807-b2008771903a
	github.com/cloudwego/eino-ext/libs/acl/openai v0.0.0-20250519084852-38fafa73d9ea
	github.com/cloudwego/gopkg v0.1.4
	github.com/cloudwego/hertz v0.9.7
	github.com/cloudwego/kitex v0.13.1
	github.com/coocood/freecache v1.2.4
	github.com/coreos/go-semver v0.3.0
	github.com/coze-dev/cozeloop-go v0.1.7
	github.com/coze-dev/cozeloop-go/spec v0.1.3
	github.com/deatil/go-encoding v1.0.3003
	github.com/dimchansky/utfbom v1.1.1
	github.com/dolthub/go-mysql-server v0.18.0
	github.com/dolthub/vitess v0.0.0-20240228192915-d55088cef56a
	github.com/expr-lang/expr v1.17.0
	github.com/fatih/structs v1.1.0
	github.com/fsnotify/fsnotify v1.8.0
	github.com/getkin/kin-openapi v0.118.0
	github.com/go-playground/validator/v10 v10.20.0
	github.com/go-redis/redis_rate/v10 v10.0.1
	github.com/go-viper/mapstructure/v2 v2.2.1
	github.com/google/generative-ai-go v0.20.1
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	github.com/google/wire v0.6.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hertz-contrib/sse v0.1.0
	github.com/jarcoal/httpmock v1.4.0
	github.com/jinzhu/copier v0.4.0
	github.com/json-iterator/go v1.1.12
	github.com/kaptinlin/jsonrepair v0.1.1
	github.com/mitchellh/mapstructure v1.5.0
	github.com/modern-go/reflect2 v1.0.2
	github.com/nicksnyder/go-i18n/v2 v2.6.0
	github.com/nikolalohinski/gonja/v2 v2.3.1
	github.com/ohler55/ojg v1.26.9
	github.com/ollama/ollama v0.10.1
	github.com/panjf2000/ants/v2 v2.11.3
	github.com/parquet-go/parquet-go v0.25.0
	github.com/pkg/errors v0.9.2-0.20201214064552-5dd12d0cfe7f
	github.com/redis/go-redis/v9 v9.7.3
	github.com/rs/xid v1.6.0
	github.com/saintfish/chardet v0.0.0-20230101081208-5e3ef4b5456d
	github.com/samber/lo v1.49.1
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cast v1.7.1
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.10.0
	github.com/valyala/fasttemplate v1.2.2
	github.com/volcengine/volcengine-go-sdk v1.1.4
	github.com/xeipuuv/gojsonschema v1.2.0
	go.opentelemetry.io/proto/otlp v1.7.1
	go.uber.org/mock v0.4.0
	golang.org/x/crypto v0.40.0
	golang.org/x/exp v0.0.0-20250606033433-dcc06ee1d476
	golang.org/x/sync v0.16.0
	golang.org/x/text v0.27.0
	gonum.org/v1/gonum v0.15.0
	google.golang.org/api v0.215.0
	gorm.io/datatypes v1.2.5
	gorm.io/driver/clickhouse v0.6.1
	gorm.io/driver/mysql v1.5.7
	gorm.io/gen v0.3.27
	gorm.io/gorm v1.30.0
	gorm.io/plugin/dbresolver v1.6.0
	gorm.io/plugin/soft_delete v1.2.1
)

require github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.1 // indirect

require (
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/ai v0.8.0 // indirect
	cloud.google.com/go/auth v0.13.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.6 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	cloud.google.com/go/longrunning v0.5.7 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/ClickHouse/ch-go v0.65.1 // indirect
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/anthropics/anthropic-sdk-go v0.2.0-alpha.8 // indirect
	github.com/aws/aws-sdk-go-v2 v1.33.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.54 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.9 // indirect
	github.com/aws/smithy-go v1.22.1 // indirect
	github.com/baidubce/bce-sdk-go v0.9.164 // indirect
	github.com/bluele/gcache v0.0.2 // indirect
	github.com/bufbuild/protocompile v0.8.0 // indirect
	github.com/bytedance/sonic/loader v0.2.4 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/cloudwego/configmanager v0.2.3 // indirect
	github.com/cloudwego/dynamicgo v0.6.2 // indirect
	github.com/cloudwego/fastpb v0.0.5 // indirect
	github.com/cloudwego/frugal v0.2.5 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/cloudwego/localsession v0.1.2 // indirect
	github.com/cloudwego/netpoll v0.7.0 // indirect
	github.com/cloudwego/runtimex v0.1.1 // indirect
	github.com/cloudwego/thriftgo v0.4.1 // indirect
	github.com/cohesion-org/deepseek-go v1.2.8 // indirect
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/denisenkom/go-mssqldb v0.12.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dolthub/flatbuffers/v23 v23.3.3-dh.2 // indirect
	github.com/dolthub/go-icu-regex v0.0.0-20230524105445-af7e7991c97e // indirect
	github.com/dolthub/jsonpath v0.0.2-0.20240227200619-19675ab05c71 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-kit/kit v0.12.1-0.20220826005032-a7ba4fa4e289 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.1 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-sql-driver/mysql v1.9.2 // indirect
	github.com/gocraft/dbr/v2 v2.7.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/pprof v0.0.0-20240827171923-fa2c70bbbfe5 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/goph/emperror v0.17.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/invopop/yaml v0.1.0 // indirect
	github.com/jhump/protoreflect v1.15.6 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lestrrat-go/strftime v1.0.4 // indirect
	github.com/lib/pq v1.10.6 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/meguminnnnnnnnn/go-openai v0.0.0-20250408071642-761325becfd6 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/nikolalohinski/gonja v1.5.3 // indirect
	github.com/nyaruka/phonenumbers v1.3.2 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/paulmach/orb v0.11.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pingcap/errors v0.11.5-0.20240311024730-e056997136bb // indirect
	github.com/pingcap/failpoint v0.0.0-20240528011301-b51a646c7c86 // indirect
	github.com/pingcap/log v1.1.0 // indirect
	github.com/pingcap/tidb/pkg/parser v0.0.0-20240407083020-62d6f4737bfb // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/slongfield/pyfmt v0.0.0-20220222012616-ea85ff4c361f // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tetratelabs/wazero v1.1.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/volcengine/volc-sdk-golang v1.0.172 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yargevad/filepathx v1.0.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.54.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect
	go.opentelemetry.io/otel v1.36.0
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/arch v0.15.0 // indirect
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	golang.org/x/tools v0.34.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.6
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/src-d/go-errors.v1 v1.0.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/driver/sqlite v1.5.5 // indirect
	gorm.io/hints v1.1.2 // indirect
	gorm.io/rawsql v1.0.3-0.20250401110442-7e49778bc820
	stathat.com/c/consistent v1.0.0 // indirect
)
