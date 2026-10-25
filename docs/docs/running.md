# Running PhilterScope

PhilterScope is a standalone CLI tool for PII redaction auditing and policy optimization. This document explains the
available commands and flags.

Note that PhilterScope is intended to be run locally and not over a network. If running over a network, be sure to use SSL/TLS connection to MongoDB and to the PhilterScope UI.

## 1. Installation

PhilterScope is written in Go. You can build the binary for your platform using the provided Makefile:

```bash
make build
```

This will create `philterscope-audit` and `philterscope-serve` binaries in the project root.

---

## 2. Commands

PhilterScope provides two primary commands: `philterscope-audit` for performing audits and `philterscope-serve` for viewing results.

### `philterscope-audit`

The `philterscope-audit` command compares raw text files against a "golden dataset" to evaluate redaction quality.

**Usage:**

```bash
PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./philterscope-audit [flags]
```

Or without MongoDB:

```bash
./philterscope-audit [flags]
```

**Commonly Used Flags:**

| Flag           | Default                 | Description                                                                  |
|:---------------|:------------------------|:-----------------------------------------------------------------------------|
| `--url`        | `https://localhost:8080` | The Philter API URL to use for redacting the raw text files.                |
| `--token`      | (none)                  | The Philter API key. Philter 4 requires one with the `redact` scope.         |
| `--ca-cert`    | (none)                  | A PEM file of certificates to trust for Philter's HTTPS, in addition to the system roots. |
| `--insecure`   | `false`                 | Do not verify Philter's HTTPS certificate. For local testing only.           |
| `--policy`     | `default`               | The name of the Philter policy to use for redaction.                         |
| `--input`      | `./raw`                 | The directory containing the raw text files or Philter `explain` JSON files. |
| `--golden`     | `golden.json`           | The path to the golden dataset file or directory.                            |
| `--output`     | `.`                     | The directory where the `report.html` and `report.json` will be saved.       |
| `--threshold`  | `0.5`                   | The default recall threshold for policy suggestions (0.0 to 1.0).            |
| `--thresholds` | (none)                  | Per-entity recall thresholds (e.g., `NAME=0.9,SSN=1.0`).                     |
| `--precision-floor` | `0.25`             | Precision below which an entity is flagged as likely misconfigured (0.0 to 1.0). |
| `--group`      | `default`               | Assign a group name to the audit for history tracking.                       |
| `--ai`         | `false`                 | Enable AI-driven policy recommendations (requires Ollama).                   |
| `--best-effort`| `false`                 | Score the files that can be scored instead of failing the run.               |
| `--version`    |                         | Print the version stamped in at build time and exit.                         |

**Example:**

```bash
./philterscope-audit --input ./examples/raw --golden ./examples/golden --output ./examples/ --threshold 0.75 --ai
```

Thresholds can also be set individually for each entity type:

```bash
./philterscope-audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75 --thresholds "NAME=0.9,SSN=1.0"
```

#### Connecting to Philter

PhilterScope works with Philter 4. Raw text files are sent to Philter's `POST /api/explain`; input files that are already Philter explain JSON are scored without being redacted again. Either way, the audit asks Philter for the `--policy` it audited (`GET /api/policies/{name}`) to record in the report, and continues without it if Philter cannot return it.

- **API key.** Every Philter 4 request needs an API key, passed with `--token`. The key needs the `redact` scope. The first key is the one Philter was started with in `PHILTER_BOOTSTRAP_API_KEY`; create a narrower one with `POST /api/api-keys`.
- **HTTPS.** Philter's Docker image serves HTTPS on port 8080 with a self-signed certificate, which PhilterScope does not trust by default. Either pass that certificate with `--ca-cert`, use a certificate your system already trusts, or run Philter with `SSL_ENABLED=false` behind a TLS-terminating proxy and point `--url` at the proxy. `--insecure` skips verification entirely and is suitable only for a Philter on your own machine.

To trust a self-signed certificate, save it and pass it with `--ca-cert`:

```bash
openssl s_client -connect localhost:8080 </dev/null 2>/dev/null | openssl x509 > philter.pem
./philterscope-audit --url https://localhost:8080 --token "$PHILTER_API_KEY" --ca-cert philter.pem
```

The hostname in `--url` has to match the certificate. Philter generates its certificate for `localhost` unless it is started with `SSL_CERTIFICATE_HOSTNAME` set to the name you connect with.

