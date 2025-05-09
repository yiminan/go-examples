package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

// PebblePerformanceTest 구조체는 성능 테스트에 필요한 모든 설정과 상태를 관리합니다
type PebblePerformanceTest struct {
	db            *pebble.DB
	tempDir       string
	numOperations int
	numWorkers    int
	batchSize     int
	keySize       int
	valueSize     int
	readRatio     float64 // 0.0 = 전부 쓰기, 1.0 = 전부 읽기, 0.5 = 읽기/쓰기 50:50
	syncWrites    bool
	stats         struct {
		readOps  uint64
		writeOps uint64
		errors   uint64
	}
}

// 새로운 성능 테스트 인스턴스를 생성합니다
func NewPebblePerformanceTest(numOps, workers, batchSize, keySize, valueSize int, readRatio float64, syncWrites bool) (*PebblePerformanceTest, error) {
	// 임시 디렉토리 생성
	tempDir, err := os.MkdirTemp("", "pebble-perf-test")
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

	return &PebblePerformanceTest{
		db:            db,
		tempDir:       tempDir,
		numOperations: numOps,
		numWorkers:    workers,
		batchSize:     batchSize,
		keySize:       keySize,
		valueSize:     valueSize,
		readRatio:     readRatio,
		syncWrites:    syncWrites,
	}, nil
}

// 테스트 정리
func (p *PebblePerformanceTest) Cleanup() {
	if p.db != nil {
		p.db.Close()
	}
	if p.tempDir != "" {
		os.RemoveAll(p.tempDir)
	}
}

// 키와 값 생성 함수
func (p *PebblePerformanceTest) generateKeyValue(idx int) ([]byte, []byte) {
	key := make([]byte, p.keySize)
	value := make([]byte, p.valueSize)

	// 키 생성 (고정 길이)
	keyStr := fmt.Sprintf("%0*d", p.keySize, idx)
	copy(key, keyStr)

	// 값 생성 (고정 길이)
	valueStr := fmt.Sprintf("v-%0*d", p.valueSize-2, idx)
	copy(value, valueStr)

	return key, value
}

// 쓰기 작업 수행
func (p *PebblePerformanceTest) writeOperation(idx int) error {
	key, value := p.generateKeyValue(idx)
	
	var writeOpt *pebble.WriteOptions
	if p.syncWrites {
		writeOpt = pebble.Sync
	} else {
		writeOpt = pebble.NoSync
	}
	
	err := p.db.Set(key, value, writeOpt)
	if err == nil {
		atomic.AddUint64(&p.stats.writeOps, 1)
	} else {
		atomic.AddUint64(&p.stats.errors, 1)
	}
	return err
}

// 읽기 작업 수행
func (p *PebblePerformanceTest) readOperation(idx int) error {
	key, _ := p.generateKeyValue(idx)
	
	value, closer, err := p.db.Get(key)
	if err == nil {
		// 값 사용 (실제로는 아무것도 하지 않음)
		_ = value
		closer.Close()
		atomic.AddUint64(&p.stats.readOps, 1)
	} else if err != pebble.ErrNotFound {
		// NotFound는 에러로 간주하지 않음 (아직 쓰여지지 않은 키일 수 있음)
		atomic.AddUint64(&p.stats.errors, 1)
	}
	return err
}

// 배치 쓰기 작업 수행
func (p *PebblePerformanceTest) writeBatch(startIdx, endIdx int) error {
	batch := p.db.NewBatch()
	defer batch.Close()

	for i := startIdx; i < endIdx; i++ {
		key, value := p.generateKeyValue(i)
		err := batch.Set(key, value, nil)
		if err != nil {
			atomic.AddUint64(&p.stats.errors, 1)
			return err
		}
	}

	var writeOpt *pebble.WriteOptions
	if p.syncWrites {
		writeOpt = pebble.Sync
	} else {
		writeOpt = pebble.NoSync
	}

	err := batch.Commit(writeOpt)
	if err == nil {
		atomic.AddUint64(&p.stats.writeOps, uint64(endIdx-startIdx))
	} else {
		atomic.AddUint64(&p.stats.errors, 1)
	}
	return err
}

