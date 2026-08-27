# Session 020 — Main Branch Governance

## Objective

Introduce GitHub main-branch governance in two phases: create a safe, disabled ruleset (Phase A), then after explicit human approval activate it and validate a real PR → CI → squash-merge → main → CI lifecycle (Phase B). No develop branch, no merge queue, no required human approvals, no branch locking.

## Starting State

```text
git status --short        (clean)
branch                    main
HEAD        = d4eb1832741c5d9a8b287fcc09a741189515543c
origin/main = d4eb1832741c5d9a8b287fcc09a741189515543c
```

`HEAD == origin/main`. GH CLI `2.93.0`, authenticated as `mhmdnsr-dev` (classic PAT, scope `repo`).

## Existing GitHub Governance

### Rulesets

```text
gh ruleset list --parents
  (none)

gh ruleset check --default
  "0 rules apply to branch main"
```

### Branch Protection

```text
GET /repos/mhmdnsr-dev/context-baggage/branches/main/protection
  404 "Branch not protected"
```

### Merge Settings

```text
allow_squash_merge   = true
allow_merge_commit   = true
allow_rebase_merge   = true
delete_branch_on_merge = false
```

## CI Context Discovery

From the successful `main` commit `d4eb183...` via `check-runs`:

```text
Verify Go 1.27.x   success   app: github-actions
Verify Go 1.22.x   success   app: github-actions
Lint               success   app: github-actions
```

Exact check contexts used: **Verify Go 1.22.x**, **Verify Go 1.27.x**, **Lint**.

## Administration Capability

The ruleset creation API returned the created object — no permission error. The `gh` token (classic `repo`) can manage repository rulesets. No additional privileges requested.

## Phase A

### Disabled Ruleset Creation

```text
POST /repos/mhmdnsr-dev/context-baggage/rulesets
name: Protect main
target: branch
enforcement: disabled
conditions.ref_name.include: ["refs/heads/main"]
RULESET_ID = 21597050
```

### Stored Ruleset Verification

Read back via `GET /repos/mhmdnsr-dev/context-baggage/rulesets/21597050`:

```text
name        = Protect main
target      = branch
enforcement = disabled
conditions  = refs/heads/main
```

Rules (as stored):

```text
1. pull_request
   allowed_merge_methods:               ["squash"]
   required_approving_review_count:     0
   dismiss_stale_reviews_on_push:       false
   require_code_owner_review:           false
   require_last_push_approval:          false
   required_review_thread_resolution:   false
   require_extra_approval_for_unattributed_changes: false
2. required_status_checks
   strict_required_status_checks_policy: false
   required_status_checks: ["Verify Go 1.22.x", "Verify Go 1.27.x", "Lint"]
3. required_linear_history
4. non_fast_forward
5. deletion
```

No `lock`, no `update`, no `creation` rule. No `bypass_actors`. No `required_signatures`. No review-thread or last-push approval gate. No code-owner gate.

### Proposed Merge Settings (Phase B, not yet applied)

```text
allow_squash_merge   = true
allow_merge_commit   = false
allow_rebase_merge   = false
```

### Emergency Rollback

To disable the ruleset at any time (no branch push required):

```bash
gh api -X PUT repos/mhmdnsr-dev/context-baggage/rulesets/21597050 \
  --input /tmp/protect-main-ruleset.json \
  -H "X-GitHub-Api-Version: 2022-11-28"
```

The payload `/tmp/protect-main-ruleset.json` has `enforcement: disabled`, so the same PUT used for creation restores the disabled (non-enforcing) state.

### Assertions

| Assertion                                    | Result    |
| -------------------------------------------- | --------- |
| Repository state inspected                   | PASS      |
| Existing rulesets inspected                  | PASS      |
| Existing branch protection inspected         | PASS      |
| Merge settings recorded                      | PASS      |
| Exact CI contexts discovered from GitHub     | PASS      |
| Ruleset administration available             | PASS      |
| `Protect main` created                       | PASS      |
| Ruleset enforcement is `disabled`            | PASS      |
| Pull Request required in prepared config     | PASS      |
| Required approvals = 0                       | PASS      |
| Only squash allowed in PR rule               | PASS      |
| Required CI contexts correct                 | PASS      |
| Linear history prepared                      | PASS      |
| Force pushes blocked in prepared config      | PASS      |
| Deletion blocked in prepared config          | PASS      |
| No Lock Branch rule                          | PASS      |
| No Restrict Updates rule                     | PASS      |
| No bypass that permits routine direct pushes | PASS      |
| Emergency disable command recorded           | PASS      |
| Repository merge change NOT applied yet      | PASS      |
| Product code unchanged                       | PASS      |

