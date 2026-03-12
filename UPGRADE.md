# UPGRADE.md

This document describes the **downstream upgrade workflow** for this fork.

It is intentionally about **source maintenance**, not live rollout.
Do not combine an upstream sync with a production binary replacement in the same rushed step.

---

## Branch model

Recommended roles:

- `upstream/main` → latest official upstream
- `origin/main` → fork default branch / review landing branch
- `downstream/integration-YYYYMMDD` → temporary integration branch for one sync cycle
- optional long-lived patch branch → only if you deliberately want one; otherwise prefer short-lived integration branches

---

## Golden rules

1. **Sync source first, deploy later**
   - finish code integration;
   - run tests/build validation;
   - only then decide whether to replace any live binary.

2. **Never force-push over uncertainty**
   - if the new sync result is not fully reviewed, push it to a new integration branch.

3. **Prefer replay over dirty carry-forward**
   - start from fresh `upstream/main`;
   - replay the known downstream patch set in order.

4. **Keep runtime artifacts out of the source diff**
   - binaries, DBs, logs, and local backups should never be part of the sync branch.

---

## Standard sync procedure

### 1) Refresh remotes

```bash
git fetch upstream --prune
git fetch origin --prune
```

### 2) Create a fresh integration branch from upstream

```bash
git checkout -B downstream/integration-YYYYMMDD upstream/main
```

### 3) Replay the downstream patch set

Replay the patches listed in `PATCHES.md`, in the documented order.

Typical pattern:

```bash
git cherry-pick -x <commit-1>
git cherry-pick -x <commit-2>
...
```

If you are replaying from an older local maintenance branch, prefer:
- small, known commits;
- tested patches first;
- docs/UI last.

---

## Conflict handling guidance

### High-risk conflict areas

The following areas deserve extra care:

- `controller/relay.go`
- `relay/channel/openai/*`
- `relay/channel/gemini/*`
- `service/channel_affinity.go`
- `web/src/helpers/render.jsx`

### Rule of thumb

When upstream and downstream both changed the same area:

- keep **upstream structural improvements** unless they are clearly wrong for the downstream use case;
- replay only the **minimal downstream behavior** that still matters;
- avoid reintroducing old local logic wholesale.

### Special note for the web patch

`web/src/helpers/render.jsx` should be treated as a **semantic merge**, not a blind cherry-pick.

Preferred resolution approach:
- preserve newer upstream pricing/render logic;
- reapply only the local dashboard-specific adjustments that are still necessary.

---

## Validation checklist

At minimum, run the targeted Go tests that cover the current downstream patch set:

```bash
go test ./model ./controller ./relay/channel/gemini ./relay/channel/openai ./service
```

If frontend files changed, also run a frontend validation step when the environment is ready:

```bash
cd web
npm run build
```

Before opening or updating a PR, confirm:

- `git status` is clean;
- the integration branch is based on current `upstream/main`;
- patch list in `PATCHES.md` still matches reality;
- any conflict resolution is reflected in commit history or PR notes.

---

## PR workflow

After validation, push the integration branch:

```bash
git push -u origin downstream/integration-YYYYMMDD
```

Then open a PR against the fork’s review branch, usually `main`:

```bash
gh pr create \
  --repo <fork-owner>/<fork-repo> \
  --base main \
  --head downstream/integration-YYYYMMDD
```

The PR description should clearly separate:

1. upstream sync content;
2. downstream local patches;
3. tests run;
4. anything intentionally **not** validated yet (for example live rollout).

---

## What happens after PR approval/merge

Merging the PR means:
- the **source maintenance line** is updated;
- the fork now has a reviewed downstream state.

It does **not** automatically mean:
- a new binary has been built;
- the live service has been restarted;
- the live deployment has been switched.

Those should happen in a separate rollout step with their own checks.

---

## When to retire or change a patch

Use this rule:

- if upstream has landed an equivalent fix, retire the downstream patch;
- if the local need is purely machine/deployment-specific, move it out of source and into deployment tooling or runbooks;
- if a patch keeps conflicting every sync and has low value, reconsider whether it should exist at all.

---

## One-sentence policy

**Fresh upstream base, replay the smallest possible downstream patch set, validate, review, and only then think about deployment.**
