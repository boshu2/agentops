// Use Codex's TOML editor in a caller-created staging home; no model session.
import { spawn } from 'node:child_process';
import { realpathSync } from 'node:fs';
import { createInterface } from 'node:readline';
import { join, resolve } from 'node:path';
const stage = realpathSync(process.argv[2]);
const agentDir = resolve(process.argv[3]);
const child = spawn('codex', ['app-server', '--stdio'], {
  env: { ...process.env, CODEX_HOME: stage }, stdio: ['pipe', 'pipe', 'ignore'],
});
let done = false;
function finish(ok) {
  if (done) return;
  done = true;
  clearTimeout(timer);
  process.exitCode = ok ? 0 : 1;
  if (!ok) process.stderr.write('Codex could not register roles in staged config; existing config was preserved.\n');
  child.kill('SIGTERM');
  const cleanup = setTimeout(() => child.kill('SIGKILL'), 1000);
  cleanup.unref();
}
const timer = setTimeout(() => finish(false), 15000);
child.on('error', () => finish(false));
child.on('exit', () => { if (!done) finish(false); });
child.stdin.on('error', () => finish(false));
function send(id, method, params) {
  child.stdin.write(JSON.stringify({ id, method, params }) + '\n');
}
createInterface({ input: child.stdout }).on('line', (line) => {
  let reply;
  try { reply = JSON.parse(line); } catch { finish(false); return; }
  if (reply.id === 1) {
    if (reply.error) { finish(false); return; }
    send(2, 'config/batchWrite', {
      filePath: join(stage, 'config.toml'),
      edits: ['bulk-reader', 'code-writer'].flatMap((role) => [
        { keyPath: `agents.${role}.description`, mergeStrategy: 'replace', value: role === 'bulk-reader' ? 'Read bounded slices; return line-referenced findings only.' : 'Write one target from a required reference; return a receipt only.' },
        { keyPath: `agents.${role}.config_file`, mergeStrategy: 'replace', value: join(agentDir, `${role}.toml`) },
      ]),
    });
  } else if (reply.id === 2) finish(!reply.error && reply.result?.status === 'ok');
});
send(1, 'initialize', { clientInfo: { name: 'agentops-role-installer', version: '1' }, capabilities: { experimentalApi: true } });
