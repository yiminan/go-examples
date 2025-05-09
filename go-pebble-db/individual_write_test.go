package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

// IndividualWriteTest 구조체는 개별 쓰기 작업에 대한 성능 테스트를 관리합니다
type IndividualWriteTest struct {
	db            *pebble.DB
	tempDir       string
	numOperations int
	numWorkers    int
	keySize       int
	valueSize     int
	syncWrites    bool
	stats         struct {
		writeOps uint64
		readOps  uint64
		errors   uint64
	}
}

// 새로운 개별 쓰기 테스트 인스턴스를 생성합니다
func NewIndividualWriteTest(numOps, workers, keySize, valueSize int, syncWrites bool) (*IndividualWriteTest, error) {
	// 임시 디렉토리 생성
	tempDir, err := os.MkdirTemp("", "pebble-individual-write-test")
	if err != nil {
		return nil, fmt.Errorf("임시 디렉토리 생성 실패: %w", err)
	}

	// Pebble DB 옵션 최적화
	opts := &pebble.Options{
		// 메모리 캐시 크기 증가 (기본값: 8MB)
		Cache: pebble.NewCache(256 * 1024 * 1024), // 256MB
		// WAL 디렉토리를 메인 DB와 같은 위치에 설정
		WALDir: tempDir,
		// 쓰기 버퍼 크기 증가
		MemTableSize: 128 * 1024 * 1024, // 128MB
		// 병렬 압축 작업 수 설정
		MaxConcurrentCompactions: func() int { return runtime.NumCPU() },
	}

	// Pebble DB 열기
	db, err := pebble.Open(tempDir, opts)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("Pebble DB 열기 실패: %w", err)
	}

	return &IndividualWriteTest{
		db:            db,
		tempDir:       tempDir,
		numOperations: numOps,
		numWorkers:    workers,
		keySize:       keySize,
		valueSize:     valueSize,
		syncWrites:    syncWrites,
	}, nil
}

// 테스트 정리
func (t *IndividualWriteTest) Cleanup() {
	if t.db != nil {
		t.db.Close()
	}
	if t.tempDir != "" {
		os.RemoveAll(t.tempDir)
	}
}

// 키와 값 생성 함수
func (t *IndividualWriteTest) generateKeyValue(idx int) ([]byte, []byte) {
	key := make([]byte, t.keySize)
	value := make([]byte, t.valueSize)

	// 키 생성 (고정 길이)
	keyStr := fmt.Sprintf("%0*d", t.keySize, idx)
	copy(key, keyStr)

	// 값 생성 (고정 길이)
	valueStr := fmt.Sprintf("v-%0*d", t.valueSize-2, idx)
	copy(value, valueStr)

	return key, value
}

// 개별 쓰기 작업 수행
func (t *IndividualWriteTest) writeOperation(idx int) error {
	key, value := t.generateKeyValue(idx)
	
	var writeOpt *pebble.WriteOptions
	if t.syncWrites {
		writeOpt = pebble.Sync // 동기 쓰기
	} else {
		writeOpt = pebble.NoSync // 비동기 쓰기
	}
	
	// 개별 쓰기 작업 수행 (각 쓰기마다 commit)
	err := t.db.Set(key, value, writeOpt)
	if err == nil {
		atomic.AddUint64(&t.stats.writeOps, 1)
	} else {
		atomic.AddUint64(&t.stats.errors, 1)
	}
	return err
}

// 읽기 작업 수행
func (t *IndividualWriteTest) readOperation(idx int) error {
	key, _ := t.generateKeyValue(idx)
	
	value, closer, err := t.db.Get(key)
	if err == nil {
		// 값 사용 (실제로는 아무것도 하지 않음)
		_ = value
		closer.Close()
		atomic.AddUint64(&t.stats.readOps, 1)
	} else if err != pebble.ErrNotFound {
		// NotFound는 에러로 간주하지 않음 (아직 쓰여지지 않은 키일 수 있음)
		atomic.AddUint64(&t.stats.errors, 1)
	}
	return err
}

