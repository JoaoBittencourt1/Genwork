# GitHub integration

GitHub pull requests can move gentasks tasks between columns on their own:

| On GitHub | The task moves to |
| --- | --- |
| A pull request citing the task is opened (also when it is reopened, edited, or marked ready for review) | the board's "pull request opened" column |
| A review is requested on that pull request | the board's "review requested" column |
| That pull request is merged into a final branch (`main` and `master` by default) | the board's "merged" column (the board's Done column by default) |

A pull request cites a task when its **title or branch name** contains the task key, such as `PLA-123: sign in` or `feat/pla-123-sign-in`. Upper or lower case does not matter. Keys in the pull request description are ignored.

## Before you start

- **GitHub must reach your Genwork over HTTPS.** Your installation needs a public address, from a domain with TLS or from a tunnel. `GENWORK_PUBLIC_URL` in `deploy/.env` must be that address, such as `https://genwork.example.com`. GitHub cannot deliver events to `localhost` or a private network address.
- The **gentasks** module is installed (`genwork modules`).
- You are an **administrator**. Connecting GitHub needs the `gentasks.github.manage` permission, which only admins have by default.

## 1. Update the environment

Update the `genwork` CLI first (`make install-cli` in a source checkout), then run `genwork up`. An older CLI adds an empty `GENTASKS_ENCRYPTION_KEY` and gentasks will not start. `genwork up` adds three variables to `deploy/.env`:

| Variable | Value |
| --- | --- |
| `GENTASKS_ENCRYPTION_KEY` | Generated automatically. It encrypts the GitHub App's credentials. **Never change it**: the stored credentials become unreadable and you have to connect again. |
| `GITHUB_WEB_URL` | `https://github.com` |
| `GITHUB_API_URL` | `https://api.github.com` |

gentasks refuses to start if any of the three is missing.

## 2. Connect GitHub

1. Open **Tasks** and click **GitHub** at the top right.
2. If the page warns that GitHub cannot reach this installation, fix `GENWORK_PUBLIC_URL` first (see [Before you start](#before-you-start)).
3. Under **GitHub organization (optional)**:
   - leave it empty if your repositories are on your personal account;
   - enter the organization login (for example `acme`) if they belong to an organization. A private GitHub App can only be installed on the account that owns it.
4. Click **Connect GitHub**. GitHub opens with a pre-filled App: read-only access to pull requests, with the webhook pointing at your Genwork.
5. If GitHub says the name is taken, change it on that page. Then click **Create GitHub App**.
6. GitHub sends you back to Genwork, which shows **GitHub connected**.

You never copy a secret by hand: Genwork receives the App's credentials from GitHub and stores them encrypted.

## 3. Choose the repositories

1. Click **Choose repositories**.
2. On GitHub, pick **Only select repositories** (or all of them) and click **Install**.
3. GitHub brings you back to the GitHub page in Genwork.

To add or remove repositories later, click **Choose repositories** again.

## 4. Configure each board

New boards only have Backlog, To-do, and Done. Before you start, add the columns you want tasks to move into, such as **In progress** and **In review**. Order them from left to right, because tasks only ever move to the right.

1. Open the board and click the gear icon (**Settings**).
2. In the **GitHub** section, pick a column, or **Do not move**, for each event:
   - **When a pull request is opened**
   - **When a review is requested**
   - **When merged into a final branch**
3. Under **Final branches**, list the branches whose merges count as done, such as `main` or `staging`. Branch names are case-sensitive.
4. Click **Save**.

Boards that are never configured still move tasks to Done when a pull request is merged into `main` or `master`. They ignore the other events.

## 5. Use it

Put the task key in the pull request title or branch name:

```text
git checkout -b feat/pla-123-sign-in
```

From then on:

- tasks only move **forward**, to a column further right. An event that would move a task back does nothing;
- once a merge moved a task, the integration **never moves it again**, even if someone drags it back;
- a pull request closed without merging moves nothing;
- a pull request can cite several tasks, even on different boards, and each one follows its own board's settings;
- people can still move any task by hand.

## Troubleshooting

**Every gentasks request fails with 502 after updating.** gentasks did not start. If `docker logs genwork-gentasks-1` says `missing required environment variables: GENTASKS_ENCRYPTION_KEY`, the `genwork` CLI is older than gentasks. Update it (`make install-cli`) and run `genwork up` again: it fills in the key.

**Tasks do not move.** On GitHub, open the App's settings (**App settings on GitHub** on the Genwork GitHub page), then **Advanced → Recent Deliveries**, and look at the response code:

| Response | Meaning |
| --- | --- |
| No deliveries | The App is not installed on that repository, or GitHub cannot reach `GENWORK_PUBLIC_URL`. |
| `401 invalid_signature` | The connection was removed or recreated, or the encryption key changed. Connect again. |
| `204` but the task stayed | The key is not in the title or branch, the board's column for that event is **Do not move**, the target column is to the left of the task, the task is archived, or a merge already moved it. |

**"The GitHub credentials can no longer be read".** `GENTASKS_ENCRYPTION_KEY` changed. Click **Disconnect**, delete the old App on GitHub, and connect again.

**The App cannot be installed on the organization's repositories.** It was created on your personal account. Disconnect, delete the App on GitHub, and connect again with the organization filled in.

## Disconnecting

1. On the Genwork GitHub page, click **Disconnect**. Genwork deletes the App's credentials and stops moving tasks. Board settings are kept for when you connect again.
2. On GitHub, open the App's settings and delete it under **Advanced → Delete GitHub App**. Genwork does not delete it for you.
