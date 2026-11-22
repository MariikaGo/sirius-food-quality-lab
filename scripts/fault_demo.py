import json
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

root=Path(__file__).resolve().parent.parent

def call(path, token=None, data=None):
    headers={"Authorization":"Bearer "+token} if token else {}
    request=urllib.request.Request("http://127.0.0.1:8000"+path,headers=headers,data=data)
    try:
        with urllib.request.urlopen(request,timeout=10) as response:
            return response.status,json.load(response)
    except urllib.error.HTTPError as error:
        return error.code,json.load(error)

def compose(*args):
    subprocess.run(["docker","compose",*args],cwd=root,check=True,stdout=subprocess.DEVNULL)

status,pair=call('/auth/login',data=urllib.parse.urlencode({'username':'student@example.com','password':'SiriusDemo2026!'}).encode())
if status!=200:
    raise SystemExit('Не удалось войти. При 429 дождитесь окончания минутного лимита входов.')
token=pair['access_token']
demo=root/'evidence/grpc-demo.json'
if not demo.exists():
    raise SystemExit('Сначала выполните ./demo.command, чтобы создать доставку.')
order=json.loads(demo.read_text())['order_id']
path='/delivery/orders/'+str(order)
before,_=call(path,token)
assert before==200,before
try:
    compose('stop','delivery')
    stopped,_=call(path,token)
    menu,_=call('/menu/items')
    assert stopped in (503,504),(stopped,'ожидалась недоступность или истечение дедлайна')
    assert menu==200,menu
    print('Доставка остановлена: HTTP '+str(stopped)+(' — истёк дедлайн.' if stopped==504 else ' — сервис недоступен.')+' Меню: HTTP 200.',flush=True)
finally:
    compose('up','-d','--no-build','--wait','delivery')
deadline=time.monotonic()+20
while time.monotonic()<deadline:
    restored,_=call(path,token)
    if restored==200:
        break
    time.sleep(.2)
assert restored==200,restored
result={'verified_on':time.strftime('%Y-%m-%d'),'before':before,'delivery_stopped':stopped,'menu_during_failure':menu,'restored':restored,'order_id':order}
(root/'evidence/docker-failure-demo.json').write_text(json.dumps(result,indent=2)+'\n')
print('Доставка восстановлена: HTTP 200. API перезапускать не пришлось.')
