# Example Session: Software Development Profile

**Profile**: `software-dev`
**Scenario**: Build new REST API feature with React frontend
**Duration**: ~2 hours
**Outcome**: Feature deployed to production

---

## Session Start (9:00 AM)

**User**: "I need to add a user preferences API to our app with a settings UI"

**System Auto-Detects**:
- Keywords: "API", "settings UI" → software-dev profile
- Context loaded: 3k (foundational) + 12k (software-dev core) = 15k tokens (7.5%)

**Claude**: "I'll help you build this feature end-to-end. Let me load the software-dev profile."

**Loaded artifacts**:
- Backend agents (applications-create-app, services-*)
- Frontend agents (documentation-create-docs)
- Pipeline agents (pipelines-gitlab-ci)
- Validation skills (test.sh, rendering.sh)

---

## Phase 1: Design (9:00-9:20 AM)

**User**: "Start with Read CLAUDE.md-task"

**Claude**: "Let me understand the requirements first."

**Questions asked**:
1. Where should this API live? (New microservice vs existing app)
2. What preferences need to be stored? (Theme, language, notifications)
3. Auth requirements? (JWT-based, user-specific)
4. Storage backend? (PostgreSQL via Crossplane)

**Design decisions**:
- Add to existing user-service (not new microservice)
- Preferences: theme, language, notification settings
- REST endpoints: GET /users/:id/preferences, PUT /users/:id/preferences
- Frontend: React settings modal in dashboard
- Database: Add preferences column to users table

**Output**: Design doc with API spec, database schema, UI mockup

---

## Phase 2: Backend Implementation (9:20-10:00 AM)

**Claude loads**: Backend agents (applications-modify-app, services-edb-databases)

**Tasks executed**:

### 2.1 Database Schema (5 min)
```sql
-- Migration: Add preferences column
ALTER TABLE users ADD COLUMN preferences JSONB DEFAULT '{}';
CREATE INDEX idx_users_preferences ON users USING gin(preferences);
```

### 2.2 API Implementation (20 min)

**Created files**:
- `services/user-service/routes/preferences.go`
- `services/user-service/handlers/preferences_handler.go`
- `services/user-service/models/preferences.go`

**Code generated** (validated by Claude):
```go
// GET /api/users/:id/preferences
func GetPreferences(c *gin.Context) {
    // Implementation with error handling
}

// PUT /api/users/:id/preferences
func UpdatePreferences(c *gin.Context) {
    // Validation + update logic
}
```

### 2.3 Tests (15 min)

**Created**:
- `services/user-service/handlers/preferences_handler_test.go`

**Tests cover**:
- Get preferences for existing user
- Update preferences successfully
- Validation errors (invalid JSON, unauthorized access)
- Edge cases (empty preferences, partial updates)

### 2.4 Validation (5 min)

**Claude runs**: `test.sh` skill
```bash
cd services/user-service
go test ./handlers/... -v
# ✅ All tests pass
```

---

## Phase 3: Frontend Implementation (10:00-10:40 AM)

**Claude loads**: Frontend agents (React component patterns)

**Context7 used**: React hooks documentation, form validation patterns

### 3.1 API Client (10 min)

**Created**: `frontend/src/api/preferences.ts`

```typescript
export const getPreferences = async (userId: string) => {
  const response = await fetch(`/api/users/${userId}/preferences`)
  return response.json()
}

export const updatePreferences = async (
  userId: string,
  preferences: UserPreferences
) => {
  const response = await fetch(`/api/users/${userId}/preferences`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(preferences)
  })
  return response.json()
}
```

### 3.2 React Component (20 min)

**Created**: `frontend/src/components/SettingsModal.tsx`

**Features**:
- Form with theme selector, language dropdown, notification toggles
- Real-time validation
- Loading states
- Error handling
- Success feedback

### 3.3 Integration (10 min)

**Modified**: `frontend/src/pages/Dashboard.tsx`
- Added settings button
- Modal integration
- State management (React Context)

### 3.4 Frontend Validation (5 min)

**Claude runs**:
```bash
npm run lint
# ✅ No errors

npm run type-check
# ✅ TypeScript passes

npm run test:unit -- SettingsModal
# ✅ Component tests pass
```

---

## Phase 4: Integration & Deployment (10:40-11:00 AM)

### 4.1 Local Testing (10 min)

**Claude**: "Starting local environment..."

```bash
# Backend
docker-compose up user-service postgres

# Frontend
npm run dev
```

**Manual testing**:
- ✅ GET /api/users/123/preferences returns defaults
- ✅ PUT updates preferences correctly
- ✅ UI shows current preferences
- ✅ Form validation works
- ✅ Updates reflect immediately

### 4.2 CI/CD Pipeline (10 min)

**Claude loads**: Pipeline agent (pipelines-gitlab-ci)

**Updated**: `.gitlab-ci.yml`

```yaml
test-preferences-api:
  stage: test
  script:
    - cd services/user-service
    - go test ./handlers/... -cover

test-settings-ui:
  stage: test
  script:
    - npm run test:unit -- SettingsModal
    - npm run test:integration -- settings
```

### 4.3 Kubernetes Deployment (5 min)

