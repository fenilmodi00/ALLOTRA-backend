# Unresolved Problems

_No blockers identified yet._

---

---

## [2026-03-02 13:35] CRITICAL BLOCKER: tools/registrars Code is Broken

**SEVERITY**: BLOCKING - Cannot proceed with Wave 2

**ISSUE**: The `tools/registrars/*` package code references undefined types from `shared` package:
- `shared.AllotmentResult` - used in kfin, bigshare, mufg clients
- `shared.DropdownOption` - used in all registrar clients  
- `shared.StatusNotFound`, `shared.StatusAllotted`, `shared.StatusNotAllotted` - status constants

**EVIDENCE**:
```bash
$ go build -v ./tools/registrars/...
tools\registrars\kfin\client.go:85:96: undefined: shared.AllotmentResult
tools\registrars\bigshare\client.go:88:96: undefined: shared.AllotmentResult
tools\registrars\mufg\client.go:132:96: undefined: shared.AllotmentResult
# ... 50+ undefined symbol errors
```

**ROOT CAUSE**:
- `tools/` directory is UNTRACKED in git (not committed)
- The code in `tools/registrars/*` imports types that don't exist in the committed `shared/` package
- This suggests incomplete/WIP code was left untracked

**IMPACT**:
- Task 5 (RegistrarCodeService) CANNOT build because it depends on `tools/registrars`
- Tasks 6-10 all depend on Task 5
- **Entire Wave 2 and Wave 3 are BLOCKED**

**ATTEMPTED WORKAROUNDS**:
1. ✗ Copied `tools/` to worktree → Still fails due to missing `shared` types
2. ✗ Tried building in main repo → Fails identically

**REQUIRED FIX**:
Either:
A) Define missing types in `shared/` package (DropdownOption, AllotmentResult, status constants)
B) Fix `tools/registrars/*` to not use those types
C) Commit working version of `tools/` from elsewhere

**STATUS**: ESCALATED TO USER - Awaiting direction
