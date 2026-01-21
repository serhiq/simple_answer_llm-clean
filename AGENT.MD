# Repository Guidelines

## Project Overview
CLI PoC для LLM-ассистента над Evotor API (read-only). Требования и сценарии описаны в `prd.md`. Репозиторий сейчас минимальный, структура будет добавляться по мере реализации.

## Project Structure & Module Organization
- `cmd/evotor-ai/` — точка входа CLI (Cobra, REPL + one-shot).
- `internal/` — бизнес-логика, агентный цикл, формат ответов, роутер тулов.
- `internal/evotor/` — клиент Evotor API и реализации T1–T4.
- `configs/` — шаблоны конфигурации (например, `.env.example`).
- `test/` или `internal/.../*_test.go` — тесты и тестовые запросы (`tests.md`, `request.http`).

## Build, Test, and Development Commands
- `go build -o evotor-ai ./cmd/evotor-ai` — сборка бинарника.
- `go test ./...` — запуск всех тестов.
- `go vet ./...` — базовая статическая проверка.

## Coding Style & Naming Conventions
- Форматирование: `gofmt` (обязательно перед коммитом).
- Имена: Go-стиль (`CamelCase` для типов, `mixedCaps` для функций).
- Пакеты и файлы: короткие, отражающие назначение (`agent`, `evotor`, `cli`).
- Для зависимостей и дефолтов ориентироваться на базовые репозитории:
  - `/mnt/qqq/code_base/dev/go/evotor-notes-backend`
  - `/mnt/qqq/code_base/dev/go/voice-order-android-backend-clean`

## Testing Guidelines
- Тесты в `*_test.go`, табличные кейсы предпочтительны.
- Покрыть: парсинг CLI, планирование тулов, обработку ошибок API.
- Полные сценарии из PRD фиксировать в `tests.md` и `request.http`.

## Commit & Pull Request Guidelines
- Формат коммитов по истории `/mnt/qqq/code_base/dev/go/evotor-notes-backend`:
  - `feat: краткое описание`
  - `ci: ...`, `docs: ...`
  - опционально: `[scope] message`
  - пример: `feat: add tool router`, `[core] migrate to go-core-fx stack`
- PR: краткое описание, список изменений, результаты `go test`, ссылки на задачи/PRD.

## Work Plan
См. подробный план в `docs/plan.md`.

## Configuration & Secrets
- Env: `EVOTOR_TOKEN`, `EVOTOR_STORE_ID`, `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`.
- Токены не коммитить; использовать `.env.example`.