**No changes needed**: Using existing user-service deployment

**Verification**:
```bash
kubectl rollout status deployment/user-service
# ✅ deployment "user-service" successfully rolled out
```

---

## Phase 5: Validation & Documentation (11:00-11:20 AM)

### 5.1 Complete Validation (10 min)

**Claude runs**: Validation suite

```bash
# Backend tests
make test APP=user-service
# ✅ 47 tests passed

# Frontend tests
npm run test
# ✅ 23 tests passed

# Integration tests
npm run test:e2e -- settings
# ✅ 5 e2e tests passed
```

### 5.2 Documentation (10 min)

**Claude creates**: `docs/api/preferences-endpoint.md`

**Includes**:
- API specification (OpenAPI)
- Request/response examples
- Error codes
- Frontend usage example

**Updated**: `CHANGELOG.md`

```markdown
## [Unreleased]
### Added
- User preferences API (GET/PUT /api/users/:id/preferences)
- Settings modal in dashboard UI
- Preferences persistence to PostgreSQL
```

---

## Phase 6: Commit & Deploy (11:20 AM)

### 6.1 Git Commit

**Claude runs**: Semantic commit with AgentOps template

```bash
git add services/user-service frontend/src docs/api
git commit -m "$(cat <<'EOF'
feat(api): add user preferences endpoint with settings UI

## Context
Users needed ability to customize app experience (theme, language, notifications).
No existing preferences system - all settings were hardcoded.

## Solution
Backend:
- Added preferences JSONB column to users table
- Implemented GET/PUT /api/users/:id/preferences endpoints
- Full test coverage (47 tests)

Frontend:
- Created SettingsModal component with form validation
- Integrated with dashboard
- Added API client with TypeScript types

## Learning
- JSONB column in Postgres works well for flexible preferences schema
- React Context for settings state avoids prop drilling
- Form validation library (react-hook-form) saved ~30 min implementation time

## Impact
- Users can now customize: theme (3 options), language (5 languages), notifications (3 types)
- Estimated adoption: 60% of users will customize within first week
- Reduces support tickets for "how do I change theme?" (currently 5/week)

EOF
)"
```

### 6.2 Push & Deploy

```bash
git push origin feature/user-preferences

# GitLab CI automatically:
# 1. Runs tests (backend + frontend)
# 2. Builds containers
# 3. Deploys to staging
# 4. Waits for approval
# 5. Deploys to production
```

**ArgoCD sync**: Automatic (GitOps pattern)

---

## Session Summary

### Time Breakdown
- Design: 20 min
- Backend implementation: 40 min
- Frontend implementation: 40 min
- Integration & deployment: 20 min
- Validation & documentation: 20 min
- **Total**: 2 hours 20 minutes

### Artifacts Created
- 8 new files (Go handlers, models, tests, React components, API client, docs)
- 2 modified files (Dashboard.tsx, .gitlab-ci.yml)
- ~600 lines of code
- 75 tests (backend + frontend)
- API documentation

### Agents Used
1. `applications-modify-app` (backend development)
2. `services-edb-databases` (database schema)
3. `documentation-create-docs` (API docs)
4. `pipelines-gitlab-ci` (CI/CD)
5. Skills: `test.sh`, `rendering.sh`

### Context Usage
- Peak: 22k tokens (11% of window)
- Average: 16k tokens (8% of window)
- Well under 40% rule ✅

### Validation
- ✅ All tests pass (backend + frontend)
- ✅ CI pipeline succeeds
- ✅ Deployed to production
- ✅ Feature working in staging environment
- ✅ Documentation complete

### Learnings Captured
1. JSONB preference storage pattern (reusable)
2. React Context for user settings (reusable)
3. Form validation library integration (saved 30 min)

---

## What Made This Efficient

### 1. Profile Auto-Detection
Keywords "API" + "settings UI" loaded correct profile immediately

### 2. Full-Stack Coverage
Single profile had both backend AND frontend tools (no context switching)

### 3. JIT Loading
Only loaded agents when needed:
- Backend agents loaded during API implementation
- Frontend agents loaded during UI work
- Pipeline agents loaded during deployment

### 4. Reusable Patterns
Drew from 7 golden patterns (gitops-apps/examples/) for app structure

### 5. Validation Gates
Continuous testing prevented bugs from accumulating

### 6. Documentation as Code
API docs generated alongside implementation (not afterthought)

---

## Alternative Without Profile

**Estimated time without software-dev profile**: ~4-6 hours

**Why slower?**
- ❌ Manual agent discovery ("which agent do I need?")
- ❌ Context switching (backend → frontend → deployment)
- ❌ Missing connections (frontend examples not linked to backend patterns)
- ❌ Duplicate research (API design + UI patterns researched separately)

**With profile**: 2.3 hours (2-3x faster)

---

## Next Steps

**Post-deployment**:
- Monitor user adoption metrics
- Gather feedback on settings options
- Consider adding more customization options based on usage

**Pattern extraction**:
- Extract "preferences API" pattern for reuse
- Document React form validation approach
- Add to golden patterns catalog

**Continuous improvement**:
- Track time savings on future similar features
- Refine profile based on what worked well
- Consider adding preferences-specific agent if frequently used
