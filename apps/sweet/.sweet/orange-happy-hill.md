---
type: fix
---

Update the release PR workflow artifact actions to use Node 24 instead of the end-of-life Node 20 runtime. Fix release PR cleanup when only one release PR job runs: the newer artifact download places a single result directly in the download directory, which cleanup did not find. Cleanup now finds results in either layout and fails when any are missing.
