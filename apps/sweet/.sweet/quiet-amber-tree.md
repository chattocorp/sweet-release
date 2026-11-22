---
type: feature
---

Add `sweet status --rev <tree-ish>` to read the release plan from a Git commit, tag, or tree object instead of the working directory, without a checkout. Only committed blobs are read; working tree changes, untracked files, and the index are ignored. Output matches `sweet status` in a checkout of that tree without Git filters (eol, LFS, ident) and without symlinks that leave the tree.
