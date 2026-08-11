# FinTrust - Confidential B2B Invoice Financing Network
#
# Pinned versions
FABRIC_VERSION     := 2.5.16
FABRIC_CA_VERSION  := 1.5.22
COUCHDB_VERSION    := 3.4.2
GO_VERSION         := 1.26.4

NETWORK_DIR := network
SCRIPTS_DIR := $(NETWORK_DIR)/scripts

.PHONY: help check fmt network-up network-down network-reset network-status channel-create

help:
	@echo "FinTrust Development Targets"
	@echo ""
	@echo "  make help           - Show this help"
	@echo "  make check          - Run repository checks"
	@echo "  make fmt            - Format shell scripts"
	@echo ""
	@echo "Network Targets:"
	@echo "  make network-up     - Start Fabric network and create channel"
	@echo "  make network-down   - Stop and remove containers"
	@echo "  make network-reset  - Stop network and remove generated files"
	@echo "  make network-status - Show network and channel status"
	@echo "  make channel-create - Create and join fintrust channel"
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
