FROM golang:1.27.1-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates nodejs npm python3 python3-yaml jq && rm -rf /var/lib/apt/lists/*
RUN npm install -g @openai/codex@0.154.0
COPY runtime /opt/agentops
RUN cd /opt/agentops/cli && go build -o /usr/local/bin/ao ./cmd/ao
ENV AO_RUNTIME_ROOT=/opt/agentops AO_SKILL_BUILDER_BIN=/usr/local/bin/ao
COPY helpers /opt/agentops/scripts/lib
COPY source /app/work
WORKDIR /app/work
RUN go test ./...
RUN git init -q && git config user.name "Evaluation fixture" && git config user.email "fixture@example.invalid" && git add . && git commit -qm baseline
