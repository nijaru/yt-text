# AI Documentation Setup Prompt

Use this prompt to request documentation creation or updates for any project:

---

## Prompt Template

Please analyze this codebase and create/update documentation optimized for AI agents (primarily) and humans (secondarily).

### Documentation Hierarchy

Follow this strict hierarchy:
1. **CLAUDE.md** (root) - Entry point, navigation map, quick context
2. **docs/** - Key documentation files
3. **docs/[subdirs]/** - Detailed implementation docs

### Required Files Structure

```
CLAUDE.md                    # Entry point, points to other docs
docs/
├── STATUS.md               # Current state, active work, known issues
├── TECH_SPEC.md           # Architecture, stack, design decisions  
├── TODO.md                # Actionable tasks, priorities
├── api/                   # API design and contracts
│   ├── endpoints.md       # REST/GraphQL/WebSocket specs
│   └── examples.md        # Request/response examples
├── implementation/        # Code patterns and conventions
│   ├── patterns.md        # Design patterns used
│   ├── conventions.md     # Coding standards
│   └── dependencies.md    # External deps and why
└── deployment/           # Infrastructure and deployment
    ├── config.md         # Environment variables
    ├── docker.md         # Container setup
    └── production.md     # Deployment guides
```

### Documentation Principles

**For AI Agents:**
- Single source of truth - no duplication across files
- Clear navigation - each file links to related docs
- Token efficient - concise but complete
- Context-aware - include file paths, line numbers
- Task-oriented - organize by what needs to be done

**For Humans:**
- Examples over exposition
- Diagrams where helpful
- Quick start guides
- Troubleshooting sections

### Content Requirements

**CLAUDE.md must include:**
- Project name and one-line description
- Documentation map (where to find what)
- Quick context (current state, stack, goals)
- Critical commands (max 5-7)
- Working rules (how to approach tasks)

**STATUS.md must include:**
- Last updated date (use `date +"%Y-%m-%d"`)
- Current phase/milestone
- What exists (✅) and issues (❌)
- Active work and blockers
- Decision log with dates

**TECH_SPEC.md must include:**
- Architecture overview
- Technology choices with rationale
- Design patterns used
- Performance targets
- Security considerations

**TODO.md must include:**
- Immediate tasks (current sprint)
- Upcoming phases
- Priority order
- Risk notes

### Special Instructions

1. **For existing projects:** Preserve useful existing docs, reorganize to fit hierarchy
2. **Add git submodules** for reference code:
   ```bash
   git submodule add [url] docs/reference/[name]
   ```
3. **Include MCP tools** context if available (context7, exa search)
4. **Update frequency:** STATUS.md weekly, others as needed

### Analysis Steps

1. Read existing docs (README, CONTRIBUTING, etc.)
2. Analyze code structure and patterns
3. Identify key workflows and commands
4. Detect testing and deployment setup
5. Note any special requirements or constraints

### Output Format

Create files that are:
- Markdown formatted
- Use relative links between docs
- Include code blocks with language hints
- Add file paths as `path/to/file.ext:line`
- Date stamp important decisions

### Questions to Answer

Before creating docs, determine:
- What tasks will AI agents perform?
- What's the current state vs desired state?
- What are the critical workflows?
- What breaks most often?
- What context is expensive to rebuild?

---

## Usage

Copy the prompt above and customize for your specific project needs. Add project-specific requirements after the template.