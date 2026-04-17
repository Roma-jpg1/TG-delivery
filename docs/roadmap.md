# Delivery Roadmap

## Phase 0 — Discovery
- Fix restaurant topology (single/multi-branch).
- Choose payment provider and refund policy.
- Define exact delivery zones and pricing model.

## Phase 1 — Platform skeleton
- API + worker runtime.
- Config, logging, healthchecks.
- Docker + local compose.

## Phase 2 — Database foundation
- Core tables, constraints, indexes.
- Order state machine enums.
- Outbox/inbox + audit tables.

## Phase 3+ (feature sequence)
1. Menu + branch pricing/availability + stop-list.
2. Cart + checkout pre-validation.
3. Payment integration + webhook ingestion.
4. Saga orchestration + compensation.
5. Telegram bot + Mini App integration.
6. Admin panel.
7. Monitoring/alerts and hardening.
