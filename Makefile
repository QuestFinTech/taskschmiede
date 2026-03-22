# Taskschmiede Makefile
# Task and project management for AI agents and humans
#
# Run `make help` for the full target list.

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------
ifeq ($(OS),Windows_NT)
    _GITEXEC := $(shell git --exec-path)
    _GITROOT := $(subst /mingw64/libexec/git-core,,$(_GITEXEC))
    SHELL := $(_GITROOT)/usr/bin/bash.exe
    .SHELLFLAGS := -c
    EXE_SUFFIX := .exe
    CURRENT_PLATFORM := windows-amd64
else
    EXE_SUFFIX :=
    UNAME_S := $(shell uname -s)
    UNAME_M := $(shell uname -m)
    ifeq ($(UNAME_S),Darwin)
        ifeq ($(UNAME_M),arm64)
            CURRENT_PLATFORM := darwin-arm64
        else
            CURRENT_PLATFORM := darwin-amd64
        endif
    else
        CURRENT_PLATFORM := linux-amd64
    endif
endif

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------
BINARY_NAME    := taskschmiede
PROXY_BINARY   := taskschmiede-proxy
PORTAL_BINARY  := taskschmiede-portal

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

BUILD_DIR := build
RUN_DIR   := run

# Version components (for bump targets)
CURRENT_TAG   := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
VERSION_PARTS := $(subst ., ,$(subst v,,$(CURRENT_TAG)))
MAJOR := $(word 1,$(VERSION_PARTS))
MINOR := $(word 2,$(VERSION_PARTS))
PATCH := $(word 3,$(VERSION_PARTS))

# ---------------------------------------------------------------------------
# Phony declarations
# ---------------------------------------------------------------------------
.PHONY: all build build-linux \
        build-proxy build-proxy-linux \
        build-portal build-portal-linux \
        deploy-development deploy-local \
        package-community \
        test lint lint-utc lint-errors lint-i18n check fmt tidy \
        docs docs-sync-content docs-export docs-hugo-pages docs-hugo-build docs-hugo-serve docs-lint \
        bump bump-patch bump-minor bump-major release \
        reset-interviews clean clean-all version help

.DEFAULT_GOAL := help
all: help

# ===================================================================
#  BUILD TARGETS
# ===================================================================

## docs-sync-content: Sync Hugo content to internal/docs/content/ for go:embed
docs-sync-content:
	@mkdir -p internal/docs/content/guides internal/docs/content/workflows
	@rsync -a --delete --exclude='_index.md' website/hugo/content/guides/ internal/docs/content/guides/
	@rsync -a --delete --exclude='_index.md' website/hugo/content/workflows/ internal/docs/content/workflows/

## build: Build core binary for current platform
build: docs-sync-content
	@mkdir -p $(BUILD_DIR)/$(CURRENT_PLATFORM)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CURRENT_PLATFORM)/$(BINARY_NAME)$(EXE_SUFFIX) ./cmd/taskschmiede
	@echo "Built: $(BUILD_DIR)/$(CURRENT_PLATFORM)/$(BINARY_NAME)$(EXE_SUFFIX)"

## build-linux: Build core binary for Linux amd64
build-linux: docs-sync-content
	@mkdir -p $(BUILD_DIR)/linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) ./cmd/taskschmiede
	@echo "Built: $(BUILD_DIR)/linux-amd64/$(BINARY_NAME)"

## build-proxy: Build proxy for current platform
build-proxy:
	@mkdir -p $(BUILD_DIR)/$(CURRENT_PLATFORM)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CURRENT_PLATFORM)/$(PROXY_BINARY)$(EXE_SUFFIX) ./cmd/taskschmiede-proxy
	@echo "Built: $(BUILD_DIR)/$(CURRENT_PLATFORM)/$(PROXY_BINARY)$(EXE_SUFFIX)"

