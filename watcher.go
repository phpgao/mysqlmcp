package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchConfig monitors the config directory for changes and triggers hot reload.
// It watches the parent directory (not the file directly) to correctly handle
// K8s Secret symlink swaps where kubelet atomically replaces the ..data symlink
// when the Secret is updated.
//
// On change, it reads and validates the new config. If valid, it applies
// the new instance configuration without restarting. On invalid config, it logs
// the error and keeps running with the current configuration.
func watchConfig(configPath string, cm *ConnectionManager) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("mysqlmcp: failed to create config watcher: %v", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	// Watch the directory containing the config file.
	// This catches K8s Secret symlink swaps which only generate events on the
	// ..data symlink in the directory, not on the config filename directly.
	dir := filepath.Dir(configPath)
	if err := watcher.Add(dir); err != nil {
		log.Printf("mysqlmcp: failed to watch config directory %q: %v", dir, err)
		return
	}

	log.Printf("mysqlmcp: watching config directory %q for changes", dir)

	const debounceInterval = 200 * time.Millisecond
	var timer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Trigger reload on any file event in the config directory.
			// K8s Secret updates generate Create events for new timestamped
			// subdirectories, and regular file changes generate Create/Write.
			if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}

			log.Printf("mysqlmcp: config change detected (%s): %s", event.Op, event.Name)

			// Debounce: reset timer on each event; reload fires after quiet period
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceInterval, func() {
				cfg, err := LoadConfig(configPath)
				if err != nil {
					log.Printf("mysqlmcp: config reload rejected (invalid config): %v", err)
					return
				}
				cm.ReloadFromConfig(cfg)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("mysqlmcp: config watcher error: %v", err)
		}
	}
}
