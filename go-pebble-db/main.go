package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cockroachdb/pebble"
)

var db *pebble.DB

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func main() {
	// Create a temporary directory for Pebble DB
	// For in-memory like behavior, we'll use a temp directory that gets cleaned up
	tempDir, err := os.MkdirTemp("", "pebble-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir) // Clean up

	// Open the Pebble database
	db, err = pebble.Open(tempDir, &pebble.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize with test data
	initData()

	// Set up HTTP handlers
	http.HandleFunc("/set", setHandler)
	http.HandleFunc("/get", getHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/scan", scanHandler)

	// Start HTTP server
	fmt.Println("🚀 HTTP Server started at http://localhost:8082")
	if err := http.ListenAndServe(":8082", nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func setHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	var kv KeyValue
	if err := json.NewDecoder(r.Body).Decode(&kv); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// In Pebble, we directly set the key-value pair
	err := db.Set([]byte(kv.Key), []byte(kv.Value), pebble.Sync)
	if err != nil {
		http.Error(w, "Failed to store value: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Saved successfully"))
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	// In Pebble, we use Get to retrieve the value
	value, closer, err := db.Get([]byte(key))
	if err != nil {
		if err == pebble.ErrNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error retrieving value: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	// We need to copy the value since closer will invalidate it
	valueCopy := append([]byte{}, value...)
	closer.Close()

	json.NewEncoder(w).Encode(KeyValue{
		Key:   key,
		Value: string(valueCopy),
	})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Only DELETE method allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing 'key' parameter", http.StatusBadRequest)
		return
	}

	// Delete the key
	err := db.Delete([]byte(key), pebble.Sync)
	if err != nil {
		http.Error(w, "Error deleting key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deleted successfully"))
}

func scanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method allowed", http.StatusMethodNotAllowed)
		return
	}

	prefix := r.URL.Query().Get("prefix")
	// If no prefix is provided, we'll scan all keys
	
	// Create an iterator
	iter, _ := db.NewIter(nil)
	defer iter.Close()

	results := make(map[string]string)
	
	// If prefix is provided, seek to that prefix
	if prefix != "" {
		iter.SeekGE([]byte(prefix))
	} else {
		iter.First()
	}

	// Iterate and collect results
	for ; iter.Valid(); iter.Next() {
		key := string(iter.Key())
		
		// If we have a prefix and the current key doesn't start with it, break
		if prefix != "" && len(key) >= len(prefix) && key[:len(prefix)] != prefix {
			break
		}
		
		value := append([]byte{}, iter.Value()...)
		results[key] = string(value)
	}

	if err := iter.Error(); err != nil {
		http.Error(w, "Error scanning keys: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(results)
}

func initData() {
	// Sample stock data similar to your Badger example
	stockData := `{
  "code": "KR7005930003",
  "shortCode": "A005930",
  "baseDate": "20250429",
  "market": "S",
  "base": 69200,
  "prevClose": 69200,
  "open": 49000,
  "high": 49000,
  "low": 49000,
  "close": 49000,
  "volume": 15235,
  "amount": 746515000.0
}`

	// Store the sample data
	err := db.Set([]byte("samsung"), []byte(stockData), pebble.Sync)
	if err != nil {
		log.Printf("Failed to initialize data: %v", err)
	} else {
		log.Println("Initialized sample data with key 'samsung'")
	}
}