package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	badger "github.com/dgraph-io/badger/v4"
)

func main() {
	// 데이터베이스 디렉토리 생성
	dbPath := filepath.Join(".", "badger-data")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		log.Fatal(err)
	}

	// BadgerDB 옵션 설정
	options := badger.DefaultOptions(dbPath)
	options.Logger = nil // 로깅 비활성화

	// 데이터베이스 열기
	db, err := badger.Open(options)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 데이터 쓰기
	err = db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte("name"), []byte("John Doe"))
		if err != nil {
			return err
		}
		err = txn.Set([]byte("email"), []byte("john@example.com"))
		return err
	})
	if err != nil {
		log.Fatal(err)
	}

	// 데이터 읽기
	err = db.View(func(txn *badger.Txn) error {
		// name 키 읽기
		item, err := txn.Get([]byte("name"))
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			fmt.Printf("Name: %s\n", val)
			return nil
		})
		if err != nil {
			return err
		}

		// email 키 읽기
		item, err = txn.Get([]byte("email"))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			fmt.Printf("Email: %s\n", val)
			return nil
		})
	})
	if err != nil {
		log.Fatal(err)
	}
}