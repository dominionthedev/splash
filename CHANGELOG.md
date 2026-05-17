# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-05-17

### Added
- **Lua DSL**: Core primitives for defining workflows (`workflow`, `step`, `scope`, `execute`, `reason`, `task`).
- **Constrained Agent**: Reasoning layer locked to workflow scope and capabilities.
- **Orchestrator**: Sequential and concurrent execution engine.
- **Capabilities**: Built-in support for `filesystem.read`, `filesystem.write`, and `process.execute`.
- **Knowledge Store**: Self-optimizing storage for run history and reinforced knowledge.
- **CLI**: `splash run` and `splash inspect` commands.
- **Examples**: Sample workflows for automated test fixing and code review.
- **CI/CD**: GitHub Actions for testing and releases.
- **Documentation**: Initial README, PROJECT.md, and ROADMAP.md.
