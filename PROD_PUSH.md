# Prod-Stack Validation Before Merge

Before merging session MRs to `main`, validate them against the real prod pipelines by temporarily pointing prod-stack at the session branches.

## Full Workflow

```
task session:finish -- <desc>        # finalize per-repo, push branches, update MRs
task session:prod-test -- <desc>     # switch prod-stack → session branches, push
                                     #   → GoCD picks up the change automatically
                                     #   → trigger prod pipelines, watch them pass
task session:prod-restore -- <desc>  # revert prod-stack → main, push
(human merges all MRs)               # session branches land on main
task session:version:<repo>:<level> -- <desc>
task session:close:<repo> -- <desc>
task session:archive -- <desc>
```

## What `session:prod-test` Does

1. Reads `sessions/<desc>/.branches` to find repos that are on a session branch (any repo whose recorded branch is not `main`).
2. For each such repo, greps `prod-stack/*.gocd.yaml` for `<repo>.git` to locate the pipeline file (handles filename mismatches like `metahub-api-build.gocd.yaml`).
3. Replaces `branch: main` → `branch: <session-branch>` in the matching file(s).
4. Commits and pushes prod-stack's `main`.

GoCD picks up the config change and the next pipeline run builds from the session branches.

## What `session:prod-restore` Does

The exact inverse: replaces every session branch ref back to `branch: main`, commits, and pushes prod-stack's `main`.

Run this **after** GoCD validation passes and **before** merging the MRs.

## Preconditions

- The prod-stack worktree must exist: `sessions/<desc>/prod-stack/`. If the session was suspended, run `task session:resume -- <desc>` first.
- `session:finish` should have been run so all session branches are pushed and up to date.

## Notes

- `_worktree-main` repos (dev-stack, prod-stack, flux/metahub, outfitter) are recorded in `.branches` as `<repo>=main` (or a non-session branch) and are automatically skipped by both tasks.
- If a session repo has no corresponding prod pipeline (e.g. a docs-only change), the grep finds nothing and that repo is silently skipped.
- prod-stack itself tracks `main` directly — both `prod-test` and `prod-restore` commit to prod-stack's `main`, not a session branch.
