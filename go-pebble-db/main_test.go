package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

// 다양한 고루틴 수로 10,000개의 키-값 쌍을 삽입하는 벤치마크 테스트
func BenchmarkPebbleDBInsert(b *testing.B) {
	// 임시 디렉토리 생성
	tempDir, err := os.MkdirTemp("", "pebble-benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Pebble DB 옵션 최적화 (일관성 유지)
	opts := &pebble.Options{
		// 메모리 캐시 크기 증가 (기본값: 8MB)
		Cache: pebble.NewCache(64 * 1024 * 1024), // 64MB
		// WAL 디렉토리를 메인 DB와 같은 위치에 설정
		WALDir: tempDir,
		// 쓰기 버퍼 크기 증가
		MemTableSize: 64 * 1024 * 1024, // 64MB
		// 데이터 일관성을 위한 설정
		DisableWAL: false, // WAL 활성화 (기본값)
	}

	// Pebble DB 열기
	db, err := pebble.Open(tempDir, opts)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 테스트할 항목 수
	const numItems = 10000

	// 테스트할 고루틴 수 배열
	// CPU 코어 수의 배수로 설정
	cpuCores := runtime.NumCPU()
	workerCounts := []int{
		cpuCores,             // CPU 코어 수와 동일
		cpuCores * 2,         // CPU 코어 수의 2배
		cpuCores * 4,         // CPU 코어 수의 4배
		cpuCores * 8,         // CPU 코어 수의 8배
	}

	// 벤치마크 실행
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, numWorkers := range workerCounts {
			// 각 고루틴 수마다 별도의 벤치마크 실행
			b.Run(fmt.Sprintf("Workers_%d", numWorkers), func(b *testing.B) {
				// 이전 테스트 데이터 정리
				clearDatabase(db)
				
				// 벤치마크 시작
				runInsertBenchmarkWithWorkers(b, db, numItems, numWorkers)
			})
		}
	}
}

// 데이터베이스 내용 정리
func clearDatabase(db *pebble.DB) {
	// 새로운 이터레이터 생성
	iter, _ := db.NewIter(nil)
	defer iter.Close()

	// 모든 키 수집
	var keys [][]byte
	for iter.First(); iter.Valid(); iter.Next() {
		key := append([]byte{}, iter.Key()...)
		keys = append(keys, key)
	}

	// 수집된 모든 키 삭제
	for _, key := range keys {
		_ = db.Delete(key, pebble.Sync)
	}
}

// 지정된 고루틴 수로 벤치마크 실행
func runInsertBenchmarkWithWorkers(b *testing.B, db *pebble.DB, numItems, numWorkers int) {
	// 시작 시간 기록
	startTime := time.Now()

	// 항목 개별 삽입 실행
	insertItemsIndividually(b, db, numItems, numWorkers)
	
	// 경과 시간 계산 및 출력
	elapsed := time.Since(startTime)
	opsPerSec := float64(numItems) / elapsed.Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")
	b.Logf("고루틴 %d개로 %d개 항목 삽입 완료, 소요 시간: %v, 초당 %v 항목", 
		numWorkers, numItems, elapsed, opsPerSec)
}

// 개별 항목 삽입 함수 (각 항목마다 완전한 일관성 보장)
func insertItemsIndividually(b *testing.B, db *pebble.DB, numItems, numWorkers int) {
	var wg sync.WaitGroup
	
	// 워커당 처리할 항목 수 계산
	itemsPerWorker := numItems / numWorkers
	// 나머지 항목 계산
	remainingItems := numItems % numWorkers

	// 동기화 옵션 - 모든 항목이 디스크에 안전하게 기록되도록 함
	syncOption := pebble.Sync

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// 시작 인덱스 계산
			start := workerID * itemsPerWorker
			// 마지막 워커에게 나머지 항목 할당
			extraItems := 0
			if workerID == numWorkers-1 {
				extraItems = remainingItems
			}
			// 종료 인덱스 계산
			end := start + itemsPerWorker + extraItems
			
			// 각 항목을 개별적으로 처리
			for i := start; i < end; i++ {
				key := []byte(fmt.Sprintf("key-%d", i))
				value := []byte(fmt.Sprintf("value-%d", i))
				
				// 각 항목을 개별적으로 디스크에 동기적으로 기록
				if err := db.Set(key, value, syncOption); err != nil {
					b.Errorf("항목 삽입 오류 (키: %s): %v", key, err)
				}
			}
		}(w)
	}
	
	wg.Wait()
	b.Logf("개별 삽입 완료: %d개 항목 (고루틴 %d개 사용)", numItems, numWorkers)
}