## build-proxy-linux: Build proxy for Linux amd64
build-proxy-linux:
	@mkdir -p $(BUILD_DIR)/linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux-amd64/$(PROXY_BINARY) ./cmd/taskschmiede-proxy
	@echo "Built: $(BUILD_DIR)/linux-amd64/$(PROXY_BINARY)"

## build-portal: Build portal for current platform
build-portal:
	@mkdir -p $(BUILD_DIR)/$(CURRENT_PLATFORM)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CURRENT_PLATFORM)/$(PORTAL_BINARY)$(EXE_SUFFIX) ./cmd/taskschmiede-portal
	@echo "Built: $(BUILD_DIR)/$(CURRENT_PLATFORM)/$(PORTAL_BINARY)$(EXE_SUFFIX)"

## build-portal-linux: Build portal for Linux amd64
build-portal-linux:
	@mkdir -p $(BUILD_DIR)/linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux-amd64/$(PORTAL_BINARY) ./cmd/taskschmiede-portal
	@echo "Built: $(BUILD_DIR)/linux-amd64/$(PORTAL_BINARY)"

# ===================================================================
#  PACKAGING
# ===================================================================

# Internal: validate version is clean for packaging
_check-version:
	@if echo "$(VERSION)" | grep -q dirty; then \
		echo "Error: Cannot package with uncommitted changes (version: $(VERSION))"; \
		exit 1; \
	fi

## package-community: Package community edition binaries for deployment (Linux amd64)
package-community: _check-version build-linux build-proxy-linux build-portal-linux
	@echo "Creating community deployment package $(VERSION)..."
	@mkdir -p $(BUILD_DIR)/package/taskschmiede-$(VERSION) $(BUILD_DIR)/release
	@for bin in $(BINARY_NAME) $(PROXY_BINARY) $(PORTAL_BINARY); do \
		cp $(BUILD_DIR)/linux-amd64/$$bin $(BUILD_DIR)/package/taskschmiede-$(VERSION)/; \
	done
	@cp deploy/install-taskschmiede.sh $(BUILD_DIR)/package/taskschmiede-$(VERSION)/
	@cp config.yaml.example $(BUILD_DIR)/package/taskschmiede-$(VERSION)/
	@cp README.md $(BUILD_DIR)/package/taskschmiede-$(VERSION)/
	@mkdir -p $(BUILD_DIR)/package/taskschmiede-$(VERSION)/systemd
	@for svc in taskschmiede.service taskschmiede-portal.service taskschmiede-proxy.service; do \
		cp deploy/systemd/$$svc $(BUILD_DIR)/package/taskschmiede-$(VERSION)/systemd/; \
	done
	@echo "$(VERSION)" > $(BUILD_DIR)/package/taskschmiede-$(VERSION)/VERSION
	@cd $(BUILD_DIR)/package && tar czf ../release/taskschmiede-$(VERSION)-community-linux-amd64.tar.gz taskschmiede-$(VERSION)/
	@rm -rf $(BUILD_DIR)/package
	@echo "Package: $(BUILD_DIR)/release/taskschmiede-$(VERSION)-community-linux-amd64.tar.gz"

# ===================================================================
#  LOCAL DEVELOPMENT
# ===================================================================

