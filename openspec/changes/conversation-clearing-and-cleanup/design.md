## Decisions

### 1. Clearing a conversation

`DELETE /api/genagents/v1/tasks/{taskToken}/conversation` needs `genagents.run.write`, the permission to talk to the bot. It also needs the caller to read the task through gentasks with their own token, as reading the conversation does. In one transaction it:
1. cancels every active run of the conversation (cause `user`);
2. deletes all its entries;
3. deletes its runs, except the runs it just cancelled while a runner still holds them;
4. deletes the conversation row when no run is left.

A runner holding a cancelled run sees the `cancelled` state on its next poll and stops. Events it posts after the clear are refused, because the run is no longer active. A later run creates a new conversation. A task without a conversation answers 204.

Rows are deleted physically: a documented exception to rule 22, since the person explicitly asked for the history to go. Nothing else references a conversation or its runs outside genagents (rule 53).

### 2. Freeing space on the server

- **Updater:**
  - `GET /v1/cleanup` lists outdated images: images whose repository is `<GENWORK_IMAGE_REGISTRY>/genwork-<component>`, whose tag is a semantic version, and whose tag is not the installed version. It returns their id, reference, and size in bytes, read from `docker image inspect`.
  - `POST /v1/cleanup` removes those images with `docker image rm` and reports the count and bytes freed. An image still used by a container is skipped and reported, never forced.
  - Development tags (`:dev`) and any image outside the registry and its `genwork-` names are never touched (rule 84 amendment).
- **Core:** `GET` and `POST /api/core/v1/system/cleanup` (`core.system.update`) relay to the updater with the shared secret, like updates.
- **Web:** a "Free up space" card on the Server page shows the reclaimable size and a button.

### 3. Freeing space on runners and in genagents

- `POST /api/genagents/v1/runners/cleanup` (`genagents.runner.manage`):
  - deletes `account_sign_ins` that ended over 7 days ago, and `mcp_server_oauth_authorizations` that were consumed or expired over 1 day ago;
  - sets `runners.cleanup_requested_at` on every runner that is not revoked;
  - returns the counts.
- The heartbeat answer carries `cleanupRequested` while `cleanup_requested_at` is later than the runner's last cleanup.
- The runner then:
  - removes `genwork-sandbox` images whose tag is not its own sandbox image's tag, skipping any in use;
  - removes workspace directories that belong to no active run;
  - reports `lastCleanup: {at, freedBytes}` in its next heartbeat, which clears the request.
- A runner also removes outdated sandbox images when it starts, right after it confirms the current image.
- The runners list shows each runner's last cleanup and bytes freed.

### 4. Display

- The stream parsers strip terminal control sequences from non-JSON lines and drop a line that is empty afterwards. The stripping function moves from the sign-in package to `adapters`, so one function serves both (rule 7).
- Web: a result entry shows `text`, falling back to `summary`. A result with `reviews[].url` lists the links to the posted reviews.

## Risks

- Removing an image the stack needs: the updater never removes the installed version's tag, and the engine refuses to remove an image a container uses. The cleanup reports such images as skipped.
- A runner without network access cannot pull an image again once it is removed. Only images of other versions are removed.
