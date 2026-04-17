# Security Checklist

- HTTPS everywhere (ingress and internal mTLS where available).
- Validate webhook secrets (`X-Telegram-Bot-Api-Secret-Token` / provider secret).
- RBAC for admin endpoints.
- Rate limiting for public + webhook endpoints.
- PII masking in logs.
- Secrets from vault/secret manager only (no plaintext in repo).
- Token/key rotation policy.
- Encrypted backups + restore drill.
- Audit logging for all privileged actions.
