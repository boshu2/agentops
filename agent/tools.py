"""
AgentOps Workflow Tools - Custom MCP tools for the 4-phase workflow

Converts the AgentOps slash commands into SDK tools:
- /research -> workflow_research
- /plan -> workflow_plan
- /implement -> workflow_implement
- /learn -> workflow_learn
- /workflow -> workflow_full
"""

from typing import Dict, Any, List, Callable
from datetime import datetime


def create_workflow_tools() -> List[Dict[str, Any]]:
    """
    Create MCP tool definitions for the AgentOps workflow.

    Returns a list of tool configurations that can be used with
    claude_agent_sdk.create_sdk_mcp_server()
    """
    return [
        {
            "name": "workflow_research",
            "description": "Start research phase - gather context, understand problem space. Use at the beginning of complex work.",
            "parameters": {
                "topic": {
                    "type": "string",
                    "description": "The topic or question to research",
                },
                "approach": {
                    "type": "string",
                    "description": "Research approach: codebase_exploration, api_research, requirements_gathering, or problem_analysis",
                    "enum": ["codebase_exploration", "api_research", "requirements_gathering", "problem_analysis"],
                },
            },
            "handler": research_handler,
        },
        {
            "name": "workflow_plan",
            "description": "Create implementation plan from research findings. Use after research phase completes.",
            "parameters": {
                "research_summary": {
                    "type": "string",
                    "description": "Summary of research findings to base the plan on",
                },
                "constraints": {
                    "type": "string",
                    "description": "Any constraints or requirements to consider",
                },
            },
            "handler": plan_handler,
        },
        {
            "name": "workflow_implement",
            "description": "Execute implementation with validation gates. Use after plan is approved.",
            "parameters": {
                "plan_summary": {
                    "type": "string",
                    "description": "Summary of the implementation plan",
                },
                "task_number": {
                    "type": "integer",
                    "description": "Current task number being implemented",
                },
            },
            "handler": implement_handler,
        },
        {
            "name": "workflow_learn",
            "description": "Extract patterns and learnings from completed work. Use after implementation.",
            "parameters": {
                "implementation_summary": {
                    "type": "string",
                    "description": "Summary of what was implemented",
                },
                "outcomes": {
                    "type": "string",
                    "description": "Outcomes and results of the implementation",
                },
            },
            "handler": learn_handler,
        },
        {
            "name": "workflow_full",
            "description": "Execute complete Research -> Plan -> Implement -> Learn workflow cycle.",
            "parameters": {
                "topic": {
                    "type": "string",
                    "description": "The topic or task to complete through full workflow",
                },
            },
            "handler": full_workflow_handler,
        },
        {
            "name": "workflow_status",
            "description": "Get current workflow status and phase information.",
            "parameters": {},
            "handler": status_handler,
        },
    ]


# Tool handlers

async def research_handler(args: Dict[str, Any]) -> Dict[str, Any]:
    """Handle research phase initiation."""
    topic = args.get("topic", "")
    approach = args.get("approach", "codebase_exploration")

    approaches = {
        "codebase_exploration": {
            "description": "Understanding existing code and architecture",
            "steps": [
                "1. Use Glob to discover file structure and find relevant files",
                "2. Use Grep to find patterns, references, and implementations",
                "3. Read files selectively based on discoveries",
                "4. Document findings and create research bundle",
            ],
            "tools": ["Glob", "Grep", "Read"],
        },
        "api_research": {
            "description": "Learning about external APIs and libraries",
            "steps": [
                "1. Use WebFetch to get official documentation",
                "2. Find example implementations and usage patterns",
                "3. Extract relevant patterns and requirements",
                "4. Document integration approach",
            ],
            "tools": ["WebFetch", "Read"],
        },
        "requirements_gathering": {
            "description": "Understanding what needs to be built",
            "steps": [
                "1. Read requirement documents and specifications",
                "2. Identify stakeholder needs and expectations",
                "3. Define success criteria and acceptance tests",
                "4. Document constraints and dependencies",
            ],
            "tools": ["Read", "Grep"],
        },
        "problem_analysis": {
            "description": "Debugging or investigating issues",
            "steps": [
                "1. Gather evidence of the problem (logs, errors, behavior)",
                "2. Form hypotheses about root cause",
                "3. Test hypotheses systematically",
                "4. Document root cause and solution approach",
            ],
            "tools": ["Read", "Grep", "Bash"],
        },
    }

    selected = approaches.get(approach, approaches["codebase_exploration"])

    return {
        "content": [{
            "type": "text",
            "text": f"""# Research Phase Initiated

**Topic:** {topic}
**Approach:** {approach} - {selected['description']}
**Timestamp:** {datetime.now().isoformat()}

## Research Strategy

{chr(10).join(selected['steps'])}

## Tools to Use
{', '.join(selected['tools'])}

## Context Management (Factor II)
- Stay under 40% context threshold
- Load only what's needed for research
- Use discovery tools (Glob/Grep) before reading

## Output Required
Create a research bundle with:
- Key findings
- Open questions
- Constraints identified
- Recommendations for plan phase

---

Begin research by exploring the topic using the recommended approach."""
        }]
    }


