# Agent Behavior Tests

Pattern-based integration tests that validate Claude/HAL agent behavior.

## Why These Tests Exist

Claude responses are non-deterministic, so we can't test for exact output. Instead, we validate that responses contain (or don't contain) certain patterns that indicate correct behavior.

These tests catch regressions in agent behavior, particularly around:
- Boundary enforcement (not doing manual workarounds without asking)
- Tool/skill failure handling
- Preference system functionality

## Running Tests

```bash
# Run all tests
./test_agent_behavior.sh

# Run a single test
./test_agent_behavior.sh skill_failure
./test_agent_behavior.sh preferences_create
./test_agent_behavior.sh boundary
```

## Prerequisites

- `claude` CLI must be in PATH
- `hal9000` CLI should be in PATH (some tests will skip if not available)

## Test Cases

| Test | What it validates |
|------|-------------------|
| `skill_failure` | When skill fails, agent reports error or asks user (doesn't proceed autonomously) |
| `preferences_create` | `hal9000 preferences set` creates new file when missing |
| `preferences_append` | `hal9000 preferences set` appends section when file exists |
| `boundary` | Agent doesn't attempt manual workarounds without asking |

## Adding New Tests

1. Add a test function following the naming pattern `test_<name>()`
2. Use helper functions:
   - `run_claude "prompt"` - Run claude with a prompt, returns output
   - `contains_pattern "$output" "regex"` - Check if output matches pattern
   - `not_contains_pattern "$output" "regex"` - Check output doesn't match
   - `log_pass "test_name"` - Record pass
   - `log_fail "test_name" "expected" "got"` - Record failure
3. Add the test to `run_all_tests()` and `run_single_test()` case statement

## Limitations

- Tests invoke real Claude sessions (slow, uses API credits)
- Pattern matching may have false positives/negatives
- Tests assume specific project structure

## Future Improvements

- Mock Claude responses for faster unit testing
- Test case configuration via YAML
- CI integration with rate limiting
