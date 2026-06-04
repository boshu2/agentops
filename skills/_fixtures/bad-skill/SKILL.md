---
description: ""
---

## Setup

Planted lint fixture for `scripts/skill-eval.sh`. Intentionally broken:
this front matter omits `name`/`id` (missing required metadata), leaves the
description empty, and hardcodes an AWS key (a secret) in the code block below.

```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```
