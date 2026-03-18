PLUGINS := $(wildcard plugins/*)
BIN_DIR := dist

.PHONY: build test e2e coverage lint ci install clean

build: $(BIN_DIR)
	@for plugin in $(PLUGINS); do \
		echo "==> Building $$plugin..."; \
		$(MAKE) -C $$plugin build BIN_DIR=../../$(BIN_DIR); \
	done

test:
	@for plugin in $(PLUGINS); do \
		echo "==> Testing $$plugin..."; \
		$(MAKE) -C $$plugin test || exit 1; \
	done

e2e:
	@for plugin in $(PLUGINS); do \
		echo "==> Running e2e tests for $$plugin..."; \
		$(MAKE) -C $$plugin e2e || exit 1; \
	done

coverage:
	@for plugin in $(PLUGINS); do \
		echo "==> Coverage for $$plugin..."; \
		$(MAKE) -C $$plugin coverage || exit 1; \
	done

lint:
	@for plugin in $(PLUGINS); do \
		echo "==> Linting $$plugin..."; \
		$(MAKE) -C $$plugin lint || exit 1; \
	done

ci: lint test e2e

install:
	@for plugin in $(PLUGINS); do \
		echo "==> Installing $$plugin..."; \
		$(MAKE) -C $$plugin install; \
	done

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

clean:
	rm -rf $(BIN_DIR)
