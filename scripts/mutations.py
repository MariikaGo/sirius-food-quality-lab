from pathlib import Path
import json,shutil,subprocess,tempfile
root=Path(__file__).resolve().parent.parent
cases=[('quantity_zero','quantity <= 0','quantity < 0'),('exact_balance','total > u.Balance','total >= u.Balance'),('availability','!i.Available','i.Available'),('blocked_user','if u.Blocked','if !u.Blocked')]
report=[]
for name,old,new in cases:
 with tempfile.TemporaryDirectory(prefix='sirius-mutation-') as folder:
  p=Path(folder)/'src';shutil.copytree(root,p,ignore=shutil.ignore_patterns('.git','.env','evidence','bin','.runtime'))
  f=p/'internal/orders/service.go';text=f.read_text();assert old in text;f.write_text(text.replace(old,new,1))
  r=subprocess.run(['go','test','-count=1','./internal/orders'],cwd=p,text=True,capture_output=True)
  assert '[build failed]' not in r.stdout+r.stderr,(name,r.stdout,r.stderr)
  killed=r.returncode!=0;report.append({'mutation':name,'killed':killed,'output':r.stdout+r.stderr});print(name, 'KILLED' if killed else 'SURVIVED',flush=True)
(root/'evidence/mutations.json').write_text(json.dumps(report,indent=2)+'\n')
assert all(x['killed'] for x in report),'surviving mutations require stronger tests'
