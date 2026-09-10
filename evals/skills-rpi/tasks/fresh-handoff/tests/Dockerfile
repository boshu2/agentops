FROM agentops-skill-eval-worker:fresh-handoff-v1
COPY source /baseline
COPY verify.py spec.json oracle_test.go test.sh /tests/
WORKDIR /baseline
RUN go test ./...
