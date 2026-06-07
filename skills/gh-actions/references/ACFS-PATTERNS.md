# ACFS Checksum Notification Patterns

## Notify ACFS on Release

When releasing a tool managed by ACFS (Authenticated Checksum File System), notify it to update checksums.

```yaml
notify-acfs:
  needs: release
  runs-on: ubuntu-latest
  steps:
    - uses: peter-evans/repository-dispatch@v3
      with:
        token: ${{ secrets.ACFS_DISPATCH_TOKEN }}
        repository: owner/acfs-checksums
        event-type: checksum-update
        client-payload: |
          {
            "tool": "${{ github.event.repository.name }}",
            "version": "${{ needs.release.outputs.version }}",
            "repo": "${{ github.repository }}"
          }
```

---

## ACFS Receiver Workflow

In the ACFS checksums repository:

```yaml
# .github/workflows/update-checksums.yml
name: Update Checksums

on:
  repository_dispatch:
    types: [checksum-update]
  workflow_dispatch:
    inputs:
      tool:
        required: true
      version:
        required: true

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Update checksums
        env:
          TOOL: ${{ github.event.client_payload.tool || inputs.tool }}
          VERSION: ${{ github.event.client_payload.version || inputs.version }}
        run: ./scripts/update-tool-checksums.sh "$TOOL" "$VERSION"

      - name: Commit changes
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add checksums/
          git commit -m "Update $TOOL to $VERSION" || exit 0
          git push
```

---

## Checksum Health Monitoring

Schedule regular verification that checksums match published artifacts:

```yaml
name: Checksum Health

on:
  schedule:
    - cron: '0 6,18 * * *'  # Twice daily
  workflow_dispatch:

permissions:
  contents: read
  issues: write

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Verify all checksums
        id: verify
        run: |
          ./scripts/verify-all-checksums.sh 2>&1 | tee results.txt
          if grep -q "MISMATCH" results.txt; then
            echo "healthy=false" >> $GITHUB_OUTPUT
          else
            echo "healthy=true" >> $GITHUB_OUTPUT
          fi

      - name: Create issue if unhealthy
        if: steps.verify.outputs.healthy == 'false'
        run: |
          gh issue create \
            --title "URGENT: Checksum verification failed" \
            --label "security,checksum-drift" \
            --body "$(cat results.txt)"
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## Multi-Tool Dispatch

Notify multiple tools at once:

```yaml
- name: Notify all package managers
  run: |
    for repo in homebrew-tap scoop-bucket acfs-checksums; do
      gh api repos/owner/$repo/dispatches \
        -f event_type=update \
        -f client_payload='{"tool":"${{ env.TOOL }}","version":"${{ env.VERSION }}"}'
    done
  env:
    GH_TOKEN: ${{ secrets.DISPATCH_TOKEN }}
```
