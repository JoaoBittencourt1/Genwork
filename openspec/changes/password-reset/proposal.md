## Why

A person who forgets their password has no way back into the instance: only an admin can invite them again. The core already sends e-mail through the configured SMTP settings (invitations), so self-service recovery needs no new infrastructure.

## What Changes

- The sign-in screen links to "Forgot your password?", which asks for an e-mail.
- If an active account has that e-mail, core sends a single-use link valid for 1 hour, through the same SMTP configuration invitations use. The response is always the same, so it never reveals whether an account exists (rule 68).
- Opening the link lets the person choose a new password (8 to 128 characters, rule 66). Saving it consumes the link, revokes every session of the account, and lifts any failed-login lockout; the person then signs in again.
- Link secrets are stored only as SHA-256 hashes (rule 67). A new request invalidates the previous link, and one account receives at most one e-mail per minute. Requests are rate limited per client address.

Components touched: **core** (migration `password_resets`, `internal/passwordreset`, e-mail templates in pt-BR and en, `users.Service.ChangePassword`) and **web** (`web/src/app/passwordreset/`, routes `/forgot-password` and `/reset-password/$secret`). No module and no other client is touched.

## Non-goals

- Changing the password while signed in.
- Sending the link through any channel other than e-mail.