// 워커 함수 - 읽기/쓰기 작업 혼합 수행
func (p *PebblePerformanceTest) worker(workerID int, wg *sync.WaitGroup, opsPerWorker int) {
	defer wg.Done()

	startIdx := workerID * opsPerWorker
	endIdx := startIdx + opsPerWorker

	// 배치 크기가 1보다 크고 쓰기 작업이 포함된 경우에만 배치 처리
	if p.batchSize > 1 && p.readRatio < 1.0 {
		// 배치 쓰기 작업 수행
		for i := startIdx; i < endIdx; i += p.batchSize {
			batchEnd := i + p.batchSize
			if batchEnd > endIdx {
				batchEnd = endIdx
			}

			// 이 배치에서 읽기 작업을 수행할지 결정
			if p.readRatio > 0 {
				for j := i; j < batchEnd; j++ {
					// 읽기 비율에 따라 읽기 또는 쓰기 결정
					if rand := float64(j % 100) / 100.0; rand < p.readRatio {
						p.readOperation(j)
					} else {
						p.writeOperation(j)
					}
				}
			} else {
				// 읽기 작업이 없는 경우 배치 쓰기 수행
				p.writeBatch(i, batchEnd)
			}
		}
	} else {
		// 개별 작업 수행 (배치 없음)
		for i := startIdx; i < endIdx; i++ {
			// 읽기 비율에 따라 읽기 또는 쓰기 결정
			if rand := float64(i % 100) / 100.0; rand < p.readRatio {
				p.readOperation(i)
			} else {
				p.writeOperation(i)
			}
		}
	}
}

// 성능 테스트 실행
func (p *PebblePerformanceTest) Run() (time.Duration, error) {
	var wg sync.WaitGroup
	opsPerWorker := p.numOperations / p.numWorkers
	
	// 시작 시간 기록
	startTime := time.Now()

	// 워커 고루틴 시작
	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		go p.worker(w, &wg, opsPerWorker)
	}

	// 모든 워커가 완료될 때까지 대기
	wg.Wait()
	
	// 경과 시간 계산
	elapsed := time.Since(startTime)
	
	return elapsed, nil
}

// 결과 출력
func (p *PebblePerformanceTest) PrintResults(elapsed time.Duration) {
	totalOps := p.stats.readOps + p.stats.writeOps
	opsPerSec := float64(totalOps) / elapsed.Seconds()
	
	fmt.Printf("===== Pebble DB 성능 테스트 결과 =====\n")
	fmt.Printf("총 작업 수: %d (읽기: %d, 쓰기: %d)\n", totalOps, p.stats.readOps, p.stats.writeOps)
	fmt.Printf("고루틴 수: %d\n", p.numWorkers)
	fmt.Printf("배치 크기: %d\n", p.batchSize)
	fmt.Printf("키 크기: %d bytes, 값 크기: %d bytes\n", p.keySize, p.valueSize)
	fmt.Printf("읽기/쓰기 비율: %.2f\n", p.readRatio)
	fmt.Printf("동기 쓰기: %v\n", p.syncWrites)
	fmt.Printf("소요 시간: %v\n", elapsed)
	fmt.Printf("초당 작업 수: %.2f ops/sec\n", opsPerSec)
	fmt.Printf("에러 수: %d\n", p.stats.errors)
	fmt.Printf("=====================================\n")
}

// 테스트 함수 - 다양한 설정으로 성능 테스트 실행
func TestPebbleMaxPerformance(t *testing.T) {
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
		batchSize  int
		keySize    int
		valueSize  int
		readRatio  float64
		syncWrites bool
	}{
		{"쓰기 전용 (비동기)", 1000000, cpuCores, 1, 16, 100, 0.0, false},
		{"쓰기 전용 (동기)", 100000, cpuCores, 1, 16, 100, 0.0, true},
		{"읽기 전용", 1000000, cpuCores, 1, 16, 100, 1.0, false},
		{"읽기/쓰기 혼합 (50:50)", 1000000, cpuCores, 1, 16, 100, 0.5, false},
		{"배치 쓰기 (비동기)", 1000000, cpuCores, 1000, 16, 100, 0.0, false},
		{"배치 쓰기 (동기)", 100000, cpuCores, 1000, 16, 100, 0.0, true},
		{"고루틴 확장 테스트", 1000000, cpuCores * 4, 1000, 16, 100, 0.5, false},
	}

	// 각 설정으로 테스트 실행
	for _, cfg := range testConfigs {
		t.Run(cfg.name, func(t *testing.T) {
			fmt.Printf("\n=== 테스트 시작: %s ===\n", cfg.name)
			
			// 테스트 인스턴스 생성
			test, err := NewPebblePerformanceTest(
				cfg.ops, cfg.workers, cfg.batchSize, 
				cfg.keySize, cfg.valueSize, 
				cfg.readRatio, cfg.syncWrites,
			)
			if err != nil {
				t.Fatalf("테스트 초기화 실패: %v", err)
			}
			defer test.Cleanup()
			
			// 테스트 실행
			elapsed, err := test.Run()
			if err != nil {
				t.Fatalf("테스트 실행 실패: %v", err)
			}
			
			// 결과 출력
			test.PrintResults(elapsed)
		})
	}
}
