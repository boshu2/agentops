"""
AgentOps Skills Loader - JIT (Just-In-Time) skill loading

Implements Factor II (JIT Context Loading) by loading domain-specific
knowledge only when needed, keeping context usage under 40%.

Available skills:
- python-testing: pytest patterns, fixtures, mocking, coverage
- javascript-patterns: ES6+, React, testing patterns
- go-best-practices: Idiomatic Go, concurrency patterns
- kubernetes-manifests: Deployments, Services, ConfigMaps
- helm-charts: Chart structure, values.yaml patterns
- gitops-patterns: ArgoCD/Flux definitions, sync policies
"""

from pathlib import Path
from typing import Dict, Any, Optional, List
from dataclasses import dataclass


@dataclass
class Skill:
    """Represents a loadable skill with metadata."""

    name: str
    description: str
    path: Path
    domain: str
    estimated_tokens: int
    loaded: bool = False
    content: Optional[str] = None


class SkillsLoader:
    """
    JIT Skills Loader for AgentOps.

    Loads domain-specific knowledge on-demand to keep context usage low.
    Each skill is ~500-1000 tokens, enabling efficient context management.
    """

    def __init__(self, base_dir: Optional[Path] = None):
        """Initialize the skills loader."""
        self.base_dir = base_dir or Path(__file__).parent.parent
        self.plugins_dir = self.base_dir / "plugins"
        self._skills: Dict[str, Skill] = {}
        self._loaded_skills: List[str] = []
        self._register_skills()

    def _register_skills(self) -> None:
        """Register all available skills from the plugins directory."""

        # Software Development Skills
        self._skills["python-testing"] = Skill(
            name="python-testing",
            description="pytest patterns, fixtures, mocking, parametrization, coverage",
            path=self.plugins_dir / "software-development/skills/python-testing",
            domain="software-development",
            estimated_tokens=500,
        )

        self._skills["javascript-patterns"] = Skill(
            name="javascript-patterns",
            description="ES6+ features, React patterns, testing with Jest/Vitest",
            path=self.plugins_dir / "software-development/skills/javascript-patterns",
            domain="software-development",
            estimated_tokens=500,
        )

        self._skills["go-best-practices"] = Skill(
            name="go-best-practices",
            description="Idiomatic Go, error handling, concurrency patterns",
            path=self.plugins_dir / "software-development/skills/go-best-practices",
            domain="software-development",
            estimated_tokens=500,
        )

        # DevOps Skills
        self._skills["kubernetes-manifests"] = Skill(
            name="kubernetes-manifests",
            description="Deployments, Services, ConfigMaps, StatefulSets, health checks",
            path=self.plugins_dir / "devops-operations/skills/kubernetes-manifests",
            domain="devops-operations",
            estimated_tokens=1000,
        )

        self._skills["helm-charts"] = Skill(
            name="helm-charts",
            description="Chart structure, values.yaml patterns, template helpers",
            path=self.plugins_dir / "devops-operations/skills/helm-charts",
            domain="devops-operations",
            estimated_tokens=1000,
        )

        self._skills["gitops-patterns"] = Skill(
            name="gitops-patterns",
            description="ArgoCD/Flux definitions, sync policies, progressive delivery",
            path=self.plugins_dir / "devops-operations/skills/gitops-patterns",
            domain="devops-operations",
            estimated_tokens=1000,
        )

    def list_skills(self) -> List[Dict[str, Any]]:
        """List all available skills with metadata."""
        return [
            {
                "name": skill.name,
                "description": skill.description,
                "domain": skill.domain,
                "estimated_tokens": skill.estimated_tokens,
                "loaded": skill.loaded,
            }
            for skill in self._skills.values()
        ]

    def get_loaded_skills(self) -> List[str]:
        """Get list of currently loaded skill names."""
        return self._loaded_skills.copy()

    def get_total_loaded_tokens(self) -> int:
        """Get estimated total tokens from loaded skills."""
        return sum(
            skill.estimated_tokens
            for skill in self._skills.values()
            if skill.loaded
        )

    def load_skill(self, skill_name: str) -> Dict[str, Any]:
        """
        Load a skill and return its content.

        Implements JIT loading - skill content is only loaded when requested.
        """
        if skill_name not in self._skills:
            return {
                "success": False,
                "error": f"Unknown skill: {skill_name}",
                "available_skills": list(self._skills.keys()),
            }

        skill = self._skills[skill_name]

        # Check if skill directory exists
        if not skill.path.exists():
            return {
                "success": False,
                "error": f"Skill directory not found: {skill.path}",
                "suggestion": "Skill may not be installed in this project",
            }

        # Load skill content from markdown files
        try:
            content_parts = []

            # Read all markdown files in the skill directory
            for md_file in sorted(skill.path.glob("**/*.md")):
                with open(md_file, "r") as f:
                    content_parts.append(f"## {md_file.stem}\n\n{f.read()}")

            skill.content = "\n\n---\n\n".join(content_parts)
            skill.loaded = True

            if skill_name not in self._loaded_skills:
                self._loaded_skills.append(skill_name)

            return {
                "success": True,
                "skill": skill_name,
                "domain": skill.domain,
                "content": skill.content,
                "estimated_tokens": skill.estimated_tokens,
                "total_loaded_tokens": self.get_total_loaded_tokens(),
            }

        except Exception as e:
            return {
                "success": False,
                "error": f"Failed to load skill: {str(e)}",
            }

    def unload_skill(self, skill_name: str) -> Dict[str, Any]:
        """
        Unload a skill to free up context budget.

        Use when switching domains or nearing context threshold.
        """
        if skill_name not in self._skills:
            return {
                "success": False,
                "error": f"Unknown skill: {skill_name}",
            }

        skill = self._skills[skill_name]

        if not skill.loaded:
            return {
                "success": True,
                "message": f"Skill {skill_name} was not loaded",
            }

        skill.content = None
        skill.loaded = False

        if skill_name in self._loaded_skills:
            self._loaded_skills.remove(skill_name)

        return {
            "success": True,
            "skill": skill_name,
            "message": f"Skill {skill_name} unloaded",
            "total_loaded_tokens": self.get_total_loaded_tokens(),
        }

    def unload_all(self) -> Dict[str, Any]:
        """Unload all skills to reset context budget."""
        unloaded = []

        for skill_name, skill in self._skills.items():
            if skill.loaded:
                skill.content = None
                skill.loaded = False
                unloaded.append(skill_name)

        self._loaded_skills.clear()

        return {
            "success": True,
            "unloaded": unloaded,
            "total_loaded_tokens": 0,
        }

    def get_skill_for_task(self, task_description: str) -> Optional[str]:
        """
        Suggest a skill based on task description.

        Implements intelligent routing (Factor VII) for skill selection.
        """
        task_lower = task_description.lower()

        # Python-related tasks
        if any(kw in task_lower for kw in ["pytest", "python", "fixture", "mock"]):
            return "python-testing"

        # JavaScript-related tasks
        if any(kw in task_lower for kw in ["javascript", "react", "jest", "vitest", "node"]):
            return "javascript-patterns"

        # Go-related tasks
        if any(kw in task_lower for kw in ["golang", "go test", "goroutine"]):
            return "go-best-practices"

        # Kubernetes-related tasks
        if any(kw in task_lower for kw in ["kubernetes", "k8s", "deployment", "service", "pod"]):
            return "kubernetes-manifests"

        # Helm-related tasks
        if any(kw in task_lower for kw in ["helm", "chart", "values.yaml"]):
            return "helm-charts"

        # GitOps-related tasks
        if any(kw in task_lower for kw in ["argocd", "flux", "gitops", "sync"]):
            return "gitops-patterns"

        return None