## deploy-development: Build and copy all binaries to run/ folder
deploy-development: build build-proxy build-portal
	@mkdir -p $(RUN_DIR)
	@for bin in $(BINARY_NAME) $(PROXY_BINARY) $(PORTAL_BINARY); do \
		cp $(BUILD_DIR)/$(CURRENT_PLATFORM)/$$bin$(EXE_SUFFIX) $(RUN_DIR)/$$bin$(EXE_SUFFIX); \
	done
	@rm -f $(RUN_DIR)/*.out $(RUN_DIR)/*.log
	@if [ -f .env ] && [ ! -f $(RUN_DIR)/.env ]; then \
		cp .env $(RUN_DIR)/.env; \
		echo "Copied .env to $(RUN_DIR)/"; \
	fi
	@if [ ! -f $(RUN_DIR)/config.yaml ]; then \
		if [ -f config.yaml ]; then \
			cp config.yaml $(RUN_DIR)/config.yaml; \
		elif [ -f config.yaml.example ]; then \
			cp config.yaml.example $(RUN_DIR)/config.yaml; \
		fi; \
		echo "Copied config.yaml to $(RUN_DIR)/"; \
	fi
	@echo "Ready: $(RUN_DIR)/"
ifeq ($(OS),Windows_NT)
	@echo "Start with: scripts\\taskschmiede.ps1 start"
else
	@echo "Start with: scripts/taskschmiede.sh start"
endif

## deploy-local: Deploy to a remote host via scripts/deploy.sh
deploy-local: package-community
	@scripts/deploy.sh $(TARGET) $(BUILD_DIR)/release/taskschmiede-$(VERSION)-community-linux-amd64.tar.gz

# ===================================================================
#  DEVELOPMENT
# ===================================================================

## test: Run all tests
test:
	go test -race -v ./...

## lint-utc: Verify all time.Now() calls use UTC [PROJECT POLICY -- DO NOT REMOVE]
##
## Taskschmiede uses UTC for all internal timestamps. Bare time.Now() is
## forbidden because it returns local time. Use storage.UTCNow() or
## time.Now().UTC() instead. This rule is a hard gate on the build -- it
## exists to prevent subtle timezone bugs in a multi-agent system where
## clients span different timezones.
lint-utc:
	@echo "Checking UTC time policy..."
	@VIOLATIONS=$$(grep -rn 'time\.Now()' --include='*.go' \
		| grep -v 'time\.Now()\.UTC()' \
		| grep -v '_test\.go' \
		| grep -v 'storage/time\.go' \
		|| true); \
	if [ -n "$$VIOLATIONS" ]; then \
		echo ""; \
		echo "ERROR: Bare time.Now() found. Use storage.UTCNow() or time.Now().UTC() instead."; \
		echo "Policy: All internal timestamps must be UTC."; \
		echo ""; \
		echo "$$VIOLATIONS"; \
		echo ""; \
		exit 1; \
	fi
	@echo "UTC time policy: OK"

## lint-errors: Catch raw error leaks through toolError("internal_error", err.Error())
lint-errors:
	@echo "Checking MCP error code policy..."
	@VIOLATIONS=$$(grep -rn 'toolError("internal_error"' --include='*.go' \
		| grep 'err\.Error()' \
		|| true); \
	if [ -n "$$VIOLATIONS" ]; then \
		echo ""; \
		echo "WARNING: toolError(\"internal_error\", ...err.Error()...) found."; \
		echo "Service validation errors should use specific codes (not_found,"; \
		echo "invalid_input, invalid_transition, conflict). If this is genuinely"; \
		echo "an internal error, use a hardcoded message and log the real error."; \
		echo ""; \
		echo "$$VIOLATIONS"; \
		echo ""; \
		exit 1; \
	fi
	@echo "MCP error code policy: OK"

## lint-i18n: Verify template key references match en.json and locale completeness
lint-i18n:
	@echo "Checking i18n key completeness..."
	@EN_FILE="internal/i18n/locales/en.json"; \
	if [ ! -f "$$EN_FILE" ]; then echo "ERROR: $$EN_FILE not found"; exit 1; fi; \
	ERRORS=0; \
	TMPL_KEYS=$$({ grep -roh 't \.[A-Za-z]* "[^"]*"' internal/portal/templates/ 2>/dev/null; \
		grep -roh 't \$$\.[A-Za-z]* "[^"]*"' internal/portal/templates/ 2>/dev/null; } \
		| sed 's/^t [$$]*\.[A-Za-z]* "//; s/"$$//' | sort -u); \
	for key in $$TMPL_KEYS; do \
		if ! grep -q "\"$$key\"" "$$EN_FILE"; then \
			echo "  MISSING in en.json: $$key"; \
			ERRORS=$$((ERRORS + 1)); \
		fi; \
	done; \
	TMPDIR_I18N=$$(mktemp -d); \
	grep -o '"[^"]*":' "$$EN_FILE" | sed 's/"//g; s/://' | sort > "$$TMPDIR_I18N/en_keys"; \
	EN_COUNT=$$(wc -l < "$$TMPDIR_I18N/en_keys" | tr -d ' '); \
	for LOCALE in internal/i18n/locales/*.json; do \
		BASENAME=$$(basename "$$LOCALE"); \
		if [ "$$BASENAME" = "en.json" ] || [ "$$BASENAME" = "_meta.json" ]; then continue; fi; \
		grep -o '"[^"]*":' "$$LOCALE" | sed 's/"//g; s/://' | sort > "$$TMPDIR_I18N/lang_keys"; \
		LANG_COUNT=$$(wc -l < "$$TMPDIR_I18N/lang_keys" | tr -d ' '); \
		MISSING=$$(comm -23 "$$TMPDIR_I18N/en_keys" "$$TMPDIR_I18N/lang_keys"); \
		if [ -n "$$MISSING" ]; then \
			MISS_COUNT=$$(echo "$$MISSING" | wc -l | tr -d ' '); \
			echo "  $$BASENAME: $$MISS_COUNT keys missing ($$LANG_COUNT/$$EN_COUNT)"; \
			ERRORS=$$((ERRORS + MISS_COUNT)); \
		fi; \
	done; \
	rm -rf "$$TMPDIR_I18N"; \
	if [ $$ERRORS -gt 0 ]; then \
		echo ""; \
		echo "ERROR: $$ERRORS i18n issue(s) found."; \
		exit 1; \
	fi
	@echo "i18n key completeness: OK"

## lint: Run linter (requires golangci-lint)
lint: lint-utc lint-errors lint-i18n
	@which golangci-lint > /dev/null || (echo "Error: golangci-lint not installed. Run: brew install golangci-lint" && exit 1)
	golangci-lint run ./...

## check: Run lint and tests
check: lint test

## fmt: Format code
fmt:
	go fmt ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

# ===================================================================
#  DOCUMENTATION
# ===================================================================

## docs: Generate documentation (alias for docs-hugo-build)
docs: docs-hugo-build

## docs-export: Export MCP tool registry as JSON and copy OpenAPI spec
docs-export: build
	@echo "Exporting MCP tools JSON..."
	@$(BUILD_DIR)/$(CURRENT_PLATFORM)/$(BINARY_NAME)$(EXE_SUFFIX) docs export --format json --output ./website/hugo/static/mcp-tools.json
	@echo "Copying OpenAPI spec..."
	@$(BUILD_DIR)/$(CURRENT_PLATFORM)/$(BINARY_NAME)$(EXE_SUFFIX) docs export --format openapi --output ./website/hugo/static/

## docs-hugo-pages: Generate Hugo Markdown pages from exported MCP tools JSON
docs-hugo-pages: docs-export
	@echo "Generating Hugo MCP tool pages..."
	@$(BUILD_DIR)/$(CURRENT_PLATFORM)/$(BINARY_NAME)$(EXE_SUFFIX) docs hugo

## docs-lint: Check documentation content for formatting violations
docs-lint:
	@echo "Linting documentation content..."
	@FOUND=0; \
	HITS=$$(grep -rn '<br\s*/\?>' website/hugo/content/ --include='*.md' 2>/dev/null || true); \
	if [ -n "$$HITS" ]; then \
		echo "ERROR: Found <br/> tags (let the browser handle line breaks):"; \
		echo "$$HITS"; \
		FOUND=1; \
	fi; \
	HITS=$$(grep -rPn '(?<!\\)\\n(?!["`])' website/hugo/content/ --include='*.md' 2>/dev/null || true); \
	if [ -n "$$HITS" ]; then \
		echo "ERROR: Found artificial \\n outside code/JSON (let the browser handle line breaks):"; \
		echo "$$HITS"; \
		FOUND=1; \
	fi; \
	if [ $$FOUND -eq 1 ]; then exit 1; fi; \
	echo "Documentation lint passed."

## docs-hugo-build: Full Hugo documentation build (export + pages + Hugo build)
docs-hugo-build: docs-hugo-pages docs-lint
	@echo "Building Hugo site..."
	@rm -rf website/hugo/public
	@cd website/hugo && hugo --minify
	@for f in apple-touch-icon.png favicon.png; do \
		if [ -f website/root/$$f ]; then cp website/root/$$f website/hugo/public/$$f; fi; \
	done
	@echo "Documentation built in website/hugo/public/"

## docs-hugo-serve: Start Hugo development server with live reload
docs-hugo-serve: docs-hugo-pages
	@echo "Starting Hugo dev server..."
	@cd website/hugo && hugo server --buildDrafts

# ===================================================================
#  RELEASE
# ===================================================================

## bump: Increment patch version and create tag (alias for bump-patch)
bump: bump-patch

## bump-patch: Increment patch version (v0.1.0 -> v0.1.1)
bump-patch:
	@if git status --porcelain | grep -q .; then \
		echo "Error: Uncommitted changes. Commit first."; \
		exit 1; \
	fi
	$(eval NEW_VERSION := v$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1))))
	@echo "Bumping version: $(CURRENT_TAG) -> $(NEW_VERSION)"
	@git tag -a $(NEW_VERSION) -m "Release $(NEW_VERSION)"
	@echo "Created tag $(NEW_VERSION)"
	@echo "Run 'git push origin $(NEW_VERSION)' to push the tag"

