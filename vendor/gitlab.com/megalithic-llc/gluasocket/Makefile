.PHONY: check
check:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go vet ./...
	staticcheck ./...
	go test ./...

.PHONY: clean
clean:
	go clean -i ./...
	go clean -testcache

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: outdated
outdated:
	go install github.com/psampaz/go-mod-outdated@latest
	go list -u -m -json -mod=mod all | go-mod-outdated -direct