# Create skill loading tool for the SDK
def create_skill_tool(loader: SkillsLoader) -> Dict[str, Any]:
    """Create an MCP tool definition for skill loading."""

    async def load_skill_handler(args: Dict[str, Any]) -> Dict[str, Any]:
        skill_name = args.get("skill_name", "")

        if skill_name == "list":
            skills = loader.list_skills()
            skills_text = "\n".join(
                f"- **{s['name']}** ({s['domain']}): {s['description']} (~{s['estimated_tokens']} tokens)"
                for s in skills
            )
            return {
                "content": [{
                    "type": "text",
                    "text": f"""# Available Skills

{skills_text}

**Currently loaded:** {', '.join(loader.get_loaded_skills()) or 'None'}
**Total loaded tokens:** {loader.get_total_loaded_tokens()}

Use `load_skill` with a skill name to load domain knowledge."""
                }]
            }

        result = loader.load_skill(skill_name)

        if result["success"]:
            return {
                "content": [{
                    "type": "text",
                    "text": f"""# Skill Loaded: {skill_name}

**Domain:** {result['domain']}
**Estimated tokens:** {result['estimated_tokens']}
**Total loaded:** {result['total_loaded_tokens']} tokens

---

{result['content']}"""
                }]
            }
        else:
            return {
                "content": [{
                    "type": "text",
                    "text": f"""# Failed to Load Skill

**Error:** {result['error']}

**Available skills:** {', '.join(result.get('available_skills', []))}"""
                }]
            }

    return {
        "name": "load_skill",
        "description": "Load domain-specific knowledge (JIT loading). Use 'list' to see available skills.",
        "parameters": {
            "skill_name": {
                "type": "string",
                "description": "Skill to load: python-testing, javascript-patterns, go-best-practices, kubernetes-manifests, helm-charts, gitops-patterns, or 'list'",
            },
        },
        "handler": load_skill_handler,
    }
