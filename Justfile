default:
    @just --list

all: build

install-deps:
    @echo "Installing Go dependencies..."
    go mod tidy
    @echo "Installing Node.js dependencies..."
    cd frontend && bun install

frontend:
    @echo "Building frontend..."
    cd frontend && bun run build

backend: frontend
    @echo "Building Go binary (pure Go, no CGO)..."
    CGO_ENABLED=0 go build -o bin/parse-dmarc ./cmd/parse-dmarc

backend-cgo: frontend
    @echo "Building Go binary (with CGO)..."
    CGO_ENABLED=1 go build -tags cgo -o bin/parse-dmarc ./cmd/parse-dmarc

build: frontend backend
    @echo "Build complete! Binary available at ./bin/parse-dmarc"

build-cgo: frontend backend-cgo
    @echo "CGO build complete! Binary available at ./bin/parse-dmarc"

dev:
    @echo "Starting development server..."
    go run ./cmd/parse-dmarc -config=config.json

config:
    go run ./cmd/parse-dmarc -gen-config

clean:
    @echo "Cleaning build artifacts..."
    rm -rf bin/
    rm -rf internal/api/dist/
    rm -rf frontend/node_modules/
    rm -f config.json

test:
    go test -v ./...

install: build
    @echo "Installing to /usr/local/bin..."
    sudo cp bin/parse-dmarc /usr/local/bin/

frontend-dev:
    cd frontend && bun run dev
