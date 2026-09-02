/* =========================================================================
   RIZOMAI — app shell.
   Hash router (#/home, #/posts, #/profiles, #/accounts, #/webhooks),
   shell com sidebar + topbar, estado (API key / profile ativo), cache leve
   de dados, logout e tratamento de erro em tela.
   ========================================================================= */
(function(){
  'use strict';

  var KEY_LS = 'rizomai.key';
  var PROFILE_LS = 'rizomai.profileId';

  var RZ = window.RZ = {
    state: {
      key: localStorage.getItem(KEY_LS) || '',
      profileId: localStorage.getItem(PROFILE_LS) || '',
    },
    cache: {},
    views: window.RZViews || {},
    route: 'home',
  };

  /* --- dados: cache leve por recurso (invalida após mutação) --- */
  RZ.getList = function(key, url){
    if (RZ.cache[key] !== undefined && RZ.cache[key] !== null) {
      return Promise.resolve(RZ.cache[key]);
    }
    return RZApi.get(url).then(function(res){
      RZ.cache[key] = res;
      return res;
    });
  };
  RZ.invalidate = function(){
    RZ.cache = {};
  };

  /* --- auth --- */
  RZ.setKey = function(k){
    RZ.state.key = k;
    localStorage.setItem(KEY_LS, k);
  };
  RZ.logout = function(msg){
    RZ.state.key = '';
    RZ.state.profileId = '';
    RZ.cache = {};
    localStorage.removeItem(KEY_LS);
    location.hash = '';
    RZ.render('login', msg);
  };

  /* --- router --- */
  var ALLOWED = ['home', 'posts', 'profiles', 'accounts', 'webhooks'];

  RZ.onHash = function(){
    var h = (location.hash || '#/').replace(/^#\/?/, '');
    var view = ALLOWED.indexOf(h) >= 0 ? h : 'home';
    if (!RZ.state.key) { RZ.render('login'); return; }
    RZ.render(view);
  };

  RZ.render = function(view, msg){
    var el = document.getElementById('app');
    var v = RZ.views[view];
    if (!v) { RZ.render('home'); return; }
    RZ.route = view;
    if (view === 'login') {
      el.innerHTML = '';
      v.render(el, msg);
      return;
    }
    RZ.renderShell(el);
    var viewEl = el.querySelector('#view');
    if (msg) RZUI.toast(msg, 'error');
    v.render(viewEl).catch(function(e){ RZ.handleError(e); });
  };

  /* --- erro em tela --- */
  RZ.handleError = function(e){
    var viewEl = document.getElementById('view');
    if (!viewEl) return;
    if (e && e.name === 'ApiError') {
      if (e.code === 'UNAUTHORIZED') { RZ.logout(e.message); return; }
      viewEl.innerHTML =
        '<div class="empty">' +
          '<div class="empty-ico">⚠️</div>' +
          '<h3>Erro</h3>' +
          '<p>' + RZUI.esc(e.message) +
            (e.code ? ' <span class="mono">(' + RZUI.esc(e.code) + ')</span>' : '') + '</p>' +
          '<button class="btn btn-primary" id="err-retry">Tentar novamente</button>' +
        '</div>';
      var retry = viewEl.querySelector('#err-retry');
      if (retry) retry.addEventListener('click', function(){ RZ.onHash(); });
      return;
    }
    viewEl.innerHTML =
      '<div class="empty"><div class="empty-ico">⚠️</div><h3>Erro inesperado</h3>' +
      '<p>' + RZUI.esc(e && e.message ? e.message : String(e)) + '</p></div>';
  };

  /* --- shell (sidebar + topbar) --- */
  var TITLES = {
    home: 'Visão geral', posts: 'Posts', profiles: 'Profiles',
    accounts: 'Contas', webhooks: 'Webhooks',
  };
  var NAV = [
    ['home', 'Visão geral', '▦'],
    ['posts', 'Posts', '✎'],
    ['profiles', 'Profiles', '▤'],
    ['accounts', 'Contas', '⚙'],
    ['webhooks', 'Webhooks', '⇄'],
  ];

  RZ.renderShell = function(el){
    var key = RZ.state.key;
    var short = key.length > 16 ? key.slice(0, 8) + '…' + key.slice(-4) : key;

    el.innerHTML =
      '<div class="layout">' +
        '<aside class="sidebar">' +
          '<div class="brand">' +
            '<div class="brand-mark">🌱</div>' +
            '<div>' +
              '<div class="brand-name">RIZOMAI</div>' +
              '<div class="brand-slogan">Um post. Todas as redes. Uma API.</div>' +
            '</div>' +
          '</div>' +
          '<nav class="nav">' +
            NAV.map(function(n){
              return '<a class="nav-item' + (RZ.route === n[0] ? ' active' : '') + '" href="#/' + n[0] + '" data-route="' + n[0] + '">' +
                '<span class="nav-ico">' + n[2] + '</span><span>' + n[1] + '</span></a>';
            }).join('') +
          '</nav>' +
          '<div class="sidebar-foot">' +
            '<div class="key-chip" title="API key ativa">' +
              '<span class="key-dot"></span><span class="mono">' + RZUI.esc(short) + '</span>' +
            '</div>' +
            '<button class="btn btn-ghost btn-sm btn-block" id="btn-logout">Sair</button>' +
          '</div>' +
        '</aside>' +
        '<main class="main">' +
          '<header class="topbar">' +
            '<button class="btn btn-ghost btn-sm nav-toggle" id="nav-toggle" aria-label="Abrir menu">☰</button>' +
            '<div class="topbar-title">' + (TITLES[RZ.route] || 'RIZOMAI') + '</div>' +
          '</header>' +
          '<section id="view" class="view" tabindex="-1"></section>' +
        '</main>' +
      '</div>';

    var layout = el.querySelector('.layout');
    el.querySelector('#nav-toggle').addEventListener('click', function(){
      layout.classList.toggle('sidebar-open');
    });
    el.querySelector('#btn-logout').addEventListener('click', function(){
      RZ.logout('Sessão encerrada.');
    });
    Array.prototype.forEach.call(el.querySelectorAll('.nav-item'), function(a){
      a.addEventListener('click', function(){ layout.classList.remove('sidebar-open'); });
    });
    document.title = (TITLES[RZ.route] || 'RIZOMAI') + ' · RIZOMAI';
  };

  /* --- boot --- */
  RZ.init = function(){
    window.addEventListener('hashchange', RZ.onHash);
    window.addEventListener('rizomai:unauthorized', function(e){
      RZ.logout(e.detail || 'Sessão expirada — entre novamente.');
    });
    window.addEventListener('rizomai:login', RZ.onHash);
    RZ.onHash();
  };

  document.addEventListener('DOMContentLoaded', function(){ RZ.init(); });
})();
