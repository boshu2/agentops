FROM golang:1.27.1-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends \
    git curl ca-certificates nodejs npm python3 jq \
    && rm -rf /var/lib/apt/lists/*
RUN npm install -g @openai/codex@0.154.0
COPY helpers /opt/agentops/scripts/lib
ENV AGENTOPS_ROOT=/opt/agentops
COPY source /app/work
WORKDIR /app/work
RUN go test ./...
RUN git init -q && git config user.name "Evaluation fixture" \
    && git config user.email "fixture@example.invalid" \
    && git add . && git commit -qm baseline
