module UptimePingPlatform/services/cli-service

go 1.24.0

require (
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.19.0
	google.golang.org/grpc v1.78.0
	google.golang.org/protobuf v1.36.1
	UptimePingPlatform/pkg v0.0.0
)

replace UptimePingPlatform/pkg => ../../pkg
