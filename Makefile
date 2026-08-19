.PHONY: setup gen fmt lint arch test build check
.PHONY: go-setup go-fmt go-vet go-test go-check
setup:    ; npm ci
gen:      ; node scripts/gen-vocab.mjs
fmt:      ; npx prettier --write .
lint:     ; npx prettier --check . && npx eslint . && npx tsc --noEmit
arch:     ; npx depcruise src
test:     ; npx vitest run --coverage
build:    ; npm run build
check:    lint arch test
go-setup: ; go mod download
go-fmt:   ; test -z "$$(gofmt -l .)"
go-vet:   ; go vet ./internal/... ./codegen/go/... ./cmd/...
go-test:  ; go test ./internal/... ./codegen/go/... ./cmd/...
go-check: go-fmt go-vet go-test
