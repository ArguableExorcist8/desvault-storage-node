// database/database.go
package database

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ArguableExorcist8/desvault-storage-node/storage" 
)

var DB *gorm.DB

// ConnectDatabase opens a SQLite database and performs auto-migration.
func ConnectDatabase() {
	var err error
	DB, err = gorm.Open(sqlite.Open("desvault.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Auto-migrate the FileMetadata struct (creates table if it doesn't exist).
	err = DB.AutoMigrate(&storage.FileMetadata{})
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
}
