package main

var uiPage = []byte(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>xAI 403 Fixer</title><style>
body{font-family:system-ui,sans-serif;margin:0;background:#0b1020;color:#e8edf8}.wrap{max-width:1100px;margin:36px auto;padding:0 20px}
.card{background:#151c30;border:1px solid #29324d;border-radius:14px;padding:20px;margin-bottom:18px}.row{display:flex;gap:12px;align-items:center;flex-wrap:wrap}
button{border:0;border-radius:9px;padding:10px 16px;font-weight:700;cursor:pointer}.primary{background:#5b8cff;color:white}.danger{background:#e35d6a;color:white}button:disabled{opacity:.5;cursor:not-allowed}
input{width:300px;max-width:100%;padding:10px;border-radius:8px;border:1px solid #394463;background:#0f1628;color:white}table{width:100%;border-collapse:collapse;margin-top:14px}th,td{padding:10px;border-bottom:1px solid #29324d;text-align:left;font-size:14px}
.ok{color:#65d39a}.bad{color:#ff8290}.muted{color:#96a0bb}code{color:#9fc0ff}@media(max-width:700px){table{display:block;overflow:auto}}
</style></head><body><div class="wrap">
<h1>xAI 403 Auth Fixer</h1><p class="muted">Set xAI OAuth credentials to <code>base_url=https://api.x.ai/v1</code> and <code>using_api=true</code>.</p>
<div class="card"><div class="row"><input id="key" type="password" placeholder="CPA Management Key"><button class="primary" id="scan">Scan</button><button class="danger" id="fix">Fix selected (or all)</button><span id="progress" class="muted"></span></div><p id="message" class="muted"></p></div>
<div class="card"><div class="row"><span>Target: <code id="target"></code></span><span>Backup: <code id="backup">-</code></span></div>
<table><thead><tr><th></th><th>File</th><th>Email</th><th>base_url</th><th>using_api</th><th>Status</th></tr></thead><tbody id="rows"></tbody></table></div>
</div><script>
const api='/v0/management/plugins/xai-403-fixer';const key=document.querySelector('#key');key.value=localStorage.getItem('xai403key')||'';
function headers(json=false){localStorage.setItem('xai403key',key.value);let h={'Authorization':'Bearer '+key.value,'X-Management-Key':key.value};if(json)h['Content-Type']='application/json';return h}
async function call(path,opt={}){let r=await fetch(api+path,{...opt,headers:{...headers(!!opt.body),...(opt.headers||{})}});let data=await r.json();if(!r.ok)throw new Error(data.error||r.statusText);return data}
function esc(v){return String(v??'').replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
function render(s){document.querySelector('#target').textContent=s.target_url;document.querySelector('#backup').textContent=s.backup_dir||'-';document.querySelector('#progress').textContent=s.busy?(s.operation+' '+s.done+'/'+s.total):'';
document.querySelector('#scan').disabled=s.busy;document.querySelector('#fix').disabled=s.busy;document.querySelector('#message').textContent=s.last_error||'';
document.querySelector('#rows').innerHTML=(s.accounts||[]).map(function(a){return '<tr><td><input type="checkbox" data-id="'+esc(a.auth_index)+'" '+(a.needs_fix?'checked':'')+'></td><td>'+esc(a.name)+'</td><td>'+esc(a.email)+'</td><td><code>'+esc(a.base_url||'(missing)')+'</code></td><td>'+(a.has_using_api?String(a.using_api):'(missing)')+'</td><td class="'+(a.error||a.needs_fix?'bad':'ok')+'">'+esc(a.error||a.last_operation||(a.needs_fix?'needs fix':'ok'))+'</td></tr>'}).join('')}
async function refresh(){try{let s=await call('/status');render(s);if(s.busy)setTimeout(refresh,800)}catch(e){document.querySelector('#message').textContent=e.message}}
document.querySelector('#scan').onclick=async function(){try{await call('/scan',{method:'POST'});refresh()}catch(e){document.querySelector('#message').textContent=e.message}};
document.querySelector('#fix').onclick=async function(){let ids=[...document.querySelectorAll('[data-id]:checked')].map(function(x){return x.dataset.id}).filter(Boolean);if(!confirm(ids.length?('Fix '+ids.length+' selected files?'):'No selection. Fix all affected files?'))return;try{await call('/fix',{method:'POST',body:JSON.stringify({auth_indexes:ids})});refresh()}catch(e){document.querySelector('#message').textContent=e.message}};
refresh();
</script></body></html>`)
