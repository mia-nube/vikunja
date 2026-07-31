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

This branch, `mia-nube/2.4.0`, is branched from the upstream tag **`v2.4.0`**
(commit `907850feae3866ae9b16ea1c7b84a4d77273415a`).

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
| 1 | `Dockerfile` | Removes the BuildKit-only constructs: `FROM --platform=$BUILDPLATFORM` on both builder stages, the implicit `TARGETOS`/`TARGETARCH`/`TARGETVARIANT` args (now defaulted), `COPY --chmod=`, and the `# syntax=` parser directive. | Upstream's Dockerfile can only be built by BuildKit. The CI runner that builds our image offers no usable BuildKit backend and forbids privileged containers, so without this patch the image cannot be built at all. The resulting image is unchanged, and building under BuildKit still works. |

## Rebasing onto a new upstream release

Rebases are deliberate, reviewed work, never automatic:

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