## Human Approval Gate

```text
Phase B activation approved by user:
YES
```

The user explicitly approved activation (equivalent to `فعّلها`).

## Phase B

### Ruleset Activation

```text
PUT /repos/mhmdnsr-dev/context-baggage/rulesets/21597050   (enforcement: disabled -> active)

enforcement = active
```

### Effective Rules

```text
gh ruleset check --default
  "5 rules apply to branch main"
  - deletion
  - non_fast_forward
  - pull_request (squash only, required_approving_review_count: 0,
                  dismiss_stale_reviews_on_push: false,
                  require_code_owner_review: false,
                  require_last_push_approval: false,
                  required_review_thread_resolution: false,
                  require_extra_approval_for_unattributed_changes: false)
  - required_linear_history
  - required_status_checks (Verify Go 1.22.x, Verify Go 1.27.x, Lint;
                            strict_required_status_checks_policy: false)
```

### Merge Settings

Applied via `PATCH /repos/mhmdnsr-dev/context-baggage`:

```text
allow_squash_merge   = true
allow_merge_commit   = false
allow_rebase_merge   = false
```

Verified by read-back.

### Test Branch

Created from `main`:

```bash
git switch -c chore/verify-main-governance
```

The changeset is this work log (`session-020`) only.

### Pull Request

```text
PR #1  https://github.com/mhmdnsr-dev/context-baggage/pull/1
base   main
head   chore/verify-main-governance
```

### PR CI

`pull_request` run `33013607605` — all three required checks pass:

```text
Verify Go 1.22.x    PASS
Verify Go 1.27.x    PASS
Lint                PASS
```

Merge eligibility: `mergeable = MERGEABLE`, `mergeStateStatus = CLEAN`, `reviewDecision = ""` (0 required approvals). Checks were attached and required before merge.

### Squash Merge

```bash
gh pr merge 1 --squash --delete-branch
```

```text
state      MERGED
squash commit  94ea7493ffcde2278e36e2a81d4c58521abf72e3
mergedBy   mhmdnsr-dev (not a bot)
```

No `--admin`, no `--merge`, no `--rebase`. The squash merge succeeded without bypassing the ruleset.

### Main Post-Merge CI

`push → main` run `33013744104` for `94ea749`:

```text
Verify Go 1.27.x    PASS
Verify Go 1.22.x    PASS
Lint                PASS
```

Both PR-before-merge CI and post-merge main CI pass.

### Branch Cleanup

```text
local branch chore/verify-main-governance   deleted
remote branch chore/verify-main-governance  deleted (via --delete-branch)
```

### Final Ruleset

```text
id          = 21597050
name        = Protect main
target      = branch
enforcement = active
conditions  = refs/heads/main
rules       = pull_request (squash-only, 0 approvals), required_status_checks
              (Verify Go 1.22.x, Verify Go 1.27.x, Lint; strict=false),
              required_linear_history, non_fast_forward, deletion
```

### Final Repository Settings

```text
allow_squash_merge   = true
allow_merge_commit   = false
allow_rebase_merge   = false
```

## Conclusion

Governance was activated, then validated end-to-end through a real short-lived-branch → PR → CI → squash-merge → main → CI lifecycle. The `Protect main` ruleset (`active`) required a pull request with 0 approvals and squash-only merges, required the three real CI checks, required linear history, blocked force pushes and branch deletion, and configured no bypass actors, no branch lock, no restrict-updates rule, no code-owner gate, and no mandatory human reviewer. The test branch push produced no unintended main CI; the PR was mergeable at 0 approvals and merged cleanly without an admin bypass; and main remained linear. Repository merge settings are now squash-only. Rollback remains a prepared single API PUT (`enforcement: active → disabled`).

MAIN BRANCH GOVERNANCE: PASS
