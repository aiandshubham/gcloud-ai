# gcloud-ai

A natural language CLI tool for GCP — convert plain English into `gcloud`, `kubectl`, `bq`, and `gsutil` commands powered by Gemini.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/Exabeam/gcloud-ai/main/install.sh | bash
```

## Prerequisites

- [`gcloud` CLI](https://cloud.google.com/sdk/docs/install) installed and authenticated
- Access to the `dev-gemini-key` secret in GCP project `ops-dist-mgmt`

```bash
gcloud auth application-default login
```

## Usage

```bash
# Natural language queries
gcloud-ai list all projects starting with ops-dist
gcloud-ai get all pods from prod-jp-gnjf cluster in asia-northeast1 from exa-cloud-prod project

# Follow-up questions using previous output
gcloud-ai from those projects get me the stopped instances

# Flags
gcloud-ai --version         # show current version
gcloud-ai --history         # show last 20 commands
gcloud-ai --history 50      # show last 50 commands
gcloud-ai --clear-session   # clear follow-up context
```

## How it works

1. Your prompt is sent to Gemini which generates the correct CLI command
2. The command is shown to you with a confirmation prompt before anything runs
3. Security policy blocks dangerous keywords (`delete`, `destroy`, `drop`, etc.)
4. All commands are logged locally to `~/.gai/history.log`

## Configuration

The Gemini API key is fetched automatically from GCP Secret Manager using your existing `gcloud` credentials. No API key setup needed.

To override locally (for development):
```bash
export GEMINI_API_KEY=your-key-here
```
