package main

import (
	"flag"
	"log/slog"
	"os"

	"ai-gateway/config"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
	"ai-gateway/internal/server"
)

func main() {
	configPath := flag.String("config", "config/gateway.yaml", "path to gateway.yaml")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	provs := createProviders(cfg, logger)
	r, err := router.NewRouter(cfg.Routes)
	if err != nil {
		logger.Error("failed to create router", "error", err)
		os.Exit(1)
	}

	validateRoutes(cfg, provs, logger)

	srv, err := server.New(cfg, r, provs, logger)
	if err != nil {
		logger.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	// Start config file watcher for hot reload
	reloader, err := config.NewReloader(*configPath, cfg, logger, func(newCfg *config.Config) error {
		return srv.Reload(newCfg)
	})
	if err != nil {
		logger.Warn("config watcher not available, hot reload disabled", "error", err)
	} else {
		logger.Info("config hot reload enabled", "path", *configPath)
		_ = reloader
	}

	if err := srv.Start(); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("gateway stopped")
}

func createProviders(cfg *config.Config, logger *slog.Logger) map[string]provider.LLMProvider {
	provs := make(map[string]provider.LLMProvider)
	for _, pCfg := range cfg.Providers {
		pLogger := logger.With("provider", pCfg.Name)
		switch pCfg.Type {
		case "openai":
			p, err := provider.NewOpenAI(pCfg, pLogger)
			if err != nil {
				logger.Error("failed to create provider", "name", pCfg.Name, "error", err)
				os.Exit(1)
			}
			provs[pCfg.Name] = p
		case "claude":
			p, err := provider.NewClaude(pCfg, pLogger)
			if err != nil {
				logger.Error("failed to create provider", "name", pCfg.Name, "error", err)
				os.Exit(1)
			}
			provs[pCfg.Name] = p
		default:
			logger.Error("unknown provider type", "type", pCfg.Type)
			os.Exit(1)
		}
	}
	return provs
}

func validateRoutes(cfg *config.Config, provs map[string]provider.LLMProvider, logger *slog.Logger) {
	for _, routeCfg := range cfg.Routes {
		for _, t := range routeCfg.Targets {
			if _, ok := provs[t.Provider]; !ok {
				logger.Error("route references unknown provider", "route", routeCfg.Name, "provider", t.Provider)
				os.Exit(1)
			}
		}
		for _, sr := range routeCfg.SemanticRules {
			if _, ok := provs[sr.Target.Provider]; !ok {
				logger.Error("route references unknown provider", "route", routeCfg.Name, "provider", sr.Target.Provider)
				os.Exit(1)
			}
		}
	}
}
