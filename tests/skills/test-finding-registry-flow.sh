#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0

pass() {
    echo "PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    echo "FAIL: $1"
    FAIL=$((FAIL + 1))
}

check_contains() {
    local file="$1"
    local pattern="$2"
    local label="$3"
    if grep -q "$pattern" "$file"; then
        pass "$label"
    else
        fail "$label"
    fi
}

PLAN_SKILL="$REPO_ROOT/skills/plan/SKILL.md"
PM_SKILL="$REPO_ROOT/skills/pre-mortem/SKILL.md"
VIBE_SKILL="$REPO_ROOT/skills/validate/references/quick-mode-vibe.md"
POST_MORTEM_SKILL="$REPO_ROOT/skills/post-mortem/SKILL.md"
REGISTRY_CONTRACT="$REPO_ROOT/docs/contracts/finding-registry.md"
REGISTRY_SCHEMA="$REPO_ROOT/docs/contracts/finding-registry.schema.json"

check_contains "$PLAN_SKILL" '.agents/planning-rules' "/plan reads compiled planning rules first"
check_contains "$PLAN_SKILL" 'registry.jsonl' "/plan falls back to registry.jsonl"
check_contains "$PLAN_SKILL" 'Applied findings:' "/plan cites applied finding IDs"
check_contains "$PM_SKILL" '.agents/pre-mortem-checks' "/pre-mortem reads compiled checks first"
check_contains "$PM_SKILL" 'known_risks' "/pre-mortem injects known_risks"
check_contains "$PM_SKILL" 'malformed line -> warn and ignore that line' "/pre-mortem fail-open reader behavior is documented"
check_contains "$PM_SKILL" 'finding-compiler.sh' "/pre-mortem refreshes compiled outputs after registry writes"
check_contains "$VIBE_SKILL" '.agents/pre-mortem-checks' "/vibe reads compiled checks first"
check_contains "$VIBE_SKILL" 'registry.jsonl' "/vibe falls back to registry.jsonl"
check_contains "$VIBE_SKILL" 'dedup_key' "/vibe write path requires dedup_key"
check_contains "$VIBE_SKILL" 'finding-compiler.sh' "/vibe refreshes compiled outputs after registry writes"
check_contains "$POST_MORTEM_SKILL" '.agents/pre-mortem-checks' "/post-mortem reads compiled prevention inputs"
check_contains "$POST_MORTEM_SKILL" 'registry.jsonl' "/post-mortem writes finding registry"
check_contains "$POST_MORTEM_SKILL" 'temp-file-plus-rename atomic write rule' "/post-mortem uses atomic registry writes"
check_contains "$POST_MORTEM_SKILL" 'finding-compiler.sh' "/post-mortem refreshes compiled outputs after registry writes"
check_contains "$REGISTRY_CONTRACT" 'dedup_key =' "registry contract defines dedup_key normalization"
check_contains "$REGISTRY_CONTRACT" 'plan-shape' "registry contract defines controlled applicable_when vocabulary"
check_contains "$REGISTRY_CONTRACT" 'atomic rename' "registry contract defines atomic write rule"

FIXTURE='{"id":"f-2026-03-09-001","version":1,"tier":"local","source":{"repo":"agentops/crew/nami","session":"2026-03-09","file":".agents/council/2026-03-09-pre-mortem-finding-compiler-v1.md","skill":"pre-mortem"},"date":"2026-03-09","severity":"significant","category":"validation-gap","pattern":"Plans can omit prior-finding injection and rediscover the same failure mode.","detection_question":"Did this plan load matching active findings before decomposition or review?","checklist_item":"Verify the relevant skill reads compiled planning/pre-mortem checks first, with registry fallback, and cites applied finding IDs or known risks.","applicable_languages":["markdown","shell"],"applicable_when":["plan-shape","validation-gap"],"status":"active","superseded_by":null,"dedup_key":"validation-gap|prior-finding-injection|plan-shape","hit_count":0,"last_cited":null,"ttl_days":30,"confidence":"high"}'

if echo "$FIXTURE" | jq -e '
    .id and
    .version == 1 and
    .tier == "local" and
    .source.repo and
    .source.skill and
    .pattern and
    .detection_question and
    .checklist_item and
    (.applicable_when | index("plan-shape")) and
    .dedup_key and
    .status == "active"
' >/dev/null; then
    pass "fixture entry matches required registry fields"
else
    fail "fixture entry matches required registry fields"
fi

if [ -f "$REGISTRY_SCHEMA" ]; then
    pass "finding-registry schema exists"
else
    fail "finding-registry schema exists"
fi

echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
