package db

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/meilisearch/meilisearch-go"
)

const (
	defaultMeiliURL   = "http://localhost:7700"
	symbolsIndexName  = "mairu_symbols"
	symbolsPrimaryKey = "id"
)

type DB struct {
	client meilisearch.ServiceManager
	root   string
}

type Config struct {
	MeiliURL    string
	MeiliAPIKey string
}

func InitDB(projectRoot string, cfg ...Config) (*DB, error) {
	var provided Config
	if len(cfg) > 0 {
		provided = cfg[0]
	}
	host, apiKey := resolveMeiliConfig(provided.MeiliURL, provided.MeiliAPIKey)

	client := meilisearch.New(host, meilisearch.WithAPIKey(apiKey))

	db := &DB{
		client: client,
		root:   projectRoot,
	}

	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

// NewTestDB creates a minimal DB for testing without connecting to Meilisearch
func NewTestDB(projectRoot string) *DB {
	return &DB{
		root: projectRoot,
	}
}

func (db *DB) migrate() error {
	if db == nil || db.client == nil {
		return errors.New("database client is not initialized")
	}

	// Let's create the index. If it exists, this will return an error but we can ignore it if it's already there.
	_, _ = db.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        symbolsIndexName,
		PrimaryKey: symbolsPrimaryKey,
	})

	// Configure searchable attributes
	_, err := db.client.Index(symbolsIndexName).UpdateSearchableAttributes(&[]string{
		"name",
		"kind",
		"file_path",
	})

	if err != nil {
		return fmt.Errorf("failed to set searchable attributes: %w", err)
	}

	return nil
}

func (db *DB) Root() string {
	return db.root
}

func (db *DB) UpsertFile(path string, hash string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path cannot be empty")
	}
	// For Meilisearch we can just return the path directly as the file ID.
	return path, nil
}

func (db *DB) InsertSymbol(id, fileID, name, kind string, exported bool, startRow, startCol, endRow, endCol uint32) error {
	if db == nil || db.client == nil {
		return errors.New("database client is not initialized")
	}

	document := map[string]interface{}{
		"id":        id,
		"file_path": fileID,
		"name":      name,
		"kind":      kind,
		"exported":  exported,
		"start_row": startRow,
		"start_col": startCol,
		"end_row":   endRow,
		"end_col":   endCol,
	}

	_, err := db.client.Index(symbolsIndexName).AddDocuments([]map[string]interface{}{document}, nil)
	return err
}

func (db *DB) Close() error {
	// Meilisearch client uses HTTP, no connection to close
	return nil
}

func resolveMeiliConfig(host, apiKey string) (string, string) {
	host = strings.TrimSpace(host)
	apiKey = strings.TrimSpace(apiKey)

	if host == "" {
		host = strings.TrimSpace(os.Getenv("MEILI_URL"))
	}
	if host == "" {
		host = defaultMeiliURL
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("MEILI_API_KEY"))
	}

	return host, apiKey
}
