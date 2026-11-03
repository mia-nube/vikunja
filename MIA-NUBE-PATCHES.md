# mia·nube patches for Vikunja

This branch carries the **modified** version of Vikunja that mia·nube deploys. It exists to
satisfy the AGPL-3.0 obligation described below, and to keep our changes reviewable as an
ordinary series of commits on top of a known upstream release.

Upstream project: <https://github.com/go-vikunja/vikunja> — © the Vikunja authors,
licensed **AGPL-3.0-only**. This fork is licensed under the same terms; see `LICENSE`.

## Licence and the source offer

Vikunja is licensed under the GNU Affero General Public License, version 3. Section 13 of that
licence requires that users interacting with a **modified** version **over a network** be
offered the complete corresponding source of that version, at no charge.

**This repository is that offer.** It is public, it can be cloned anonymously, and every commit
that is built and deployed is reachable here. The deployed build links to this repository from
its user interface so that any user can obtain the source of the exact version they are using.

Nothing here is optional or ceremonial: shipping a modified AGPL network service without a
working source offer is a licence violation.

## Branch layout

| branch | purpose |
| --- | --- |
| `main` | untouched mirror of upstream's default branch. **Never commit here.** |
| `mia-nube/<version>` | the patch series for a given upstream release, branched from that release's tag |

This branch, `mia-nube/2.7.1`, continues `mia-nube/2.4.0` (branched from the upstream tag
**`v2.4.0`**, commit `907850feae3866ae9b16ea1c7b84a4d77273415a`) and **merges** the upstream tag
**`v2.7.1`** (commit `5619c4a9f66ddfc13f7bbba24ada07c3e82ae7a0`) on top of its tip
`de889a3513217d692730dcac8eedcdc4a3abcea0`. It was merged rather than rebased so that every
published `mia-nube/2.4.0` commit — each one the corresponding source of a build that was
deployed — stays an ancestor of this branch with its SHA unchanged. Both `v2.4.0` and `v2.7.1`
are ancestors of this branch.

The base is pinned to an **exact commit**, never to a moving branch. A build from this fork
verifies that the commit it is asked to build genuinely descends from the recorded upstream
release tag, and fails if it does not — so a pin that does not match its claimed base is loud at
build time rather than a surprise at runtime.

## Modification-notice convention

AGPL-3.0 §5(a) requires modified files to carry prominent notices stating that they were changed
and when. Every commit on a `mia-nube/*` branch therefore follows these rules:

1. **Commit messages** begin with `mia-nube:` and explain *why* the change exists, not merely
   what it does.
2. **Modified upstream files** carry a notice near the top of the file, in the file's own
   comment syntax:

   ```
   Modified by mia·nube on <YYYY-MM-DD>: <one-line summary of the change>.
   ```

3. **New files** added by mia·nube carry the upstream AGPL header plus a line identifying them
   as mia·nube additions.
4. Changes are kept **upstream-shaped** — small, self-contained, and written so they could
   plausibly be offered upstream — rather than as one sprawling diff. This keeps rebases onto
   new upstream releases tractable.

## Patches carried by this branch

Keep this list current. On a rebase it is the checklist of what must be replayed; a patch that
quietly fails to come across is otherwise invisible until something breaks.

