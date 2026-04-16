# Environment Variables

PhilterScope can be configured using environment variables. These variables are particularly useful for sensitive
information or for configuring shared resources like a database.

| Variable                                 | Default  | Description                                                                                                                                                 |
|:-----------------------------------------|:---------|:------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `PHILTERSCOPE_OLLAMA_URL`                | (none)   | The URL of your [Ollama](https://ollama.com/) server (e.g., `http://localhost:11434`). Required if you use the `--ai` flag.                                 |
| `PHILTERSCOPE_OLLAMA_MODEL`              | `gemma4` | The LLM model to use for AI-driven policy recommendations.                                                                                                  |
| `PHILTERSCOPE_MONGODB_CONNECTION_STRING` | (none)   | The MongoDB connection string for storing audit history (e.g., `mongodb://localhost:27017/philterscope`). If not set, audit history is only stored locally. |

## 1. AI Recommendation Engine

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

## 2. Shared History Storage (MongoDB)

By default, PhilterScope stores your audit history in a local directory called `.philterscope`. If you want to share
this history across multiple users or machines, you can use a MongoDB database.

**Usage:**

```bash
export PHILTERSCOPE_MONGODB_CONNECTION_STRING="mongodb://user:password@localhost:27017/philterscope"
./philterscope-audit
```

When this variable is set, every successful audit run will automatically save the result to the MongoDB collection.
The history command in philterscope-serve will also prioritize results from MongoDB.
