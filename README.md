# Evotor AI CLI (PoC)

CLI PoC for an LLM-assisted interface over Evotor API (read-only). The implementation follows the PRD in `prd.md` and uses a small, DI-based Go setup with standard `flag` parsing and Zap logging.

## Requirements
- Go 1.24+

## Quick Start
```bash
go build -o evotor-ai ./cmd/evotor-ai
./evotor-ai "Найди товар \"пиво\""
```

## REPL Mode
```bash
./evotor-ai
> Сумма продаж за январь
```

## Environment
Copy `.env.example` to `.env` and fill values as needed.

Required:
- `EVOTOR_TOKEN`
- `LLM_API_KEY`
- `LLM_MODEL`

Optional:
- `EVOTOR_STORE_ID`
- `LLM_BASE_URL`
- `DEBUG` (`true`/`false`)
- `LOG_FILE` (default `./evotor-ai.log`)
- `TIMEOUT` (e.g. `20s`)

## Commands
- Build: `go build -o evotor-ai ./cmd/evotor-ai`
- Test: `go test ./...`
- Lint (basic): `go vet ./...`

## Flags
- `--token` Evotor API token (overrides `EVOTOR_TOKEN`)
- `--store-id` default store ID
- `--from` / `--to` date range (YYYY-MM-DD)
- `--json` JSON output
- `--debug` debug logging
- `--log-file` log path (default `./evotor-ai.log`)
- `--timeout` timeout in seconds
- `--llm-base-url`, `--llm-api-key`, `--llm-model`

## Project Layout
- `cmd/evotor-ai/` CLI entrypoint
- `internal/` app modules (config, logging, CLI)
- `docs/plan.md` PRD execution plan
- `docs/tools.md` Tools list and service separation
- `docs/agent_flow.md` Agent behavior for PRD scenarios
- `tests/request.hurl` Evotor API smoke checks
- `tests/request.http` REST client smoke checks (same requests as Hurl)

## Notes
- Current output is a placeholder until tool integrations and LLM orchestration are implemented.
- Commit style follows `evotor-notes-backend` (e.g. `feat: ...`, `docs: ...`, optional `[scope] ...`).
 - If a month is provided without a year, the CLI uses the текущий год and states it explicitly (both REPL and one-shot).
