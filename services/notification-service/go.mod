module UptimePingPlatform/services/notification-service

go 1.24.0

require (
	UptimePingPlatform/pkg v0.0.0
	github.com/rabbitmq/amqp091-go v1.10.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
)

replace UptimePingPlatform/gen => ../../gen

replace UptimePingPlatform/pkg => ../../pkg
