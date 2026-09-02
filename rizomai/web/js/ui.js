/* =========================================================================
   RIZOMAI — helpers de UI: escape (anti-XSS), datas, toast, modal,
   badges de status, ícones de plataforma, estados vazios/loading.
   ========================================================================= */
(function(){
  'use strict';

  var UI = {};

  /* --- texto seguro --- */
  UI.esc = function(s){
    return String(s === null || s === undefined ? '' : s)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  };

  /* --- datas (pt-BR) --- */
  UI.fmtDate = function(iso){
    if (!iso) return '—';
    var d = new Date(iso);
    if (isNaN(d.getTime())) return '—';
    return d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric' });
  };
  UI.fmtDateTime = function(iso){
    if (!iso) return '—';
    var d = new Date(iso);
    if (isNaN(d.getTime())) return '—';
    return d.toLocaleString('pt-BR', {
      day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
    });
  };
  UI.sameDay = function(a, b){
    var da = new Date(a), db = new Date(b);
    if (isNaN(da.getTime()) || isNaN(db.getTime())) return false;
    return da.getFullYear() === db.getFullYear() &&
           da.getMonth() === db.getMonth() &&
           da.getDate() === db.getDate();
  };

  /* --- plataformas --- */
  UI.platform = function(id){
    return (window.RZ_PLATFORMS_MAP && RZ_PLATFORMS_MAP[id]) || null;
  };
  UI.platformIco = function(id, size){
    var p = UI.platform(id);
    var color = p ? p.color : '#556b63';
    var ink = p && p.dark ? '#000' : '#fff';
    var label = p ? p.label : (id || '?');
    var font = size === 'lg' ? '16px' : '12px';
    return '<span class="platform-ico" title="' + UI.esc(label) + '" style="background:' + color + ';color:' + ink + ';font-size:' + font + '">' +
      UI.esc(p ? p.icon : '?') + '</span>';
  };

  /* --- status (mapeia status da API → cor/badge) --- */
  var STATUS = {
    published:        { label: 'publicado',  cls: 'ok' },
    scheduled:        { label: 'agendado',   cls: 'warn' },
    publishing:       { label: 'publicando', cls: 'info' },
    pending:          { label: 'pendente',   cls: 'muted' },
    failed:           { label: 'falhou',     cls: 'err' },
    skipped:          { label: 'ignorado',   cls: 'muted' },
    partial:          { label: 'parcial',    cls: 'warn' },
    cancelled:        { label: 'cancelado',  cls: 'muted' },
    ok:               { label: 'ok',         cls: 'ok' },
    expired:          { label: 'expirado',   cls: 'err' },
    revoked:          { label: 'revogado',   cls: 'err' },
    needs_attention:  { label: 'atenção',    cls: 'warn' },
  };
  UI.statusCls = function(status){
    var s = STATUS[status];
    return s ? s.cls : 'muted';
  };
  UI.dot = function(status){
    return '<span class="dot d-' + UI.statusCls(status) + '"></span>';
  };
  UI.badge = function(status){
    var s = STATUS[status];
    var label = s ? s.label : (status || 'desconhecido');
    return '<span class="badge b-' + UI.statusCls(status) + '">' + UI.dot(status) + UI.esc(label) + '</span>';
  };
  UI.tokenBadge = function(status){
    // health de contas: ok → saudável; demais → atenção/erro
    var map = {
      ok: { label: 'saudável', cls: 'ok' },
      expired: { label: 'expirado', cls: 'err' },
      revoked: { label: 'revogado', cls: 'err' },
      needs_attention: { label: 'atenção', cls: 'warn' },
    };
    var s = map[status] || { label: status || 'desconhecido', cls: 'muted' };
    return '<span class="badge b-' + s.cls + '">' + UI.dot(status) + UI.esc(s.label) + '</span>';
  };
  // chip compacto: ícone da plataforma + dot de status do target
  UI.platformChip = function(platform, status, accountId){
    var title = accountId ? ' title="' + UI.esc(accountId) + '"' : '';
    return '<span class="chip"' + title + '>' + UI.platformIco(platform) + UI.dot(status) + '</span>';
  };
  UI.platformChipLabeled = function(platform, status){
    var p = UI.platform(platform);
    var label = p ? p.label : platform;
    return '<span class="chip">' + UI.platformIco(platform) + UI.dot(status) + UI.esc(label) + '</span>';
  };

  /* --- toast --- */
  UI.toast = function(msg, type){
    type = type || 'info';
    var root = document.getElementById('toast-root');
    if (!root) return;
    var t = document.createElement('div');
    t.className = 'toast t-' + type;
    var icon = type === 'success' ? '✓' : (type === 'error' ? '✕' : 'ℹ');
    t.innerHTML = '<span class="toast-ico">' + icon + '</span><span>' + UI.esc(msg) + '</span>';
    root.appendChild(t);
    requestAnimationFrame(function(){ t.classList.add('show'); });
    setTimeout(function(){
      t.classList.remove('show');
      setTimeout(function(){ if (t.parentNode) t.parentNode.removeChild(t); }, 350);
    }, 4200);
  };

  /* --- modal --- */
  UI.modal = function(html, opts){
    opts = opts || {};
    var root = document.getElementById('modal-root');
    var ov = document.createElement('div');
    ov.className = 'modal-overlay' + (opts.wide ? ' modal-wide' : '');
    ov.innerHTML = '<div class="modal" role="dialog" aria-modal="true">' + html + '</div>';

    var closed = false;
    function close(){
      if (closed) return;
      closed = true;
      ov.classList.add('closing');
      setTimeout(function(){ if (ov.parentNode) ov.parentNode.removeChild(ov); }, 160);
      document.removeEventListener('keydown', onKey);
      if (opts.onClose) opts.onClose();
    }
    function onKey(e){ if (e.key === 'Escape') close(); }

    ov.addEventListener('mousedown', function(e){ if (e.target === ov) close(); });
    Array.prototype.forEach.call(ov.querySelectorAll('[data-close]'), function(b){
      b.addEventListener('click', close);
    });
    document.addEventListener('keydown', onKey);
    root.appendChild(ov);

    var first = ov.querySelector('input, select, textarea, button');
    if (first) setTimeout(function(){ first.focus(); }, 30);

    return { el: ov, close: close };
  };

  /* --- estados --- */
  UI.loading = function(){
    return '<div class="loading">Carregando…</div>';
  };
  UI.empty = function(title, hint){
    return '<div class="empty"><div class="empty-ico">🌱</div><h3>' + UI.esc(title) + '</h3>' +
      (hint ? '<p>' + UI.esc(hint) + '</p>' : '') + '</div>';
  };

  window.RZUI = UI;
  window.RZViews = window.RZViews || {};
})();
