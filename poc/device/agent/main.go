package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	var configFile = flag.String("config", "config/config.yaml", "Path to the configuration file")
	flag.Parse() // Parse command-line flags

	// Initialize the logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sugar := logger.Sugar()

	// Create ConfigManager
	configManager := NewConfigManager(*configFile) // Pass the config file path

	// Load Config
	config, err := configManager.LoadAndValidateConfig()
	if err != nil {
		sugar.Fatalf("Failed to load/validate config: %v", err)
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create APIClientFactory
	sbiClient := &apiClient{
		SBIUrl: config.WfmSbiUrl,
	}

	// create database
	// Create HelmClient and DockerComposeClient (replace with actual implementations)

	// Create DeviceAgent
	agent, err := NewDeviceAgent(
		config,
		sugar,
		sbiClient,
	)
	if err != nil {
		sugar.Fatalf("Failed to create device agent: %v", err)
	}

	if err := agent.Start(); err != nil {
		sugar.Fatalf("Failed to start device agent: %v", err)
	}

	defer agent.Stop()

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		sugar.Infow("Received shutdown signal", "signal", sig)
		cancel() // Cancel the context to trigger shutdown
	case <-ctx.Done():
		sugar.Info("Context cancelled")
	}

	sugar.Info("Initiating graceful shutdown...")

	// Create a timeout context for shutdown operations with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop the agent
	done := make(chan error, 1)
	go func() {
		done <- agent.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			sugar.Errorw("Error during shutdown", "error", err)
		} else {
			sugar.Info("Graceful shutdown completed successfully")
		}
	case <-shutdownCtx.Done():
		sugar.Warn("Shutdown timeout exceeded, forcing exit")
	}
}