| # | files | what it does | why |
| --- | --- | --- | --- |
| 2 | `pkg/models/task_attachment.go`, `pkg/migration/20260731120000.go` | Adds *link attachments*: an attachment row may reference a file in an external system (`link_provider` + `link_ref`, plus cached name/size/mime) instead of owning a stored blob. `file_id` becomes nullable. | An attachment that is a live reference rather than a copy stays current when the original changes, and never duplicates the file or its access rules. Kept deliberately provider-agnostic and upstream-shaped: nothing in this patch names a particular external system. |
| 3 | `pkg/config/config.go`, `pkg/modules/linkattachments/providers.go`, `pkg/models/task_attachment.go`, `pkg/models/error.go`, `pkg/routes/routes.go`, `pkg/routes/api/v1/task_attachment.go`, `pkg/routes/api/v2/task_attachments.go`, `pkg/routes/api/shared/info.go`, `pkg/web/files/task_attachment.go` | Adds the link-attachment *endpoints*: a create-link route on both API versions, gated by the same task write permission an upload needs; a download route that redirects a link row to its provider; and a configured registry of providers, published on `/info` so a client knows which are available. | The model patch (2) made a link representable; this makes it reachable. Resolution is a redirect on purpose — the user's own browser goes to the provider, so the provider applies that user's permissions and Vikunja never holds a credential for it. Providers are configuration, so no external system is named in the source. |
| 4 | `frontend/src/components/tasks/partials/Attachments.vue`, `frontend/src/components/tasks/partials/LinkAttachmentPicker.vue`, `frontend/src/models/attachment.ts`, `frontend/src/modelTypes/IAttachment.ts`, `frontend/src/services/attachment.ts`, `frontend/src/stores/config.ts`, `frontend/src/types/ILinkAttachmentProvider.ts`, `frontend/src/i18n/lang/{en,de-DE}.json` | The attachment section gains an "attach from …" action per configured provider, which opens that provider's own picker in a modal and turns the chosen file into a link attachment. Link rows render with an icon and the provider's name, open at the provider, and are never previewed or used as a cover. | The point of the feature is that the user never leaves the task to attach a file. The picker is the provider's page in the user's own session, so nothing about its contents crosses into Vikunja except the identifier of the file the user picked — and that crossing is validated on origin *and* source frame before it is trusted. |
| 5 | `pkg/version/version.go`, `pkg/routes/api/shared/info.go`, `frontend/src/stores/config.ts`, `frontend/src/views/About.vue`, `frontend/src/i18n/lang/{en,de-DE}.json` | Adds the **AGPL §13 source offer to the About dialog**, beside the version. A new link-time variable `version.Commit` records the source revision the binary was built from; `/info` publishes it as `commit`; About renders the offer and links to that exact revision. Empty commit renders nothing. | §13 requires that a user interacting with a modified version over a network be offered its corresponding source. The revision is taken from what the running build reports about **itself**, never restated as prose in a template, so the offer cannot drift away from the build actually serving the user — which is the failure mode a hand-written notice has by construction. A plain hyperlink, so the dialog loads nothing from a third party. |
| 1 | `Dockerfile` | Removes the BuildKit-only constructs: `FROM --platform=$BUILDPLATFORM` on both builder stages, the implicit `TARGETOS`/`TARGETARCH`/`TARGETVARIANT` args (now defaulted), `COPY --chmod=`, and the `# syntax=` parser directive. | Upstream's Dockerfile can only be built by BuildKit. The CI runner that builds our image offers no usable BuildKit backend and forbids privileged containers, so without this patch the image cannot be built at all. The resulting image is unchanged, and building under BuildKit still works. |
| 6 | `frontend/src/components/home/UpdateNotification.vue` | Guards the `controllerchange` handler so it only reloads the document when the page already had a service-worker controller **before** the listener was registered. | `clientsClaim()` in `sw.ts` makes the very first service-worker activation claim the already-loaded page, which fires `controllerchange` on **every first visit** for a reason that has nothing to do with an update — and the unguarded handler reloaded the document unconditionally, discarding anything held only in client state. The distinguishing state is captured at registration time, before the event can fire; reading `navigator.serviceWorker.controller` inside the handler is populated in both cases and discriminates nothing. The user-initiated update path (page was controlled → user clicks Update → `skipWaiting` → reload) is left unchanged. |
| 7 | `frontend/src/main.ts`, `frontend/src/sentry.ts` (removed), `frontend/vite.config.ts`, `frontend/env.d.ts`, `frontend/package.json`, `pkg/routes/static.go`, `pkg/config/config.go`, `config-raw.json` | Removes the frontend Sentry integration entirely: the `window.SENTRY_ENABLED`-gated dynamic import in `main.ts` and the `sentry.ts` module it loaded, the `@sentry/vite-plugin` wiring and `sentry` manual chunk in `vite.config.ts`, the unread `VIKUNJA_SENTRY_*`/`SENTRY_*` env types, the `@sentry/vue`/`@sentry/vite-plugin` dependencies, and the backend `window.SENTRY_ENABLED`/`window.SENTRY_DSN` injection plus the `sentry.frontendenabled`/`sentry.frontenddsn` config keys that fed it. | The flag was `false` in production and nothing was leaking, but it was a single boolean in shipped JavaScript with no second line of defence — the deployment serves no Content-Security-Policy — and the full `@sentry/vue` SDK (454 KB) was served as a fetchable static asset regardless of the flag's value. The backend-only `sentry.enabled`/`sentry.dsn` keys (API error tracking, never reaches a user's browser) are untouched. |

| 8 | `frontend/src/i18n/lang/de-DE.json` | Modified by mia·nube on 2026-10-16: rewrites the AGPL §13 source offer in the About dialog (`about.sourceOffer.modified`) in the du-form, so the German sentence addresses the user as du like every other German string in the product. No other key changes. | The source offer is product copy, not a legal text: in an open-source project the legal texts are the licence file and the file headers, and both are English. The sentence is a UI string that tells the user what they are entitled to, so it follows the product's register rule. The §13 content (component, AGPL-3.0, modified status, fork URL, commit) is unchanged; only the address changes. JSON has no comment syntax, so this row is the §5(a) notice for this file. |

