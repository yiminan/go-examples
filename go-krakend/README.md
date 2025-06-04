# go-krakend

- Go KrakenD를 사용한 API Gateway 구현

## 1. KrakenD 환경 셋팅 방법

### 1.1. 로컬 설치 (Mac/Linux)

```bash
# Homebrew (Mac)
brew install krakend

# 또는 바이너리 다운로드 (Linux/Mac)
curl -L https://github.com/krakendio/krakend-ce/releases/latest/download/krakend_amd64 -o krakend
chmod +x krakend
sudo mv krakend /usr/local/bin
```

### 1.2. Docker로 설치

```bash
docker run -it -p 8080:8080 -v "$PWD:/etc/krakend" devopsfaith/krakend run -c /etc/krakend/krakend.json
```

## 2. 기본 설정 예시 (krakend.json)

: <http://localhost:8080/api/users> 로 요청 시 <http://localhost:3001/users>로 요청

```yml
{
  "version": 3,
  "name": "my-krakend-gateway",
  "port": 8080,
  "timeout": "3000ms",
  "endpoints": [
    {
      "endpoint": "/api/users",
      "method": "GET",
      "backend": [
        {
          "url_pattern": "/users",
          "host": ["http://localhost:3001"],
          "method": "GET"
        }
      ]
    }
  ]
}
```

- `krakend run -c krakend.json` 명령어로 KrakenD를 실행

## 3. Prometheus Metrics 활성화 (Optional)

: Prometheus에서 <http://localhost:8090/__stats> 로 metrics 수집 가능

### 3.1. `krakend.json` 설정 예시

```yml
{
  "version": 3,
  "name": "krakend-gateway",
  "port": 8080,
  "timeout": "3000ms",
  "extra_config": {
    "telemetry/metrics": {
      "collection_time": "60s", 
      "listen_address": ":8090"
    },
    "telemetry/logging": {
      "level": "DEBUG",
      "prefix": "[KRAKEND]",
      "syslog": false,
      "stdout": true
    }
  },
  "endpoints": [
    {
      "endpoint": "/api/users",
      "method": "GET",
      "backend": [
        {
          "host": [
            "http://localhost:3001"
          ],
          "url_pattern": "/users",
          "method": "GET"
        }
      ]
    }
  ]
}
```

- namespace: Prometheus metric prefix (ex. krakend_request_duration)
- port: metrics를 노출할 별도 포트
- endpoint: Prometheus가 scrape할 path (ex. /__stats)
- expose_endpoint: true로 설정 시 endpoint가 활성화됨
-> 위 설정으로 <http://localhost:8090/__stats> 에서 Prometheus-compatible 메트릭이 제공됩니다.

### 3.2. Prometheus 설정 (prometheus.yml)

: Docker 환경에서는 localhost 대신 host.docker.internal 또는 내부 네트워크 IP 사용

```yml
scrape_configs:
  - job_name: 'krakend'
    static_configs:
      - targets: ['host.docker.internal:8090']
```

### 3.3. 수집 가능한 주요 메트릭

Metric 이름 : 설명
krakend_request_duration : 요청 처리 시간
krakend_request_size : 요청 크기
krakend_response_size : 응답 크기
krakend_concurrent_requests : 동시 처리 요청 수
krakend_backend_latency : 백엔드 호출 지연시간
krakend_backend_errors : 백엔드 오류 횟수
krakend_status_count : HTTP 상태코드별 횟수 (200, 500 등)

### 3.4. Grafana 대시보드 연동

3.4.1. Grafana에서 Prometheus Data Source 연결
3.4.2. Dashboard → Import
3.4.3. 아래 중 하나 사용:

- Grafana Dashboard ID: 11835
- 또는 JSON: krakend-prometheus-dashboard

## 4.  Docker-Compose 통합 예시 (KrakenD + Prometheus + Grafana)

```yml
version: '3'

services:
  krakend:
    image: devopsfaith/krakend
    ports:
      - "8080:8080"
      - "8090:8090"
    volumes:
      - ./krakend.json:/etc/krakend/krakend.json
    command: ["run", "-c", "/etc/krakend/krakend.json"]

  prometheus:
    image: prom/prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana
    ports:
      - "3000:3000"
```
