# go-badger-db-and-grpc

## grpc settings

### 1. protoc-gen-go 설치

```bash
# 실행 후, ~/go/bin에 바이너리가 생성됩니다.
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# export PATH="$PATH:$(go env GOPATH)/bin" 추가
vim ~/.zshrc
source ~/.zshrc
# 적용 확인
which protoc-gen-go
```

### 2. protoc-gen-go-grpc 설치

```bash
# gRPC용 코드도 생성하려면 gRPC 플러그인 설치
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 3. Generate grpc .proto files

```bash
mkdir -p proto/generated
protoc --go_out=proto/generated --go-grpc_out=proto/generated proto/get_stockmaster.proto
```

### 4. go.mod 초기화 및 의존성 설치

```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
```
