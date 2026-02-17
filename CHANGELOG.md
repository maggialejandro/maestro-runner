# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.6] - 2026-02-17

### Fixed
- iOS WDA: off-screen elements no longer returned by `findElement` — `assertVisible`, `tapOn`, `scrollUntilVisible`, and all element commands now correctly reject elements not visible in the viewport
- iOS WDA: `scrollUntilVisible` no longer skips scrolling when the target element exists in the accessibility tree but is off-screen
- iOS WDA: `scrollUntilVisible` direction matching is now case-insensitive (e.g., `direction: "DOWN"` works)
- iOS WDA: `waitForIdleTimeout` now works on iOS via WDA quiescence
- `when: platform` condition was ignored in `runFlow` blocks ([#8](https://github.com/devicelab-dev/maestro-runner/issues/8))

### Contributors

[@janfreund](https://github.com/janfreund)
1. Reported `scrollUntilVisible` and element visibility issues on iOS ([#2](https://github.com/devicelab-dev/maestro-runner/issues/2))

[@kavithamahesh](https://github.com/kavithamahesh)
1. Reported `when: platform` condition being ignored ([#8](https://github.com/devicelab-dev/maestro-runner/issues/8))

## [1.0.5] - 2026-02-16

### Added
- `tapOn: point` now supports absolute pixel coordinates (e.g., `point: "286, 819"`) in addition to percentages
- Coordinate validation: negative values, out-of-bounds pixels, and percentage range (0-100%) are all rejected with clear error messages
- Screen size cached at session startup instead of fetching on every tap/swipe/scroll
- `launchApp: environment` for passing environment variables via WDA `launchEnvironment`

### Changed
- Extracted shared helpers (`ParsePointCoords`, `ParsePercentageCoords`, `RandomString`, `SuccessResult`, etc.) from drivers into `pkg/core`
- Removed hardcoded 1080x1920 screen size fallback in UIAutomator2 scroll/swipe

### Fixed
- `launchApp: arguments` silently failed on real iOS devices — early return after session creation, unpopulated env map, activate vs launch, missing variable expansion
- Removed unused AI flags (`--analyze`, `--api-url`, `--api-key`)

### Contributors

[@mahesh-e27](https://github.com/mahesh-e27)
1. Reported `tapOn: point` not supporting absolute pixel coordinates ([#6](https://github.com/devicelab-dev/maestro-runner/issues/6))
2. Spotted unused AI flags (`--analyze`, `--api-url`, `--api-key`)

[@majdukovic](https://github.com/majdukovic)
1. Reported `launchApp: arguments` not working on real iOS devices ([#7](https://github.com/devicelab-dev/maestro-runner/issues/7))

## [1.0.4] - 2026-02-13

### Added
- `keyPress` option for character-by-character text input on Android
- Stale socket cleanup on force-stop (Ctrl+C / kill -9) with PID-based locking

### Fixed
- iOS Appium driver: element finding and tap reliability (use `label` instead of `content-desc` for accessibility)
- iOS Appium driver: `pressKey` command support
- iOS Appium driver: `tapOn` and `inputText` reliability improvements
- iOS Appium driver: skip `--app-file` and `--team-id` pre-checks (not needed for Appium)
- iOS Appium driver: skip `clearState` on real devices (`mobile: clearApp` only works on simulators)
- iOS WDA driver: auto-alert handling on simulators (accept/dismiss permission dialogs)
- `takeScreenshot` command now correctly saves PNG files
- GitHub star link in HTML report
- All `errcheck` violations fixed with proper error logging

### Contributors

[@SuperRoach](https://github.com/SuperRoach)
1. Suggested the `keyPress` feature for character-by-character input
2. Suggested the `--team-id` pre-check for WDA driver
3. Reported the `takeScreenshot` bug

[u/Healthy_Carpet_26](https://www.reddit.com/user/Healthy_Carpet_26/)
1. Reported the stale socket issue on force-stop (Ctrl+C)

[@kavithamahesh](https://github.com/kavithamahesh)
1. Reported iOS element finding issue — `label` instead of `content-desc` ([#3](https://github.com/devicelab-dev/maestro-runner/issues/3))
2. Reported `pressKey` not working for iOS on Saucelabs ([#4](https://github.com/devicelab-dev/maestro-runner/issues/4))

[@janfreund](https://github.com/janfreund)
1. Reported clearState and iOS permission dialog handling issues ([#2](https://github.com/devicelab-dev/maestro-runner/issues/2))

## [0.1.0] - 2026-01-27

### Added
- CLI with `validate` and `run` commands
- Configuration loading from `config.yaml`
- YAML flow parser with support for all Maestro commands
- Flow validator with dependency resolution
- Tag-based test filtering (include/exclude)
- UIAutomator2 driver with native element waiting
- Appium driver with per-flow sessions and capabilities file support
- WDA driver for iOS via WebDriverAgent
- JavaScript scripting engine (`evalScript`, `assertTrue`, `runScript`)
- Regex pattern matching for element selectors (`assertVisible`, `copyTextFrom`)
- Coordinate-based swipe and percentage-based tap support
- Nested relative selector support
- Step-level and command-level configurable timeouts
- Context-based timeout management
- Configurable `waitForIdleTimeout` for UIAutomator2
- `inputRandom` with DataType support
- JSON report output with real-time updates
- HTML report generator with sub-command expansion for `runFlow`, `repeat`, `retry`
- Clickable element prioritization for Appium

### Fixed
- JS `evalScript` and `assertTrue` parsing for Maestro `${...}` syntax
- Step counting accuracy in reports
- Appium driver regex matching
