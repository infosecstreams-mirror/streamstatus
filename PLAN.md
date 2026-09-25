# StreamStatus Task Plan

## Active Sprints

### Sprint 1: PostgreSQL REST API Migration
- [x] Create backend abstraction interface (`store.go`)
- [x] Implement `PostgresStore` backend (`postgres_store.go`)
- [x] Implement `CsvStore` fallback backend (`csv_store.go`)
- [x] Rip out `go-git` dependencies from `main.go` and `StreamStatus.go`
- [x] Create `GET /api/status` endpoint for frontend hydration
- [x] Create `POST /api/streamers` admin endpoint for adding new streamers
- [x] Wire Twitch EventSub webhook subscriptions into the new `POST /api/streamers` endpoint

## Notes
- We are migrating away from the Git-based `data` branch to a proper REST API.
- The `wupinyin-iac-go` project in the private workspace manages the actual container and reverse proxy. Keep this repo strictly focused on the Go application logic.
