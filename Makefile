
dev_containers:
	docker compose stop || true
	docker compose rm -f || true
	docker compose up -d --wait --remove-orphans

fmt:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/daixiang0/gci@latest
	go fmt ./...
	goimports -w .
	gci -w .

clean:
	go clean -testcache