// 데이터 미리 로드 (읽기 테스트를 위해)
func (t *IndividualWriteTest) preloadData(count int) error {
	fmt.Printf("데이터 미리 로드 중... (%d 항목)\n", count)
	
	var writeOpt *pebble.WriteOptions
	if t.syncWrites {
		writeOpt = pebble.Sync
	} else {
		writeOpt = pebble.NoSync
	}
	
	for i := 0; i < count; i++ {
		key, value := t.generateKeyValue(i)
		if err := t.db.Set(key, value, writeOpt); err != nil {
			return err
		}
	}
	
	fmt.Printf("데이터 로드 완료: %d 항목\n", count)
	return nil
}

// 쓰기 전용 워커
func (t *IndividualWriteTest) writeWorker(workerID int, wg *sync.WaitGroup, opsPerWorker int) {
	defer wg.Done()

	startIdx := workerID * opsPerWorker
	endIdx := startIdx + opsPerWorker

	for i := startIdx; i < endIdx; i++ {
		t.writeOperation(i)
	}
}

// 읽기 전용 워커
func (t *IndividualWriteTest) readWorker(workerID int, wg *sync.WaitGroup, opsPerWorker int) {
	defer wg.Done()

	startIdx := workerID * opsPerWorker
	endIdx := startIdx + opsPerWorker

	for i := startIdx; i < endIdx; i++ {
		t.readOperation(i)
	}
}

// 읽기/쓰기 혼합 워커
func (t *IndividualWriteTest) mixedWorker(workerID int, wg *sync.WaitGroup, opsPerWorker int, readRatio float64) {
	defer wg.Done()

	startIdx := workerID * opsPerWorker
	endIdx := startIdx + opsPerWorker

	// 난수 생성기 초기화 (각 워커마다 다른 시드)
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

	for i := startIdx; i < endIdx; i++ {
		// 읽기 비율에 따라 읽기 또는 쓰기 결정
		if r.Float64() < readRatio {
			// 읽기 작업 수행
			t.readOperation(i % (startIdx + 1)) // 이미 쓰여진 키만 읽기 위해 인덱스 조정
		} else {
			// 쓰기 작업 수행
			t.writeOperation(i)
		}
	}
}

// 쓰기 전용 테스트 실행
func (t *IndividualWriteTest) RunWriteTest() time.Duration {
	var wg sync.WaitGroup
	opsPerWorker := t.numOperations / t.numWorkers
	
	// 시작 시간 기록
	startTime := time.Now()

	// 워커 고루틴 시작
	for w := 0; w < t.numWorkers; w++ {
		wg.Add(1)
		go t.writeWorker(w, &wg, opsPerWorker)
	}

	// 모든 워커가 완료될 때까지 대기
	wg.Wait()
	
	// 경과 시간 계산
	return time.Since(startTime)
}

// 읽기 전용 테스트 실행
func (t *IndividualWriteTest) RunReadTest() time.Duration {
	var wg sync.WaitGroup
	opsPerWorker := t.numOperations / t.numWorkers
	
	// 시작 시간 기록
	startTime := time.Now()

	// 워커 고루틴 시작
	for w := 0; w < t.numWorkers; w++ {
		wg.Add(1)
		go t.readWorker(w, &wg, opsPerWorker)
	}

	// 모든 워커가 완료될 때까지 대기
	wg.Wait()
	
	// 경과 시간 계산
	return time.Since(startTime)
}

// 읽기/쓰기 혼합 테스트 실행
func (t *IndividualWriteTest) RunMixedTest(readRatio float64) time.Duration {
	var wg sync.WaitGroup
	opsPerWorker := t.numOperations / t.numWorkers
	
	// 시작 시간 기록
	startTime := time.Now()

	// 워커 고루틴 시작
	for w := 0; w < t.numWorkers; w++ {
		wg.Add(1)
		go t.mixedWorker(w, &wg, opsPerWorker, readRatio)
	}

	// 모든 워커가 완료될 때까지 대기
	wg.Wait()
	
	// 경과 시간 계산
	return time.Since(startTime)
}

