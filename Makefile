.PHONY: setup gen fmt lint arch test build check
setup:  ; npm ci
gen:    ; node scripts/gen-vocab.mjs
fmt:    ; npx prettier --write .
lint:   ; npx prettier --check . && npx eslint . && npx tsc --noEmit
arch:   ; npx depcruise src
test:   ; npx vitest run --coverage
build:  ; npm run build
check:  lint arch test
