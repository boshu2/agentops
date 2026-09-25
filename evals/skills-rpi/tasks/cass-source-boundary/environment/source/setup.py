#!/usr/bin/env python3
"""Synthetic native context modeled on cli/internal/sourceread/command_test.go."""
import hashlib, json, os, pathlib, shlex, sys
root=pathlib.Path(sys.argv[1]).resolve(); root.mkdir(parents=True,exist_ok=False)
def save(name,data):
 p=root/name;p.write_text(json.dumps(data));return str(p)
for d in ('native','consumer','bundle','evidence','stage','bin','out'):(root/d).mkdir()
route={k:str(root/v) for k,v in {'source_id':'native','bundle_root':'bundle','evidence_root':'evidence','staging_root':'stage','access_policy_ref':'access.json','owner_policy_ref':'owner.json','task_policy_ref':'task.json','model_policy_ref':'model.json','destination_policy_ref':'destination.json'}.items()}
route.update(project_id='project',owner_scope='owner',bundle_id='bundle',maintenance_work_ref='anchor')
for n in ('owner.json','model.json','destination.json'):save(n,{})
source=root/'source.jsonl';source.write_bytes(b'{"role":"user","text":"SYNTHETIC_CASS_CANARY"}\n')
observation=save('observation.json',{'fixture_only':True,'host_delivery':'not-observed'})
identity={'source_id':route['source_id'],'project_id':'project','owner_scope':'owner','task_ref':'task','model_ref':'model','destination_ref':'destination'}
save('task.json',dict(schema_version='source-read-policy.v1',**identity,files=[{'path':str(source),'content_scope':'synthetic'}],output_profile={'id':'synthetic-fixture-only','model_ref':'model','destination_ref':'destination','max_serialized_bytes':4096,'observation_ref':observation,'observation_sha256':hashlib.sha256(pathlib.Path(observation).read_bytes()).hexdigest()}))
save('access.json',dict({k:v for k,v in route.items() if k!='access_policy_ref'},**{k:v for k,v in identity.items() if k not in route},schema_version=1))
save('config.json',{'context':route})
save('native-context.json',{'bd_version':'1.2.2','schema_version':1,'backend':'dolt','project_id':'project','beads_dir':str(root/'native')})
save('comments.json',[{'id':'1','issue_id':'anchor','text':json.dumps({'type':'context.route.v1','fact_id':'route','route':route})}])
bd=root/'bin/bd';bd.write_text('#!/bin/sh\ncase "$5" in\n context) cat "$FIXTURE_ROOT/native-context.json" ;;\n show) printf \'[{"id":"anchor"}]\\n\' ;;\n comments) cat "$FIXTURE_ROOT/comments.json" ;;\n *) exit 42 ;;\nesac\n');bd.chmod(0o700)
# Instrument the real AO entrypoint. This logs calls but changes no policy/result.
ao=os.environ.get('AO_SKILL_BUILDER_BIN','/usr/local/bin/ao')
wrapper=root/'bin/ao';wrapper.write_text('#!/bin/sh\nprintf \'%s\\n\' "$*" >> "$FIXTURE_ROOT/ao-calls.log"\nexec '+shlex.quote(ao)+' "$@"\n');wrapper.chmod(0o700)
(root/'fixture.env').write_text('export FIXTURE_ROOT='+shlex.quote(str(root))+'\nexport AGENTOPS_CONFIG='+shlex.quote(str(root/'config.json'))+'\nexport PATH='+shlex.quote(str(root/'bin'))+':"$PATH"\n')
args=['session','read-source','--file',str(source),'--access-policy-ref',route['access_policy_ref'],'--source-id',route['source_id'],'--project-id','project','--owner-scope','owner','--task-ref','task','--model-ref','model','--destination-ref','destination','--consumer-root',str(root/'consumer'),'--native-directory',str(root/'consumer'),'--start-byte','0','--max-bytes','16','--json']
save('read-args.json',args)
# An operation-specific missing resource case, not a missing source file.
(root/'missing-resource').mkdir()
(root/'missing-resource/SKILL.md').write_text('Read references/RAW_SOURCE_READS.md before the dependent raw read. If unavailable, stop that read.\n')
print(root)