// 결과 출력
func (t *IndividualWriteTest) PrintResults(testName string, elapsed time.Duration) {
	totalOps := t.stats.readOps + t.stats.writeOps
	opsPerSec := float64(totalOps) / elapsed.Seconds()
	
	fmt.Printf("\n===== Pebble DB 개별 쓰기 테스트 결과: %s =====\n", testName)
	fmt.Printf("총 작업 수: %d (읽기: %d, 쓰기: %d)\n", totalOps, t.stats.readOps, t.stats.writeOps)
	fmt.Printf("고루틴 수: %d\n", t.numWorkers)
	fmt.Printf("키 크기: %d bytes, 값 크기: %d bytes\n", t.keySize, t.valueSize)
	fmt.Printf("동기 쓰기: %v\n", t.syncWrites)
	fmt.Printf("소요 시간: %v\n", elapsed)
	fmt.Printf("초당 작업 수: %.2f ops/sec\n", opsPerSec)
	fmt.Printf("에러 수: %d\n", t.stats.errors)
	fmt.Printf("=====================================\n")
}

// 테스트 함수 - 다양한 설정으로 개별 쓰기 성능 테스트 실행
func TestPebbleIndividualWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// CPU 코어 수 확인
	cpuCores := runtime.NumCPU()
	fmt.Printf("CPU 코어 수: %d\n", cpuCores)

	// 테스트 설정
	testConfigs := []struct {
		name       string
		ops        int
		workers    int
		keySize    int
		valueSize  int
		syncWrites bool
		testType   string
		readRatio  float64
	}{
		{"비동기 개별 쓰기", 1000000, cpuCores, 16, 100, false, "write", 0.0},
		{"동기 개별 쓰기", 100000, cpuCores, 16, 100, true, "write", 0.0},
		{"읽기 전용", 1000000, cpuCores, 16, 100, false, "read", 1.0},
		{"읽기/쓰기 혼합 (50:50)", 1000000, cpuCores, 16, 100, false, "mixed", 0.5},
		{"읽기/쓰기 혼합 (80:20)", 1000000, cpuCores, 16, 100, false, "mixed", 0.8},
		{"고루틴 확장 쓰기", 1000000, cpuCores * 4, 16, 100, false, "write", 0.0},
	}

	// 각 설정으로 테스트 실행
	for _, cfg := range testConfigs {
		t.Run(cfg.name, func(t *testing.T) {
			fmt.Printf("\n=== 테스트 시작: %s ===\n", cfg.name)
			
			// 테스트 인스턴스 생성
			test, err := NewIndividualWriteTest(
				cfg.ops, cfg.workers, cfg.keySize, cfg.valueSize, cfg.syncWrites,
			)
			if err != nil {
				t.Fatalf("테스트 초기화 실패: %v", err)
			}
			defer test.Cleanup()
			
			var elapsed time.Duration
			
			// 테스트 유형에 따라 실행
			switch cfg.testType {
			case "write":
				elapsed = test.RunWriteTest()
			case "read":
				// 읽기 테스트를 위해 데이터 미리 로드
				if err := test.preloadData(cfg.ops / 10); err != nil {
					t.Fatalf("데이터 미리 로드 실패: %v", err)
				}
				elapsed = test.RunReadTest()
			case "mixed":
				// 혼합 테스트를 위해 일부 데이터 미리 로드
				if err := test.preloadData(cfg.ops / 10); err != nil {
					t.Fatalf("데이터 미리 로드 실패: %v", err)
				}
				elapsed = test.RunMixedTest(cfg.readRatio)
			}
			
			// 결과 출력
			test.PrintResults(cfg.name, elapsed)
		})
	}
}
