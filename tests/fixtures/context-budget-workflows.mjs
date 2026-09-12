import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

// Exercise the actual workflow body with native agents replaced by controlled
// responses. The metadata probe's generated shell command runs for real, but
// no model or Claude session is invoked. WORKFLOW_SUBJECT permits before/after
// regression checks against an immutable checkout using this same harness.
const subject = process.env.WORKFLOW_SUBJECT || path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const fixture = fs.mkdtempSync(path.join(os.tmpdir(), 'context-workflows-test-'));
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
fs.mkdirSync(path.join(fixture, 'sub'));
fs.writeFileSync(path.join(fixture, 'out.js'), 'export const old = true;\n');
fs.writeFileSync(path.join(fixture, 'reference.js'), 'export const reference = true;\n');
fs.symlinkSync('out.js', path.join(fixture, 'alias.js'));
fs.symlinkSync('.', path.join(fixture, 'dir-alias'));
fs.linkSync(path.join(fixture, 'out.js'), path.join(fixture, 'hardlink.js'));
fs.mkdirSync(path.join(fixture, 'physical/deep'), { recursive: true });
fs.writeFileSync(path.join(fixture, 'physical/out.js'), 'export const physical = true;\n');
fs.symlinkSync('physical/deep', path.join(fixture, 'deep-link'));

