.PHONY: dev backend frontend tidy

dev:
	@echo "start backend and frontend separately: make backend / make frontend"

backend:
	cd backend && go run ./cmd/server

frontend:
	cd frontend && npm run dev

tidy:
	cd backend && go mod tidy

build-web:
	cd frontend && npm run build
