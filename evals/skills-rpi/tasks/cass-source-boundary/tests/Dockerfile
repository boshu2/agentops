FROM agentops-skill-eval-worker:cass-source-boundary-v1
COPY source /baseline
COPY verify.py spec.json oracle_test.go test.sh /tests/
WORKDIR /baseline
RUN go test ./...

# Replay code may recheck the installed resource paths it used in the worker.
# These shared baseline resources are fixture inputs, not an instruction treatment.
RUN if [ ! -r /root/.agents/skills/cass/SKILL.md ]; then mkdir -p /root/.agents/skills && cp -R /opt/agentops/skills/. /root/.agents/skills/; fi
