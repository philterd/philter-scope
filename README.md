# PhilterScope

PhilterScope is a standalone CLI tool for PII redaction auditing and policy optimization. It allows you to audit the performance of [Philter](https://www.philterd.ai/) by comparing its redaction results against a "golden dataset" of labeled PII.

## Core Features

- **Audit**: Compare raw text against a golden dataset to calculate Precision, Recall, and F1-Score.
- **Privacy Lab UI**: A local web server to display interactive, self-contained HTML reports with "Leak" (False Negative) highlighting.
- **Policy Suggestions**: Automatically suggests Philter policy improvements based on audit results.
- **Multi-Format Support**: Supports golden datasets in both tagged text (e.g., `<NAME>John Doe</NAME>`) and JSON Span-style formats.

## Installation

```bash
make build
```

The binary will be created as `philterscope` in the root directory.

## Usage

### 1. Audit Redaction Quality

Compare files in a directory against their golden counterparts:

```bash
./philterscope audit --input ./raw --url http://localhost:8080
```

This generates `report.html` and `report.json`.

### 2. Launch Privacy Lab UI

View the generated audit results in your browser:

```bash
./philterscope serve --report report.json --port 5000
```

### 3. Get Policy Suggestions

Get actionable policy recommendations in your terminal:

```bash
./philterscope suggest --report report.json
```

## Storage

By default, audit results are stored locally in the `.philterscope` directory. You can optionally store results in a MongoDB database by setting the following environment variable:

- `PHILTERSCOPE_MONGODB_CONNECTION_STRING`: The MongoDB connection string (e.g., `mongodb://localhost:27017/philterscope`).

## Development

- **Tests**: Run `go test ./...` to execute the test suite.
- **Build**: Use `make build` to compile the binary for your platform.
- **Makefile**: Supports cross-compilation for Linux, Mac, and Windows.

## License

Copyright 2026 Philterd, LLC. "Philter" is a registered trademark of Philterd, LLC. All rights reserved.

Licensed under the Apache License, Version 2.0.