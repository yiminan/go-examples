# go-krakend

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

```json
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

## 3. Prometheus Metrics 활성화 (Optional)

: Prometheus에서 <http://localhost:8090/__stats> 로 metrics 수집 가능

```json
"telemetry": {
  "metrics": {
    "namespace": "krakend",
    "port": 8090,
    "expose_endpoint": true,
    "endpoint": "/__stats"
  }
}
```
