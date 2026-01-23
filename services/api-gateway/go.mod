module UptimePingPlatform/services/api-gateway

go 1.24.0

replace (
	UptimePingPlatform/pkg/config => ../..
	UptimePingPlatform/pkg/errors => ../..
	UptimePingPlatform/pkg/logger => ../..
	UptimePingPlatform/pkg/metrics => ../..
	UptimePingPlatform/pkg/ratelimit => ../..
	UptimePingPlatform/pkg/redis => ../..
	UptimePingPlatform/services/api-gateway/internal/handler => ../..
)
