package main

import (
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchConfig monitors the config file for changes and triggers hot reload.
// On file change, it reads and validates the new config. If valid, it applies
// the new instance configuration without restarting. On invalid config, it logs
// the error and keeps running with the current configuration.
func watchConfig(path string, cm *ConnectionManager) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("mysqlmcp: failed to create config watcher: %v", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(path); err != nil {
		log.Printf("mysqlmcp: failed to watch config file %q: %v", path, err)
		return
	}

	log.Printf("mysqlmcp: watching config file %q for changes", path)

	const debounceInterval = 200 * time.Millisecond
	var timer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only react to WRITE and CREATE events (CREATE handles atomic rename)
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce: reset timer on each event; reload fires after quiet period
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceInterval, func() {
				cfg, err := LoadConfig(path)
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
