.PHONY: build build-web dev dev-frontend test lint clean release-local init entity desktop icons

# --- AI-native scaffolding --------------------------------------------------

# Rebrand a fresh copy of this template:
#   make init MODULE=github.com/you/myapp NAME=MyApp
init:
	go run ./cmd/generator init --module "$(MODULE)" --name "$(NAME)"

# Generate a full CRUD vertical (store+migration, handlers, routes, typed
# client, page, nav). FIELDS is a space-separated list of name:type:
#   make entity NAME=task FIELDS="title:string body:text done:bool estimate:int"
entity:
	go run ./cmd/generator entity $(NAME) $(if $(FIELDS),$(foreach f,$(FIELDS),--field $(f)),)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILDINFO = github.com/Potterluo/dream-interviewer/internal/buildinfo
LDFLAGS  = -s -w \
	-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) \
	-X $(BUILDINFO).Version=$(VERSION) -X $(BUILDINFO).Commit=$(COMMIT) -X $(BUILDINFO).Date=$(DATE)

# Build options:
#   MARKDOWN=full|lite  — lite drops Shiki code highlighting (~11MB less)
#   e.g. make build MARKDOWN=lite
MARKDOWN ?= full

# --- Frontend ---------------------------------------------------------------

# build-web compiles the Next.js static export and copies it into the
# go:embed directory. Run this BEFORE `go build` — the binary bakes in
# whatever is in internal/server/dist at compile time.
build-web:
	cd web && pnpm install --frozen-lockfile && MARKDOWN_FULL=$(MARKDOWN) pnpm build
	rm -rf internal/server/dist
	cp -r web/out internal/server/dist

# --- Backend ----------------------------------------------------------------

build: build-web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/app ./cmd/server

test:
	go test ./...

lint:
	go vet ./...
	cd web && pnpm lint

# --- Development ------------------------------------------------------------

# The dev loop:
#   terminal 1:  make dev-frontend     (next dev on :3000, HMR)
#   terminal 2:  make dev              (Go on :8080, proxying UI to :3000)
# Open http://localhost:8080 — API hits Go directly, everything else is
# live-reloaded by Next. `go build` is NOT needed for frontend changes;
# restart `make dev` when Go code changes (or use air).
dev-frontend:
	cd web && pnpm dev

dev:
	APP_DEV_PROXY=http://localhost:3000 go run ./cmd/server

# --- Release ----------------------------------------------------------------

clean:
	rm -rf bin/ dist/ web/out web/.next

# Desktop build (Wails shell over the same server + embedded UI).
#   Windows: works with CGO disabled (WebView2 ships with Win10/11).
#   Linux:   needs webkit2gtk-4.1 dev packages and CGO_ENABLED=1.
#   macOS:   needs the Xcode command line tools.
# Dev mode with native-window hot reload: install the Wails CLI
#   (go install github.com/wailsapp/wails/v2/cmd/wails@latest) then `wails dev`.
desktop: build-web icons
	@# -H windowsgui hides the console window on Windows builds (ignored elsewhere via the GOOS test).
	CGO_ENABLED=0 go build -trimpath -tags desktop,production -ldflags "$(LDFLAGS) $(if $(filter windows,$(shell go env GOOS)),-H windowsgui)" -o bin/app-desktop ./cmd/desktop

# Regenerate Windows icon resources after changing the brand artwork.
# Replace build/appicon.png with the new square master, regenerate the
# canonical PNG set in build/ (icon-256/48/32/16.png) from it, then run
# `make icons` — which packs build/windows/icon.ico AND recompiles the
# .syso resources the desktop exe actually links in (the .ico alone does
# not change the exe icon).
icons:
	python build/windows/mkico.py
	go run github.com/tc-hib/go-winres@v0.3.3 make --in build/winres.json --out cmd/desktop/rsrc

# release-local cross-compiles the embedded binary for the usual
# desktop/server targets. CI (release.yml) does the same on tags.
release-local: build-web
	@mkdir -p dist
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/app_darwin_arm64/app  ./cmd/server
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/app_darwin_amd64/app  ./cmd/server
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/app_linux_arm64/app   ./cmd/server
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/app_linux_amd64/app   ./cmd/server
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/app_windows_amd64/app.exe ./cmd/server
	@cd dist && for d in app_darwin_* app_linux_*; do tar -czf "$${d}.tar.gz" -C "$$d" app; done
	@echo "Release artifacts in dist/:"
	@ls -lh dist/*.tar.gz 2>/dev/null