async def plan_handler(args: Dict[str, Any]) -> Dict[str, Any]:
    """Handle plan phase initiation."""
    research_summary = args.get("research_summary", "")
    constraints = args.get("constraints", "")

    return {
        "content": [{
            "type": "text",
            "text": f"""# Plan Phase Initiated

**Research Summary:** {research_summary}
**Constraints:** {constraints}
**Timestamp:** {datetime.now().isoformat()}

## Planning Process

1. **Analyze Research Findings**
   - Review key discoveries from research phase
   - Identify patterns and existing solutions
   - Note constraints and requirements

2. **Design Solution**
   - Choose architectural approach
   - Define component boundaries
   - Plan data flow and interfaces

3. **Create Task Breakdown**
   - Break into discrete, verifiable tasks
   - Order by dependencies
   - Estimate effort per task (total = 40% of workflow)

4. **Identify Risks**
   - Technical risks
   - Integration risks
   - Timeline risks
   - Mitigation strategies

## Output Required

Create implementation specification with:
- [ ] Solution architecture
- [ ] Task breakdown (numbered, ordered)
- [ ] Success criteria per task
- [ ] Risk mitigation plan
- [ ] Validation approach

## Approval Gate (Factor VIII)

Plan requires approval before implementation phase.
Present plan summary and wait for confirmation.

---

Begin planning by analyzing the research findings."""
        }]
    }


async def implement_handler(args: Dict[str, Any]) -> Dict[str, Any]:
    """Handle implementation phase."""
    plan_summary = args.get("plan_summary", "")
    task_number = args.get("task_number", 1)

    return {
        "content": [{
            "type": "text",
            "text": f"""# Implementation Phase - Task {task_number}

**Plan Summary:** {plan_summary}
**Current Task:** {task_number}
**Timestamp:** {datetime.now().isoformat()}

## Implementation Process

1. **Load Task Details**
   - Get task from implementation spec
   - Review success criteria
   - Identify files to modify

2. **Execute Task**
   - Write code following plan
   - Follow existing patterns in codebase
   - Add appropriate tests

3. **Validate (Factor IV - MANDATORY)**
   - Run tests: `pytest` / `npm test` / `go test`
   - Check types: `mypy` / `tsc` / `go vet`
   - Lint code: `ruff` / `eslint` / `golangci-lint`
   - Verify functionality works

4. **Commit with Learnings**
   Use this commit message format:
   ```
   [type]: Brief description

   Context: Why this change was needed
   Solution: What was done
   Learning: Insights gained
   Impact: Value delivered
   ```

## Validation Gates (REQUIRED)

Before moving to next task:
- [ ] All tests pass
- [ ] No type errors
- [ ] No lint errors
- [ ] Functionality verified
- [ ] Changes committed

## Metrics to Track (Factor V)

- Time spent on task
- Tests added/modified
- Files changed
- Errors caught by validation

---

Execute the current task with continuous validation."""
        }]
    }


