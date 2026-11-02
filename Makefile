# Sabeel WhatsApp Tools: build, test and package.
#
#   make check        lint + tests (what CI runs first)
#   make demo         build and run in demo mode
#   make dist         Linux, Windows and macOS packages in dist/
#   make e2e          drive the whole app in a headless browser (demo mode)
#
# Everything is pure Go (CGO_ENABLED=0), so every platform cross-compiles
# from Linux.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
ifeq ($(VERSION),)
VERSION := 0.0.0-dev
endif
# Numeric form for Windows resources and the macOS Info.plist.
NUMVER := $(shell printf '%s' '$(VERSION)' | sed 's/[^0-9.].*$$//' | grep -E '^[0-9]+(\.[0-9]+){0,3}$$' || echo 0.0.0)

GO      ?= go
CMD     := ./cmd/sabeel-whatsapp-tools
DIST    := dist
LDFLAGS := -s -w -X main.version=$(VERSION)
GOBUILD := CGO_ENABLED=0 $(GO) build -trimpath
WINRES  := github.com/tc-hib/go-winres@v0.3.3
MAKEFAT := github.com/randall77/makefat@v0.0.0-20260406194835-1b91746796b7
ICONS   := 16 24 32 48 64 128 256 512

.PHONY: all web check vet test lint-scripts build run demo dist dist-linux dist-windows dist-macos e2e e2e-deps screenshots icons clean

all: check build

web/node_modules/.installed: web/package-lock.json
	cd web && npm ci --no-fund --no-audit
	touch $@

web: web/node_modules/.installed
	cd web && npm run build

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

lint-scripts:
	shellcheck packaging/linux/*.sh scripts/*.sh

check: web vet test lint-scripts
	cd web && npm run check

build: web
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/linux/sabeel-whatsapp-tools $(CMD)

run: build
	$(DIST)/linux/sabeel-whatsapp-tools --verbose

demo: build
	$(DIST)/linux/sabeel-whatsapp-tools --demo --verbose

dist: dist-linux dist-windows dist-macos

dist-linux: web
	rm -rf $(DIST)/linux $(DIST)/linux-arm64
	mkdir -p $(DIST)/linux/icons
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/linux/sabeel-whatsapp-tools $(CMD)
	cp packaging/linux/sabeel-whatsapp-tools.desktop packaging/linux/install.sh packaging/linux/uninstall.sh $(DIST)/linux/
	chmod 0755 $(DIST)/linux/install.sh $(DIST)/linux/uninstall.sh
	for s in $(ICONS); do cp assets/icon/icon-$$s.png $(DIST)/linux/icons/; done
	cp assets/icon/icon-256.png $(DIST)/linux/sabeel-whatsapp-tools.png
	tar -C $(DIST) -czf $(DIST)/sabeel-whatsapp-tools-$(VERSION)-linux-amd64.tar.gz \
		--transform 's,^linux,sabeel-whatsapp-tools,' linux
	cp -r $(DIST)/linux $(DIST)/linux-arm64
	GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/linux-arm64/sabeel-whatsapp-tools $(CMD)
	tar -C $(DIST) -czf $(DIST)/sabeel-whatsapp-tools-$(VERSION)-linux-arm64.tar.gz \
		--transform 's,^linux-arm64,sabeel-whatsapp-tools,' linux-arm64
	rm -rf $(DIST)/linux-arm64

dist-windows: web
	rm -rf $(DIST)/windows
	mkdir -p $(DIST)/windows
	$(GO) run $(WINRES) make --in packaging/windows/winres.json --out cmd/sabeel-whatsapp-tools/rsrc \
		--arch amd64 --product-version $(NUMVER) --file-version $(NUMVER)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS) -H=windowsgui" \
		-o $(DIST)/windows/SabeelWhatsAppTools.exe $(CMD)
	rm -f cmd/sabeel-whatsapp-tools/rsrc_windows_*.syso
	cd $(DIST)/windows && rm -f ../sabeel-whatsapp-tools-$(VERSION)-windows-amd64.zip && \
		zip -q ../sabeel-whatsapp-tools-$(VERSION)-windows-amd64.zip SabeelWhatsAppTools.exe

# A universal (Intel + Apple silicon) app: lipo on a Mac, makefat elsewhere.
dist-macos: web
	rm -rf $(DIST)/macos
	mkdir -p $(DIST)/macos/bin
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/macos/bin/sabeel-whatsapp-tools-arm64 $(CMD)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/macos/bin/sabeel-whatsapp-tools-amd64 $(CMD)
	if command -v lipo >/dev/null 2>&1; then \
		lipo -create -output $(DIST)/macos/bin/sabeel-whatsapp-tools $(DIST)/macos/bin/sabeel-whatsapp-tools-arm64 $(DIST)/macos/bin/sabeel-whatsapp-tools-amd64; \
	else \
		$(GO) run $(MAKEFAT) $(DIST)/macos/bin/sabeel-whatsapp-tools $(DIST)/macos/bin/sabeel-whatsapp-tools-arm64 $(DIST)/macos/bin/sabeel-whatsapp-tools-amd64; \
	fi
	scripts/package-macos.sh $(DIST)/macos/bin/sabeel-whatsapp-tools $(NUMVER) $(DIST)/macos
	cd $(DIST)/macos && rm -f ../sabeel-whatsapp-tools-$(VERSION)-macos-universal.zip && \
		if command -v ditto >/dev/null 2>&1; then \
			ditto -c -k --keepParent "Sabeel WhatsApp Tools.app" ../sabeel-whatsapp-tools-$(VERSION)-macos-universal.zip; \
		else \
			zip -qry ../sabeel-whatsapp-tools-$(VERSION)-macos-universal.zip "Sabeel WhatsApp Tools.app"; \
		fi

e2e/node_modules/.installed: e2e/package-lock.json
	cd e2e && npm ci --no-fund --no-audit
	touch $@

e2e-deps: e2e/node_modules/.installed
	cd e2e && npx playwright install chromium

# Drives the full workflow against the demo build and saves screenshots.
e2e: build e2e-deps
	cd e2e && APP_BIN=../$(DIST)/linux/sabeel-whatsapp-tools npx playwright test

screenshots: build e2e-deps
	cd e2e && APP_BIN=../$(DIST)/linux/sabeel-whatsapp-tools SCREENSHOTS=../docs/screenshots npx playwright test

icons:
	uv run scripts/make-icons.py --brand ../brand

clean:
	rm -rf $(DIST) internal/webui/dist/assets internal/webui/dist/*.html internal/webui/dist/*.png
	rm -f cmd/sabeel-whatsapp-tools/rsrc_windows_*.syso
