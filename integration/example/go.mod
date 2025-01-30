module github.com/GoogleCloudPlatform/workloadagentplatform/integration/example

go 1.23

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/cmd v0.0.0 => ./cmd

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/service v0.0.0 => ./service

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/protos v0.0.0 => ./protos

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/onetime v0.0.0 => ./onetime

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/example/usagemetrics v0.0.0 => ./usagemetrics

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/common v0.0.0 => ../common

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/protos v0.0.0 => ../common/protos

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/onetime v0.0.0 => ../common/onetime

replace github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/usagemetrics v0.0.0 => ../common/usagemetrics

require (
	github.com/GoogleCloudPlatform/workloadagentplatform/integration/common v0.0.0
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.24.0
)

require (
	cloud.google.com/go v0.112.0 // indirect
	cloud.google.com/go/compute v1.23.4 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/logging v1.9.0 // indirect
	cloud.google.com/go/longrunning v0.5.5 // indirect
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jonboulle/clockwork v0.4.1-0.20230717050334-b1209715e43c // indirect
	github.com/kardianos/service v1.2.2 // indirect
	github.com/natefinch/lumberjack v0.0.0-20230119042236-215739b3bcdc // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.24.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/oauth2 v0.17.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/api v0.168.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20240205150955-31a09d347014 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240205150955-31a09d347014 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240304161311-37d4d3c04a78 // indirect
	google.golang.org/grpc v1.62.0 // indirect
	google.golang.org/protobuf v1.34.3-0.20240708074925-b46f280f9725 // indirect
)