## bump-minor: Increment minor version (v0.1.0 -> v0.2.0)
bump-minor:
	@if git status --porcelain | grep -q .; then \
		echo "Error: Uncommitted changes. Commit first."; \
		exit 1; \
	fi
	$(eval NEW_VERSION := v$(MAJOR).$(shell echo $$(($(MINOR)+1))).0)
	@echo "Bumping version: $(CURRENT_TAG) -> $(NEW_VERSION)"
	@git tag -a $(NEW_VERSION) -m "Release $(NEW_VERSION)"
	@echo "Created tag $(NEW_VERSION)"
	@echo "Run 'git push origin $(NEW_VERSION)' to push the tag"

## bump-major: Increment major version (v0.1.0 -> v1.0.0)
bump-major:
	@if git status --porcelain | grep -q .; then \
		echo "Error: Uncommitted changes. Commit first."; \
		exit 1; \
	fi
	$(eval NEW_VERSION := v$(shell echo $$(($(MAJOR)+1))).0.0)
	@echo "Bumping version: $(CURRENT_TAG) -> $(NEW_VERSION)"
	@git tag -a $(NEW_VERSION) -m "Release $(NEW_VERSION)"
	@echo "Created tag $(NEW_VERSION)"
	@echo "Run 'git push origin $(NEW_VERSION)' to push the tag"

