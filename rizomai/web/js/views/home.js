/* =========================================================================
   RIZOMAI — view: Visão geral.
   Stat-cards (profiles, contas /13, posts hoje, /healthz) + posts recentes
   com badges + grade de cobertura de plataformas + ações rápidas.
   ========================================================================= */
(function(){
  'use strict';

  var V = {};

  V.render = async function(el){
    el.innerHTML = RZUI.loading();
    try {
      var profiles = (await RZ.getList('profiles', '/v1/profiles?limit=100')).data || [];
      var accounts = (await RZ.getList('accounts', '/v1/accounts?limit=100')).data || [];
      var posts = (await RZ.getList('posts', '/v1/posts?limit=100')).data || [];

      var health = null;
      try {
        var hr = await fetch('/healthz');
        health = await hr.json();
      } catch (e) { health = null; }

      var today = new Date();
      var postsToday = posts.filter(function(p){ return RZUI.sameDay(p.createdAt, today); }).length;

      var byPlatform = {};
      accounts.forEach(function(a){ byPlatform[a.platform] = (byPlatform[a.platform] || 0) + 1; });
      var coverage = Object.keys(byPlatform).length;
      var healthOk = health && health.status === 'ok';

      el.innerHTML =
        '<div class="welcome">' +
          '<div>' +
            '<h2>Visão geral</h2>' +
            '<p class="muted">Sua operação multi-rede em um só lugar.</p>' +
          '</div>' +
          '<div class="welcome-actions">' +
            '<button class="btn btn-primary" id="go-post">✎ Novo post</button>' +
            '<button class="btn btn-ghost" id="go-account">⚙ Conectar conta</button>' +
          '</div>' +
        '</div>' +
        '<div class="cards-grid">' +
          statCard('Profiles', profiles.length, 'agrupam suas contas', 'profiles') +
          statCard('Contas conectadas', accounts.length, coverage + ' de 13 plataformas', 'accounts') +
          statCard('Posts hoje', postsToday, posts.length + ' no total', 'posts') +
          statCard('Sistema', healthOk ? 'Operacional' : 'Degradado',
            'db: ' + (health ? (health.db || '?') : 'indisponível'), null, healthOk ? 'ok' : 'err') +
        '</div>' +
        '<div class="split-grid">' +
          '<section class="card">' +
            '<div class="card-head"><h3>Posts recentes</h3><a class="link" href="#/posts">Ver todos</a></div>' +
            '<div class="card-body">' +
              (posts.length
                ? posts.slice(0, 5).map(postRow).join('')
                : RZUI.empty('Nenhum post ainda', 'Crie o primeiro post no botão acima.')) +
            '</div>' +
          '</section>' +
          '<section class="card">' +
            '<div class="card-head"><h3>Plataformas</h3><span class="muted">' + coverage + '/13</span></div>' +
            '<div class="card-body"><div class="platform-grid">' + platformGrid(byPlatform) + '</div></div>' +
          '</section>' +
        '</div>';

      el.querySelector('#go-post').addEventListener('click', function(){ location.hash = '#/posts'; });
      el.querySelector('#go-account').addEventListener('click', function(){ location.hash = '#/accounts'; });
    } catch (e) {
      el.innerHTML = '';
      RZ.handleError(e);
    }
  };

  function statCard(title, value, sub, route, tone){
    var inner =
      '<div class="stat-value">' + RZUI.esc(value) + '</div>' +
      '<div class="stat-title">' + RZUI.esc(title) + '</div>' +
      '<div class="muted stat-sub">' + RZUI.esc(sub) + '</div>';
    var cls = 'stat-card' + (tone ? ' tone-' + tone : '');
    return route
      ? '<a class="' + cls + '" href="#/' + route + '">' + inner + '</a>'
      : '<div class="' + cls + '">' + inner + '</div>';
  }

  function postRow(p){
    var chips = (p.platforms || []).map(function(t){
      return RZUI.platformChip(t.platform, t.status, t.accountId);
    }).join('');
    return (
      '<div class="post-row">' +
        '<div class="post-content">' + RZUI.esc(p.content) + '</div>' +
        '<div class="post-meta">' + RZUI.badge(p.status) + '<span class="muted">' + RZUI.fmtDateTime(p.createdAt) + '</span></div>' +
        (chips ? '<div class="chips">' + chips + '</div>' : '') +
      '</div>'
    );
  }

  function platformGrid(byPlatform){
    return window.RZ_PLATFORMS.map(function(p){
      var n = byPlatform[p.id] || 0;
      return (
        '<div class="platform-tile' + (n ? '' : ' missing') + '">' +
          RZUI.platformIco(p.id) + RZUI.esc(p.label) +
          '<span class="cnt">' + n + '</span>' +
        '</div>'
      );
    }).join('');
  }

  window.RZViews.home = V;
})();
