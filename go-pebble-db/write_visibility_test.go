package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

// WriteVisibilityTest는 쓰기 작업이 읽기 작업에 반영되는 시간을 측정합니다
type WriteVisibilityTest struct {
	db            *pebble.DB
	tempDir       string
	syncWrites    bool
	keyPrefix     string
	valueSize     int
	writeCount    int
	readAttempts  int
	readInterval  time.Duration
	stats         struct {
		writeOps      uint64
		readOps       uint64
		readMisses    uint64
		readLatencies []time.Duration
	}
}

// 새로운 쓰기 가시성 테스트 인스턴스를 생성합니다
func NewWriteVisibilityTest(syncWrites bool, writeCount, readAttempts int, readInterval time.Duration) (*WriteVisibilityTest, error) {
	// 임시 디렉토리 생성
	tempDir, err := os.MkdirTemp("", "pebble-write-visibility-test")
	if err != nil {
		return nil, fmt.Errorf("임시 디렉토리 생성 실패: %w", err)
	}

	// Pebble DB 옵션 최적화
	opts := &pebble.Options{
		// 메모리 캐시 크기 증가 (기본값: 8MB)
		Cache: pebble.NewCache(64 * 1024 * 1024), // 64MB
		// WAL 디렉토리를 메인 DB와 같은 위치에 설정
		WALDir: tempDir,
		// 쓰기 버퍼 크기 증가
		MemTableSize: 64 * 1024 * 1024, // 64MB
	}

	// Pebble DB 열기
	db, err := pebble.Open(tempDir, opts)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("Pebble DB 열기 실패: %w", err)
	}

	return &WriteVisibilityTest{
		db:            db,
		tempDir:       tempDir,
		syncWrites:    syncWrites,
		keyPrefix:     "test-key-",
		valueSize:     100,
		writeCount:    writeCount,
		readAttempts:  readAttempts,
		readInterval:  readInterval,
		stats: struct {
			writeOps      uint64
			readOps       uint64
			readMisses    uint64
			readLatencies []time.Duration
		}{
			readLatencies: make([]time.Duration, writeCount),
		},
	}, nil
}

// 테스트 정리
func (t *WriteVisibilityTest) Cleanup() {
	if t.db != nil {
		t.db.Close()
	}
	if t.tempDir != "" {
		os.RemoveAll(t.tempDir)
	}
}

// 키와 값 생성 함수
func (t *WriteVisibilityTest) generateKeyValue(idx int) ([]byte, []byte) {
	key := []byte(fmt.Sprintf("%s%d", t.keyPrefix, idx))
	value := make([]byte, t.valueSize)
	
	// 값 생성 (현재 시간을 포함)
	valueStr := fmt.Sprintf("v-%d-%d", idx, time.Now().UnixNano())
	copy(value, valueStr)
	
	return key, value
}

// 쓰기 작업 수행
func (t *WriteVisibilityTest) writeKey(idx int, writeTime *time.Time) error {
	key, value := t.generateKeyValue(idx)
	
	var writeOpt *pebble.WriteOptions
	if t.syncWrites {
		writeOpt = pebble.Sync // 동기 쓰기
	} else {
		writeOpt = pebble.NoSync // 비동기 쓰기
	}
	
	// 쓰기 시간 기록
	*writeTime = time.Now()
	
	// 쓰기 작업 수행
	err := t.db.Set(key, value, writeOpt)
	if err == nil {
		atomic.AddUint64(&t.stats.writeOps, 1)
	}
	return err
}

// 읽기 작업 수행 및 가시성 확인
func (t *WriteVisibilityTest) checkReadVisibility(idx int, writeTime time.Time, wg *sync.WaitGroup) {
	defer wg.Done()
	
	key := []byte(fmt.Sprintf("%s%d", t.keyPrefix, idx))
	
	// 쓰기가 읽기에 반영될 때까지 반복 시도
	var readLatency time.Duration
	found := false
	
	for i := 0; i < t.readAttempts && !found; i++ {
		// 읽기 시도
		_, closer, err := t.db.Get(key)
		if err == nil {
			// 키를 찾음 - 가시성 확인됨
			readLatency = time.Since(writeTime)
			closer.Close()
			found = true
			atomic.AddUint64(&t.stats.readOps, 1)
			break
		} else if err != pebble.ErrNotFound {
			// 예상치 못한 오류
			fmt.Printf("읽기 오류 (키: %s): %v\n", key, err)
		}
		
		// 키를 찾지 못함 - 잠시 대기 후 재시도
		time.Sleep(t.readInterval)
	}
	
	if found {
		// 읽기 지연 시간 기록
		t.stats.readLatencies[idx] = readLatency
	} else {
		// 최대 시도 후에도 키를 찾지 못함
		atomic.AddUint64(&t.stats.readMisses, 1)
		t.stats.readLatencies[idx] = -1 // 찾지 못함을 표시
	}
}

