#!/bin/bash
set -e

echo "=== Doctor: Checking Prerequisites ==="
echo ""

ERRORS=0

# Check Go version
echo -n "Checking Go... "
if ! command -v go &> /dev/null; then
	echo "FAIL: go not found in PATH"
	ERRORS=$((ERRORS + 1))
else
	GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
	GO_MAJOR=$(echo $GO_VERSION | cut -d. -f1)
	GO_MINOR=$(echo $GO_VERSION | cut -d. -f2)
	
	if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 22 ]); then
		echo "FAIL: Go version $GO_VERSION found, need >= 1.22"
		ERRORS=$((ERRORS + 1))
	else
		echo "PASS: Go $GO_VERSION"
	fi
fi

# Check Docker
echo -n "Checking Docker... "
if ! command -v docker &> /dev/null; then
	echo "FAIL: docker not found in PATH"
	ERRORS=$((ERRORS + 1))
else
	if ! docker ps &> /dev/null; then
		echo "FAIL: docker not running or not accessible"
		ERRORS=$((ERRORS + 1))
	else
		DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
		echo "PASS: Docker $DOCKER_VERSION"
	fi
fi

# Check Docker Compose
echo -n "Checking Docker Compose... "
if command -v docker-compose &> /dev/null; then
	COMPOSE_VERSION=$(docker-compose --version | awk '{print $3}' | sed 's/,//')
	echo "PASS: docker-compose $COMPOSE_VERSION"
elif docker compose version &> /dev/null 2>&1; then
	COMPOSE_VERSION=$(docker compose version | awk '{print $4}')
	echo "PASS: docker compose $COMPOSE_VERSION"
else
	echo "FAIL: docker-compose not found"
	ERRORS=$((ERRORS + 1))
fi

# Check protoc
echo -n "Checking protoc... "
if ! command -v protoc &> /dev/null; then
	echo "FAIL: protoc not found in PATH"
	echo "   Install: brew install protobuf (macOS) or apt-get install protobuf-compiler (Linux)"
	ERRORS=$((ERRORS + 1))
else
	PROTOC_VERSION=$(protoc --version | awk '{print $2}')
	echo "PASS: protoc $PROTOC_VERSION"
fi

# Check protoc-gen-go
echo -n "Checking protoc-gen-go... "
if ! command -v protoc-gen-go &> /dev/null; then
	echo "FAIL: protoc-gen-go not found in PATH"
	echo "   Install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
	ERRORS=$((ERRORS + 1))
else
	echo "PASS: protoc-gen-go"
fi

# Check protoc-gen-go-grpc
echo -n "Checking protoc-gen-go-grpc... "
if ! command -v protoc-gen-go-grpc &> /dev/null; then
	echo "FAIL: protoc-gen-go-grpc not found in PATH"
	echo "   Install: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
	ERRORS=$((ERRORS + 1))
else
	echo "PASS: protoc-gen-go-grpc"
fi

# Check docker ps works
echo -n "Checking docker ps... "
if ! docker ps &> /dev/null; then
	echo "FAIL: docker ps failed"
	ERRORS=$((ERRORS + 1))
else
	echo "PASS: docker ps works"
fi

echo ""
if [ $ERRORS -eq 0 ]; then
	echo "PASS: All checks passed!"
	exit 0
else
	echo "FAIL: $ERRORS check(s) failed"
	exit 1
fi

