# TEST ARCHITECTURE BOOK

## Simple 3-Layer Testing Strategy

**Core Principle**: Each layer has clear boundaries and specific responsibilities. No overlap.

---

## 🎯 When to Use Each Test Type

### Unit Tests (`*_test.go` in same package)
**Use for**: Single function logic, algorithms, data validation

**When**:
- ✅ Testing pure functions with no external dependencies
- ✅ Validating business logic (debouncing, parsing)
- ✅ Fast feedback needed (<100ms per test)
- ✅ Mocking git/filesystem/network calls

**Run**: `make test-unit` (specific package focus via test filtering)
**Time**: <30 seconds for full unit suite

### Integration Tests (`*_integration_test.go`)
**Use for**: Component interactions with real dependencies

**When**:
- ✅ Testing SyncService + GitManager together
- ✅ Real file system operations (not mocked)
- ✅ Actual git repository operations
- ✅ Medium complexity (1-5 seconds per test)

**Run**: `make test-integration`
**Time**: <2 minutes for full integration suite

### E2E Tests (`test/scenarios/*.go`)
**Use for**: Complete user workflows from CLI

**When**:
- ✅ Testing full DSM commands (`dsm add`, `dsm sync`, `dsm start`)
- ✅ Cross-platform compatibility
- ✅ Real git remotes and SSH authentication
- ✅ **Real editor functionality** (`$EDITOR`, vim/nano/micro integration)
- ✅ Complex scenarios (>5 seconds per test)

**Run**: `./test/scripts/run-e2e.sh`
**Time**: <10 minutes for full E2E suite

**Editor Testing**: Only E2E can test real editor behavior:
- `$EDITOR` environment variable handling
- Editor launch and file opening
- Conflict resolution with real editors
- Editor-specific behaviors (vim recovery, nano safety)

---

## 🚫 Clear Boundaries (What NOT to Test)

### Unit Tests SHOULD NOT:
- ❌ Make real network calls
- ❌ Touch actual filesystem
- ❌ Execute external commands
- ❌ Depend on git operations

### Integration Tests SHOULD NOT:
- ❌ Test full CLI user workflows
- ❌ Use Docker containers
- ❌ Test cross-platform behavior
- ❌ Make network calls to external services

### E2E Tests SHOULD NOT:
- ❌ Test individual function optimizations
- ❌ Validate internal algorithm details
- ❌ Mock external dependencies
- ❌ Mock editor behavior (use real editors instead)

---

## 📊 Coverage Goals

| Layer | Target | What to Cover |
|-------|--------|---------------|
| Unit | 85% | Business logic, algorithms |
| Integration | 70% | Component interactions |
| E2E | User Scenarios | Complete workflows |

**Priority Areas**:
1. Git operations (currently 54% → target 75%)
2. Process management (currently 55% → target 70%)
3. CLI commands (currently 58% → target 80%)

---

## ⚡ Performance Targets

| Test Type | Target Time | Max Time | Parallel |
|-----------|-------------|-----------|----------|
| Unit | <10ms | 100ms | 8x |
| Integration | <500ms | 2s | 4x |
| E2E | <5s | 30s | 2x |

---

## 🔧 Local Development Workflow

```bash
# Fast feedback - unit tests only
make test-unit

# Medium feedback - unit + integration
make test-integration

# Full feedback - everything
make test-all
```

**🚨 IMPORTANT**: Always use `make` commands - the Makefile is the single source of truth for all testing operations.

## 🏗️ CI Pipeline Strategy

```yaml
# GitHub Actions jobs
test:           # Unit + Integration (fast, parallel)
  timeout: 10m

e2e-tests:      # Full scenarios (slower, sequential)
  timeout: 20m
  depends: test
```

---

## 📋 Test Decision Flow

```
Are you testing a single function? → Unit Test
Are you testing component interaction? → Integration Test
Are you testing a complete user workflow? → E2E Test
```

**Examples**:
- `TestDebouncer_Trigger()` → Unit Test
- `TestSyncService_WithRealGit()` → Integration Test
- `TestScenario_CompleteSyncWorkflow()` → E2E Test
- `TestScenario_EditorBasicWorkflow()` → E2E Test (editor integration)
- `TestScenario_DsmAddWorkflow()` → E2E Test (dsm add workflow)

---

## ✅ Simple Rules

1. **One concern per test**: Don't mix unit and integration logic
2. **Use descriptive names**: `TestSyncService_ShouldCommit_WhenFilesChanged`
3. **Clean up after yourself**: Use `t.Cleanup()` and `t.TempDir()`
4. **Test behavior, not implementation**: Focus on what users see
5. **Keep tests fast**: If test is slow, move it up a layer

---

**That's it. Simple boundaries, clear responsibilities, fast feedback.**