// 테스트 실행
func (t *WriteVisibilityTest) Run() error {
	var wg sync.WaitGroup
	writeTimes := make([]time.Time, t.writeCount)
	
	fmt.Printf("쓰기 가시성 테스트 시작 (키: %d개, 동기화: %v)\n", t.writeCount, t.syncWrites)
	
	// 각 키에 대해 쓰기 작업 수행 후 즉시 읽기 작업 시작
	for i := 0; i < t.writeCount; i++ {
		// 쓰기 작업 수행
		if err := t.writeKey(i, &writeTimes[i]); err != nil {
			return fmt.Errorf("쓰기 오류 (키: %d): %w", i, err)
		}
		
		// 읽기 작업 시작 (별도 고루틴)
		wg.Add(1)
		go t.checkReadVisibility(i, writeTimes[i], &wg)
		
		// 다음 쓰기 작업 전에 짧은 대기 (부하 분산)
		if i < t.writeCount-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	// 모든 읽기 작업이 완료될 때까지 대기
	wg.Wait()
	
	return nil
}

// 결과 분석 및 출력
func (t *WriteVisibilityTest) AnalyzeResults() {
	// 지연 시간 통계 계산
	var totalLatency time.Duration
	var minLatency time.Duration = -1
	var maxLatency time.Duration
	successCount := 0
	
	for _, latency := range t.stats.readLatencies {
		if latency >= 0 {
			successCount++
			totalLatency += latency
			
			if minLatency == -1 || latency < minLatency {
				minLatency = latency
			}
			
			if latency > maxLatency {
				maxLatency = latency
			}
		}
	}
	
	// 결과 출력
	fmt.Printf("\n===== Pebble DB 쓰기 가시성 테스트 결과 =====\n")
	fmt.Printf("총 쓰기 작업: %d\n", t.stats.writeOps)
	fmt.Printf("읽기 성공: %d (%.1f%%)\n", successCount, float64(successCount)/float64(t.writeCount)*100)
	fmt.Printf("읽기 실패: %d (%.1f%%)\n", t.stats.readMisses, float64(t.stats.readMisses)/float64(t.writeCount)*100)
	
	if successCount > 0 {
		avgLatency := totalLatency / time.Duration(successCount)
		fmt.Printf("평균 가시성 지연 시간: %v\n", avgLatency)
		fmt.Printf("최소 가시성 지연 시간: %v\n", minLatency)
		fmt.Printf("최대 가시성 지연 시간: %v\n", maxLatency)
		
		// 지연 시간 분포 계산
		latencyBuckets := map[string]int{
			"0-1μs":    0,
			"1-10μs":   0,
			"10-100μs": 0,
			"100μs-1ms": 0,
			"1-10ms":   0,
			"10-100ms": 0,
			"100ms+":   0,
		}
		
		for _, latency := range t.stats.readLatencies {
			if latency < 0 {
				continue
			}
			
			switch {
			case latency < 1*time.Microsecond:
				latencyBuckets["0-1μs"]++
			case latency < 10*time.Microsecond:
				latencyBuckets["1-10μs"]++
			case latency < 100*time.Microsecond:
				latencyBuckets["10-100μs"]++
			case latency < 1*time.Millisecond:
				latencyBuckets["100μs-1ms"]++
			case latency < 10*time.Millisecond:
				latencyBuckets["1-10ms"]++
			case latency < 100*time.Millisecond:
				latencyBuckets["10-100ms"]++
			default:
				latencyBuckets["100ms+"]++
			}
		}
		
		fmt.Println("\n지연 시간 분포:")
		for bucket, count := range latencyBuckets {
			if count > 0 {
				fmt.Printf("  %s: %d (%.1f%%)\n", bucket, count, float64(count)/float64(successCount)*100)
			}
		}
	}
	
	fmt.Printf("=====================================\n")
}

// 테스트 함수 - 비동기 쓰기의 가시성 지연 시간 측정
func TestPebbleWriteVisibility(t *testing.T) {
	// 테스트 설정
	testConfigs := []struct {
		name         string
		syncWrites   bool
		writeCount   int
		readAttempts int
		readInterval time.Duration
	}{
		{"비동기 쓰기 가시성", false, 1000, 1000, 10 * time.Microsecond},
		{"동기 쓰기 가시성", true, 1000, 1000, 10 * time.Microsecond},
	}
	
	// 각 설정으로 테스트 실행
	for _, cfg := range testConfigs {
		t.Run(cfg.name, func(t *testing.T) {
			fmt.Printf("\n=== 테스트 시작: %s ===\n", cfg.name)
			
			// 테스트 인스턴스 생성
			test, err := NewWriteVisibilityTest(
				cfg.syncWrites, cfg.writeCount, cfg.readAttempts, cfg.readInterval,
			)
			if err != nil {
				t.Fatalf("테스트 초기화 실패: %v", err)
			}
			defer test.Cleanup()
			
			// 테스트 실행
			if err := test.Run(); err != nil {
				t.Fatalf("테스트 실행 실패: %v", err)
			}
			
			// 결과 분석
			test.AnalyzeResults()
		})
	}
}
