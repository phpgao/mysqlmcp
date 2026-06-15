package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	dbPool    = make(map[string]*sql.DB)
	instInfos = make(map[string]InstanceConfig)
)

// InitConnections opens and verifies all MySQL connections from config.
// Must be called before any tool is invoked.
func InitConnections(cfg *Config) error {
	for _, inst := range cfg.Instances {
		db, err := sql.Open("mysql", inst.DSN)
		if err != nil {
			return fmt.Errorf("failed to open connection for %q: %w", inst.InstanceID, err)
		}

		// Connection pool settings
		db.SetMaxOpenConns(5)
		db.SetMaxIdleConns(2)
		db.SetConnMaxLifetime(5 * time.Minute)

		// Verify connectivity
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("failed to connect to instance %q: %w", inst.InstanceID, err)
		}

		dbPool[inst.InstanceID] = db
		instInfos[inst.InstanceID] = inst
		log.Printf("mysqlmcp: connected to instance %q (env=%s, read_only=%v)", inst.InstanceID, inst.Environment, inst.ReadOnly)
	}
	return nil
}

// GetDB returns the database connection for the given instance ID.
func GetDB(instanceID string) (*sql.DB, error) {
	db, ok := dbPool[instanceID]
	if !ok {
		return nil, fmt.Errorf("instance %q not found (available: %v)", instanceID, getInstanceIDs())
	}
	return db, nil
}

// GetInstance returns the configuration for the given instance ID.
func GetInstance(instanceID string) (InstanceConfig, error) {
	inst, ok := instInfos[instanceID]
	if !ok {
		return InstanceConfig{}, fmt.Errorf("instance %q not found", instanceID)
	}
	return inst, nil
}

func getInstanceIDs() []string {
	ids := make([]string, 0, len(dbPool))
	for id := range dbPool {
		ids = append(ids, id)
	}
	return ids
}
