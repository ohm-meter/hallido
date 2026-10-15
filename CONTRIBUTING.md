# Contributing

Thanks for your interest! Bug reports, ideas, and pull requests are welcome.
The project language (docs, commits, comments) is English; issues and PRs in
German are also fine.

## Before you start

- Please outline larger changes as an issue first.
- Rules and architecture in short form: [AGENTS.md](AGENTS.md) (applies to
  humans and AI agents alike).
- Don't report security vulnerabilities as issues; report them as described in
  [SECURITY.md](SECURITY.md).

## Development environment

- Go 1.22+ (standard library only — no new Go modules without an ADR)
- Node.js 22 only for the GUI layout tests (Playwright)

```bash
cp config.example.json config.json   # set a token
make run                             # mock driver, no hardware needed
git config core.hooksPath .githooks  # pre-commit: tests
```

## Rules

- **Tests green before every commit:** `make test`. For changes under
  `internal/web/static/`, also run `npm ci` and `make test-gui`
  (three viewports: mobile, tablet, desktop).
- **Every new code path gets a test**, including error paths.
  The Sonos driver is tested against a simulated Sonos (HTTP test server),
  never against real speakers.
- **Conventional Commits:** `feat:`, `fix:`, `docs:`, `test:`, `refactor:`.
  One commit = one topic.
- **Changed the API?** Update `docs/api/openapi.yaml` and `docs/reference/api.md`.
- **Architecture decision?** Add a new ADR under `docs/adr/`.
- **Docs** follow [Diátaxis](https://diataxis.fr/) under `docs/` (tutorial,
  how-to, reference, explanation).
- No secrets, real tokens, or private network data in code, tests, or docs.

## Pull requests

1. Fork and create your own branch (`feat/…`, `fix/…`).
2. Tests green, docs updated, entry in `CHANGELOG.md` under `[Unreleased]`.
3. Open the PR against `master` with a short description: what, why, how it was
   tested (mock or real speakers, which models).

By submitting a PR you agree that your contribution is released under the
project's [MIT License](LICENSE).
