# GCloud Services Reference

## Table of Contents
- [Cloud Functions](#cloud-functions)
- [GKE](#gke)
- [Cloud SQL](#cloud-sql)
- [BigQuery](#bigquery)
- [Pub/Sub](#pubsub)
- [Secret Manager](#secret-manager)
- [Cloud Build](#cloud-build)
- [Artifact Registry](#artifact-registry)
- [Logging](#logging)
- [App Engine](#app-engine)

---

## Cloud Functions

```bash
# List functions
gcloud functions list

# Deploy function (2nd gen)
gcloud functions deploy my-function \
  --gen2 \
  --runtime=nodejs20 \
  --region=us-central1 \
  --trigger-http \
  --entry-point=handler \
  --source=.

# View logs
gcloud functions logs read my-function --region=us-central1

# Delete function
gcloud functions delete my-function --region=us-central1
```

---

## GKE

```bash
# List clusters
gcloud container clusters list

# Get credentials for kubectl
gcloud container clusters get-credentials my-cluster \
  --zone=us-central1-a

# Create cluster
gcloud container clusters create my-cluster \
  --zone=us-central1-a \
  --num-nodes=3

# Resize node pool
gcloud container clusters resize my-cluster \
  --node-pool=default-pool \
  --num-nodes=5 \
  --zone=us-central1-a
```

---

## Cloud SQL

```bash
# List instances
gcloud sql instances list

# Create instance
gcloud sql instances create my-instance \
  --database-version=POSTGRES_15 \
  --tier=db-f1-micro \
  --region=us-central1

# Connect via proxy
gcloud sql connect my-instance --user=postgres

# Create database
gcloud sql databases create mydb --instance=my-instance
```

---

## BigQuery

```bash
# List datasets
bq ls

# Run query
bq query --use_legacy_sql=false 'SELECT * FROM dataset.table LIMIT 10'

# Create dataset
bq mk --dataset my_dataset

# Load data
bq load --source_format=CSV my_dataset.my_table gs://bucket/data.csv
```

---

## Pub/Sub

```bash
# List topics
gcloud pubsub topics list

# Create topic
gcloud pubsub topics create my-topic

# Publish message
gcloud pubsub topics publish my-topic --message="Hello"

# Create subscription
gcloud pubsub subscriptions create my-sub --topic=my-topic

# Pull messages
gcloud pubsub subscriptions pull my-sub --auto-ack
```

---

## Secret Manager

```bash
# List secrets
gcloud secrets list

# Create secret
echo -n "my-secret-value" | gcloud secrets create my-secret --data-file=-

# Access secret
gcloud secrets versions access latest --secret=my-secret

# Add new version
echo -n "new-value" | gcloud secrets versions add my-secret --data-file=-
```

---

## Cloud Build

```bash
# Submit build
gcloud builds submit --tag gcr.io/PROJECT/IMAGE

# List builds
gcloud builds list

# View build logs
gcloud builds log BUILD_ID
```

---

## Artifact Registry

```bash
# List repositories
gcloud artifacts repositories list

# Configure Docker
gcloud auth configure-docker us-central1-docker.pkg.dev
```

---

## Logging

```bash
# Read logs
gcloud logging read "resource.type=cloud_run_revision" --limit=50

# Tail logs
gcloud logging tail "resource.type=gce_instance"
```

---

## App Engine

```bash
# Deploy app
gcloud app deploy

# View logs
gcloud app logs tail

# Browse app
gcloud app browse
```
