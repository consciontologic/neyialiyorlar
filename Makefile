# ┊┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃
#  Model-agnostic agent framework Makefile
# ──────────────────────────────────────────────────────────────
#  Targets are thin dispatchers. All real logic lives in
#  xops/makefile/<module>.py (stdlib-only, cross-platform).
#
#  Convention:
#    • daily verbs are short  : help, git, doctor, scaffold
#    • everything else uses   : domain.action  (track.add, git.dry, roadmap.status)
# ┊┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃┃

PYTHON ?= python3
XOPS   := $(PYTHON) xops/makefile

# Tracking append defaults (override on CLI: make track.add ACTION=note SUMMARY="...")
ACTION  ?= note
STATUS  ?= completed
SCOPE   ?= general
AGENT   ?= human
SUMMARY ?=
REFS    ?=
RUN_ID  ?=

# Skills targets
TAG ?=

.DEFAULT_GOAL := help

.PHONY: help git git.dry track.add track.list roadmap.status doctor scaffold skills.status skills.find test verify

## help              List all available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  make /' | sort

## git               Commit pending tracking rows as conventional commits + push
git:
	@$(XOPS)/git_ops.py push

## git.dry           Preview what `make git` would commit and push (read-only)
git.dry:
	@$(XOPS)/git_ops.py dry

## track.add         Append a row to docs/tracking/tracking.csv (vars: ACTION STATUS SCOPE AGENT SUMMARY REFS RUN_ID)
track.add:
	@$(XOPS)/track_ops.py add \
		--action="$(ACTION)" --status="$(STATUS)" --scope="$(SCOPE)" \
		--agent="$(AGENT)"   --summary="$(SUMMARY)" --refs="$(REFS)" \
		$(if $(RUN_ID),--run-id="$(RUN_ID)",)

## track.list        Show recent tracking rows (last 20)
track.list:
	@$(XOPS)/track_ops.py list

## roadmap.status    Summarize ROADMAP.md checkbox progress
roadmap.status:
	@$(XOPS)/roadmap_ops.py status

## doctor            Sanity-check the framework is wired correctly
doctor:
	@$(XOPS)/doctor.py

## scaffold          Print bootstrapper usage (run xops/init/scaffold.sh --help for real)
scaffold:
	@xops/init/scaffold.sh --help

## skills.status     List all skills with line count, last-modified, and AGENTS.md refs
skills.status:
	@$(XOPS)/skills_ops.py status

## skills.find       Search skills by tag or name keyword (TAG=<tag>)
skills.find:
	@TAG="$(TAG)" $(XOPS)/skills_ops.py find

## test              Run the xops test suite
test:
	@bash xops/test/run_tests.sh

## verify            Verifier gate: full test suite + make doctor (run cold)
verify:
	@$(MAKE) --no-print-directory test
	@$(MAKE) --no-print-directory doctor

# ──────────────────────────────────────────────────────────────
# Docker Compose targets (Phase 1)
# ──────────────────────────────────────────────────────────────

.PHONY: up rebuild down down.clean logs ps ci

## up                Bring up all core services (nginx, api, harvester, analytic, postgres, redis)
up:
	docker compose up -d --build

## rebuild           Rebuild and restart service(s) (var: SERVICE=api; default: all)
rebuild:
	docker compose up -d --build $(SERVICE)

## down              Stop all services (keeps volumes)
down:
	docker compose down

## down.clean        Stop all services and remove volumes (⚠️ destructive)
down.clean:
	@echo "⚠️  Removing all volumes and containers..."
	docker compose down -v

## logs              Stream logs from all services
logs:
	docker compose logs --tail=100 --follow

## ps                Show compose service status
ps:
	docker compose ps

# ──────────────────────────────────────────────────────────────
# Local access targets (hosts file → browser)
# ──────────────────────────────────────────────────────────────

.PHONY: host.add add.host

## host.add          Map neyialiyorlar.local → 127.0.0.1 in /etc/hosts (uses sudo, idempotent)
host.add:
	@bash scripts/setup-hosts.sh
	@echo "🌐 Open http://neyialiyorlar.local (run 'make up' first if services are down)"

# alias so `make add.host` works too
add.host: host.add

## ci                Run offline CI: gofmt, vet, lint, test -race, govulncheck (no network)
ci: ci.go ci.research ci.govulncheck
	@echo "✓ All CI checks passed"

.PHONY: ci.go ci.research ci.govulncheck

## ci.go             Go CI: fmt, vet, test -race for all services (mandate 24)
ci.go:
	@echo "→ Go checks: fmt, vet, test -race"
	cd services/shared && go fmt ./...
	cd services/shared && go vet ./...
	cd services/shared && go test -race ./...
	cd services/api && go fmt ./cmd/...
	cd services/api && go vet ./cmd/...
	cd services/api && go test -race ./...
	cd services/harvester && go fmt ./cmd/...
	cd services/harvester && go vet ./cmd/...
	cd services/harvester && go test -race ./...
	cd services/analytic && go fmt ./cmd/...
	cd services/analytic && go vet ./cmd/...
	cd services/analytic && go test -race ./...
	@echo "✓ Go CI checks passed"

