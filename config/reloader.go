package config

import (
	"log/slog"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type Reloader struct {
	path   string
	mu     sync.RWMutex
	cfg    *Config
	logger *slog.Logger
	onReload func(*Config) error
}

func NewReloader(path string, initial *Config, logger *slog.Logger, onReload func(*Config) error) (*Reloader, error) {
	r := &Reloader{
		path:     path,
		cfg:      initial,
		logger:   logger,
		onReload: onReload,
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return nil, err
	}

	go r.watch(watcher)
	return r, nil
}

func (r *Reloader) Config() *Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

func (r *Reloader) watch(watcher *fsnotify.Watcher) {
	defer watcher.Close()

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				r.logger.Info("config file changed, reloading", "path", r.path)
				newCfg, err := Load(r.path)
				if err != nil {
					r.logger.Error("failed to reload config", "error", err)
					continue
				}
				r.mu.Lock()
				r.cfg = newCfg
				r.mu.Unlock()

				if err := r.onReload(newCfg); err != nil {
					r.logger.Error("reload callback failed", "error", err)
				} else {
					r.logger.Info("config reloaded successfully")
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			r.logger.Error("config watcher error", "error", err)
		}
	}
}
