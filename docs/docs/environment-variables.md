# Environment Variables

PhilterScope can be configured using environment variables. These variables are particularly useful for sensitive
information or for configuring shared resources like a database.

| Variable                                 | Default  | Description                                                                                                                                                 |
|:-----------------------------------------|:---------|:------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `PHILTERSCOPE_PHILTER_TOKEN`             | (none)   | The Philter API token (API key) used to authenticate to Philter. Falls back from the `--token` flag. Required only when PhilterScope communicates with Philter (redacting text or fetching the policy); not needed when auditing pre-redacted explain JSON. |
| `PHILTERSCOPE_OLLAMA_URL`                | (none)   | The URL of your [Ollama](https://ollama.com/) server (e.g., `http://localhost:11434`). Required if you use the `--ai` flag.                                 |
| `PHILTERSCOPE_OLLAMA_MODEL`              | `gemma4` | The LLM model to use for AI-driven policy recommendations.                                                                                                  |
| `PHILTERSCOPE_MONGODB_CONNECTION_STRING` | (none)   | The MongoDB connection string for storing audit history (e.g., `mongodb://localhost:27017/philterscope`). If not set, audit history is only stored locally. |

## 1. Philter API Authentication

Philter requires an API token (API key) on the endpoints PhilterScope calls, such as `/api/explain` and `/api/policies`. Provide it with the `--token` flag or the `PHILTERSCOPE_PHILTER_TOKEN` environment variable; when `--token` is not given, PhilterScope falls back to the environment variable.

A token is required only when PhilterScope actually communicates with Philter: redacting raw input through the API, or fetching the policy for the report. If every input is pre-redacted [Philter explain JSON](running.md#data-formats), PhilterScope makes no Philter calls and no token is needed. When a token is needed but not set, PhilterScope stops with a clear error (rather than failing partway through with authentication errors); when it is not needed, the audit runs without one and simply omits the policy from the report.

Using the environment variable is recommended so the token does not appear in your shell history or in the process list.

**Usage:**

```bash
export PHILTERSCOPE_PHILTER_TOKEN="sk_your_philter_api_key"
./philterscope-audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/
```

## 2. AI Recommendation Engine

To enable AI-driven redaction policy suggestions, PhilterScope integrates with [Ollama](https://ollama.com/), which
allows you to run local Large Language Models (LLMs).

**Usage:**

```bash
export PHILTERSCOPE_OLLAMA_URL="http://localhost:11434"
export PHILTERSCOPE_OLLAMA_MODEL="gemma4"
./philterscope-audit --ai
```

If these environment variables are not set and you use the `--ai` flag, PhilterScope will attempt to use the default
Ollama URL and model.

## 3. Shared History Storage (MongoDB)

By default, PhilterScope stores your audit history in a local directory called `.philterscope`. If you want to share
this history across multiple users or machines, you can use a MongoDB database.

**Usage:**

```bash
export PHILTERSCOPE_MONGODB_CONNECTION_STRING="mongodb://user:password@localhost:27017/philterscope"
./philterscope-audit
```

When this variable is set, every successful audit run will automatically save the result to the MongoDB collection.
The history feature will also prioritize results from MongoDB.

## 4. Local Configuration

If no environment variables are set, PhilterScope uses the following defaults:

- **Audit History**: Stored in `$HOME/.philterscope/history`.
- **Philter API**: Defaults to `http://localhost:8080`.
- **Ollama API**: Defaults to `http://localhost:11434`.