async def learn_handler(args: Dict[str, Any]) -> Dict[str, Any]:
    """Handle learning phase."""
    implementation_summary = args.get("implementation_summary", "")
    outcomes = args.get("outcomes", "")

    return {
        "content": [{
            "type": "text",
            "text": f"""# Learn Phase Initiated

**Implementation Summary:** {implementation_summary}
**Outcomes:** {outcomes}
**Timestamp:** {datetime.now().isoformat()}

## Learning Process (The Five Laws)

### Law 1: Extract Learnings
- What patterns were discovered?
- What worked well?
- What was harder than expected?
- What would you do differently?

### Law 2: Improve Self or System
Identify at least ONE improvement:
- Process improvement
- Tool improvement
- Documentation improvement
- Code quality improvement

### Law 3: Document Context for Future
For each significant decision:
- Context: Why was it needed?
- Solution: What was done?
- Learning: What was learned?
- Impact: What value was delivered?

### Law 4: Validate Learnings
- Are the patterns generalizable?
- Have we tested the learnings?
- Can others reproduce the results?

### Law 5: Share Patterns
- Document reusable patterns
- Add to knowledge base
- Help others avoid mistakes

## Output Required

Create learning document with:
- [ ] Key patterns discovered (reusable)
- [ ] Improvement opportunities (≥1 required)
- [ ] Decisions documented (Context/Solution/Learning/Impact)
- [ ] Metrics summary
- [ ] Recommendations for similar work

---

Begin learning extraction by reviewing the implementation."""
        }]
    }


async def full_workflow_handler(args: Dict[str, Any]) -> Dict[str, Any]:
    """Handle full workflow execution."""
    topic = args.get("topic", "")

    return {
        "content": [{
            "type": "text",
            "text": f"""# Full Workflow Initiated

**Topic:** {topic}
**Timestamp:** {datetime.now().isoformat()}

## Workflow Phases

### Phase 1: Research (20%)
- Gather context and understand problem
- Create research bundle
- Transition: Research complete -> Plan

### Phase 2: Plan (30%)
- Design solution from research
- Create implementation spec
- Get approval
- Transition: Plan approved -> Implement

### Phase 3: Implement (40%)
- Execute plan with validation gates
- Track metrics
- Commit with learnings
- Transition: Implementation complete -> Learn

### Phase 4: Learn (10%)
- Extract patterns
- Identify improvements
- Document for future
- Workflow complete

## Phase Transitions

Each transition requires:
- Current phase output complete
- Validation criteria met
- User confirmation (for plan approval)

## Effort Distribution

| Phase | Effort | Status |
|-------|--------|--------|
| Research | 20% | Starting |
| Plan | 30% | Pending |
| Implement | 40% | Pending |
| Learn | 10% | Pending |

---

Starting with Research phase. Use `workflow_research` to begin."""
        }]
    }


async def status_handler(args: Dict[str, Any]) -> Dict[str, Any]:
    """Get workflow status."""
    return {
        "content": [{
            "type": "text",
            "text": f"""# Workflow Status

**Timestamp:** {datetime.now().isoformat()}

## Available Workflow Commands

| Command | Description |
|---------|-------------|
| workflow_research | Start research phase |
| workflow_plan | Create implementation plan |
| workflow_implement | Execute with validation |
| workflow_learn | Extract patterns |
| workflow_full | Complete 4-phase cycle |

## The Five Laws Status

Ensure ALL work follows:
1. ALWAYS Extract Learnings
2. ALWAYS Improve Self or System
3. ALWAYS Document Context for Future
4. ALWAYS Validate Before Execute
5. ALWAYS Share Patterns Publicly

## 12-Factor Compliance

Key factors to maintain:
- Factor II: JIT Context Loading (stay <40%)
- Factor IV: Validation Gates (test before commit)
- Factor VI: Session Continuity (use bundles)
- Factor XI: Constitutional Guardrails (Five Laws)
"""
        }]
    }


# Utility function to create tool from definition
def create_tool_function(tool_def: Dict[str, Any]) -> Callable:
    """Create a tool function from a tool definition."""
    handler = tool_def["handler"]

    async def tool_function(args: Dict[str, Any]) -> Dict[str, Any]:
        return await handler(args)

    tool_function.__name__ = tool_def["name"]
    tool_function.__doc__ = tool_def["description"]

    return tool_function
