package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_redis "UptimePingPlatform/pkg/redis"
	forgev1 "UptimePingPlatform/proto/api/forge/v1"
	grpcHandler "UptimePingPlatform/services/forge-service/internal/handler/grpc"
	"UptimePingPlatform/services/forge-service/internal/service"
	"encoding/json"
	"google.golang.org/grpc"
	"net"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "forge-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting Forge Service...")

	// Initialize metrics
	appMetrics := metrics.NewMetrics("forge-service")
	metricsHandler := appMetrics.GetHandler()

	// Initialize health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Initialize Redis client
	redisClient, err := pkg_redis.Connect(context.Background(), &pkg_redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		appLogger.Error("Failed to connect to Redis", logger.Error(err))
	} else {
		defer redisClient.Close()
	}

	// Start gRPC server for Forge Service
	grpcPort := cfg.GRPC.Port
	if grpcPort == 0 {
		grpcPort = 50052
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		appLogger.Error("Failed to listen for gRPC", logger.Error(err))
		log.Fatalf("Failed to listen: %v", err)
	}

	// Initialize forge service components (parser, code generator, validator)
	protoParser := service.NewProtoParser(cfg.Forge.ProtoDir)
	// codeGenerator/validator can be nil for now
	forgeSvc := service.NewForgeService(appLogger, protoParser, nil, nil)
	grpcHandler := grpcHandler.NewForgeHandler(forgeSvc, appLogger)

	grpcServer := grpc.NewServer()
	forgev1.RegisterForgeServiceServer(grpcServer, grpcHandler)

	// Start gRPC server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting gRPC server on port %d", grpcPort))
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			appLogger.Error("gRPC server failed", logger.Error(err))
		}
	}()

	// Start HTTP server for metrics and health (on separate HTTP/metrics port)
	httpAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: setupHTTPHandler(metricsHandler, healthChecker, appLogger, forgeSvc),
	}

	// Start HTTP server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting HTTP server on port %s", httpAddr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", logger.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		appLogger.Error("Server shutdown failed", logger.Error(err))
	}

	appLogger.Info("Server stopped")
}

func setupHTTPHandler(metricsHandler http.Handler, healthChecker health.HealthChecker, appLogger logger.Logger, forgeSvc service.ForgeService) http.Handler {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", metricsHandler)

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"forge-service"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"forge-service"}`))
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"live","service":"forge-service"}`))
	})

	// Forge service endpoints
	mux.HandleFunc("/api/v1/forge/templates", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Forge Service - Templates endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/forge/build", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Forge Service - Build endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/forge/deploy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Forge Service - Deploy endpoint","status":"ok"}`))
	})
	// Compatibility endpoints for programmatic access
	mux.HandleFunc("/api/v1/forge/generate_config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			ProtoContent string                 `json:"proto_content"`
			Options      map[string]interface{} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			appLogger.Error("Failed to decode generate_config request", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid JSON"})
			return
		}
		var opts *service.ConfigOptions
		if payload.Options != nil {
			opts = &service.ConfigOptions{}
			if v, ok := payload.Options["target_host"].(string); ok {
				opts.TargetHost = v
			}
			if v, ok := payload.Options["target_port"].(float64); ok {
				opts.TargetPort = int(v)
			}
			if v, ok := payload.Options["check_interval"].(float64); ok {
				opts.CheckInterval = int(v)
			}
			if v, ok := payload.Options["timeout"].(float64); ok {
				opts.Timeout = int(v)
			}
			if v, ok := payload.Options["tenant_id"].(string); ok {
				opts.TenantID = v
			}
		}
		configYaml, checkConfig, err := forgeSvc.GenerateConfig(r.Context(), payload.ProtoContent, opts)
		if err != nil {
			appLogger.Error("GenerateConfig failed", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": fmt.Sprintf("Generation failed: %s", err.Error())})
			return
		}
		resp := map[string]interface{}{
			"success":     true,
			"config_yaml": configYaml,
			"check_config": map[string]interface{}{
				"name":     checkConfig.Name,
				"type":     checkConfig.Type,
				"target":   checkConfig.Target,
				"interval": checkConfig.Interval,
				"timeout":  checkConfig.Timeout,
				"config":   checkConfig.Config,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/v1/forge/parse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			ProtoContent string `json:"proto_content"`
			FileName     string `json:"file_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			appLogger.Error("Failed to decode parse request", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid JSON"})
			return
		}
		serviceInfo, isValid, warnings, err := forgeSvc.ParseProto(r.Context(), payload.ProtoContent, payload.FileName)
		if err != nil {
			appLogger.Error("ParseProto failed", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": fmt.Sprintf("Parse failed: %s", err.Error())})
			return
		}
		resp := map[string]interface{}{
			"success":      true,
			"service_info": serviceInfo,
			"is_valid":     isValid,
			"warnings":     warnings,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Add code generation endpoint
	mux.HandleFunc("/api/v1/forge/code", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			ProtoContent string                 `json:"proto_content"`
			Options      map[string]interface{} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			appLogger.Error("Failed to decode code request", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid JSON"})
			return
		}
		var opts *service.CodeOptions
		if payload.Options != nil {
			opts = &service.CodeOptions{}
			if v, ok := payload.Options["language"].(string); ok {
				opts.Language = v
			}
			if v, ok := payload.Options["framework"].(string); ok {
				opts.Framework = v
			}
			if v, ok := payload.Options["template"].(string); ok {
				opts.Template = v
			}
		}
		code, filename, language, err := forgeSvc.GenerateCode(r.Context(), payload.ProtoContent, opts)
		if err != nil {
			appLogger.Error("GenerateCode failed", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": fmt.Sprintf("Code generation failed: %s", err.Error())})
			return
		}
		resp := map[string]interface{}{
			"success":  true,
			"code":     code,
			"filename": filename,
			"language": language,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Add validate endpoint
	mux.HandleFunc("/api/v1/forge/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			ProtoContent string `json:"proto_content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			appLogger.Error("Failed to decode validate request", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid JSON"})
			return
		}
		isValid, errors, warnings, err := forgeSvc.ValidateProto(r.Context(), payload.ProtoContent)
		if err != nil {
			appLogger.Error("ValidateProto failed", logger.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": fmt.Sprintf("Validation failed: %s", err.Error())})
			return
		}
		resp := map[string]interface{}{
			"success":  true,
			"is_valid": isValid,
			"errors":   errors,
			"warnings": warnings,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	return mux
}
