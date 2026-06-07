# GCloud IAM & Security Reference

## Table of Contents
- [Service Accounts](#service-accounts)
- [IAM Bindings](#iam-bindings)
- [Auth Commands](#auth-commands)
- [Project Configuration](#project-configuration)

---

## Service Accounts

```bash
# List service accounts
gcloud iam service-accounts list

# Create service account
gcloud iam service-accounts create my-sa \
  --display-name="My Service Account"

# Create key
gcloud iam service-accounts keys create key.json \
  --iam-account=my-sa@PROJECT.iam.gserviceaccount.com

# List keys
gcloud iam service-accounts keys list \
  --iam-account=my-sa@PROJECT.iam.gserviceaccount.com

# Delete key
gcloud iam service-accounts keys delete KEY_ID \
  --iam-account=my-sa@PROJECT.iam.gserviceaccount.com
```

---

## IAM Bindings

```bash
# Add IAM binding
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:my-sa@PROJECT.iam.gserviceaccount.com" \
  --role="roles/storage.admin"

# Remove IAM binding
gcloud projects remove-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:my-sa@PROJECT.iam.gserviceaccount.com" \
  --role="roles/storage.admin"

# Get IAM policy
gcloud projects get-iam-policy PROJECT_ID

# Set IAM policy from file
gcloud projects set-iam-policy PROJECT_ID policy.json
```

---

## Auth Commands

```bash
# Check current auth
gcloud auth list

# Login interactively
gcloud auth login

# Login with service account
gcloud auth activate-service-account --key-file=key.json

# Application default credentials
gcloud auth application-default login

# Print access token
gcloud auth print-access-token

# Revoke credentials
gcloud auth revoke
```

---

## Project Configuration

```bash
# List projects
gcloud projects list

# Set default project
gcloud config set project PROJECT_ID

# Show current config
gcloud config list

# Create named configuration
gcloud config configurations create my-config
gcloud config configurations activate my-config

# Set default region/zone
gcloud config set compute/region us-central1
gcloud config set compute/zone us-central1-a

# List configurations
gcloud config configurations list
```

---

## Common Roles

| Role | Description |
|------|-------------|
| `roles/owner` | Full access |
| `roles/editor` | Edit access |
| `roles/viewer` | Read-only access |
| `roles/storage.admin` | Storage admin |
| `roles/storage.objectViewer` | Read storage objects |
| `roles/cloudsql.admin` | Cloud SQL admin |
| `roles/run.admin` | Cloud Run admin |
| `roles/container.admin` | GKE admin |
