#!/usr/bin/env node
//
// commit-your-code banner  --  presentation web app
//
// Serves a big "hello, commit your code!" banner styled after
// commityourcode.com, and reads the log written by scripts/evilnode.sh.
// If the latest scan found escapable boundaries it screams
// "UH OH YOU HAVE VULNERABILITIES" and lists them; once every boundary is
// CONTAINED it flips to "YOU'RE SECURE". Re-run the probe, refresh, done.
//
// Zero dependencies -- Node's built-in http/fs only.  Node >= 18.

'use strict';

const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = process.env.PORT || 3000;

// If EVILNODE_LOG is set we read EXACTLY that file (no fallback) so a stale
// log can never sneak in during the demo. Otherwise we try the probe's own
// default targets, most-privileged first.
const DEFAULT_LOGS = ['/var/log/evilnode.log', '/tmp/evilnode.log'];

const LOGO = fs.readFileSync(path.join(__dirname, 'public', 'large_cyc_logo.svg'));

// ---------------------------------------------------------------------------
// log parsing
// ---------------------------------------------------------------------------

function pickLog() {
  const candidates = process.env.EVILNODE_LOG ? [process.env.EVILNODE_LOG] : DEFAULT_LOGS;
  for (const p of candidates) {
    try {
      fs.accessSync(p, fs.constants.R_OK);
      return p;
    } catch {
      /* try next */
    }
  }
  return null;
}

// Parse ONLY the most recent run. The probe uses `>>`, so the file accumulates
// runs; each run starts with a "# evilnode escape probe @ <stamp>" header.
function parseLog(text) {
  const lines = text.split(/\r?\n/);

  let start = 0;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].includes('evilnode escape probe @')) start = i;
  }
  const run = lines.slice(start);

  const stampRe = /evilnode escape probe @ (.+?)\s*$/;
  const secRe = /^==\s*(.+?)\s*==\s*$/;
  const verdictRe = /\[(ESCAPED|LEAK|CONTAINED|INFO)\]\s+(.*\S)\s*$/;

  let stamp = null;
  let section = null;
  let flag = null;
  const items = [];

  for (const line of run) {
    const sm = line.match(stampRe);
    if (sm) { stamp = sm[1].trim(); continue; }

    const secm = line.match(secRe);
    if (secm) {
      const title = secm[1];
      const pm = title.match(/\(([^)]*)\)/);
      flag = pm ? pm[1].trim() : null;
      section = title.replace(/\s*\([^)]*\)\s*/, '').trim();
      continue;
    }

    const vm = line.match(verdictRe);
    if (vm && section && section.toLowerCase() !== 'summary') {
      items.push({ section, flag, status: vm[1], message: vm[2].trim() });
    }
  }
  return { stamp, items };
}

function buildView() {
  const logPath = pickLog();
  if (!logPath) {
    return { state: 'nodata', reason: 'no-log', logPath: null, stamp: null, items: [] };
  }
  let text;
  try {
    text = fs.readFileSync(logPath, 'utf8');
  } catch {
    return { state: 'nodata', reason: 'unreadable', logPath, stamp: null, items: [] };
  }

  const { stamp, items } = parseLog(text);
  if (items.length === 0) {
    return { state: 'nodata', reason: 'empty', logPath, stamp, items: [] };
  }

  const vulns = items.filter((i) => i.status === 'ESCAPED' || i.status === 'LEAK');
  const state = vulns.length > 0 ? 'vuln' : 'secure';
  return { state, logPath, stamp, items, vulnCount: vulns.length, total: items.length };
}

// ---------------------------------------------------------------------------
// rendering
// ---------------------------------------------------------------------------

