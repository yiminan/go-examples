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

	// 테스트 데이터 저장
	initData()

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

func initData() {
	stockData := `{
  "code": "KR7005930003",
  "shortCode": "A005930",
  "baseDate": "20250429",
  "boardId": "G4",
  "sessionId": "99",
  "market": "S",
  "base": 69200,
  "prevClose": 69200,
  "prevVolume": 234222,
  "open": 49000,
  "high": 49000,
  "low": 49000,
  "close": 49000,
  "changeType": "5",
  "stockGroupId": "ST",
  "volume": {
    "G1": 15235
  },
  "amount": {
    "G1": 746515000.0
  },
  "openTime": "11:32:13.678067",
  "highTime": "11:32:13.678067",
  "lowTime": "11:32:13.678067",
  "tradeTime": "16:00:30.04466",
  "upperLimitPrice": 89900,
  "lowerLimitPrice": 48500,
  "afterSingleOpen": 0,
  "afterSingleHigh": 0,
  "afterSingleLow": 0,
  "afterSingleClose": 49000,
  "afterSingleChangeType": "3",
  "afterSingleUpperLimitPrice": 53900,
  "afterSingleLowerLimitPrice": 48500,
  "totalAccumQuantity": 15235,
  "totalAccumAmount": 746515000.0,
  "limitPrice": {
    "G2": {
      "dt": "2025-04-29T11:10:00.04914",
      "sellVolumeTotal": 0,
      "buyVolumeTotal": 1,
      "sellVolumeTotalChange": 0,
      "buyVolumeTotalChange": 0,
      "sellPrice": null,
      "sellVolume": null,
      "sellVolumeChange": null,
      "sellVolumeLP": null,
      "buyPrice": null,
      "buyVolume": null,
      "buyVolumeChange": null,
      "buyVolumeLP": null,
      "midPrice": null,
      "midPriceOfferVolumeTotal": 0,
      "midPriceBidVolumeTotal": 0,
      "midPriceOfferVolumeTotalChange": 0,
      "midPriceBidVolumeTotalChange": 0,
      "estimatedPrice": 0,
      "estimatedVolume": 0,
      "sessionId": null,
      "tradingType": null
    },
    "G1": {
      "dt": "2025-04-29T16:00:30.044759",
      "sellVolumeTotal": 3572,
      "buyVolumeTotal": 0,
      "sellVolumeTotalChange": 0,
      "buyVolumeTotalChange": 0,
      "sellPrice": [
        52700,
        52800,
        53000,
        55100,
        55600,
        55800,
        56200,
        56400,
        56500,
        61300
      ],
      "sellVolume": [
        3393,
        5,
        5,
        3,
        3,
        3,
        1,
        150,
        4,
        5
      ],
      "sellVolumeChange": [
        0,
        0,
        0,
        3,
        3,
        3,
        1,
        150,
        4,
        5
      ],
      "sellVolumeLP": null,
      "buyPrice": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "buyVolume": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "buyVolumeChange": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "buyVolumeLP": null,
      "midPrice": 0,
      "midPriceOfferVolumeTotal": 0,
      "midPriceBidVolumeTotal": 0,
      "midPriceOfferVolumeTotalChange": 0,
      "midPriceBidVolumeTotalChange": 0,
      "estimatedPrice": 49000,
      "estimatedVolume": 2,
      "sessionId": "99",
      "tradingType": null
    },
    "I2": {
      "dt": "2025-04-29T11:30:01.346523",
      "sellVolumeTotal": 0,
      "buyVolumeTotal": 0,
      "sellVolumeTotalChange": 0,
      "buyVolumeTotalChange": 0,
      "sellPrice": null,
      "sellVolume": null,
      "sellVolumeChange": null,
      "sellVolumeLP": null,
      "buyPrice": null,
      "buyVolume": null,
      "buyVolumeChange": null,
      "buyVolumeLP": null,
      "midPrice": null,
      "midPriceOfferVolumeTotal": 0,
      "midPriceBidVolumeTotal": 0,
      "midPriceOfferVolumeTotalChange": 0,
      "midPriceBidVolumeTotalChange": 0,
      "estimatedPrice": 0,
      "estimatedVolume": 0,
      "sessionId": null,
      "tradingType": "unknown"
    },
    "I1": {
      "dt": "2025-04-29T15:30:00.436976",
      "sellVolumeTotal": 0,
      "buyVolumeTotal": 0,
      "sellVolumeTotalChange": 0,
      "buyVolumeTotalChange": 0,
      "sellPrice": null,
      "sellVolume": null,
      "sellVolumeChange": null,
      "sellVolumeLP": null,
      "buyPrice": null,
      "buyVolume": null,
      "buyVolumeChange": null,
      "buyVolumeLP": null,
      "midPrice": null,
      "midPriceOfferVolumeTotal": 0,
      "midPriceBidVolumeTotal": 0,
      "midPriceOfferVolumeTotalChange": 0,
      "midPriceBidVolumeTotalChange": 0,
      "estimatedPrice": 0,
      "estimatedVolume": 0,
      "sessionId": null,
      "tradingType": "unknown"
    },
    "G3": {
      "dt": "2025-04-29T16:30:00.075473",
      "sellVolumeTotal": 0,
      "buyVolumeTotal": 0,
      "sellVolumeTotalChange": 0,
      "buyVolumeTotalChange": 0,
      "sellPrice": null,
      "sellVolume": null,
      "sellVolumeChange": null,
      "sellVolumeLP": null,
      "buyPrice": null,
      "buyVolume": null,
      "buyVolumeChange": null,
      "buyVolumeLP": null,
      "midPrice": null,
      "midPriceOfferVolumeTotal": 0,
      "midPriceBidVolumeTotal": 0,
      "midPriceOfferVolumeTotalChange": 0,
      "midPriceBidVolumeTotalChange": 0,
      "estimatedPrice": 0,
      "estimatedVolume": 0,
      "sessionId": null,
      "tradingType": null
    },
    "G4": {
      "dt": "2025-04-29T17:00:00.082989",
      "sellVolumeTotal": 0,
      "buyVolumeTotal": 0,
      "sellVolumeTotalChange": 0,
      "buyVolumeTotalChange": 0,
      "sellPrice": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "sellVolume": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "sellVolumeChange": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "sellVolumeLP": null,
      "buyPrice": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "buyVolume": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "buyVolumeChange": [
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
      ],
      "buyVolumeLP": null,
      "midPrice": 0,
      "midPriceOfferVolumeTotal": 0,
      "midPriceBidVolumeTotal": 0,
      "midPriceOfferVolumeTotalChange": 0,
      "midPriceBidVolumeTotalChange": 0,
      "estimatedPrice": 0,
      "estimatedVolume": 0,
      "sessionId": "99",
      "tradingType": null
    }
  },
  "volumeByTradingType": {},
  "listedShares": 5919637922,
  "tradingHalt": false,
  "unitTrade": false,
  "viApplyCode": "2",
  "viTriggerCount": 84,
  "viTriggerTime": "15:44:06.564064",
  "viClearTime": "15:46:12.563",
  "viKind": "2",
  "staticVITrgBasePrice": 49000,
  "dynamicVITrgBasePrice": 49000,
  "viTriggerPrice": 52700,
  "staticVITriggerPriceGapRate": 12.244898,
  "dynamicVITriggerPriceGapRate": 7.55102,
  "estimatedStaticViBasePrice": 49000,
  "estimatedStaticViUpperPrice": 53900,
  "estimatedStaticViLowerPrice": 0,
  "statusOfAllocation": "0"
}`

	db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("stock:20250428:KR7005930003"), []byte(stockData))
	})
}
