# PATCHES.md

This repository is maintained as a **downstream fork** of `QuantumNous/new-api`.

The goal of this file is to keep the local patch set small, explain **why each patch exists**, and make future upstream syncs predictable.

## Principles

1. **Upstream first, downstream minimal**
   - Prefer upstream fixes when possible.
   - Keep downstream patches focused on real operational gaps.

2. **One patch, one reason**
   - Each patch should have a crisp purpose and a narrow blast radius.
   - Avoid bundling runtime policy, deployment secrets, or machine-local behavior into source patches.

3. **Tests or clear validation required**
   - Behavior-changing patches should include tests when practical.
   - At minimum, each patch needs a repeatable validation note.

4. **Do not mix source maintenance with live rollout**
   - Syncing upstream, replaying patches, building binaries, and replacing live services are separate steps.

---

## Current downstream patch set

> Canonical integration branch at the time of writing: `downstream/integration-20260312`

| # | Topic | Current integrated commit | Original local commit | Why it exists | Validation |
|---|---|---|---|---|---|
| 1 | relay timeout header | `55d19e44` | `d9026ed1` | Adds `RELAY_RESPONSE_HEADER_TIMEOUT_SECONDS` support to reduce upstream proxy / Cloudflare 524 timeout risk on slow response-header paths. | Targeted service/controller tests + manual relay verification |
| 2 | mixed sqlite `channel_info` scan | `d324e97f` | `d7dee1b7` | Handles mixed SQLite `TEXT` / `BLOB` storage forms when scanning `channel_info`, avoiding brittle reads across legacy/live DB states. | `model/channel_info_scan_test.go` |
| 3 | Gemini thought signature | `65860609` | `f017d0fe` | Ensures eligible Gemini function-call requests consistently attach the thought signature needed by the local relay flow. | `relay/channel/gemini/relay_gemini_usage_test.go` |
| 4 | Responses pre-write retry | `970eef89` | `02904200` | Prevents fake success / zero-usage behavior when pre-write fails; retries instead of returning a misleading 200-shaped outcome. | `relay/channel/openai/relay_responses_test.go` + controller/openai relay tests |
| 5 | stale affinity protection | `7e5fcdaf` | `3ccde437` | Ignores stale lower-priority preferred channels so channel affinity does not lock requests onto an outdated weaker choice. | `service/channel_affinity_template_test.go` |
| 6 | local dashboard adjustments | `5b781baf` | `b7049153` | Keeps local web/dashboard rendering adjustments needed by the downstream workspace while preserving newer upstream UI logic during rebase. | Visual diff + downstream integration conflict resolution review |
| 7 | troubleshooting notes | `cd084b8c` | `45a8646d` | Documents local channel troubleshooting patterns that are useful to downstream operators but not required by upstream core. | Doc review |

---

## Notes on the web/dashboard patch

The web patch is intentionally called out because it is the most likely place to conflict during future upstream syncs.

- The **original local commit** was `b7049153`.
- The **integrated downstream commit** is `5b781baf`.
- These commits are **not patch-identical**.

Why:
- upstream changed nearby UI/rendering logic after the original local patch was authored;
- during downstream integration, the conflict was resolved by **keeping upstream billing/render logic** and replaying only the local dashboard-specific adjustments that still made sense.

So for future upgrades, treat this patch as:
- **semantic patch**, not “blind cherry-pick and trust the result”.

---

## What is *not* a downstream patch

These items should not live in this file unless they become source-level changes:

- local databases
- built binaries
- environment files
- service restart procedures
- deployment-machine paths
- one-off troubleshooting shell history

Those belong in deployment runbooks or workspace-level maintenance docs, not in the downstream source patch inventory.

---

## Patch ordering

When replaying onto a fresh upstream base, keep this order unless there is a strong reason to change it:

1. relay timeout header
2. mixed sqlite `channel_info` scan
3. Gemini thought signature
4. Responses pre-write retry
5. stale affinity protection
6. local dashboard adjustments
7. troubleshooting notes

This order keeps core relay/runtime fixes ahead of UI/docs work and reduces avoidable conflict churn.

---

## Retirement rule

If upstream lands an equivalent fix, do **not** keep the downstream patch out of habit.

Instead:
1. verify upstream behavior is truly equivalent;
2. drop the downstream patch on the next sync branch;
3. update this file to record that the patch was retired.
