# AGENTS.md

## Required development workflow

Develop every behavior change using test-driven development:

1. Add or update a test that describes the expected behavior.
2. Run the relevant test and confirm that it fails for the expected reason.
3. Implement the smallest change that makes the test pass.
4. Refactor while keeping the tests green.

Add a regression test for every bug fix. During development, use targeted tests
for fast feedback. Before completing a development task, format the code and run
the complete validation suite:

```sh
make fmt
go test -race ./...
make lint
```

Never report a development task as complete while formatting, tests, or lint
are failing.

## README scope

Keep the README focused on general information about the application, including
its purpose, main capabilities, installation, and essential configuration. It
does not need to document every interaction, key binding, layout detail,
implementation detail, or edge case. Prefer the interface itself, focused
documentation, and tests for detailed behavior.

## Test isolation

Unit tests must exercise behavior through abstractions and must not perform real
external operations. Keep side effects behind small interfaces and inject fakes,
stubs, or spies in tests. In particular, unit tests must not:

- read, write, or delete real user files;
- execute real shell commands or scripts;
- open editors or other applications;
- access the real clipboard;
- terminate the process;
- mutate the developer's working directory or persistent environment;
- access network services.

Use the existing `FileManagerInterface`, `UtilsInterface`,
`ViewBuilderInterface`, and package-local interfaces where applicable. Introduce
a focused abstraction when new code crosses an external boundary instead of
calling the operating system directly from business or orchestration logic.
Fakes should record inputs and provide deterministic outputs so tests can assert
both results and intended side effects.

Tests specifically covering filesystem or process adapters may perform real
operations only when the behavior cannot be verified through a unit-level fake.
Keep filesystem operations inside `t.TempDir()`, replace executables with
controlled test doubles, restore any temporary environment changes with
`t.Setenv()` or `t.Cleanup()`, and never use user paths or live external
services.

## Automatic version bumps

Every completed implementation must include an automatic application version
bump before the final formatting and validation commands. Classify the entire
change by its highest-impact item and bump the version only once:

- **Feature:** increment the minor version and reset the patch version, for
  example `0.1.0` -> `0.2.0`.
- **Bug fix or adjustment:** increment the patch version, for example `0.1.0`
  -> `0.1.1`. This includes fixes, small usability changes and internal
  adjustments that do not add a new capability.
- **Major change:** increment the major version only when the implementation
  substantially changes how the application works. Follow the project
  convention represented by `0.1.0` -> `1.1.0`.

Keep the application version in `internal/version/version.go` and every other
version occurrence synchronized, including the version shown by the TUI. Do
not confuse the application version with data or configuration schema versions.
Documentation-only changes do not require a version bump unless they accompany
an implementation.
