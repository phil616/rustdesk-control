# Management protocol v1

Control URL is an HTTPS **origin**, without path, credentials, query or fragment.
The Go server exposes `/api/v1/agent/bootstrap`, `/enroll`, and `/heartbeat`.
All JSON responses are `Cache-Control: no-store`. Unknown request JSON fields,
oversized bodies (>64 KiB), invalid identifiers and invalid versions are rejected.

Enrollment and heartbeat use the same headers:

```
X-RDC-Device-ID: lowercase UUID v4
X-RDC-Timestamp: Unix seconds, decimal
X-RDC-Nonce: unique random value, 16–128 characters
X-RDC-Signature: standard padded Base64 Ed25519 detached signature
```

Sign the UTF-8 bytes below, with LF separators and **no trailing LF**:

```
HTTP_METHOD
REQUEST_PATH
TIMESTAMP
NONCE
lowercase_hex_SHA256(raw_request_body)
```

Query strings are rejected on signed endpoints. Enrollment additionally includes
`timestamp`, `device_uuid` and `public_key` in its JSON, matching the headers.
The public key is 32 bytes, standard Base64. The remaining enrollment metadata
and heartbeat fields follow TASK.md. The exact raw JSON bytes are signed; JSON
field ordering is not canonicalized by the server.

Time skew is limited to ±300 seconds. Nonces are inserted transactionally into
SQLite and retained beyond the complete acceptance window. They survive server
restarts. Existing UUID/key pairs enroll idempotently without changing approval;
changing the key requires an explicit administrator identity reset.

Bootstrap returns `protocol_version=1`, `server_time`, `policy_version`, and
`rustdesk`. Heartbeat adds `device_status` and, only for approved devices,
`managed_access`. Every approved heartbeat includes the current credential so a
restarted or reinstalled runtime can reapply it. `managed_access` is completely
absent for pending/rejected devices. Credentials never appear in inventory APIs.

The agent reports versions only after its host adapter accepts the configuration.
After restart it re-applies received state. Secrets are never written to the
agent's `state.json`; RustDesk persists its own password hash/storage through its
existing internal configuration implementation.

Admin login accepts `{ "password": "..." }`. The response returns `csrf_token`
and sets a Secure, HttpOnly, SameSite=Strict server-session cookie. Retrieve the
current CSRF token with `GET /api/v1/admin/session`. All authenticated mutations
require `X-CSRF-Token`, and cross-origin requests are rejected. Password changes
use `PUT /api/v1/admin/password` with `current_password` and `new_password` and
invalidate all sessions.

List queries: `page`, `page_size` (1–100), `search` (ID/hostname), `status`
(pending/approved/rejected), `online` (true/false). Audit supports pagination.

Reject/delete/reset stop future delivery. They do **not** erase a password from
an offline device or terminate existing RustDesk sessions. Reset opens one UUID
for a new self-signed enrollment, requiring approval again. Investigate the new
key/device before approving; there is deliberately no shared enrollment secret.
