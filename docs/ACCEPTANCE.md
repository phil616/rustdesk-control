# Windows x64 客户端验收

These are real-machine acceptance tests, not claims of completed validation.
Record build hashes, OS version, device UUID, RustDesk ID, timing and audit IDs;
never record a password or private key in the report.

## Windows x86_64

1. Build a signed/packaged modified 1.4.9 with the deployment's HTTPS control
   origin and the published corresponding source URL. Deploy official OSS
   hbbs/hbbr separately and save valid server parameters in Settings.
2. Install using the normal visible RustDesk installer; keep OS security prompts.
   Confirm the original RustDesk service exists and no extra service was created.
3. Without entering Control URL or RustDesk Server parameters, verify a pending
   device appears, with its hostname, OS, versions and RustDesk ID.
4. Open About and verify the modified/managed notice, Connected status, and
   functional Source Code link. Ordinary users cannot edit the control origin.
5. Before approval verify there is no managed password response. Approve in the
   console and wait a normal heartbeat. Check policy/password synchronization.
6. Reveal the managed password and connect using an ordinary RustDesk client and
   the shown ID. Verify original remote desktop functionality and safety prompts.
7. Verify ordinary UI and `--password` cannot change the managed password.
   Rotate from the console, wait for acknowledgement, verify new password works
   and the old password no longer authenticates a new connection.
8. Close the GUI; verify heartbeat continues. Restart the OS; repeat login and
   heartbeat verification without manually opening the GUI.
9. Test locked screen and RDP session transitions on Windows, including stable
   service identity and configuration synchronization.
10. Attempt overlapping RustDesk processes. Confirm only one effective agent uses
    the single service identity and the lock is released after process exit.

## Offline and policy change

1. Stop only rustdesk-control while leaving hbbs/hbbr running. Confirm existing
   server settings and managed password still work for a new normal connection.
2. Restore the control plane. Confirm agent recovery through bounded retry,
   no new device identity, and heartbeat/version acknowledgement.
3. Change ID/Relay/Key to another valid OSS server. Verify policy increments and
   every connected agent receives it within a normal heartbeat cycle.
4. Reject a device and verify later heartbeats omit managed_access. Existing local
   offline access is deliberately not erased; rejection is not remote revocation.
5. Reset Identity; confirm requests under the old key are rejected until new
   enrollment, the device returns to pending, and approval is required again.
6. Restore a consistent SQLite backup with the matching master key, restart
   clients, and verify policy and password converge even if versions went back.

## Persistence / platform security

Inspect identity directory permissions/ACL as a non-administrator. Private keys
must not be readable. Ensure copied machine images are prepared without an
existing managed identity; cloning an enrolled installation duplicates its key.
Verify the service account retains access after reboot and updates.

Exercise disk-full / unwritable RustDesk config storage. The client must not
acknowledge an unapplied password. Inspect only versions and errors, never secrets.
Confirm HTTPS rejects an untrusted CA, changed hostname and HTTP redirect.
