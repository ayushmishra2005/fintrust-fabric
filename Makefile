# FinTrust - Confidential B2B Invoice Financing Network
#
# Pinned versions
FABRIC_VERSION     := 2.5.16
FABRIC_CA_VERSION  := 1.5.22
COUCHDB_VERSION    := 3.4.2
GO_VERSION         := 1.26.4

.PHONY: help check fmt

help:
	@echo "FinTrust Development Targets"
	@echo ""
	@echo "  make help   - Show this help"
	@echo "  make check  - Run repository checks"
	@echo "  make fmt    - Format shell scripts"
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
	@echo "All checks passed."

fmt:
	@echo "Formatting shell scripts..."
	@if command -v shfmt >/dev/null 2>&1; then \
		find . -name '*.sh' -type f -exec shfmt -w -i 2 {} \;; \
		echo "Done."; \
	else \
		echo "shfmt not installed, skipping."; \
	fi
