# ERD (MVP + Production Baseline)

```mermaid
erDiagram
    users ||--o{ addresses : has
    restaurants ||--o{ branches : has
    restaurants ||--o{ categories : has
    restaurants ||--o{ menu_items : has
    categories ||--o{ menu_items : groups
    branches ||--o{ branch_menu_items : prices_and_availability
    menu_items ||--o{ branch_menu_items : branch_variant

    users ||--o{ carts : owns
    carts ||--o{ cart_items : contains
    menu_items ||--o{ cart_items : referenced

    users ||--o{ orders : places
    branches ||--o{ orders : serves
    carts ||--o| orders : source
    orders ||--o{ order_items : snapshot
    orders ||--o{ order_status_history : transitions

    orders ||--o{ payments : paid_by
    payments ||--o{ refunds : compensated_by

    orders ||--o{ saga_instances : orchestrated
    saga_instances ||--o{ saga_steps : contains

    orders ||--o{ courier_assignments : delivered_by
    couriers ||--o{ courier_assignments : assigned

    outbox_events }o--|| orders : aggregate_optional
    inbox_events }o--|| payments : source_optional

    branches ||--o{ menu_item_availability_log : logs
    menu_items ||--o{ menu_item_availability_log : logs

    audit_log }o--|| branches : optional
```

## Notes
- `branch_menu_items` is the source of branch-specific price + stop-list status.
- `orders` and `order_items` keep immutable snapshots for historical correctness.
- `outbox_events` and `inbox_events` are mandatory for reliable async processing.
- `saga_instances`/`saga_steps` model orchestration and compensations.
