#!/usr/bin/env python3
"""Retained source plus a real failed Codex projection, not a fake builder."""
import hashlib,json,os,pathlib,shutil,subprocess,sys
r=pathlib.Path(sys.argv[1]).resolve();r.mkdir(parents=True,exist_ok=False)
runtime=pathlib.Path(os.environ.get('AO_RUNTIME_ROOT','/opt/agentops')).resolve()
repo=r/'repo';repo.mkdir()
# Public runtime source only. The fixture never copies operator state or .git.
for name in ('skills','scripts','skills-codex','skills-codex-overrides','docs','images','.claude-plugin'):
 shutil.copytree(runtime/name,repo/name)
shutil.copy2(runtime/'registry.json',repo/'registry.json')
(r/'out').mkdir();(r/'evidence').mkdir()
# Exact authorized obstruction: a regular file where the generator needs a directory.
obstruction=repo/'skills-codex/recovery-pilot';obstruction.write_text('INJECTED_PROJECTION_OBSTRUCTION\n')
ao=os.environ.get('AO_SKILL_BUILDER_BIN','/usr/local/bin/ao')
cmd=[ao,'skills','build','from-scratch','recovery-pilot','--repo',str(repo),'--report',str(r/'evidence/failed-build.json')]
result=subprocess.run(cmd,capture_output=True)
(r/'evidence/build.stdout').write_bytes(result.stdout);(r/'evidence/build.stderr').write_bytes(result.stderr);(r/'evidence/build.rc').write_text(str(result.returncode)+'\n')
source=repo/'skills/recovery-pilot/SKILL.md'
if result.returncode==0 or not source.is_file():raise SystemExit('fixture did not reach source-created/projection-failed seam')
# Simulate caller content retained after the incomplete operation.
with source.open('a') as f:f.write('\nRetained caller note: fixture-retained-729.\n')
(r/'evidence/retained-source.md').write_bytes(source.read_bytes())
(r/'evidence/source-before.sha256').write_text(hashlib.sha256(source.read_bytes()).hexdigest()+'\n')
print(r)