const esc = (s) =>
  String(s).replace(/[&<>"']/g, (c) =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

const PAGE = (resultsHTML) => `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Commit Your Code</title>
<style>
  :root{
    --bg:#030712; --bg2:#071238; --panel:#0b1220; --panel2:#111a33;
    --blue:#2B7FFF; --blue2:#60A5FA; --line:#1e2a4a;
    --text:#e8eefc; --muted:#8ea0c9;
    --red:#ff5470; --amber:#ffb020; --green:#22e39a;
  }
  *{box-sizing:border-box}
  html,body{margin:0}
  body{
    font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Inter,Helvetica,Arial,sans-serif;
    color:var(--text);
    background:
      radial-gradient(1200px 600px at 50% -10%, rgba(43,127,255,.28), transparent 60%),
      radial-gradient(900px 500px at 90% 20%, rgba(96,165,250,.12), transparent 55%),
      linear-gradient(180deg, var(--bg) 0%, var(--bg2) 100%);
    background-attachment:fixed;
    min-height:100vh;
    -webkit-font-smoothing:antialiased;
  }
  .wrap{max-width:960px;margin:0 auto;padding:48px 24px 80px}
  .logo{display:flex;justify-content:center;margin-bottom:8px}
  .logo svg{width:132px;height:auto;filter:drop-shadow(0 8px 30px rgba(43,127,255,.35))}
  .kicker{text-align:center;letter-spacing:.35em;text-transform:uppercase;
    font-size:12px;color:var(--muted);margin:6px 0 22px}
  .banner{text-align:center;font-weight:800;line-height:1.02;
    font-size:clamp(40px,9vw,104px);letter-spacing:-.02em;margin:0 0 6px}
  .banner .grad{
    background:linear-gradient(180deg,#ffffff 0%, var(--blue2) 55%, var(--blue) 100%);
    -webkit-background-clip:text;background-clip:text;color:transparent;
    text-shadow:0 0 60px rgba(43,127,255,.25);
  }
  .sub{text-align:center;color:var(--muted);font-size:16px;margin:0 0 40px}

  .card{border:1px solid var(--line);border-radius:20px;overflow:hidden;
    background:linear-gradient(180deg, rgba(17,26,51,.75), rgba(11,18,32,.75));
    backdrop-filter:blur(6px);box-shadow:0 30px 80px -30px rgba(0,0,0,.7)}
  .card-head{padding:26px 28px;border-bottom:1px solid var(--line);
    display:flex;align-items:center;gap:16px}
  .card-head .dot{width:14px;height:14px;border-radius:50%;flex:0 0 auto;
    box-shadow:0 0 0 6px rgba(255,255,255,.04)}
  .card-head h2{margin:0;font-size:clamp(22px,3.4vw,34px);font-weight:800;letter-spacing:-.01em}
  .card-head .sub2{margin:2px 0 0;font-size:14px;color:var(--muted)}

  .head-vuln{background:linear-gradient(180deg, rgba(255,84,112,.16), transparent)}
  .head-vuln .dot{background:var(--red);box-shadow:0 0 24px var(--red)}
  .head-secure{background:linear-gradient(180deg, rgba(34,227,154,.16), transparent)}
  .head-secure .dot{background:var(--green);box-shadow:0 0 24px var(--green)}
  .head-nodata .dot{background:var(--muted)}

  ul.vulns{list-style:none;margin:0;padding:8px}
  ul.vulns li{display:flex;gap:14px;align-items:flex-start;padding:16px 18px;
    border-radius:14px}
  ul.vulns li + li{border-top:1px solid rgba(255,255,255,.04)}
  .pill{flex:0 0 auto;font:700 11px/1 ui-monospace,SFMono-Regular,Menlo,monospace;
    letter-spacing:.06em;padding:7px 9px;border-radius:8px;text-transform:uppercase}
  .pill.ESCAPED{background:rgba(255,84,112,.15);color:var(--red);border:1px solid rgba(255,84,112,.4)}
  .pill.LEAK{background:rgba(255,176,32,.14);color:var(--amber);border:1px solid rgba(255,176,32,.4)}
  .pill.CONTAINED{background:rgba(34,227,154,.14);color:var(--green);border:1px solid rgba(34,227,154,.4)}
  .vbody{min-width:0}
  .vtitle{font-weight:700;font-size:16px;display:flex;gap:10px;align-items:baseline;flex-wrap:wrap}
  .flag{font:600 12px/1 ui-monospace,SFMono-Regular,Menlo,monospace;color:var(--blue2);
    background:rgba(43,127,255,.12);border:1px solid rgba(43,127,255,.3);
    padding:4px 7px;border-radius:6px}
  .vmsg{margin:6px 0 0;color:var(--muted);font-size:14px;line-height:1.5}

  .secure-body,.nodata-body{padding:30px 28px;color:var(--muted);font-size:15px;line-height:1.6}
  .secure-body code,.nodata-body code{background:rgba(43,127,255,.12);
    border:1px solid rgba(43,127,255,.25);padding:2px 6px;border-radius:6px;
    font-family:ui-monospace,Menlo,monospace;color:var(--blue2)}
  .checks{display:grid;grid-template-columns:1fr 1fr;gap:8px;margin:18px 0 0}
  @media(max-width:560px){.checks{grid-template-columns:1fr}}
  .checks .ok{display:flex;gap:10px;align-items:center;font-size:14px;color:var(--text)}
  .checks .ok .tick{color:var(--green);font-weight:800}

  .foot{margin-top:22px;text-align:center;color:var(--muted);font-size:12.5px}
  .foot code{font-family:ui-monospace,Menlo,monospace}
  .live{display:inline-flex;align-items:center;gap:7px;margin-left:8px}
  .live .beat{width:8px;height:8px;border-radius:50%;background:var(--green);
    animation:beat 1.6s ease-in-out infinite}
  @keyframes beat{0%,100%{opacity:.35;transform:scale(.8)}50%{opacity:1;transform:scale(1.15)}}
</style>
</head>
<body>
  <div class="wrap">
    <div class="logo">${LOGO}</div>
    <div class="kicker">Commit Your Code &middot; CYC26</div>
    <h1 class="banner"><span class="grad">hello, commit your&nbsp;code!</span></h1>
    <p class="sub">The web dev conference that advances careers while advancing causes.</p>

    <div id="results">${resultsHTML}</div>

    <div class="foot" id="foot"><span id="footmeta"></span><span class="live"><span class="beat"></span>live</span></div>
    <noscript><div class="foot">(live auto-refresh needs JS; reload the page after re-running the probe.)</div></noscript>
  </div>

<script>
const STATUS = {
  ESCAPED: 'ESCAPED', LEAK: 'LEAK', CONTAINED: 'CONTAINED'
};
let lastSig = null;
function render(v){
  // Only touch the DOM when the scan actually changed. Re-rendering every poll
  // would recreate the .beat node and restart its animation, making the live
  // dot snap back to small every few seconds.
  const sig = JSON.stringify(v);
  if(sig === lastSig) return;
  lastSig = sig;

  const results = document.getElementById('results');
  if(v.state === 'vuln'){
    const items = v.items.filter(i => i.status==='ESCAPED' || i.status==='LEAK')
      .sort((a,b)=> (a.status==='ESCAPED'?0:1)-(b.status==='ESCAPED'?0:1));
    results.innerHTML =
      '<div class="card">'
      + '<div class="card-head head-vuln"><span class="dot"></span><div>'
      + '<h2>UH OH &mdash; YOU HAVE VULNERABILITIES</h2>'
      + '<p class="sub2">'+v.vulnCount+' of '+v.total+' isolation boundaries would let an attacker escape or leak host info.</p>'
      + '</div></div>'
      + '<ul class="vulns">'
      + items.map(function(i){
          return '<li><span class="pill '+i.status+'">'+i.status+'</span>'
            + '<div class="vbody"><div class="vtitle">'+escapeHtml(i.section)
            + (i.flag ? ' <span class="flag">'+escapeHtml(i.flag)+'</span>' : '')
            + '</div><p class="vmsg">'+escapeHtml(i.message)+'</p></div></li>';
        }).join('')
      + '</ul></div>';
  } else if(v.state === 'secure'){
    results.innerHTML =
      '<div class="card">'
      + '<div class="card-head head-secure"><span class="dot"></span><div>'
      + '<h2>YOU&rsquo;RE SECURE &#10003;</h2>'
      + '<p class="sub2">All '+v.total+' isolation boundaries are CONTAINED. Nice and locked down.</p>'
      + '</div></div>'
      + '<div class="secure-body">Every namespace held and the fork bomb is capped by the cgroup. '
      + 'Nothing in this container can see, touch, or take down the host.'
      + '<div class="checks">'
      + v.items.map(function(i){
          return '<div class="ok"><span class="tick">&#10003;</span>'+escapeHtml(i.section)
            + (i.flag ? ' <span class="flag">'+escapeHtml(i.flag)+'</span>' : '')+'</div>';
        }).join('')
      + '</div></div></div>';
  } else {
    results.innerHTML =
      '<div class="card"><div class="card-head head-nodata"><span class="dot"></span><div>'
      + '<h2>No scan yet</h2><p class="sub2">Run the escape probe inside the container first.</p>'
      + '</div></div><div class="nodata-body">'
      + 'Start the evil container and run <code>node</code> (that&rsquo;s the probe). '
      + 'It writes <code>'+escapeHtml(v.logPath || '/var/log/evilnode.log')+'</code>, then reload this page.'
      + '</div></div>';
  }
  // Update only the meta text; the persistent .beat node is left alone so its
  // animation keeps running smoothly.
  document.getElementById('footmeta').innerHTML =
    'log: <code>'+escapeHtml(v.logPath || '—')+'</code>'
    + (v.stamp ? ' &middot; scanned '+escapeHtml(v.stamp) : '') + ' ';
}
function escapeHtml(s){return String(s).replace(/[&<>"']/g,function(c){
  return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c];});}
async function poll(){
  try{ const r = await fetch('/api/status',{cache:'no-store'}); render(await r.json()); }
  catch(e){ /* keep last view */ }
}
poll();
setInterval(poll, 2500);
</script>
</body>
</html>`;

// server-side results (so the page is correct before JS runs)
function resultsHTML(v) {
  if (v.state === 'vuln') {
    const items = v.items
      .filter((i) => i.status === 'ESCAPED' || i.status === 'LEAK')
      .sort((a, b) => (a.status === 'ESCAPED' ? 0 : 1) - (b.status === 'ESCAPED' ? 0 : 1));
    return `<div class="card">
      <div class="card-head head-vuln"><span class="dot"></span><div>
        <h2>UH OH &mdash; YOU HAVE VULNERABILITIES</h2>
        <p class="sub2">${v.vulnCount} of ${v.total} isolation boundaries would let an attacker escape or leak host info.</p>
      </div></div>
      <ul class="vulns">${items.map((i) => `<li>
        <span class="pill ${i.status}">${i.status}</span>
        <div class="vbody"><div class="vtitle">${esc(i.section)}${
          i.flag ? ` <span class="flag">${esc(i.flag)}</span>` : ''
        }</div><p class="vmsg">${esc(i.message)}</p></div></li>`).join('')}</ul></div>`;
  }
  if (v.state === 'secure') {
    return `<div class="card">
      <div class="card-head head-secure"><span class="dot"></span><div>
        <h2>YOU&rsquo;RE SECURE &#10003;</h2>
        <p class="sub2">All ${v.total} isolation boundaries are CONTAINED. Nice and locked down.</p>
      </div></div>
      <div class="secure-body">Every namespace held and the fork bomb is capped by the cgroup.
      Nothing in this container can see, touch, or take down the host.</div></div>`;
  }
  return `<div class="card"><div class="card-head head-nodata"><span class="dot"></span><div>
      <h2>No scan yet</h2><p class="sub2">Run the escape probe inside the container first.</p>
    </div></div><div class="nodata-body">Start the evil container and run <code>node</code> (the probe).
    It writes <code>${esc(v.logPath || '/var/log/evilnode.log')}</code>, then reload.</div></div>`;
}

// ---------------------------------------------------------------------------
// server
// ---------------------------------------------------------------------------

const server = http.createServer((req, res) => {
  const url = req.url.split('?')[0];

  if (url === '/api/status') {
    res.writeHead(200, { 'content-type': 'application/json', 'cache-control': 'no-store' });
    res.end(JSON.stringify(buildView()));
    return;
  }
  if (url === '/logo.svg') {
    res.writeHead(200, { 'content-type': 'image/svg+xml' });
    res.end(LOGO);
    return;
  }
  if (url === '/' || url === '/index.html') {
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store' });
    res.end(PAGE(resultsHTML(buildView())));
    return;
  }
  res.writeHead(404, { 'content-type': 'text/plain' });
  res.end('not found');
});

server.listen(PORT, () => {
  const v = buildView();
  console.log(`commit-your-code banner  ->  http://localhost:${PORT}`);
  console.log(`  log source : ${v.logPath || '(none found yet)'}`);
  console.log(`  scan state : ${v.state}${v.state === 'vuln' ? ` (${v.vulnCount}/${v.total} boundaries open)` : ''}`);
  if (v.stamp) console.log(`  scanned at : ${v.stamp}`);
});
