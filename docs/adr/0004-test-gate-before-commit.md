# ADR 0004: Passing tests as a prerequisite for commits

Status: Accepted · Date: 2026-05-28

## Context
Multiple agents work on the project; the codebase should be runnable at all
times.

## Decision
A commit is made only if `make test` (vet + all tests) is green. In CI, the
same run is the merge gate (.github/workflows/ci.yml). Tests run without
hardware via the mock driver.

## Rationale
Keeps main green at all times, makes contributions from different agents safe
to combine, and documents expected behavior.

## Consequences
Every new code path needs a test, including error paths.
