.PHONY: build build-frontend build-backend dev clean

build: build-frontend build-backend

build-frontend:
	cd web && npm ci && npm run build
	rm -rf internal/static/dist
	cp -r web/dist internal/static/dist

build-backend:
	go build -o bin/gateway ./cmd/gateway

dev:
	@echo "Start the Go server: go run ./cmd/gateway --config config/gateway.yaml"
	@echo "Then: cd web && npm run dev"
	@echo "Visit http://localhost:5173/admin/dashboard/"

clean:
	rm -rf web/dist internal/static/dist bin/
