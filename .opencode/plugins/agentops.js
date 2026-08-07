/**
 * AgentOps plugin for OpenCode.ai
 *
 * Hooks (7 active):
 *   1. experimental.chat.system.transform — Bootstrap context + prescriptive skill-to-tool mapping
 *   2. tool.execute.before — Guard task tool crash (sst/opencode#13933)
 *   3. tool.definition — Enrich task + skill tool descriptions for Devstral
 *   4. command.execute.before — Redirect slashcommand skill calls to skill tool
 *   5. tool.execute.after — Audit log all tool calls to .agents/audit/
 *   6. shell.env — Inject CLI paths (bd, ao, gt, cass) into shell environment
 *   7. experimental.session.compacting — Preserve AgentOps context through compaction
 */

import path from 'path';
import fs from 'fs';
import os from 'os';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Simple frontmatter extraction (avoid dependency on skills-core for bootstrap)
const extractAndStripFrontmatter = (content) => {
  const match = content.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) return { frontmatter: {}, content };

  const frontmatterStr = match[1];
  const body = match[2];
  const frontmatter = {};

  for (const line of frontmatterStr.split('\n')) {
    const colonIdx = line.indexOf(':');
    if (colonIdx > 0) {
      const key = line.slice(0, colonIdx).trim();
      const value = line.slice(colonIdx + 1).trim().replace(/^["']|["']$/g, '');
      frontmatter[key] = value;
    }
  }

  return { frontmatter, content: body };
};

// Normalize a path: trim whitespace, expand ~, resolve to absolute
const normalizePath = (p, homeDir) => {
  if (!p || typeof p !== 'string') return null;
  let normalized = p.trim();
  if (!normalized) return null;
  if (normalized.startsWith('~/')) {
    normalized = path.join(homeDir, normalized.slice(2));
  } else if (normalized === '~') {
    normalized = homeDir;
  }
  return path.resolve(normalized);
};

// Find CLI binary paths for injection into shell env
const findCliPaths = (homeDir) => {
  const dirs = new Set();
  const knownLocations = [
    ['/opt/homebrew/bin', ['bd', 'ao', 'gt', 'cass']],
    [path.join(homeDir, 'bin'), ['ntm', 'bv', 'cass']],
    [path.join(homeDir, 'go/bin'), ['ao', 'bd', 'gt']],
    [path.join(homeDir, '.local/bin'), ['bd', 'ao']],
  ];

  for (const [dir, clis] of knownLocations) {
    for (const cli of clis) {
      try {
        if (fs.existsSync(path.join(dir, cli))) {
          dirs.add(dir);
          break;
        }
      } catch { /* ignore */ }
    }
  }
  return [...dirs];
};

export const AgentOpsPlugin = async ({ client, directory }) => {
  const homeDir = os.homedir();
  const agentopsSkillsDir = path.resolve(__dirname, '../../skills');
  const envConfigDir = normalizePath(process.env.OPENCODE_CONFIG_DIR, homeDir);
  const configDir = envConfigDir || path.join(homeDir, '.config/opencode');

  // Pre-compute CLI paths once at plugin load
  const cliPathDirs = findCliPaths(homeDir);

  // Audit log helper
  const auditDir = path.join(directory || process.cwd(), '.agents', 'audit');
  const getAuditPath = () => {
    const date = new Date().toISOString().slice(0, 10);
    return path.join(auditDir, `${date}-opencode.jsonl`);
  };

  // Helper to generate bootstrap content
  const getBootstrapContent = () => {
    // Try to load using-agentops skill
    const skillPath = path.join(agentopsSkillsDir, 'using-agentops', 'SKILL.md');
    if (!fs.existsSync(skillPath)) return null;

    const fullContent = fs.readFileSync(skillPath, 'utf8');
    const { content } = extractAndStripFrontmatter(fullContent);

    const toolMapping = `**Tool Mapping for OpenCode:**
When skills reference tools you don't have, substitute OpenCode equivalents:
- \`TodoWrite\` → \`update_plan\`
- \`Task\` tool with subagents → \`task\` tool with \`subagent_type\` parameter (REQUIRED)
- \`Skill\` tool → OpenCode's native \`skill\` tool (READ-ONLY — see below)
- \`Read\`, \`Write\`, \`Edit\`, \`Bash\`, \`Glob\`, \`Grep\` → Your native tools (same names)
- \`AskUserQuestion\` → \`question\` tool (or skip in headless mode)
- \`TeamCreate\`, \`SendMessage\`, \`TaskCreate\`, \`TaskList\` → Not available; work inline

**CRITICAL — Skill Chaining Rules:**
OpenCode's \`skill\` tool is READ-ONLY. It loads skill content into your context. It does NOT execute skills.

When a loaded skill tells you to invoke another skill (e.g., \`Skill(skill="council")\` or \`/council validate\`):
1. Use the \`skill\` tool to LOAD that skill's content
2. Then FOLLOW the loaded instructions INLINE in your current turn
3. NEVER use the \`slashcommand\` tool to invoke a skill — this will crash

Example — skill says \`Skill(skill="pre-mortem", args="--quick")\`:
  ✅ CORRECT: Use skill tool to load "pre-mortem", then follow its --quick instructions inline
  ❌ WRONG: Use slashcommand {"command":"pre-mortem"} — this crashes OpenCode

When a skill references \`spawn_agent()\` or \`TeamCreate\` for parallel agents:
  → Use \`task(subagent_type="general", prompt="...")\` for parallel work
  → Or execute serially inline if task tool unavailable

**Prescriptive Skill-to-Tool Mapping:**
When running these skills, use these EXACT OpenCode tools:

| Skill | Primary Tools | Spawn Pattern |
|-------|--------------|---------------|
| /research | \`task(subagent_type="explore")\` for codebase exploration, \`bash\` for ao/bd | Spawn explore agent with search prompt |
| /plan | \`bash\` for bd commands, \`write\` for plan docs, \`task(subagent_type="explore")\` for investigation | Inline planning, explore for codebase understanding |
| /council | \`task(subagent_type="general")\` for each judge (2-3 parallel) | Spawn N general agents as judges, collect verdicts |
| /vibe | \`bash\` for complexity analysis, \`read\` for code review, inline for --quick | Inline validation, no spawn needed for --quick |
| /implement | \`read\`, \`write\`, \`edit\`, \`bash\` for code changes | Direct implementation, no spawn |
| /crank | \`task(subagent_type="general")\` for workers per wave, \`bash\` for bd | Spawn workers per wave, validate between waves |
| /swarm | \`task(subagent_type="general")\` per worker | One task per parallel unit of work |
| /pre-mortem | Same as /council (wrapper) | Inline for --quick, spawn judges otherwise |
| /post-mortem | Same as /council + \`write\` for retro | Inline for --quick |
| /retro | \`read\` for artifacts, \`write\` for learnings | Inline, no spawn |

**Skills location:**
AgentOps skills are in \`${configDir}/skills/\`
Use OpenCode's native \`skill\` tool to list and load skills.`;

    return `<EXTREMELY_IMPORTANT>
You have AgentOps superpowers.

**IMPORTANT: The using-agentops skill content is included below. It is ALREADY LOADED - you are currently following it. Do NOT use the skill tool to load "using-agentops" again - that would be redundant.**

${content}

${toolMapping}
</EXTREMELY_IMPORTANT>`;
  };

  // Known skill names for command interception
  const skillNames = [
    'council', 'vibe', 'pre-mortem', 'post-mortem', 'retro', 'crank',
    'swarm', 'research', 'plan', 'implement', 'rpi', 'status',
    'complexity', 'knowledge', 'bug-hunt', 'doc', 'handoff', 'learn',
    'release', 'product', 'quickstart', 'trace', 'inbox', 'recover',
    'evolve', 'codex-team', 'beads', 'standards', 'inject', 'extract',
    'forge', 'provenance', 'ratchet', 'update', 'using-agentops'
  ];

  return {
    // --- Hook 1: System prompt injection with prescriptive tool mapping ---
    'experimental.chat.system.transform': async (_input, output) => {
      const bootstrap = getBootstrapContent();
      if (bootstrap) {
        (output.system ||= []).push(bootstrap);
      }
    },

    // --- Hook 2: Guard task tool crash (sst/opencode#13933) ---
    'tool.execute.before': async (input, output) => {
      if (input.tool === 'task' && output.args) {
        if (!output.args.subagent_type) {
          output.args.subagent_type = 'general';
        }
      }
    },

    // --- Hook 3: Enrich tool definitions for Devstral ---
    'tool.definition': async (input, output) => {
      if (input.toolID === 'task') {
        output.description = `CRITICAL: The "subagent_type" parameter is REQUIRED and MUST always be a non-empty string. ` +
          `Valid values: "general" (default), "explore" (read-only search), "build" (code changes), "plan" (architecture). ` +
          `NEVER omit subagent_type or set it to null/undefined — this will crash the application.\n\n` +
          output.description;
      }

      if (input.toolID === 'skill') {
        output.description = `AgentOps skills are available. This tool LOADS skill content into your context (READ-ONLY). ` +
          `After loading, READ the content and FOLLOW the instructions INLINE. The skill will NOT execute automatically. ` +
          `Available AgentOps skills: ${skillNames.slice(0, 20).join(', ')}, and more. ` +
          `NEVER use slashcommand to invoke a skill — always use this tool.\n\n` +
          output.description;
      }
    },

    // --- Hook 4: Redirect slashcommand skill calls ---
    'command.execute.before': async (input, output) => {
      if (skillNames.includes(input.command)) {
        (output.parts ||= []).push({
          type: 'text',
          text: `⚠️ "${input.command}" is an AgentOps skill. Do NOT use slashcommand to invoke it — use the \`skill\` tool to load "${input.command}" content, then follow its instructions inline.`
        });
      }
    },

    // --- Hook 5: Audit log all tool calls ---
    'tool.execute.after': async (input, _output) => {
      try {
        const entry = {
          ts: new Date().toISOString(),
          tool: input.tool,
          args: input.args ? Object.keys(input.args) : [],
          title: input.title || null,
          error: input.metadata?.error || null,
        };
        fs.mkdirSync(auditDir, { recursive: true });
        fs.appendFileSync(getAuditPath(), JSON.stringify(entry) + '\n');
      } catch { /* audit is best-effort, never crash */ }
    },

    // --- Hook 6: Inject CLI paths into shell environment ---
    'shell.env': async (_input, output) => {
      if (cliPathDirs.length > 0) {
        const currentPath = output.env?.PATH || process.env.PATH || '';
        const newDirs = cliPathDirs.filter(d => !currentPath.includes(d));
        if (newDirs.length > 0) {
          (output.env ||= {}).PATH = [...newDirs, currentPath].join(':');
        }
      }
    },

    // --- Hook 7: Preserve AgentOps context through session compaction ---
    'experimental.session.compacting': async (_input, output) => {
      const compactionNote = `\n\nIMPORTANT — AgentOps Context Preservation:\n` +
        `This agent has AgentOps skills installed. After compaction, maintain:\n` +
        `1. The skill tool is READ-ONLY — load skill content, then follow instructions inline\n` +
        `2. The task tool REQUIRES subagent_type parameter (use "general" if unsure)\n` +
        `3. NEVER use slashcommand to invoke skills — use the skill tool\n` +
        `4. Tool mapping: Task→task, Skill→skill(read-only), AskUserQuestion→question, TodoWrite→todo\n` +
        `5. If an RPI workflow was in progress, check .agents/rpi/ for phase summaries\n` +
        `6. Skills location: ~/.config/opencode/skills/ (symlinked from ~/.agents/skills/)`;
      output.prompt = (output.prompt || '') + compactionNote;
    },
  };
};