#### Exit Codes and Skipped Files

| Code | Meaning |
|:-----|:--------|
| `0` | The audit ran and a report was written. |
| `1` | The audit did not produce a usable result. |

An audit fails rather than reporting a score it cannot stand behind:

- If Philter cannot be reached, or refuses the API key with `401` or `403` (a key without the `redact` scope), the run stops at the first file with one error, instead of warning once per file and then scoring nothing.
- If no input file could be scored, the run fails rather than writing a report whose precision and recall are zero because nothing was measured.

A file that cannot be read or redacted, while other files can, is skipped and the run continues. Skipped files are counted in the summary line and recorded in the JSON report under `files_skipped` and `skipped`, each with the reason. A skipped file is not the same as a file that scored zero: it contributed nothing to the metrics.

`--best-effort` opts out of both failures, for callers that would rather have a partial score than an error. The report still records what was skipped.

---

### `philterscope-serve`

The `philterscope-serve` command launches the Evaluation UI, allowing you to view and interact with the results of a previous audit.

**Usage:**

```bash
./philterscope-serve [flags]
```

**Flags:**

| Flag        | Default       | Description                                            |
|:------------|:--------------|:-------------------------------------------------------|
| `--report`  | `report.json` | The JSON report file generated by the `audit` command. |
| `--port`    | `5000`        | The port on which the UI will be served.               |
| `--privacy` | `false`       | Enable privacy mode (obfuscates PII in UI).            |
| `--threshold` | `0.5`       | The recall threshold the report is shown against, when serving a single report. |
| `--thresholds` | (none)     | Per-entity recall thresholds (e.g., `NAME=0.9,SSN=1.0`), when serving a single report. |
| `--version` |               | Print the version stamped in at build time and exit.    |

**Example:**

```bash
PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./philterscope-serve --privacy
```

Or without MongoDB:

```bash
./philterscope-serve --report ./examples/report.json --port 5000 --privacy
```

With no arguments, `philterscope-serve` picks a source in this order:

1. MongoDB, if `PHILTERSCOPE_MONGODB_CONNECTION_STRING` is set and reachable.
2. The report named by `--report`, or a `report.json` in the current directory.
3. The local `.philterscope` audit history.

Serving a single report shows that one audit. The other two serve the full history, where you can browse past audits, add notes, and resolve or dismiss recommendations.

### Health Endpoint

`philterscope-serve` exposes an unauthenticated health endpoint at `/api/health` for container runtimes, load balancers, and uptime checks. It is a liveness probe: it reports that the server is answering and does not check MongoDB.

```bash
curl http://localhost:5000/api/health
```

```json
{"status": "UP", "applicationVersion": "1.2.3"}
```

A healthy server returns `200` with `"status": "UP"`. The version comes from the `VERSION` set at build time (`make build VERSION=1.2.3`, or `docker build --build-arg VERSION=1.2.3`); builds made without it report `dev`.

This is a liveness check. It reports that the server is answering and deliberately does not touch storage, so a database outage does not make a working server look dead to a container runtime that would restart it. Use the readiness endpoint for that.

### Readiness Endpoint

`GET /api/readyz` reports whether the storage behind the server can be reached. It is unauthenticated.

```bash
curl http://localhost:5000/api/readyz
```

```json
{"status": "READY", "mode": "mongodb"}
```

`mode` says which backend the server came up with: `mongodb`, `file` for the local `.philterscope` history, or `report` when serving a single report file, which has no backend and is always ready.

When MongoDB is configured but cannot be reached, the endpoint returns `503` and says why:

```json
{"status": "NOT_READY", "mode": "mongodb", "reason": "failed to reach MongoDB: server selection error..."}
```

Point a load balancer at `/api/readyz` so it stops sending requests that would fail, and a container runtime's liveness probe at `/api/health` so it only restarts a server that has actually stopped serving.

### Metrics Endpoint

`GET /metrics` exposes Prometheus text format, unauthenticated:

```bash
curl http://localhost:5000/metrics
```

Alongside the standard Go runtime and process metrics:

| Metric | Type | Labels | Description |
|:-------|:-----|:-------|:------------|
| `philterscope_http_requests_total` | counter | `route`, `method`, `code` | API requests served. |
| `philterscope_http_request_duration_seconds` | histogram | `route`, `method` | API request duration. |

