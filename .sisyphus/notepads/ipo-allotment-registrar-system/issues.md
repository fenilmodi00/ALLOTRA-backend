# Issues - IPO Allotment Registrar System

## [2026-03-02] Task 4: Repository Table Name Bug

**Problem**: Initial implementation used wrong table name `registrar_codes` instead of `ipo_registrar_codes`

**Impact**: All SQL queries would fail

**Resolution**: 
- Session resumed: `ses_3527bb2feffelE5hG7W69SKqjQ`
- Fixed all queries in `registrar_code_repository.go`
- Verified: grep for table name, build passes

**Lesson**: Always cross-reference schema file when writing repository queries

## [2026-03-02] Task 5: Missing Type Definitions Blocker

**Problem**: 
- `tools/registrars/*` package code referenced types that didn't exist in `shared`:
  - `shared.AllotmentResult`
  - `shared.DropdownOption`
  - Status constants

**Root Cause**: 
- `tools/` directory untracked in git
- Interface expected types from `shared` but implementations had them locally

**Resolution**:
- Created `shared/registrar_types.go` with proper type definitions
- `DropdownOption` uses `ID` field (not `Code`)
- Status constants: `StatusAllotted`, `StatusNotAllotted`, `StatusNotFound`, `StatusError`
- Committed to main repo: `65ec40b`

**Lesson**: Check for untracked directories that might contain crucial type definitions

## [2026-03-02] Worktree Environment Issues

**Problem**: Worktree missing `tools/` directory (untracked in git)

**Resolution**: Manually copied `tools/` to worktree to enable builds

**Impact**: Build system requires `tools/` present even though it's untracked

**Note**: `shared/` package was tracked and present in worktree after fix
