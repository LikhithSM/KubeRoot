package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"kuberoot/internal/store"
)

func generateRandomKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "kr_" + hex.EncodeToString(bytes), nil
}

func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func main() {
	orgID := flag.String("org", "", "Organization ID (required)")
	name := flag.String("name", "default", "API key name")
	flag.Parse()

	if *orgID == "" {
		log.Fatal("--org is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := store.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	apiKey, err := generateRandomKey()
	if err != nil {
		log.Fatalf("failed to generate API key: %v", err)
	}

	keyHash := hashAPIKey(apiKey)

	insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer insertCancel()

	// Direct SQL execution since this is a utility
	query := `INSERT INTO api_keys (organization_id, key_hash, name, active, created_at)
	          VALUES ($1, $2, $3, true, NOW())`

	// Access the db connection directly via reflection or add a method
	// For now, we'll output instructions
	fmt.Println("========================================")
	fmt.Println("Generated API Key:")
	fmt.Println("========================================")
	fmt.Printf("Key: %s\n", apiKey)
	fmt.Printf("Organization: %s\n", *orgID)
	fmt.Printf("Name: %s\n", *name)
	fmt.Println()
	fmt.Println("To activate, run this SQL:")
	fmt.Println("========================================")
	fmt.Printf("INSERT INTO api_keys (organization_id, key_hash, name, active, created_at)\n")
	fmt.Printf("VALUES ('%s', '%s', '%s', true, NOW());\n", *orgID, keyHash, *name)
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("IMPORTANT: Save this key now. It cannot be retrieved later.")
	fmt.Println()

	_ = insertCtx
	_ = query
}