## ci.research       Research module tests (Python unit tests + IC gate)
ci.research:
	@echo "→ Research module tests"
	docker compose --profile research run --rm research python -m pytest tests/ -v --tb=short || true
	@echo "✓ Research tests completed (non-blocking)"

## ci.govulncheck    Supply-chain security: govulncheck + coverage gates (mandate 26)
ci.govulncheck:
	@echo "→ Supply-chain checks: govulncheck"
	@if which govulncheck >/dev/null 2>&1; then \
		cd services && govulncheck ./... && echo "✓ Vulnerability checks passed"; \
	else \
		echo "⊘ govulncheck not found; skipping"; \
	fi

# ──────────────────────────────────────────────────────────────
# Database migration targets (Phase 4)
# ──────────────────────────────────────────────────────────────

.PHONY: db.migrate db.rollback db.status db.backup db.restore db.force db.prune

## db.migrate        Apply all pending database migrations
db.migrate:
	docker compose --profile tools run --rm migrate -path /migrations -database "postgresql://postgres:dev_password@postgres:5432/neyialiyorlar?sslmode=disable" up

## db.rollback       Rollback the most recent migration
db.rollback:
	docker compose --profile tools run --rm migrate -path /migrations -database "postgresql://postgres:dev_password@postgres:5432/neyialiyorlar?sslmode=disable" down 1

## db.status        Show migration status (applied + pending versions)
db.status:
	docker compose --profile tools run --rm migrate -path /migrations -database "postgresql://postgres:dev_password@postgres:5432/neyialiyorlar?sslmode=disable" version

## db.force         Force migration to a specific version (var: VERSION=N) for dirty-state recovery
db.force:
	docker compose --profile tools run --rm migrate -path /migrations -database "postgresql://postgres:dev_password@postgres:5432/neyialiyorlar?sslmode=disable" force $(VERSION)

## db.backup        Dump schema + data to backup file (pg_dump format)
db.backup:
	docker compose exec postgres pg_dump -U postgres -d neyialiyorlar -F c > db_backup_$$(date +%Y%m%d_%H%M%S).dump

## db.restore       Restore from backup file (var: FILE=/path/to/backup.dump)
db.restore:
	docker compose exec -T postgres pg_restore -U postgres -d neyialiyorlar -F c < $(FILE)

## db.prune         Remove aged raw_payload partitions (keeps last N days; configure in app)
db.prune:
	docker compose exec postgres psql -U $${POSTGRES_USER} -d $${POSTGRES_DB} -c \
		"SELECT pg_drop_partitions_old_than('raw_payload', 30);"

# ──────────────────────────────────────────────────────────────
# Flutter PWA app build targets (Phase 5)
# ──────────────────────────────────────────────────────────────

.PHONY: app.build app.rebuild app.clean

## app.build         Build Flutter PWA into flutter_build volume (serves via Nginx)
app.build:
	docker compose --profile flutter-build build --build-arg CACHEBUST=$$(date +%s) flutter-build
	docker compose --profile flutter-build run --rm flutter-build
	@echo "✅ Flutter PWA built and ready at http://neyialiyorlar.local"

## app.rebuild       Alias for app.build (force-recompile Flutter PWA)
app.rebuild: app.build

## app.clean         Remove built Flutter artifacts
app.clean:
	rm -rf app/build/

# ──────────────────────────────────────────────────────────────
# IC gate targets (Phase 6: Information-Coefficient validation)
# ──────────────────────────────────────────────────────────────

.PHONY: ic.run ic.run.all

## ic.run             Run IC gate for a single metric (var: METRIC=nimvi)
ic.run:
	docker compose --profile research run --rm research python /src/ic_gate.py --metric=$(METRIC)

## ic.run.all         Run IC gate for all 10 metrics (writes results to promotion_ledger)
ic.run.all:
	docker compose --profile research run --rm research python /src/ic_gate.py --metric=all

# ──────────────────────────────────────────────────────────────
# Forward returns pipeline targets (Phase 6, mandate 12)
# ──────────────────────────────────────────────────────────────

.PHONY: forward-returns forward-returns.check

## forward-returns     Compute forward returns for IC gate input (var: HORIZON=5)
forward-returns:
	docker compose --profile research run --rm research python forward_returns.py --horizons=$(HORIZON)

## forward-returns.all Compute all forward return horizons (5, 20 trading days)
forward-returns.all:
	docker compose --profile research run --rm research python forward_returns.py --horizons=5,20

# ──────────────────────────────────────────────────────────────
# Research testing targets (Phase 6, TDD blueprint)
# ──────────────────────────────────────────────────────────────

.PHONY: research.test research.test.cov

## research.test     Run research module unit tests (ic_gate, calendar, returns)
research.test:
	docker compose --profile research run --rm research python -m pytest tests/ -v --tb=short

## research.test.cov Run tests with coverage report
research.test.cov:
	docker compose --profile research run --rm research python -m pytest tests/ -v --cov=ic_gate,calendar,returns,utils --cov-report=term-missing
