# FinTrust - Confidential B2B Invoice Financing Network
#
# Pinned versions
FABRIC_VERSION     := 2.5.16
FABRIC_CA_VERSION  := 1.5.22
COUCHDB_VERSION    := 3.4.2
GO_VERSION         := 1.26.4

NETWORK_DIR := network
SCRIPTS_DIR := $(NETWORK_DIR)/scripts
CHAINCODE_DIR := chaincode/invoice
E2E_DIR := test/e2e

.PHONY: help check fmt network-up network-down network-reset network-status channel-create chaincode-deploy chaincode-status e2e verify-e2e

help:
	@echo "FinTrust Development Targets"
	@echo ""
	@echo "  make help             - Show this help"
	@echo "  make check            - Run repository checks"
	@echo "  make fmt              - Format shell scripts"
	@echo ""
	@echo "Network Targets:"
	@echo "  make network-up       - Start Fabric network and create channel"
	@echo "  make network-down     - Stop and remove containers"
	@echo "  make network-reset    - Stop network and remove generated files"
	@echo "  make network-status   - Show network and channel status"
	@echo "  make channel-create   - Create and join fintrust channel"
	@echo ""
	@echo "Chaincode Targets:"
	@echo "  make chaincode-deploy - Package, install, approve, and commit chaincode"
	@echo "  make chaincode-status - Show chaincode installation and commit status"
	@echo ""
	@echo "Testing Targets:"
	@echo "  make e2e              - Run E2E integration tests (requires running network)"
	@echo "  make verify-e2e       - Full clean E2E verification cycle"
	@echo ""
	@echo "Pinned Versions:"
	@echo "  Fabric:    $(FABRIC_VERSION)"
	@echo "  Fabric CA: $(FABRIC_CA_VERSION)"
	@echo "  CouchDB:   $(COUCHDB_VERSION)"
	@echo "  Go:        $(GO_VERSION)"

check:
	@echo "Running repository checks..."
	@echo ""
	@echo "Checking for trailing whitespace..."
	@! git grep -n '[[:blank:]]$$' -- ':!Makefile' ':!*.md' || (echo "Found trailing whitespace" && exit 1)
	@echo "OK"
	@echo ""
	@echo "Checking shell scripts..."
	@find . -name '*.sh' -type f 2>/dev/null | while read f; do \
		bash -n "$$f" || exit 1; \
	done
	@echo "OK"
	@echo ""
	@echo "Checking YAML syntax..."
	@for f in $$(find . -name '*.yml' -o -name '*.yaml' 2>/dev/null | grep -v '.git'); do \
		python3 -c "import yaml; yaml.safe_load(open('$$f'))" 2>/dev/null || \
		echo "Warning: could not validate $$f (python3/pyyaml not available)"; \
	done
	@echo "OK"
	@echo ""
	@echo "Validating Docker Compose..."
	@if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then \
		docker compose -f $(NETWORK_DIR)/compose/compose.yaml config --quiet && echo "OK"; \
	else \
		echo "Skipped (docker compose not available)"; \
	fi
	@echo ""
	@echo "Checking Go chaincode..."
	@cd $(CHAINCODE_DIR) && go mod tidy && go fmt ./... && go vet ./... && echo "OK"
	@echo ""
	@echo "Running chaincode unit tests..."
	@cd $(CHAINCODE_DIR) && go test -v ./...
	@echo ""
	@echo "Checking E2E test module..."
	@cd $(E2E_DIR) && go mod tidy && go fmt ./... && go vet ./... && echo "OK"
	@echo ""
	@echo "All checks passed."

fmt:
	@echo "Formatting shell scripts..."
	@if command -v shfmt >/dev/null 2>&1; then \
		find . -name '*.sh' -type f -exec shfmt -w -i 2 {} \;; \
		echo "Done."; \
	else \
		echo "shfmt not installed, skipping."; \
	fi

network-up:
	@$(SCRIPTS_DIR)/network-up.sh

network-down:
	@$(SCRIPTS_DIR)/network-down.sh

network-reset:
	@$(SCRIPTS_DIR)/network-reset.sh

network-status:
	@$(SCRIPTS_DIR)/network-status.sh

channel-create:
	@$(SCRIPTS_DIR)/channel-create.sh

chaincode-deploy:
	@$(SCRIPTS_DIR)/chaincode-deploy.sh

chaincode-status:
	@$(SCRIPTS_DIR)/chaincode-status.sh

e2e:
	@echo "Running E2E integration tests..."
	@cd $(E2E_DIR) && FINTRUST_E2E=1 go test -v -timeout 10m ./...

verify-e2e:
	@echo "=== Full E2E Verification Cycle ==="
	@$(MAKE) network-reset || true
	@$(MAKE) network-up
	@$(MAKE) chaincode-deploy
	@$(MAKE) e2e; E2E_RESULT=$$?; $(MAKE) network-down; exit $$E2E_RESULT
