# Go Pebble DB Example

이 프로젝트는 [Pebble DB](https://github.com/cockroachdb/pebble)를 사용한 키-값 저장소 예제입니다. Badger DB와 비교하여 Pebble DB의 사용법을 보여줍니다.

## 주요 기능

- 키-값 저장 및 조회
- 키 삭제
- 접두사 기반 스캔 (범위 쿼리)
- 임시 디렉토리를 사용한 인메모리 유사 동작

## Pebble DB vs Badger DB 비교

| 기능 | Pebble DB | Badger DB |
|------|----------|-----------|
| 개발사 | CockroachDB | Dgraph Labs |
| 저장 방식 | LSM 트리 기반 | LSM 트리 기반 |
| 트랜잭션 | 단일 작업 트랜잭션 | 다중 작업 트랜잭션 |
| 메모리 사용 | 상대적으로 적음 | 상대적으로 많음 |
| API 스타일 | 간결함 | 다소 복잡함 |
| 값 접근 | Get 후 Closer 패턴 | Get 후 ValueCopy 패턴 |

## 실행 방법

```bash
go run main.go
```

서버가 시작되면 다음 엔드포인트를 사용할 수 있습니다:

- `GET /get?key=<키>`: 지정된 키의 값을 조회합니다.
- `POST /set`: 키-값 쌍을 저장합니다. JSON 형식 `{"key": "키", "value": "값"}`을 본문에 포함해야 합니다.
- `DELETE /delete?key=<키>`: 지정된 키를 삭제합니다.
- `GET /scan?prefix=<접두사>`: 지정된 접두사로 시작하는 모든 키-값 쌍을 반환합니다. 접두사가 없으면 모든 키-값 쌍을 반환합니다.

## 예제 사용법

### 값 저장하기

```bash
curl -X POST http://localhost:8082/set \
  -H "Content-Type: application/json" \
  -d '{"key": "test1", "value": "Hello Pebble!"}'
```

### 값 조회하기

```bash
curl http://localhost:8082/get?key=test1
```

### 샘플 데이터 조회하기

```bash
curl http://localhost:8082/get?key=samsung
```

### 값 삭제하기

```bash
curl -X DELETE http://localhost:8082/delete?key=test1
```

### 모든 키-값 쌍 스캔하기

```bash
curl http://localhost:8082/scan
```

### 특정 접두사로 스캔하기

```bash
curl http://localhost:8082/scan?prefix=test
```
