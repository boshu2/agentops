# Security Fundamentals for GitHub Actions

## Table of Contents
- [Pin Actions to SHA](#pin-actions-to-sha)
- [Minimal Permissions](#minimal-permissions)
- [OIDC Authentication](#oidc-authentication)
- [Secrets Handling](#secrets-handling)
- [Fork and PR Security](#fork-and-pr-security)
- [Self-Hosted Runner Security](#self-hosted-runner-security)
- [Workflow Tampering Prevention](#workflow-tampering-prevention)

---

## Pin Actions to SHA

**Why:** Tags and branches can be moved. Only commit SHAs are immutable.

```yaml
# BAD - tag can be overwritten
uses: actions/checkout@v4

# GOOD - immutable reference
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2
```

### Finding SHA for a tag

```bash
gh api repos/actions/checkout/git/refs/tags/v4.2.2 --jq '.object.sha'
```

---

## Minimal Permissions

```yaml
# Default: read-only
permissions:
  contents: read

# For releases only
permissions:
  contents: write

# For OIDC (sigstore, cloud auth)
permissions:
  contents: write
  packages: write
  id-token: write  # Required for OIDC

# For creating issues
permissions:
  issues: write
```

---

## OIDC Authentication

Eliminates long-lived secrets for cloud providers.

### AWS

```yaml
permissions:
  id-token: write
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::123456789:role/github-actions
          aws-region: us-east-1
```

### GCP

```yaml
- uses: google-github-actions/auth@v2
  with:
    workload_identity_provider: projects/123/locations/global/workloadIdentityPools/github/providers/github
    service_account: github-actions@project.iam.gserviceaccount.com
```

### Azure

```yaml
- uses: azure/login@v2
  with:
    client-id: ${{ secrets.AZURE_CLIENT_ID }}
    tenant-id: ${{ secrets.AZURE_TENANT_ID }}
    subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}
```

---

## Secrets Handling

### Masking Values in Logs

```yaml
- name: Process sensitive data
  run: |
    API_KEY=$(get_api_key)
    echo "::add-mask::$API_KEY"
    echo "Using API key"
```

### Secrets in Composite Actions

```yaml
# In composite action
inputs:
  token:
    required: true
runs:
  using: composite
  steps:
    - run: echo "::add-mask::${{ inputs.token }}"
      shell: bash
    - run: use_token "${{ inputs.token }}"
      shell: bash
```

---

## Fork and PR Security

### Protect Secrets from Forks

```yaml
# secrets.* is empty for fork PRs by default
- name: Deploy (not on forks)
  if: github.event.pull_request.head.repo.full_name == github.repository
  run: deploy.sh
  env:
    DEPLOY_TOKEN: ${{ secrets.DEPLOY_TOKEN }}
```

### Avoid pull_request_target with Checkout

```yaml
# DANGEROUS - never do this
on: pull_request_target
steps:
  - uses: actions/checkout@v4
    with:
      ref: ${{ github.event.pull_request.head.sha }}  # Untrusted code!

# SAFE - checkout base only
on: pull_request_target
steps:
  - uses: actions/checkout@v4  # Checks out base branch
```

---

## Self-Hosted Runner Security

```yaml
# Restrict to private repos only
jobs:
  build:
    runs-on: self-hosted
    if: github.event.repository.private == true
```

### Ephemeral Runners

Use GitHub's ARC (Actions Runner Controller) for Kubernetes:
- Runners are destroyed after each job
- No state persists between jobs
- Better isolation

---

## Workflow Tampering Prevention

```yaml
# Require review for workflow changes
on:
  push:
    paths-ignore:
      - '.github/workflows/**'
  pull_request:
    paths:
      - '.github/workflows/**'
    # Requires approval before running
```

### Branch Protection Rules

1. Require PR reviews before merging
2. Require status checks to pass
3. Require signed commits
4. Restrict who can push to protected branches
