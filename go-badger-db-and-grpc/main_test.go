package main

import (
	"encoding/json"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

type StockInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func TestStockStorage(t *testing.T) {
	// Open BadgerDB in memory
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Test data
	key := "stock:20250428:KR7005930003"
	stockInfo := StockInfo{
		Code: "KR7005930003",
		Name: "삼성전자",
	}

	// Convert stock info to JSON
	value, err := json.Marshal(stockInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Store the data
	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve and verify the data
	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			var retrievedStock StockInfo
			if err := json.Unmarshal(val, &retrievedStock); err != nil {
				return err
			}

			// Print the retrieved data
			t.Logf("Retrieved Key: %s", key)
			t.Logf("Retrieved Value: %+v", retrievedStock)
			t.Logf("Retrieved JSON: %s", string(val))

			// Verify the retrieved data
			if retrievedStock.Code != stockInfo.Code {
				t.Errorf("Expected code %s, got %s", stockInfo.Code, retrievedStock.Code)
			}
			if retrievedStock.Name != stockInfo.Name {
				t.Errorf("Expected name %s, got %s", stockInfo.Name, retrievedStock.Name)
			}

			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}