## release: Create GitHub release with community binaries (requires gh CLI)
release: _check-version
	@which gh > /dev/null || (echo "Error: gh CLI not installed. Run: brew install gh" && exit 1)
	@if ! echo "$(VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+'; then \
		echo "Error: VERSION must be a semver tag (e.g., v0.1.0). Current: $(VERSION)"; \
		echo "Create a tag first: make bump"; \
		exit 1; \
	fi
	@echo "Building community binaries for release..."
	@$(MAKE) --no-print-directory package-community
	@echo "Creating release $(VERSION)..."
	gh release create $(VERSION) \
		$(BUILD_DIR)/release/taskschmiede-$(VERSION)-community-linux-amd64.tar.gz \
		--title "Taskschmiede $(VERSION)" \
		--generate-notes
	@echo "Release $(VERSION) created successfully"

# ===================================================================
#  MAINTENANCE
# ===================================================================

## reset-interviews: Delete test/agent users and all interview data from the run database
reset-interviews:
	@if [ ! -f $(RUN_DIR)/taskschmiede.db ]; then \
		echo "Error: $(RUN_DIR)/taskschmiede.db not found"; exit 1; \
	fi
	@sqlite3 $(RUN_DIR)/taskschmiede.db "\
		DELETE FROM onboarding_injection_review; \
		DELETE FROM onboarding_step0; \
		DELETE FROM onboarding_section_metric; \
		DELETE FROM onboarding_attempt; \
		DELETE FROM onboarding_cooldown; \
		DELETE FROM entity_relation WHERE source_entity_id IN (SELECT id FROM resource WHERE id IN (SELECT resource_id FROM user WHERE user_type = 'agent')); \
		DELETE FROM organization_resource WHERE resource_id IN (SELECT resource_id FROM user WHERE user_type = 'agent'); \
		DELETE FROM user_endeavour WHERE user_id IN (SELECT id FROM user WHERE user_type = 'agent'); \
		DELETE FROM agent_session_metric WHERE user_id IN (SELECT id FROM user WHERE user_type = 'agent'); \
		DELETE FROM agent_health_snapshot WHERE user_id IN (SELECT id FROM user WHERE user_type = 'agent'); \
		DELETE FROM resource WHERE id IN (SELECT resource_id FROM user WHERE user_type = 'agent'); \
		DELETE FROM pending_user WHERE 1=1; \
		DELETE FROM user WHERE user_type = 'agent'; \
	"
	@echo "Interview data and agent users removed from $(RUN_DIR)/taskschmiede.db"

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	@echo "Cleaned $(BUILD_DIR)/"

