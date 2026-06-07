# GitHub API — Reference

## Table of Contents
- [REST API](#rest-api)
- [GraphQL API](#graphql-api)
- [Common Patterns](#common-patterns)

---

## REST API

### Basic Requests

```bash
# GET request
gh api repos/owner/repo

# GET with query params
gh api repos/owner/repo/issues?state=open

# POST request
gh api repos/owner/repo/issues -f title="New issue" -f body="Description"

# PATCH request
gh api repos/owner/repo/issues/123 -X PATCH -f state="closed"

# DELETE request
gh api repos/owner/repo/issues/123/comments/456 -X DELETE
```

### Pagination

```bash
# Paginate results
gh api repos/owner/repo/issues --paginate

# Limit pages
gh api repos/owner/repo/issues --paginate --limit 100
```

### Output Formatting

```bash
# JSON output (default)
gh api repos/owner/repo

# JQ filtering
gh api repos/owner/repo --jq '.name'
gh api repos/owner/repo/issues --jq '.[].title'

# Template output
gh api repos/owner/repo --template '{{.name}} - {{.description}}'
```

---

## GraphQL API

### Basic Queries

```bash
# Simple query
gh api graphql -f query='{ viewer { login } }'

# Query with variables
gh api graphql -f query='
  query($owner: String!, $repo: String!) {
    repository(owner: $owner, name: $repo) {
      name
      description
      stargazerCount
    }
  }
' -f owner="owner" -f repo="repo"
```

### Common Queries

```bash
# Get viewer info
gh api graphql -f query='{
  viewer {
    login
    name
    email
  }
}'

# List repositories
gh api graphql -f query='{
  viewer {
    repositories(first: 10) {
      nodes {
        name
        url
      }
    }
  }
}'

# Get issue details
gh api graphql -f query='
  query($owner: String!, $repo: String!, $number: Int!) {
    repository(owner: $owner, name: $repo) {
      issue(number: $number) {
        title
        body
        state
        author { login }
      }
    }
  }
' -f owner="owner" -f repo="repo" -F number=123
```

### Mutations

```bash
# Add reaction
gh api graphql -f query='
  mutation($id: ID!) {
    addReaction(input: {subjectId: $id, content: THUMBS_UP}) {
      reaction { content }
    }
  }
' -f id="ISSUE_NODE_ID"
```

---

## Common Patterns

### Get PR Comments

```bash
gh api repos/owner/repo/pulls/123/comments
```

### Get Issue Comments

```bash
gh api repos/owner/repo/issues/123/comments
```

### Create Comment

```bash
gh api repos/owner/repo/issues/123/comments -f body="Comment text"
```

### Get Workflow Runs

```bash
gh api repos/owner/repo/actions/runs
```

### Get Repository Info

```bash
gh api repos/owner/repo --jq '{name, description, stars: .stargazers_count}'
```

### List Organization Repos

```bash
gh api orgs/ORGNAME/repos --paginate --jq '.[].name'
```

### Get User Info

```bash
gh api users/USERNAME
```

---

## Tips

- Use `-F` for non-string values: `-F number=123` (integer)
- Use `--silent` to suppress output
- Use `--include` to show response headers
- Use `--cache 1h` to cache responses
