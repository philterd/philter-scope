# Development

This guide explains how to build and develop PhilterScope.

## Building PhilterScope

To build the PhilterScope binaries, run:

```bash
make build
```

The compiled binaries `philterscope-audit` and `philterscope-serve` will be placed in the root of the project. Refer to the `Makefile` for building for other paltforms.

## Running Tests

To run the unit tests, use:

```bash
make test
```

## Project Structure

- `cmd/`: Entry points for `philterscope-audit` and `philterscope-serve`.
- `internal/`: Private library code.
    - `audit/`: The core audit engine and matching logic.
    - `ollama/`: Client for interacting with local LLMs.
    - `philter/`: Client for the Philter API.
    - `server/`: The web-based Evaluation UI.
    - `storage/`: History storage implementations (local and MongoDB).
    - `suggest/`: Policy recommendation logic.
- `pkg/model/`: Shared data models used across the project.
- `docs/`: Documentation and the MkDocs site configuration.
- `examples/`: Sample raw data and golden datasets for testing.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](../../CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.
