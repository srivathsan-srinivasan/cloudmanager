# Release System

CloudManager uses branch lines for work and tags for versions.

## Branches

- `main`: normal integration branch
- `dev/*`: all working branches and release-prep branches
- `release/v1`: protected stable v1 release branch
- `gh-pages`: GitHub Pages only

Do not create permanent branches such as `release/v1.0.2`. Exact versions are
tags: `v1.0.0`, `v1.0.1`, `v1.0.2`.

## Patch Release Flow

1. Start from the current stable release branch:

   ```bash
   git fetch origin
   git switch -c dev/release-v1.0.2 origin/release/v1
   ```

2. Prepare the release pins:

   ```bash
   scripts/release v1.0.2 --push
   ```

3. Open a pull request from `dev/release-v1.0.2` into `release/v1`.

4. Merge the PR after checks pass.

5. In GitHub Actions, run **Release Go Module** from `release/v1` with:

   ```text
   version: v1.0.2
   ```

6. The workflow validates the merged pins and pushes only tag `v1.0.2`.

7. The tag push runs GoReleaser. GoReleaser publishes artifacts and opens a
   formula PR with real release checksums.

8. Merge the formula PR.

9. Delete the short-lived `dev/*` branch.

## Branch Protection

Protect `main` and `release/v1`.

Required behavior:

- changes land through pull requests
- direct pushes to protected branches are blocked
- release workflow has permission to push tags
- formula updates land through a GoReleaser pull request

## First Setup

If `release/v1` does not exist yet, create it from the current stable release
branch once:

```bash
git fetch origin
git switch -c release/v1 origin/release/v1.0.0
git push origin release/v1
```

Then retarget release PRs to `release/v1` and retire `release/v1.0.0` after the
next release is proven.
