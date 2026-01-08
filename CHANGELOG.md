# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [1.0.0] - 2026-01-08

### Added
- Thread-safe tag caching to prevent race conditions during concurrent uploads
- Filename sanitization for disk writes (removes invalid filesystem characters)
- ZIP slip vulnerability protection for archive extraction
- File validation at process start to prevent deadlocks
- Comprehensive unit tests (coverage: 25.9% → 40.1%)

### Fixed
- Worker early termination bug where `break` was used instead of `continue`
- Tag cache infinite retry loop in integration tests
- Error chain preservation (replaced `%v` with `%w` throughout codebase)
- HTTP client timeout consistency (100s → 10s)

### Security
- Path traversal protection in ZIP file extraction
- Invalid filename character handling for disk operations

[1.0.0]: https://github.com/yourusername/enex2paperless/releases/tag/v1.0.0