`route` is the registered route, not the request path, so an audit ID in a query string never becomes a label value. A labelled series appears once the first matching request has been served. The health, readiness and metrics endpoints are not counted: probe and scrape traffic would otherwise swamp the request counts they report.

---

### History and Audit Management

PhilterScope maintains a history of your audit runs. When using `philterscope-serve`, you can browse previous audits and their results.

- **Local Storage**: By default, audits are stored as one JSON file per audit in a `.philterscope` directory, created in the directory you run the command from. `philterscope-serve` reads that history, so audits are browsable without a database. It assumes a single user on one machine: nothing coordinates concurrent writers.
- **Shared Storage**: Set `PHILTERSCOPE_MONGODB_CONNECTION_STRING` for a centralized audit repository across your team. It takes precedence over local storage.
- **Privacy Mode**: When `--privacy` is enabled, the text of each labeled and detected span (both expected and actual) is replaced in the UI with a short tag, the first five hex characters of its MD5 hash. This is obfuscation for viewing a report, not protection of the data: text outside those spans is shown as it is, and the report files and stored history still hold the original text.

### Running in Docker

The image carries both binaries. `philterscope-serve` is the default command, the working directory is `/data`, and the container runs as a non-root user. It is published on [Docker Hub](https://hub.docker.com/r/philterd/philter-scope) for `linux/amd64` and `linux/arm64`:

```bash
docker pull philterd/philter-scope:latest
```

`latest` tracks the most recent release. Pin a version for anything you deploy:

```bash
docker pull philterd/philter-scope:0.2.0
```

Serve a report:

```bash
docker run --rm -p 5000:5000 \
  -v "$PWD/examples/report.json:/data/report.json:ro" \
  philterd/philter-scope:latest
```

Setting `PHILTERSCOPE_MONGODB_CONNECTION_STRING` takes precedence over the report file.

Run an audit by overriding the command. `--user` is needed for the non-root container to write the reports back to a bind-mounted directory:

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD/examples:/data" \
  philterd/philter-scope:latest \
  philterscope-audit --golden /data/golden/ --input /data/raw/ --output /data/ --threshold 0.75
```

The running container reports its version at `/api/health` and from `philterscope-serve --version`. Building the image yourself is covered in the [development guide](development.md).

#### Docker Compose

The repository ships a [`docker-compose.yaml`](https://github.com/philterd/philter-scope/blob/main/docker-compose.yaml) that runs both steps in order. The audit runs once and writes its reports into `./data`, then the dashboard starts and serves them on port 5000:

```bash
curl -O https://raw.githubusercontent.com/philterd/philter-scope/main/docker-compose.yaml

mkdir -p data/golden data/raw   # your golden dataset, and the text to score

export PHILTER_URL=https://philter.internal:8080
export PHILTER_TOKEN=sk_...     # a Philter API key with the redact scope
docker compose up
```

To trust Philter's self-signed certificate, save it as `data/philter.pem` (see [Connecting to Philter](#connecting-to-philter)) and set `PHILTER_CA_CERT=/data/philter.pem`.

The file reads its settings from the environment or from a `.env` file next to it.

| Variable                 | Default                 | Description                                                   |
|:-------------------------|:------------------------|:--------------------------------------------------------------|
| `PHILTER_URL`            | `https://localhost:8080` | Philter API used to redact the raw text.                     |
| `PHILTER_TOKEN`          | (none)                  | Philter API key with the `redact` scope.                      |
| `PHILTER_CA_CERT`        | (none)                  | Path inside the container to a PEM certificate to trust for Philter's HTTPS, such as `/data/philter.pem`. |
| `PHILTERSCOPE_VERSION`   | `0.2.0`                 | Image tag to run.                                             |
| `PHILTERSCOPE_THRESHOLD` | `0.75`                  | Recall threshold, applied to both the audit and the dashboard. |
| `PHILTERSCOPE_UID`       | `1000`                  | Your `id -u`, so the audit can write into `./data`.           |
| `PHILTERSCOPE_GID`       | `1000`                  | Your `id -g`.                                                 |

---

## 3. Data Formats

PhilterScope is designed to be flexible with your data. It supports multiple formats for both input files and golden
datasets.

### Input Files

The input directory (`--input`) can contain:

1. **Raw Text Files**: Simple `.txt` files that will be sent to the Philter API for redaction.
2. **Philter Explain JSON**: If you have already redacted text using Philter's `explain` API, you can provide the JSON
   response directly, as a `.json` file. This allows you to audit pre-redacted data without redacting it again.

### Golden Datasets

The golden dataset (`--golden`) defines the expected redactions. PhilterScope looks for a match in several ways:

1. **Tagged Text**: Wrap PII in your raw text files with tags like `<NAME>John Doe</NAME>`. PhilterScope can parse these
   directly.
2. **JSON Spans**: A JSON file that defines the text and the character offsets for each PII entity. This is the
   recommended format for large datasets. The file name must end in `.json`; any other golden file is read as tagged text.

Example JSON Span format:

```json
{
  "text": "My name is John Doe and I live at 123 Main St.",
  "labels": [
    {
      "text": "John Doe",
      "characterStart": 11,
      "characterEnd": 19,
      "label": "NAME"
    }
  ]
}
```

`characterStart` is the offset of the first character and `characterEnd` the offset after the last. A label whose offsets are both `0`, or both missing, is ignored.

### Matching Logic

For each input file, PhilterScope uses the first of these that exists:

1. The file given by `--golden`, if it is a file. It is used for every input file.
2. If `--golden` is a directory, a file in it with the input file's name. For an input named `redacted*.json`, the same name with `redacted` replaced by `golden` is also tried, so `redacted1.json` matches `golden1.json`.
3. `<filename>.golden` in the input directory, read as tagged text.
4. A file with the input file's name in a `golden/` directory next to the input directory (for `--input ./data/raw`, that is `./data/golden/`), including the `redacted` to `golden` name for `.json` inputs.
5. Inline tags within the input file itself.

A candidate whose name ends in `.json` is read as JSON spans and skipped if it does not parse; any other candidate is read as tagged text. So a JSON golden dataset for a `.txt` input has to be passed with `--golden` as a file.

---

## 4. Understanding the Report

After an audit completes, PhilterScope generates an HTML report and a JSON report containing overall metrics (precision, recall, F1-score), per-entity recall and precision, a confusion matrix, per-document results, and recommended policy changes.

The recommendations are advisory. PhilterScope measures and explains; applying a change is your decision, and nothing here edits a policy for you. See [Policy Recommendations](recommendations.md) for what a recommendation contains and how to consume it.

### Recall and precision in the report

Recall is the target. It is the share of the labeled spans that Philter found, it is what `--threshold` is set against, and it is what a PASS or FAIL in the entity table reports.

Precision, the share of detections that matched a labeled span, is reported next to it but is not scored against a target. It is there to bound the advice. Suggestions driven by recall alone all point the same way, since a policy that redacts everything scores perfect recall, so precision is what shows when a filter has gone further than intended. An entity whose precision falls below `--precision-floor` is marked CHECK in the entity table and raises a recommendation to look at the filter, because a filter matching several times more text than the gold standard labels is usually misconfigured. PhilterScope does not suggest redacting less in order to raise it.

### Confusion Matrix

The confusion matrix shows how each expected entity type was classified by Philter. It is displayed immediately below the PII Detection Performance table in the report.

Each row represents an **expected** entity type from the golden dataset, and each column represents what Philter **detected** it as. The cells contain the count of occurrences.

There are two special labels in the matrix:

- **(missed)** (column): The entity was present in the golden dataset but Philter did not detect it at all. These are false negatives.
- **(none)** (row): Philter detected an entity that was not present in the golden dataset. These are false positives (spurious detections).

Cells are color-coded:

- **Green**: Correct classifications (expected and detected types match).
- **Orange**: Misclassifications (Philter detected the entity but assigned the wrong type, e.g., a NAME detected as an ADDRESS).
- **Red**: Missed entities or spurious detections.

#### Using the Confusion Matrix for Policy Tuning

The confusion matrix helps identify specific weaknesses in your Philter policy:

- **High counts in the (missed) column** for a given entity type indicate that Philter is failing to detect that type. Consider adjusting the policy to add or tune the relevant filter.
- **Off-diagonal orange cells** reveal type confusion. For example, if LOCATION entities are frequently detected as ADDRESS, the policy may need more specific patterns to distinguish the two.
- **High counts in the (none) row** indicate false positives. Philter is flagging text that is not PII, which may require tightening filter rules or adjusting confidence thresholds.

The confusion matrix data is also included in the JSON report under the `confusion_matrix` field for programmatic analysis.
