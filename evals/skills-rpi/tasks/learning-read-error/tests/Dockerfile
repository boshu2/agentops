FROM agentops-skill-eval-worker:learning-read-error-v1
COPY cli /baseline
COPY test.sh oracle_test.go check_scope.py /tests/
WORKDIR /baseline
RUN go test ./internal/gates/checks