### Changes made on the v2.7.1 merge (2026-11-03)

Upstream v2.5–v2.7 rewrote several areas this series patches. Each patch was carried across
by intent, not by text; where the upstream shape changed, the patch now lives in the upstream
successor and the file carries a dated notice.

| # | what changed on the merge |
| --- | --- |
| 1 | Builder images follow v2.7.1's pins (node 24.21.0, xgo go-1.27.x); the BuildKit-only constructs stay removed. |
| 2 | `pkg/migration/20260731120000.go` uses `partialSync` instead of `tx.Sync2`: a plain sync of the partial struct dropped every `task_attachments` index it does not declare (upstream #3244), and would undo upstream's index repair `20260720120000`, which sorts before it. Guarded by `pkg/migration/20260731120000_test.go`. |
| 3 | Upstream replaced `LoadTaskAttachmentForDownload` with `GetTaskAttachmentForDownload` / `GetTaskAttachmentPreview`. A link row now surfaces from the download path as `ErrTaskAttachmentIsLink` (access already checked), which both API versions turn into the provider redirect; the preview path reports a link as not previewable (422, code 4038). The automatic merge had moved the v2 redirect into the *preview* handler. Error codes renumbered **4029 → 4090** (`ErrCodeUnknownLinkAttachmentProvider`) and **4030 → 4091** (`ErrCodeInvalidLinkAttachment`), because v2.7.1 assigned 4029/4030 to task errors. `/info` keeps `commit` and `link_attachment_providers`. Guarded by `pkg/webtests/link_attachment_test.go`. |
| 4 | Upstream removed `models/attachment.ts`, `modelTypes/IAttachment.ts` and `services/attachment.ts` in favour of the generated API client. The link guards (`isLink`, no preview/cover) moved to `frontend/src/helpers/attachmentPreview.ts`, the create-link call to a mutation in `frontend/src/client/queries/attachments.ts`, and `frontend/src/client/generated/` was regenerated from the merged API (`taskAttachmentsCreateLink`, link fields). Wire fields are snake_case (`link_provider`, `link_url`, `picker_url`); the config store drops providers without a navigable picker URL. Guarded by `frontend/src/helpers/linkAttachment.test.ts`. |
| 5 | `frontend/src/stores/config.ts` now derives its state from the generated `VikunjaInfos`, which carries `commit`; `About.vue` keeps the offer beside upstream's new "Pro active" line. |
| 6 | Upstream v2.5.0 added its own guard (reload only after the user opted into an update via `refreshApp()`). Both guards are kept. |
| 7 | Upstream's reworked `frontend/src/sentry.ts` and its new `sentry.test.ts` are removed; the `@sentry/vue` / `@sentry/vite-plugin` dependencies, the plugin wiring and the source-map switch stay out; the generated plugin symbols for the removed `sentry.frontend*` config keys are dropped from `pkg/yaegi_symbols/vikunja_config.go`. `frontend/src/helpers/sentryFilters.ts` is kept: it is a pure filter helper with no SDK and no network access, and upstream's chunk-load reload handler imports it. The merge commit carries upstream's `frontend/pnpm-lock.yaml` unchanged, so it still lists the Sentry packages and does not match `package.json`; it is regenerated with pnpm 11.27.0 in a follow-up commit before any build. |
| 8 | Unchanged; the du-form source offer survives the locale merge. |

## Rebasing onto a new upstream release

Since v2.7.1 the series is carried forward by **merging** the new upstream tag into a new
`mia-nube/<new-version>` branch made from the previous branch's tip, which keeps every published
SHA reachable. The cherry-pick route below is the historical procedure. Either way it is
deliberate, reviewed work, never automatic:

```bash
git remote add upstream https://github.com/go-vikunja/vikunja.git   # once
git fetch upstream --tags
git checkout -b mia-nube/<new-version> v<new-version>
git cherry-pick <first-patch>..<last-patch>      # replay this branch's commits
# resolve, build, test, then update the pinned commit in the deployment repo
```

The previous `mia-nube/<old-version>` branch is **kept**, not deleted: it remains the
corresponding source for any build that was deployed from it, which the AGPL requires to stay
available to the users who were served by it.

## What must never appear in this repository

It is **public and permanent** — a force-push does not remove a pushed secret from GitHub's
API, and anything committed here should be assumed to be permanently disclosed.

- No credentials, tokens, keys or certificates.
- No hostnames, URLs, IP addresses or other environment-specific configuration.
- No infrastructure, deployment or pipeline configuration.

All of that belongs in the private deployment repository. This fork contains **only** the
Vikunja source and the modifications made to it.
