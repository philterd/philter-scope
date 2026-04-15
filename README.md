# PhilterScope

PhilterScope is a standalone CLI tool for PII redaction auditing and policy optimization. It allows you to audit the performance of [Philter](https://www.philterd.ai/) by comparing its redaction results against a "golden dataset" of labeled PII.

## Installation

```bash
make build
```

The binary will be created as `philterscope` in the root directory.

## Quick Start

The following command will compare the raw text in the `examples/raw` directory against the golden dataset in `examples/golden` and generate an HTML report in the `examples/` directory. See below for more details and options for this command.

```
./philterscope audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75
```

Check the `examples/report.html` file for an example of the generated report.

## Usage

### 1. Audit Redaction Quality

Compare files in a directory against their golden counterparts. Input files can be raw text or Philter `explain` JSON files:

```bash
./philterscope audit --input ./raw --url http://localhost:8080
```

This generates `report.html` and `report.json`.

### 2. Launch Evaluation UI

View the generated audit results in your browser:

```bash
./philterscope serve --report report.json --port 5000
```

### 3. Get Policy Suggestions

Get actionable policy recommendations in your terminal:

```bash
./philterscope suggest --report report.json
```

## Golden Dataset Formats

PhilterScope supports two formats for the golden dataset:

### 1. Tagged Text

The simplest format is to wrap PII in your raw text files with tags. PhilterScope will automatically parse these:

```text
My name is <NAME>John Doe</NAME> and I live at <ADDRESS>123 Main St</ADDRESS>.
```

### 2. JSON Spans (`golden.json`)

Alternatively, you can provide a `golden.json` file that defines the text and the character offsets for each PII entity:

```json
{
  "text": "My name is John Doe and I live at 123 Main St.",
  "labels": [
    {
      "text": "John Doe",
      "start": 11,
      "end": 19,
      "label": "NAME"
    },
    {
      "text": "123 Main St",
      "start": 34,
      "end": 45,
      "label": "ADDRESS"
    }
  ]
}
```

The `labels` array contains objects with the following fields:
- `text`: The literal text being labeled.
- `start`: The starting character index (0-based).
- `end`: The ending character index (exclusive).
- `label`: The entity type (e.g., `NAME`, `ADDRESS`, `PHONE_NUMBER`).

### 3. Philter Explain JSON

If you have already redacted text using Philter's `explain` API, you can provide the JSON response directly in the input directory. This allows you to audit pre-redacted data without calling the Philter API again.

PhilterScope recognizes the standard Philter `explain` JSON structure:

```json
{
  "filteredText": "Hello [NAME], welcome to [LOCATION].",
  "explanation": {
    "appliedSpans": [
      {
        "text": "John Doe",
        "characterStart": 6,
        "characterEnd": 14,
        "filterType": "NAME"
      },
      {
        "text": "New York",
        "characterStart": 27,
        "characterEnd": 35,
        "filterType": "LOCATION"
      }
    ]
  }
}
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