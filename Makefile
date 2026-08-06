.PHONY: build build-frontend build-backend dev clean

build:
	go run ./cmd/build

build-frontend:
	go run ./cmd/build -target frontend

build-backend:
	go run ./cmd/build -target backend

dev:
	@echo "Start the Go server: go run ./cmd/gateway --config config/gateway.yaml"
	@echo "Then: cd web && npm run dev"
	@echo "Visit http://localhost:5173/admin/dashboard/"

clean:
	go run ./cmd/build -target clean
