package reload

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"github.com/lahabana/api-play/pkg/api"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
)

func reload(ctx context.Context, log *slog.Logger, configFile string, reloader api.Reloader) error {
	log.InfoContext(ctx, "reloading file", "conf", configFile)
	b, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	apis := api.ParamsAPI{}
	if err := yaml.Unmarshal(b, &apis); err != nil {
		return err
	}
	if err := reloader.Reload(ctx, apis); err != nil {
		return err
	}
	return nil
}

func BackgroundConfigReload(ctx context.Context, log *slog.Logger, configFile string, reloader api.Reloader) {
	if err := reload(ctx, log, configFile, reloader); err != nil {
		log.ErrorContext(ctx, "config loading failed, server will start with empty config", "error", err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	dirPath := filepath.Dir(configFile)
	if err := w.Add(dirPath); err != nil {
		panic(err)
	}
	go func() {
		log.InfoContext(ctx, "listening for config change events", "path", dirPath)
		defer func() {
			_ = w.Close()
			log.InfoContext(ctx, "stopping watcher")
		}()
		for {
			select {
			case e, ok := <-w.Events:
				if !ok {
					return
				}
				if e.Name == filepath.Base(configFile) {
					log.InfoContext(ctx, "got event", "op", e.Op.String(), "name", e.Name)
					err := reload(ctx, log, configFile, reloader)
					if err != nil {
						log.ErrorContext(ctx, "reloading config failed", "error", err)
					} else {
						log.InfoContext(ctx, "config successfully reloaded")
					}
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				log.ErrorContext(ctx, "watcher received error failed", "error", err)
			case <-ctx.Done():
				return
			}
		}
	}()
}
