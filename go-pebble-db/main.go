package main

import (
	"log"

	"github.com/cockroachdb/pebble"
)

func main() {
	dbPath := "./pebble-data"

	// default options 로 db open
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		// 초기화 단계에서 복구할 수 없는 오류는 log.Fatal을,
		// 런타임 중 발생할 수 있는 예외적 상황은 panic을 사용하는 것이 좋습니다.
		log.Fatal(err)

		// panic은 프로그램 실행 중에 복구할 수 없는 오류가 발생했음을 나타내는 메커니즘입니다.
		// 일반적으로 정상적인 에러 처리(error)로 해결할 수 없는 치명적인 상황에서 사용됩니다.
		// panic(err)
	}
	// main 함수 종료 시 db를 닫음
	defer db.Close()

	// db.Set(key, value, pebble.Sync)
	// 	•	이 옵션은 WAL 기록 후, fsync() 또는 fdatasync() 호출을 통해 디스크에 실제로 flush합니다.
	// 	•	즉, 전원이 꺼져도 해당 write는 보존됨.
	// 	•	Write to WAL → Flush to disk (fsync) → Success return
	//  •	중요한 금융/거래 로그 → pebble.Sync 사용 필수.
	// ✅ 장점
	// 	•	Crash Safety 보장: 시스템 장애나 충돌이 나도 기록 보존.
	// 	•	WAL이 SSD에 안전하게 기록됨.
	// ❌ 단점
	// 	•	성능 비용 높음: 매번 syscall (fsync)로 디스크 flush.
	// 	•	많은 TPS 상황에서는 latency 증가.
	key1 := []byte("key1")
	value1 := []byte("value1")
	if err := db.Set(key1, value1, pebble.Sync); err != nil {
		log.Fatal(err)
	}

	findValue1, err := getValue(db, key1)
	if err != nil {
		if err == pebble.ErrNotFound {
			log.Printf("Key(%s) not found", key1)
		} else {
			log.Fatal(err)
		}
	} else {
		log.Printf("Value: %s", findValue1)
	}

	// db.Set(key, value, pebble.NoSync)
	// 	•	WAL에는 기록되지만, 디스크로 flush는 지연되거나 skip됨.
	// 	•	OS의 page cache에만 머물 수 있음.
	// 	•	향후 flush나 compaction 과정에서 디스크로 반영됨.
	//  •	캐시, 로그 buffer, 메트릭 수집 → pebble.NoSync로 성능 최적화 가능.
	// ✅ 장점
	// 	•	쓰기 속도 빠름: fsync를 하지 않아 syscall 비용 없음.
	// 	•	높은 write TPS를 감당할 수 있음.
	// ❌ 단점
	// 	•	전원 손실/충돌 시 데이터 유실 가능 (Crash Safety X)
	// 	•	OS 버퍼에만 있던 WAL 레코드는 날아감.
	// 	•	따라서 durability는 전혀 보장되지 않음.
	key2 := []byte("key2")
	value2 := []byte("value2")
	if err := db.Set(key2, value2, pebble.NoSync); err != nil {
		log.Fatal(err)
	}

	findValue2, err := getValue(db, key2)
	if err != nil {
		if err == pebble.ErrNotFound {
			log.Printf("Key(%s) not found", key2)
		} else {
			log.Printf("Error: %v", err)
		}
	} else {
		log.Printf("Value: %s", findValue2)
	}

}

func getValue(db *pebble.DB, key []byte) (string, error) {
	value, closer, err := db.Get(key)
	if err != nil {
		return "", err // 에러를 호출자에게 전달
	}

	// 값 복사 (closer.Close() 이후에 value가 무효화될 수 있음)
	valueCopy := string(value)

	// 반드시 리소스 해제
	closer.Close()

	return valueCopy, nil
}
