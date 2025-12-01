---
name: Bug Report
about: Report a bug or unexpected behavior
title: '[BUG] '
labels: bug
assignees: ''
---

## Bug Description
<!-- A clear and concise description of what the bug is -->

## Steps to Reproduce
<!-- Steps to reproduce the behavior -->
1. 
2. 
3. 

## Expected Behavior
<!-- What you expected to happen -->

## Actual Behavior
<!-- What actually happened -->

## Environment
**Mistokenly Version:** 
<!-- e.g., v1.0.0, commit hash, or "latest" -->

**Deployment Method:**
<!-- e.g., Docker, Kubernetes, Local -->

**Operating System:**
<!-- e.g., Ubuntu 22.04, macOS 14, Windows 11 -->

**Go Version:**
<!-- If building from source -->

**Additional Environment Details:**
<!-- Database versions, Redis version, etc. -->

## Logs and Error Messages
<details>
<summary>Click to expand logs</summary>

```
<!-- Paste relevant logs here -->
```

</details>

## Configuration
<!-- Relevant configuration (REDACT ANY SECRETS!) -->
```yaml
# Example: .env values (DO NOT include KEK or JWT secrets)
REDIS_HOST=localhost
STORAGE_DB_HOST=localhost
```

## Additional Context
<!-- Any other context, screenshots, or information -->

## Possible Solution
<!-- Optional: suggestions for fixing the bug -->

## Checklist
- [ ] I have searched existing issues to ensure this is not a duplicate
- [ ] I have included all relevant environment details
- [ ] I have redacted any sensitive information from logs/config
- [ ] I can consistently reproduce this issue
