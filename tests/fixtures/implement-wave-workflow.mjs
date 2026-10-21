import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Exercise the actual implement-wave body with native agents replaced by
// controlled responses. No git, model or Claude session is invoked.
// WORKFLOW_SUBJECT permits before/after regression checks against an immutable
// checkout using this same harness.
const subject = process.env.WORKFLOW_SUBJECT || path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;

function load() {
  const source = fs.readFileSync(path.join(subject, 'workflows', 'implement-wave.js'), 'utf8');
  assert(source.startsWith('export const meta = {\n'));
  const metaEnd = source.indexOf('\n};\n');
  assert(metaEnd > 0);
  const meta = new Function(source.slice('export '.length, metaEnd + 3) + '\nreturn meta;')();
  return { meta, body: source.slice(metaEnd + 4) };
}

async function run(args, worker) {
  const body = new AsyncFunction('agent', 'parallel', 'log', 'phase', 'args', load().body);
  const calls = [], logs = [], phases = [];
  let active = 0, maxActive = 0, result, error;
  try {
    result = await body(async (prompt, options) => {
      calls.push({ prompt, options });
      active++;
      maxActive = Math.max(maxActive, active);
      try { return await worker(prompt, options); }
      finally { active--; }
    }, (thunks) => Promise.all(thunks.map(async (thunk) => { try { return await thunk(); } catch { return null; } })),
    (line) => logs.push(line), (name) => phases.push(name), args);
  } catch (caught) { error = caught; }
  const lanes = calls.filter((call) => call.options.label.startsWith('lane:'));
  const verifier = calls.filter((call) => call.options.label === 'verify:wave');
  return { result, error, calls, lanes, verifier, logs, phases, maxActive };
}

const lane = (key) => ({ key, scope: ['src/' + key + '/**'], brief: 'Change ' + key + '.', acceptance: 'The ' + key + ' test passes.' });
const waveArgs = { context: 'fixture repository', lanes: ['a', 'b', 'c'].map(lane) };
const report = (key) => ({
  summary: 'changed ' + key, red_repro: 'red ' + key, green_proof: 'green ' + key,
  files_changed: ['src/' + key + '/x.js'], constraints: [],
});
const verdicts = { verdicts: [{ item: 'a', verdict: 'RESOLVED', evidence: 'fixture' }], residuals: [] };
async function worker(prompt, options) {
  await new Promise((resolve) => setTimeout(resolve, 10));
  if (options.label === 'verify:wave') return verdicts;
  return report(options.label.slice('lane:'.length));
}
function good(run) { assert.ifError(run.error); return run; }

const tests = {
  async sequential() {
    const result = good(await run(waveArgs, worker));
    assert.equal(result.maxActive, 1, 'lanes overlapped: ' + result.maxActive + ' agents were active at once');
    assert.deepEqual(result.calls.map((call) => call.options.label), ['lane:a', 'lane:b', 'lane:c', 'verify:wave']);
    assert.equal(result.verifier.length, 1);
    assert.deepEqual(Object.keys(result.result), ['implementers', 'verification']);
    assert.deepEqual(result.result.implementers.map((value) => value.key), ['a', 'b', 'c']);
    assert.deepEqual(result.result.verification, verdicts);
    // Args are still accepted as the JSON-encoded string the harness may send.
    const encoded = good(await run(JSON.stringify(waveArgs), worker));
    assert.equal(encoded.maxActive, 1);
    assert.equal(encoded.calls.length, 4);
  },
  async 'dead-lane'() {
    const result = good(await run(waveArgs, async (prompt, options) => {
      if (options.label === 'lane:b') throw Error('lane b died');
      return worker(prompt, options);
    }));
    assert.deepEqual(result.calls.map((call) => call.options.label), ['lane:a', 'lane:b', 'lane:c', 'verify:wave']);
    assert.equal(result.result.implementers[1].key, 'b');
    assert.equal(result.result.implementers[1].error, 'implementer agent failed');
    assert.equal(result.result.implementers[2].summary, 'changed c');
    const charter = result.verifier[0].prompt;
    assert(charter.includes('## Lane: b'));
    assert(charter.includes('lane agent failed'));
  },
  async 'prompt-honesty'() {
    const result = good(await run(waveArgs, worker));
    assert.equal(result.lanes.length, 3);
    for (const call of result.lanes) {
      assert(!call.prompt.includes('at the same time'), call.options.label + ' claims concurrent lanes');
      assert(call.prompt.includes('not yours to revert, reset or stash'), call.options.label + ' lacks the shared-tree rule');
    }
    assert(result.verifier[0].prompt.includes('Lanes ran sequentially in this order: a, b, c'));
    const { meta } = load();
    for (const text of [meta.description, meta.whenToUse, ...meta.phases.map((value) => value.detail)]) {
      assert(!/strictly disjoint|disjoint-ownership|in parallel|parallel implementer/i.test(text), 'meta still promises: ' + text);
    }
    assert(meta.description.includes('sequential implementer lanes with declared scopes (instructed, not enforced)'));
  },
  async 'invalid-args-before-dispatch'() {
    const noScope = { ...waveArgs, lanes: [lane('a'), { ...lane('b'), scope: [] }] };
    for (const args of [null, undefined, { context: 'fixture repository' }, { ...waveArgs, lanes: [] }, noScope, JSON.stringify(noScope)]) {
      const result = await run(args, worker);
      assert(result.error, 'Expected rejection before dispatch for ' + JSON.stringify(args));
      assert.match(result.error.message, /implement-wave: bad args/);
      assert.equal(result.calls.length, 0);
    }
  },
};

const name = process.argv[2];
assert(tests[name], 'Unknown test: ' + name);
await tests[name]();
console.log('PASS ' + name);
