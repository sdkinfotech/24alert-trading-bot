#!/bin/bash
set -e

COVERAGE_THRESHOLD=70

# Packages excluded from the coverage gate because they wrap
# external SDKs/gRPC and cannot be unit-tested without a real server.
EXCLUDED_PACKAGES="internal/order internal/risk"

echo "=== Running tests with coverage ==="
go test -coverprofile=coverage.out -count=1 ./...

echo ""
echo "=== Checking coverage threshold (>= ${COVERAGE_THRESHOLD}%) ==="
echo "    (excludes: $EXCLUDED_PACKAGES)"

FAIL=0
while IFS= read -r line; do
    # Lines look like: "ok  	github.com/.../pkg  0.1s  coverage: 75.0% of statements"
    if [[ "$line" =~ coverage:\ ([0-9]+\.[0-9]+)% ]]; then
        pct="${BASH_REMATCH[1]}"
        pkg=$(echo "$line" | awk '{print $2}')
        pct_int=${pct%.*}

        # Skip excluded packages
        skip=0
        for excl in $EXCLUDED_PACKAGES; do
            if [[ "$pkg" == *"$excl"* ]]; then
                echo "  ⏭  SKIP (external deps): $pkg — ${pct}%"
                skip=1
                break
            fi
        done
        [ "$skip" -eq 1 ] && continue

        if [ "$pct_int" -lt "$COVERAGE_THRESHOLD" ]; then
            echo "  ❌ BELOW THRESHOLD: $pkg — ${pct}% (need ${COVERAGE_THRESHOLD}%)"
            FAIL=1
        else
            echo "  ✅ OK: $pkg — ${pct}%"
        fi
    fi
done < <(go test -cover -count=1 ./... 2>/dev/null)

if [ "$FAIL" -eq 1 ]; then
    echo ""
    echo "❌ Coverage check failed: one or more packages below ${COVERAGE_THRESHOLD}%"
    exit 1
fi

echo ""
echo "=== Running linter ==="
golangci-lint run ./...

echo ""
echo "=== Building binaries ==="
go build -o bin/gateway ./cmd/gateway
go build -o bin/order-svc ./cmd/order-svc
go build -o bin/marketdata-svc ./cmd/marketdata-svc
go build -o bin/portfolio-svc ./cmd/portfolio-svc
go build -o bin/risk-svc ./cmd/risk-svc

echo ""
echo "All CI checks passed"
