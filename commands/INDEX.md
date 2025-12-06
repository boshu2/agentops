# Commands Index

## By Category

### RPI Workflow (Core)
| Command | Description |
|---------|-------------|
| [research](research.md) | Phase 1 - Deep exploration before planning |
| [plan](plan.md) | Phase 2 - Specify exact changes with file:line |
| [implement](implement.md) | Phase 3 - Execute approved plan with validation |

### Bundles (Context Persistence)
| Command | Description |
|---------|-------------|
| [bundle-save](bundle-save.md) | Save compressed context to filesystem |
| [bundle-load](bundle-load.md) | Restore context from saved bundle |
| [bundle-search](bundle-search.md) | Semantic search across bundles |
| [bundle-list](bundle-list.md) | List all available bundles |
| [bundle-prune](bundle-prune.md) | Garbage collection with knowledge extraction |
| [bundle-load-multi](bundle-load-multi.md) | Load multiple bundles at once |

### Session Management
| Command | Description |
|---------|-------------|
| [session-start](session-start.md) | Initialize session with progress check |
| [session-end](session-end.md) | Save state before ending |
| [session-resume](session-resume.md) | Single-command resume with auto-detection |

### Metrics & Calibration
| Command | Description |
|---------|-------------|
| [vibe-check](vibe-check.md) | Analyze git history for session metrics |
| [vibe-level](vibe-level.md) | Classify task trust level (0-5) |

### Learning & Improvement
| Command | Description |
|---------|-------------|
| [learn](learn.md) | Extract patterns from completed work |
| [retro](retro.md) | Post-work retrospective with analysis |

### Project Management
| Command | Description |
|---------|-------------|
| [project-init](project-init.md) | Initialize 2-Agent Harness for multi-day projects |
| [progress-update](progress-update.md) | Update progress files interactively |

### Quality & Review
| Command | Description |
|---------|-------------|
| [code-review](code-review.md) | Comprehensive code quality review |
| [architecture-review](architecture-review.md) | Architecture analysis and recommendations |
| [generate-tests](generate-tests.md) | Generate comprehensive test suites |

### Documentation
| Command | Description |
|---------|-------------|
| [update-docs](update-docs.md) | Systematically update project documentation |
| [create-architecture-documentation](create-architecture-documentation.md) | Generate architecture docs with diagrams |
| [create-onboarding-guide](create-onboarding-guide.md) | Create developer onboarding guides |

### Utilities
| Command | Description |
|---------|-------------|
| [ultra-think](ultra-think.md) | Deep analysis with multi-dimensional thinking |
| [maintain](maintain.md) | Weekly knowledge maintenance |
| [containerize-application](containerize-application.md) | Containerize with optimized Docker config |

### Multi-Agent
| Command | Description |
|---------|-------------|
| [research-multi](research-multi.md) | Launch 3 parallel agents for 3x faster research |

## Installation

```bash
# Install all commands
cp *.md /path/to/project/.claude/commands/

# Install specific commands
cp research.md plan.md implement.md /path/to/project/.claude/commands/
```
