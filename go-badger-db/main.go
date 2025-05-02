package main

import (
	"fmt"
	"log"

	badger "github.com/dgraph-io/badger/v4"
)

// openInMemoryDB opens and returns an in-memory BadgerDB instance
func openInMemoryDB() (*badger.DB, error) {
	// In-memory BadgerDB 옵션 설정
	options := badger.DefaultOptions("").WithInMemory(true)
	options.Logger = nil // 로깅 비활성화

	// 데이터베이스 열기
	db, err := badger.Open(options)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// writeData writes key-value pairs to the database
func writeData(db *badger.DB, data map[string]string) error {
	return db.Update(func(txn *badger.Txn) error {
		for key, value := range data {
			if err := txn.Set([]byte(key), []byte(value)); err != nil {
				return err
			}
		}
		return nil
	})
}

// readData reads and displays data for the specified keys
func readData(db *badger.DB, keys []string) error {
	return db.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get([]byte(key))
			if err != nil {
				return err
			}
			
			if err := item.Value(func(val []byte) error {
				fmt.Printf("%s: %s\n", key, val)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func main() {
	// In-memory BadgerDB 열기
	db, err := openInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 데이터 쓰기
	data := map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
	}
	if err := writeData(db, data); err != nil {
		log.Fatal(err)
	}

	// 데이터 읽기
	keys := []string{"name", "email"}
	if err := readData(db, keys); err != nil {
		log.Fatal(err)
	}
}