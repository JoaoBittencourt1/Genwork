## Why

Three gaps showed up once bots were reviewing pull requests every day:

1. A task's bot conversation only grows. People cannot start over, and every run's messages, tool calls, and status entries stay in the database forever.
2. Disk fills up. Every release leaves its Genwork images on the server and its `genwork-sandbox` image (1–2 GB with the provider CLIs) on every runner, and nothing ever removes them. Ended sign-ins and used OAuth authorizations also pile up in genagents.
3. The conversation shows noise and hides results. A terminal escape the provider CLI prints on exit (`ESC[?25h`) appears as a bot message (`[?25h`). Every result entry shows "(no text)", because the runner sends a `summary` and the web app reads `text`.

## What Changes

1. **Clear a task's bot conversation (genagents, web).** A "Clear conversation" action on the task's bot panel stops the active run, if any, and deletes the conversation's entries and runs from the database. This is a documented exception to rule 22: the person asked for the data to go.
2. **Free up space (core, cli/updater, genagents, runner, web).**
   - **Server → Free up space:** shows the images of earlier Genwork versions on the server and their size, and removes them. Core relays the request to the updater, which removes only Genwork component images from the configured registry whose tag is a version other than the installed one.
   - **AI → Runners → Free up space:** removes genagents' transient rows (sign-ins ended over 7 days ago, OAuth authorizations used or expired) and asks every runner to clean up through its heartbeat. A runner removes `genwork-sandbox` images of other versions and leftover workspaces, then reports what it freed. A runner also removes old sandbox images when it starts.
3. **Conversation display (runner, web).** The runner drops terminal control sequences from CLI output. Result entries show their summary, and a review result links to the reviews it posted.
4. **Version 2.3.0.**

Components touched: **core** (cleanup relay), **cli** (updater cleanup), **genagents** (conversation clearing, transient rows, runner cleanup requests), **runner** (cleanup, output filtering), **web**. The span follows ownership. Only the updater may touch the server's Docker (rule 84). Each module deletes its own data (rule 53). Each runner owns its machine.

### Constitution

Rule 84 gains one permitted action: besides installing releases, the updater removes, when core asks, images of Genwork components from the configured registry whose tag is a version other than the installed one. It still accepts no other image or command.

## Capabilities

### New Capabilities

- `genagents-conversation-clearing`: clearing a task's bot conversation.
- `platform-storage-cleanup`: freeing disk space on the server and the runners, and removing transient rows.
- `genagents-conversation-display`: readable bot output and result entries.
