package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// ConnectionManager manages the pool of MySQL connections across all instances.
// It is safe for concurrent use by HTTP handlers and the config watcher.
type ConnectionManager struct {
	mu        sync.RWMutex
	dbPool    map[string]*sql.DB
	instInfos map[string]InstanceConfig
}

// NewConnectionManager creates an empty connection manager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		dbPool:    make(map[string]*sql.DB),
		instInfos: make(map[string]InstanceConfig),
	}
}

// connectInstance opens and pings a single MySQL instance.
func connectInstance(inst InstanceConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", inst.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping: %w", err)
	}

	return db, nil
}

// InitFromConfig opens connections for all instances in the config.
// Any connection failure causes the entire init to fail (fail-fast at startup).
func (cm *ConnectionManager) InitFromConfig(cfg *Config) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, inst := range cfg.Instances {
		db, err := connectInstance(inst)
		if err != nil {
			// Close any already-opened connections before returning
			cm.closeAllLocked()
			return fmt.Errorf("failed to connect to instance %q: %w", inst.InstanceID, err)
		}

		cm.dbPool[inst.InstanceID] = db
		cm.instInfos[inst.InstanceID] = inst
		log.Printf("mysqlmcp: connected to instance %q (env=%s, read_only=%v)", inst.InstanceID, inst.Environment, inst.ReadOnly)
	}

	log.Printf("mysqlmcp: all %d instance(s) connected, server starting", len(cfg.Instances))
	return nil
}

// ReloadFromConfig diffs the current instances against the new config and applies changes.
// New instances are connected, removed instances are closed, changed DSNs are reconnected.
// Individual connection failures are logged but do not block the reload.
func (cm *ConnectionManager) ReloadFromConfig(cfg *Config) {
	cm.mu.RLock()
	oldIDs := make(map[string]bool, len(cm.instInfos))
	for id := range cm.instInfos {
		oldIDs[id] = true
	}
	cm.mu.RUnlock()

	newConfigs := make(map[string]InstanceConfig, len(cfg.Instances))
	newIDs := make(map[string]bool, len(cfg.Instances))
	for _, inst := range cfg.Instances {
		newIDs[inst.InstanceID] = true
		newConfigs[inst.InstanceID] = inst
	}

	// Build change lists
	var toAdd []InstanceConfig
	var toRemove []string
	var toReconnect []InstanceConfig

	for _, inst := range cfg.Instances {
		if !oldIDs[inst.InstanceID] {
			toAdd = append(toAdd, inst)
		} else {
			// Check if DSN changed
			cm.mu.RLock()
			oldInst, exists := cm.instInfos[inst.InstanceID]
			cm.mu.RUnlock()
			if exists && oldInst.DSN != inst.DSN {
				toReconnect = append(toReconnect, inst)
			}
			// Also update instInfos for non-DSN field changes (remark, timeout, etc.)
		}
	}

	for id := range oldIDs {
		if !newIDs[id] {
			toRemove = append(toRemove, id)
		}
	}

	// No changes
	if len(toAdd) == 0 && len(toRemove) == 0 && len(toReconnect) == 0 {
		return
	}

	log.Printf("mysqlmcp: config reload: +%d add, -%d remove, ~%d reconnect", len(toAdd), len(toRemove), len(toReconnect))

	// Apply changes under write lock
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Add new instances
	for _, inst := range toAdd {
		db, err := connectInstance(inst)
		if err != nil {
			log.Printf("mysqlmcp: failed to add instance %q: %v", inst.InstanceID, err)
			continue
		}
		cm.dbPool[inst.InstanceID] = db
		cm.instInfos[inst.InstanceID] = inst
		log.Printf("mysqlmcp: added instance %q (env=%s, read_only=%v)", inst.InstanceID, inst.Environment, inst.ReadOnly)
	}

	// Remove instances
	for _, id := range toRemove {
		if db, ok := cm.dbPool[id]; ok {
			_ = db.Close()
			delete(cm.dbPool, id)
			delete(cm.instInfos, id)
			log.Printf("mysqlmcp: removed instance %q", id)
		}
	}

	// Reconnect instances with changed DSN
	for _, inst := range toReconnect {
		// Close old connection
		if oldDB, ok := cm.dbPool[inst.InstanceID]; ok {
			_ = oldDB.Close()
		}

		db, err := connectInstance(inst)
		if err != nil {
			log.Printf("mysqlmcp: failed to reconnect instance %q (keeping old): %v", inst.InstanceID, err)
			continue
		}
		cm.dbPool[inst.InstanceID] = db
		cm.instInfos[inst.InstanceID] = inst
		log.Printf("mysqlmcp: reconnected instance %q (env=%s, read_only=%v)", inst.InstanceID, inst.Environment, inst.ReadOnly)
	}

	// Update instance config for unchanged instances (remark, timeout, etc. may have changed)
	for _, inst := range cfg.Instances {
		if _, exists := cm.dbPool[inst.InstanceID]; exists {
			cm.instInfos[inst.InstanceID] = inst
		}
	}
}

// GetDB returns the database connection for the given instance ID.
func (cm *ConnectionManager) GetDB(instanceID string) (*sql.DB, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	db, ok := cm.dbPool[instanceID]
	if !ok {
		return nil, fmt.Errorf("instance %q not found (available: %v)", instanceID, cm.getInstanceIDsLocked())
	}
	return db, nil
}

// GetInstance returns the configuration for the given instance ID.
func (cm *ConnectionManager) GetInstance(instanceID string) (InstanceConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	inst, ok := cm.instInfos[instanceID]
	if !ok {
		return InstanceConfig{}, fmt.Errorf("instance %q not found", instanceID)
	}
	return inst, nil
}

// GetInstanceIDs returns all instance IDs in sorted order.
func (cm *ConnectionManager) GetInstanceIDs() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.getInstanceIDsLocked()
}

func (cm *ConnectionManager) getInstanceIDsLocked() []string {
	ids := make([]string, 0, len(cm.dbPool))
	for id := range cm.dbPool {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Close shuts down all database connections.
func (cm *ConnectionManager) Close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.closeAllLocked()
}

func (cm *ConnectionManager) closeAllLocked() {
	for id, db := range cm.dbPool {
		_ = db.Close()
		delete(cm.dbPool, id)
		delete(cm.instInfos, id)
	}
	log.Printf("mysqlmcp: all connections closed")
}
