# Frontend Apps

## Delivery Website (`frontend/miniapp`)
- React + Vite
- Features:
  - restaurant delivery storefront
  - category navigation and menu search
  - delivery / pickup mode switch
  - cart drawer and checkout draft flow
  - payment session request
  - addresses and order history

## Admin (`frontend/admin`)
- React + Vite
- Features:
  - stop-list screen (quick set unavailable/available)
  - manual review queue
  - manual resolution actions (confirm/cancel/refund)

## Run locally
- `make website-dev`
- `make admin-dev`

Or with Docker Compose:
- `docker compose up website admin`
