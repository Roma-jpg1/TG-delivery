# Frontend Apps

## Mini App (`frontend/miniapp`)
- React + Vite
- Features:
  - fetch branch menu
  - manage cart
  - create checkout draft
  - request payment session

## Admin (`frontend/admin`)
- React + Vite
- Features:
  - stop-list screen (quick set unavailable/available)
  - manual review queue
  - manual resolution actions (confirm/cancel/refund)

## Run locally
- `make miniapp-dev`
- `make admin-dev`

Or with Docker Compose:
- `docker compose up miniapp admin`