## clean-all: Remove build artifacts and run folder (including db)
clean-all: clean
	rm -rf $(RUN_DIR)
	@echo "Cleaned $(RUN_DIR)/"

## version: Show version info
version:
	@echo "Version:  $(VERSION)"
	@echo "Commit:   $(COMMIT)"
	@echo "Built:    $(BUILD_TIME)"
	@echo "Platform: $(CURRENT_PLATFORM)"

## help: Show this help
help:
	@echo "Taskschmiede -- Task and project management for AI agents and humans"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  build              Build core server for current platform ($(CURRENT_PLATFORM))"
	@echo "  build-proxy        Build proxy for current platform"
	@echo "  build-portal       Build portal for current platform"
	@echo "  build-*-linux      Build any of the above for Linux amd64"
	@echo ""
	@echo "Development:"
	@echo "  deploy-development Build all binaries and copy to run/"
	@echo "  test               Run all tests"
	@echo "  lint               Run all linters (UTC + errors + i18n + golangci-lint)"
	@echo "  check              Run lint and tests"
	@echo "  fmt                Format code"
	@echo "  tidy               Tidy go modules"
	@echo ""
	@echo "Documentation:"
	@echo "  docs               Build documentation (alias for docs-hugo-build)"
	@echo "  docs-hugo-serve    Start Hugo dev server with live reload"
	@echo ""
	@echo "Deploy:"
	@echo "  deploy-local       Deploy to remote host (usage: make deploy-local TARGET=staging)"
	@echo "  package-community  Create deployment package (Linux amd64)"
	@echo ""
	@echo "Release:"
	@echo "  bump / bump-patch  Increment patch version and create tag"
	@echo "  bump-minor         Increment minor version"
	@echo "  bump-major         Increment major version"
	@echo "  release            Create GitHub release with community binaries"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean              Remove build/ folder"
	@echo "  clean-all          Remove build/ and run/ folders"
	@echo "  reset-interviews   Delete agent users and interview data"
	@echo "  version            Show version info"
