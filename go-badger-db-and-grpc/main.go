package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/dgraph-io/badger/v4"

	"context"

	"net"
	pb "github.com/yiminan/go-examples/go-badger-db-and-grpc/proto/generated"

	"google.golang.org/grpc"
)

var db *badger.DB

type stockServer struct {
	pb.UnimplementedStockServiceServer
}

func (s *stockServer) GetStockMaster(ctx context.Context, req *pb.StockRequest) (*pb.StockMaster, error) {
	log.Printf("Received request for key: %s", req.Key)

	// BadgerDB에서 데이터 조회
	var valCopy []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(req.Key))
		if err != nil {
			return err
		}
		valCopy, err = item.ValueCopy(nil)
		return err
	})

	if err != nil {
		// 키를 찾을 수 없는 경우 기본 응답 반환
		return &pb.StockMaster{Value: fmt.Sprintf("Stock Info for key: %s (not found in DB)", req.Key)}, nil
	}

	// 비즈니스 로직에 따라 응답
	return &pb.StockMaster{Value: string(valCopy)}, nil
}

func main() {
	// BadgerDB를 in-memory로 오픈
	opts := badger.DefaultOptions("").WithInMemory(true)
	var err error
	db, err = badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// HTTP 서버 설정
	http.HandleFunc("/set", setHandler)
	http.HandleFunc("/get", getHandler)

	// HTTP 서버를 goroutine으로 실행
	go func() {
		fmt.Println("🚀 HTTP Server started at http://localhost:8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// gRPC 서버 설정
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterStockServiceServer(grpcServer, &stockServer{})

	fmt.Println("🚀 gRPC Server is running on port :50051")
	
	// gRPC 서버 실행 (메인 스레드에서 실행)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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

	err := db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(kv.Key), []byte(kv.Value))
	})

	if err != nil {
		http.Error(w, "Failed to store value", http.StatusInternalServerError)
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

	var valCopy []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		valCopy, err = item.ValueCopy(nil)
		return err
	})

	if err != nil {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(KeyValue{
		Key:   key,
		Value: string(valCopy),
	})
}
