package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/applauseoss/metronomikon/api"
	"github.com/applauseoss/metronomikon/config"
	"github.com/applauseoss/metronomikon/kube"
)

var defaultConfigFile = "/etc/metronomikon/config.yaml"

func main() {
	var debug bool
	var configFile string
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.StringVar(&configFile, "config", "", fmt.Sprintf("Path to config file (defaults to %s, if it exists)", defaultConfigFile))
	flag.Parse()
	// Setup logger
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}),
	)
	slog.SetDefault(logger)
	// Use default config file path if none was provided and it exists
	if configFile == "" {
		if _, err := os.Stat(defaultConfigFile); err == nil {
			configFile = defaultConfigFile
		}
	}
	// Load config file if specified
	if configFile != "" {
		if err := config.LoadConfig(configFile); err != nil {
			slog.Error(fmt.Sprintf("Failed to load config file: %s", err))
			os.Exit(1)
		}
	}
	if err := kube.TestClientConnection(); err != nil {
		slog.Error(fmt.Sprintf("Failed to connect to Kubernetes API: %s", err))
		os.Exit(1)
	} else {
		if debug {
			slog.Info("Successfully initialized kubernetes client")
		}
	}
	a := api.New(debug)
	_ = a.Start()
}
