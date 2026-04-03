# gcloud-ai

A natural language CLI tool for Google Cloud Platform — convert plain English into `gcloud`, `kubectl`, `bq`, and `gsutil` commands, powered by Google Gemini.

Instead of remembering exact CLI syntax, just describe what you want:

```bash
gcloud-ai list all clusters in my-project
gcloud-ai get pods from prod cluster in asia-northeast1
gcloud-ai show stopped instances in exa-cloud-prod
gcloud-ai list all bigquery datasets in my-project
```

---

## How It Works

```
You type a natural language query
        ↓
Gemini AI generates the correct CLI command
        ↓
gcloud-ai shows you the command and asks for confirmation
        ↓
Security checks run (blocked keywords, dangerous patterns)
        ↓
Command executes and output is shown
        ↓
Session is saved — follow-up questions just work
```

Nothing ever runs without your explicit confirmation.

---

## Prerequisites

- [`gcloud` CLI](https://cloud.google.com/sdk/docs/install) installed and authenticated
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/) (only needed for Kubernetes commands)
- A free Gemini API key from [Google AI Studio](https://aistudio.google.com)

---

## Install

```bash
curl -sSL https://raw.githubusercontent.com/aiandshubham/gcloud-ai/main/install.sh | bash
```

> **Requirements:** `curl` and `jq` must be installed.
> Mac: `brew install jq` | Linux: `apt install jq`

### Install a specific version

```bash
GAI_VERSION=v1.0.0 curl -sSL https://raw.githubusercontent.com/aiandshubham/gcloud-ai/main/install.sh | bash
```

### Install to a custom directory

```bash
GAI_INSTALL_DIR=$HOME/bin curl -sSL https://raw.githubusercontent.com/aiandshubham/gcloud-ai/main/install.sh | bash
```

---

## Setup

**1. Get a free Gemini API key**

Go to [https://aistudio.google.com](https://aistudio.google.com) → Get API key → Create API key

**2. Set your API key**

```bash
export GEMINI_API_KEY=your_key_here

# Make it permanent
echo 'export GEMINI_API_KEY=your_key_here' >> ~/.zshrc   # or ~/.bashrc
```

**3. Try it**

```bash
gcloud-ai list all my gcp projects
```

---

## Usage

```bash
# Basic queries
gcloud-ai list all my gcp projects
gcloud-ai list all clusters in my-project
gcloud-ai list all bigquery datasets in my-project
gcloud-ai list all storage buckets in my-project

# Kubernetes
gcloud-ai get all pods from prod-cluster in us-central1 from my-project
gcloud-ai get logs for nginx pod in prod-cluster
gcloud-ai describe deployment frontend in prod-cluster

# Follow-up questions (uses output from previous command)
gcloud-ai list projects starting with ops
gcloud-ai from those projects show me all running instances

# Flags
gcloud-ai --version          # show current version
gcloud-ai --history          # show last 20 commands
gcloud-ai --history 50       # show last N commands
gcloud-ai --clear-session    # clear follow-up context
```

---

## Configuration

Create `~/.gai/config.yml` to set defaults and avoid repeating yourself:

```yaml
# Gemini API key (alternative to GEMINI_API_KEY env var)
gemini_api_key: your_key_here

# Gemini model (default: gemini-2.5-pro)
gemini_model: gemini-2.5-pro

# Default GCP project — skip typing "in my-project" every time
default_project: my-gcp-project

# Default region
default_region: us-central1

# Default GKE cluster
default_cluster: my-cluster
```

---

## Security

gcloud-ai has two layers of protection before any command runs:

**1. Command Sanitizer** — only allows `gcloud`, `kubectl`, `bq`, and `gsutil` commands. Blocks shell operators like  `|`, `>`, `` ` ``, `$()`.

**2. Policy Enforcement** — blocks dangerous keywords by default:

| Blocked | Reason |
|---|---|
| `delete`, `destroy`, `drop` | Destructive operations |
| `rm`, `truncate` | Data loss risk |
| `--force` | Skips safety prompts |

You can customize the policy by creating `~/.gai/policy.yml`:

```yaml
blocked_keywords:
  - delete
  - destroy
  - drop
  - rm
  - truncate

restricted_patterns:
  - "--force"
```

---

## Auto Updates

gcloud-ai checks for updates once per day. When a new version is available:

```
🆕 New version available: v1.1.0 (you have v1.0.0)
   Update now? (y/n):
```

Type `y` and it downloads, verifies the checksum, and replaces itself automatically.

---

## Command History

Every command is logged to `~/.gai/history.log`:

```bash
gcloud-ai --history        # last 20 commands
gcloud-ai --history 50     # last 50 commands
```

```
✅ [2026-03-27T10:32:11] list all clusters in my-project
   └─ gcloud container clusters list --project=my-project

⏭️  [2026-03-27T10:33:45] delete the prod cluster
   └─ gcloud container clusters delete prod --project=my-project
   └─ error: command blocked due to keyword: delete

❌ [2026-03-27T10:35:01] get pods from unknown-cluster
   └─ gcloud container clusters get-credentials unknown-cluster ... && kubectl get pods
   └─ error: step 1 failed: cluster not found
```

| Icon | Meaning |
|---|---|
| ✅ | Executed successfully |
| ⏭️ | Cancelled by user |
| 🚫 | Blocked by policy |
| ❌ | Failed during execution |

---

## Debug Mode

```bash
GAI_DEBUG=1 gcloud-ai your query
```

Prints the raw Gemini API request and response — useful for understanding why a command was generated a certain way.

---

## Building from Source

```bash
git clone https://github.com/aiandshubham/gcloud-ai.git
cd gcloud-ai
go build -ldflags="-X gcloud-ai/internal/version.Version=dev" -o gcloud-ai .
./gcloud-ai --version
```

---

## Contributing

Contributions are welcome. Please open an issue first to discuss what you'd like to change.

1. Fork the repo
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Commit your changes: `git commit -m "feat: your feature"`
4. Push and open a Pull Request

---

## License

Apache 2.0 — see [LICENSE](LICENSE)
