# Contributing to Folio

Thanks for your interest in contributing! Bug reports, feature ideas and pull requests are all welcome.

## Reporting issues

Open an issue on [GitHub](https://github.com/GustavoCaso/folio/issues). For bugs, include:

- What you did, what you expected, and what happened instead
- Folio version (Docker tag or commit) and how you run it
- Relevant logs (set `LOG_LEVEL=debug` for more detail)
- If possible, an EPUB that reproduces the problem

For larger changes, please open an issue to discuss the approach before sending a pull request.

## Development setup

Folio uses [mise](https://mise.jdx.dev/) to pin Go, Node and pnpm.

```bash
mise install
pnpm install
make build
go run . -data ./data -db ./data/folio.db
```

Then open http://localhost:8080.

## Making changes

- **Templates:** after editing a `.templ` file run `make templ`. Don't hand-edit `*_templ.go`.
- **Styles:** after changing Tailwind classes run `make css`. Don't hand-edit `internal/handlers/static/tailwind/output.css`.
- **JavaScript:** client JS is embedded in the binary — restart the server and hard-reload the browser after changes.
- **Architecture decisions** are recorded in [`docs/adr/`](docs/adr). If your change alters one of them, add a new ADR.

Generated files are checked in, so commit them alongside your source changes.

## Before opening a pull request

```bash
make test        # Go tests
make test-js     # JS tests (vitest)
make lint        # golangci-lint
make format      # gofmt via golangci-lint
```

Pre-commit hooks are configured with [prek](https://github.com/j178/prek) (`prek.toml`) and check lint, formatting and that generated templ/Tailwind files are up to date. Run `prek install` to enable them.

Keep pull requests focused, add tests for new behaviour, and describe what changed and why.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
