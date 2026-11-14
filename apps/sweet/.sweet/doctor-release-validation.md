---
type: fix
---

Make `sweet doctor` fail on invalid pending release notes, package configuration,
and tag ownership problems, with file paths in diagnostics. Validate release
preparation without changing files, including configured version targets.
Keep `sweet status` informational and its JSON problems format unchanged.
Add a reusable `doctor.yml` workflow for pull request CI.
