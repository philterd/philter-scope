# Development

This guide explains how to build and develop PhilterScope.

## Building PhilterScope

To build the PhilterScope binaries, run:

```bash
make build
```

The compiled binaries `philterscope-audit` and `philterscope-serve` will be placed in the root of the project. Refer to the `Makefile` for building for other paltforms.

The version is baked into both binaries at build time. It defaults to `git describe`; override it with:

```bash
make build VERSION=1.2.3
```

Either binary reports it, and `philterscope-serve` also returns it at `/api/health`:

```bash
./philterscope-audit --version
```

Builds made outside the Makefile report `dev`.

## Building the Docker Image

`build-image.sh` builds the image for `linux/amd64` and `linux/arm64`, loading each under its own tag:

```bash
./build-image.sh 1.2.3
```

The version is stamped into the binaries and reported at `/api/health`. Passing no version builds `latest` tags stamped with the git description.

## Publishing the Docker Image

`push-image.sh` pushes the tags that `build-image.sh` produced and joins them into one multi-architecture tag. It builds nothing:

```bash
./push-image.sh 1.2.3
```

Run it by hand, from a machine holding the credential, after `docker login`. Nothing in CI pushes an image: the Docker workflow builds with `push: false` so a broken `Dockerfile` fails a pull request, and no registry write token is held in the repository.

The push is gated on a [Trivy](https://trivy.dev) scan of the exact images being pushed, failing on HIGH and CRITICAL vulnerabilities that have a fix available. Record an accepted finding in `.trivyignore`. The script also refuses to push a `-dirty` version, since a build from a working tree that matches no commit cannot be reproduced later.

| Variable | Effect |
|:---------|:-------|
| `DRY_RUN=1` | Scan and print the plan, push nothing. |
| `SKIP_SCAN=1` | Push without the vulnerability scan. |
| `ALLOW_DIRTY=1` | Push a version built from a dirty tree. |
| `IMAGE` | Override the image name (default `philterd/philter-scope`). |
| `ARCHES` | Override the architectures (default `amd64 arm64`). |

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
