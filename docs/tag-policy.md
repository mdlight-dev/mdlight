# Tag Policy — MDLight

> **Canonical reference for versioning. Read this before tagging.**
> Policy chosen: **Option A — strict semver** (2026-09-03).
> See `proposal.md:81-112` (phase gates) and `design.md:204-213` (milestones) for feature scope.

## Scheme

```
vMAJOR.MINOR.PATCH   e.g. v0.1.15, v0.2.0, v1.0.0
```

Tag is the single source of truth:
- Injected at build time via `main.go:78` / `Makefile:29` as `-ldflags "-X main.version=${{ github.ref_name }}"`
- Drives nfpm `build/linux/nfpm.yml:4` version substitution, AUR `build/aur/PKGBUILD:45`, and `scripts/install.sh:27-48` download URL.

## Rules (strict semver)

| Bump | When | Examples |
|------|------|----------|
| **PATCH** `v0.1.14 → v0.1.15` | Bugfix, no new behaviour, no API change | CSS brace fix, watcher debounce tweak, `mdlight .` friendly error |
| **MINOR** `v0.1.x → v0.2.0` | New feature, new UI, new binding, user-visible behaviour change | **find-in-document** (should have been `v0.2.0`, shipped as `v0.1.11-0.1.15` by mistake), `mdlight <dir>` listing, TOC, edit mode |
| **MAJOR** `v0.x → v1.0.0` | Phase gate per `proposal.md:83-112` — v1.0 "Reading complete" requires **all** of: TOC + find + recent files + 6 themes + front-matter card + edit mode. No major until milestone 10-13 in `design.md:204` are done. |

Pre-release: not used. Hotfix on a released tag requires new PATCH, never `git tag -f` on a published release (force-push only allowed while `gh release` for that tag has not yet succeeded).

## Tagging checklist

1. `git log --oneline -10` — confirm `main` is green (`gh run list --workflow release.yml --limit 3`).
2. Decide bump per table above. When in doubt, favour MINOR over PATCH if any UI changes.
3. Update `CHANGELOG`/`README` if user-visible (not enforced, but preferred).
4. `git tag vX.Y.Z && git push origin main && git push origin vX.Y.Z` — single command; `release.yml` (`on: push: tags: "v*.*.*"`) must trigger.
5. Verify `gh release view vX.Y.Z` shows all assets (`mdlight_vX.Y.Z_linux_amd64`, `*.deb/.rpm/.apk`, `install.sh`, `checksums.txt`).
6. Update this file and `memory.md` session log with outcome.

## History note

`v0.1.1-0.1.15` were patch-churn on a feature that warranted `v0.2.0` (see `lessons.md:18-19`). Retained for history; going forward, follow this policy. Next feature after `v0.1.15` is `v0.2.0`.

## Where to find policy

- **This file** — `docs/tag-policy.md` whitelisted in `.gitignore:6` (`docs/*` + `!docs/tag-policy.md`) so fresh clones have it. Session handoff via `memory.md:18`.