async function run(name, args, worker, probeOverride) {
  const source = fs.readFileSync(path.join(subject, 'workflows', name + '.js'), 'utf8');
  assert(source.startsWith('export const meta = {\n'));
  const metaEnd = source.indexOf('\n};\n');
  assert(metaEnd > 0);
  const body = new AsyncFunction('agent', 'parallel', 'log', 'phase', 'args', source.slice(metaEnd + 4));
  const calls = [], logs = [], phases = [];
  let active = 0, maxActive = 0, result, error;
  try {
    result = await body(async (prompt, options) => {
      calls.push({ prompt, options });
      if (options.label === 'code-write:target-identities') {
        if (probeOverride) return probeOverride(prompt, options);
        const command = prompt.slice(prompt.lastIndexOf('\n\n') + 2);
        assert(command.startsWith('node -e '));
        return JSON.parse(execFileSync('bash', ['-c', command], { cwd: fixture, encoding: 'utf8', timeout: 5000 }));
      }
      active++;
      maxActive = Math.max(maxActive, active);
      try { return await worker(prompt, options); }
      finally { active--; }
    }, (thunks) => Promise.all(thunks.map(async (thunk) => { try { return await thunk(); } catch { return null; } })),
    (line) => logs.push(line), (name) => phases.push(name), args);
  } catch (caught) { error = caught; }
  const writers = calls.filter((call) => call.options.label.startsWith('code-write:') && call.options.label !== 'code-write:target-identities');
  return { result, error, calls, writers, logs, phases, maxActive };
}
const reader = (file = 'large.js', extra = {}) => ({ file, bullets: [{ ref: file + ':1', text: 'Relevant fact.' }], lines_covered: 400, complete: true, ...extra });
const item = (target = 'out.js', key = 'one') => ({ key, spec: 'Write a boolean export.', reference: 'reference.js', target });
const receipt = (target = 'out.js', key = 'one', extra = {}) => ({ key, target, written: true, lines: 1, check_ran: false, check_ok: false, summary: 'Added a boolean export.', ...extra });
const readArgs = { root: fixture, question: 'What is relevant?', files: ['large.js'] };
const writeArgs = { root: fixture, items: [item()] };
function good(run) { assert.ifError(run.error); return run; }
function rejected(run) { assert(run.error, 'Expected rejection before dispatch'); assert.equal(run.calls.length, 0); }
function unknownReader(run) {
  good(run);
  assert.match(run.result.files[0].error, /coverage unknown/);
  assert.equal(run.result.files[0].lines_covered, null);
  assert.equal(run.result.files[0].complete, false);
  assert.deepEqual(run.result.files[0].bullets, []);
}
function unknownWriter(run) {
  good(run);
  assert.match(run.result.items[0].error, /state unknown/);
  for (const key of ['written', 'lines', 'check_ran', 'check_ok']) assert.equal(run.result.items[0][key], null);
  assert.equal(run.result.items[0].summary, '');
}
const tests = {
  async 'reader-normal'() {
    const args = { ...readArgs, files: ['large.js', 'second.js'], maxBullets: 2, budgetLines: 100 };
    const result = good(await run('bulk-read', JSON.stringify(args), (prompt) => {
      const file = prompt.match(/File to read: (.+)/)[1];
      return reader(file, { bullets: [1, 2, 3].map((line) => ({ ref: file + ':' + line, text: 'fact' })) });
    }));
    assert.equal(result.calls.length, 2);
    assert.equal(result.result.bullets_total, 4);
    assert(result.logs.some((line) => line.includes('dropped 1')));
    for (const call of result.calls) {
      assert.equal(call.options.model, 'haiku');
      assert.match(call.prompt, /offset, limit\) with limit ≤ 100/);
    }
    const missing = good(await run('bulk-read', readArgs, () => reader('large.js', { bullets: [], lines_covered: 0, complete: false, note: 'File missing.' })));
    assert.equal(missing.result.files[0].note, 'File missing.');
  },
  async 'reader-output'() {
    const source = Array.from({ length: 400 }, (_, i) => 'source line ' + i).join('\n');
    for (const output of [
      reader('large.js', { bullets: [{ ref: 'large.js:1', text: source }] }),
      reader('large.js', { bullets: [{ ref: 'large.js:1', text: 'X'.repeat(201) }] }),
      reader('large.js', { note: source }), reader('large.js', { note: 'X'.repeat(301) }),
      reader('large.js', { bullets: [{ ref: 'no ref', text: 'fact' }] }),
      reader('large.js', { bullets: [{ ref: 'large.js:9-2', text: 'fact' }] }),
      reader('large.js', { bullets: [{ ref: 'large.js:401', text: 'fact' }] }),
      reader('large.js', { lines_covered: -2.5 }), reader('large.js', { lines_covered: Infinity }),
      reader('wrong-file.js'), reader('large.js', { raw_content: source }),
      reader('large.js', { bullets: null }), null,
    ]) unknownReader(await run('bulk-read', readArgs, () => output));
    unknownReader(await run('bulk-read', readArgs, () => { throw Error('dead after reading'); }));
  },
  async 'invalid-args'() {
    for (const name of ['bulk-read', 'code-write']) {
      for (const args of [null, 'null', [], undefined]) rejected(await run(name, args, () => null));
      for (const budgetLines of [0.5, Infinity, NaN, 0, -1, Number.MAX_SAFE_INTEGER + 1]) {
        rejected(await run(name, { ...(name === 'bulk-read' ? readArgs : writeArgs), budgetLines }, () => null));
      }
    }
    for (const maxBullets of [0.5, Infinity, NaN, 0, -1]) rejected(await run('bulk-read', { ...readArgs, maxBullets }, () => null));
  },
  async 'writer-normal'() {
    const inputs = [item(), item('second.js', 'two')];
    const result = good(await run('code-write', { ...writeArgs, items: inputs }, async (prompt, options) => {
      await new Promise((resolve) => setTimeout(resolve, 10));
      const target = inputs.find((value) => options.label === 'code-write:' + value.key);
      assert.match(prompt, /offset, limit\) with limit ≤ 350/);
      assert.equal(options.model, 'haiku');
      return receipt(target.target, target.key);
    }));
    assert.equal(result.writers.length, 2);
    assert.equal(result.calls.length, 3);
    assert.equal(result.maxActive, 1);
    assert.deepEqual(result.phases, ['Targets', 'Write']);
    assert.equal(result.result.items.length, 2);
    const checked = good(await run('code-write', JSON.stringify({ ...writeArgs, model: 'sonnet', budgetLines: 10, items: [{ ...item(), check: 'node --check out.js' }] }), () => receipt('out.js', 'one', { check_ran: true, check_ok: true })));
    assert.equal(checked.calls[0].options.model, 'sonnet');
    assert.match(checked.calls[0].prompt, /limit ≤ 10/);
    assert(!Object.hasOwn(checked.result.items[0], 'check_output_tail'));
  },
  async 'required-reference'() {
    for (const reference of [undefined, '', ' ']) rejected(await run('code-write', { ...writeArgs, items: [{ ...item(), reference }] }, () => receipt()));
  },
  async 'target-aliases'() {
    for (const target of ['out.js', './out.js', 'sub/../out.js', path.join(fixture, 'out.js'), 'alias.js', 'hardlink.js']) {
      const result = await run('code-write', { ...writeArgs, items: [item(), item(target, 'two')] }, () => receipt());
      assert(result.error, 'Alias accepted: ' + target);
      assert.equal(result.writers.length, 0);
    }
    const result = await run('code-write', { ...writeArgs, items: [item('missing/new.js'), item('dir-alias/missing/new.js', 'two')] }, () => receipt());
    assert(result.error, 'Missing target aliases accepted');
    assert.equal(result.writers.length, 0);
    const physical = await run('code-write', { ...writeArgs, items: [item('physical/out.js'), item('deep-link/../out.js', 'two')] }, () => receipt());
    assert(physical.error, 'Symlink/.. physical alias accepted');
    assert.equal(physical.writers.length, 0);
  },
  async 'target-failures'() {
    const args = { ...writeArgs, items: [item(), item('second.js', 'two')] };
    for (const probe of [null, {}, { ok: false, targets: [] }, { ok: true, targets: [] }, { ok: true, targets: [{ target: 'other', canonical: '/tmp/other', identity: null }, {}] }]) {
      const result = await run('code-write', args, () => receipt(), () => probe);
      assert(result.error); assert.equal(result.writers.length, 0);
    }
    const result = await run('code-write', args, () => receipt(), () => { throw Error('probe dead'); });
    assert(result.error); assert.equal(result.writers.length, 0);
  },
  async 'target-probe'() {
    const target = "odd ' ; touch SHOULD_NOT_EXIST ; $(touch ALSO_NOT_CREATED).js";
    const result = good(await run('code-write', { ...writeArgs, items: [item(), item(target, 'two')] }, (prompt, options) => options.label.endsWith(':one') ? receipt() : receipt(target, 'two')));
    assert.equal(result.writers.length, 2);
    assert(!fs.existsSync(path.join(fixture, 'SHOULD_NOT_EXIST')));
    assert(!fs.existsSync(path.join(fixture, 'ALSO_NOT_CREATED')));
    fs.symlinkSync('missing-target', path.join(fixture, 'dangling'));
    const dangling = await run('code-write', { ...writeArgs, items: [item(), item('dangling', 'two')] }, () => receipt());
    assert(dangling.error); assert.equal(dangling.writers.length, 0);
  },
  async 'writer-output'() {
    for (const output of [
      receipt('out.js', 'one', { check_output_tail: 'export const leaked = true;\n' }),
      receipt('out.js', 'one', { summary: 'X'.repeat(5000) }), receipt('out.js', 'one', { summary: 'line one\nline two' }),
      receipt('out.js', 'one', { lines: -1 }), receipt('out.js', 'one', { lines: 0.5 }),
      receipt('wrong-target.js'), receipt('out.js', 'wrong-key'), receipt('out.js', 'one', { check_ok: true }),
      receipt('out.js', 'one', { check_ran: true }),
    ]) unknownWriter(await run('code-write', writeArgs, () => output));
  },
  async 'writer-crash'() {
    const target = path.join(fixture, 'written-before-crash.js');
    const result = await run('code-write', { ...writeArgs, items: [item(target)] }, () => {
      fs.writeFileSync(target, 'export const created = true;\n');
      throw Error('worker died after writing');
    });
    assert(fs.existsSync(target));
    unknownWriter(result);
  },
};
try {
  const name = process.argv[2];
  assert(tests[name], 'Unknown test: ' + name);
  await tests[name]();
  console.log('PASS ' + name);
} finally { fs.rmSync(fixture, { recursive: true, force: true }); }
