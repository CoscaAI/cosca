/* =========================================================================
   RIZOMAI — view: Posts (o coração do produto).
   Lista com badge agregado + chips por plataforma (status por target).
   Modal "Novo post": conteúdo, checkboxes das 13 plataformas (só contas do
   profile ativo), agendamento opcional + timezone IANA. Trata 409 CONFLICT
   (dedup content-hash — ADR-005 §1.3).
   ========================================================================= */
(function(){
  'use strict';

  var V = {};

  var STATUS_FILTERS = [
    ['', 'Todos os status'],
    ['published', 'Publicados'],
    ['scheduled', 'Agendados'],
    ['publishing', 'Publicando'],
    ['partial', 'Parciais'],
    ['failed', 'Falhos'],
    ['cancelled', 'Cancelados'],
  ];

  V.render = async function(el){
    el.innerHTML = RZUI.loading();
    try {
      var res = await RZ.getList('posts', '/v1/posts?limit=100');
      renderList(el, res.data || []);
    } catch (e) {
      el.innerHTML = '';
      RZ.handleError(e);
    }
  };

  function renderList(el, posts){
    el.innerHTML =
      '<div class="page-head">' +
        '<div><h2>Posts</h2><p class="muted">Fan-out: um conteúdo → N plataformas, com status por target.</p></div>' +
        '<div class="page-head-actions">' +
          '<select id="filter-status" class="input select">' +
            STATUS_FILTERS.map(function(f){
              return '<option value="' + f[0] + '">' + f[1] + '</option>';
            }).join('') +
          '</select>' +
          '<button class="btn btn-primary" id="btn-new-post">✎ Novo post</button>' +
        '</div>' +
      '</div>' +
      '<div class="demo-note">💡 <b>Modo demo:</b> com <span class="mono">RIZOMAI_QUEUE=simulated</span> o post é publicado na hora — os badges ficam <b>🟢 publicados</b>.</div>' +
      '<div id="posts-list" class="posts-list">' +
        (posts.length
          ? posts.map(postCard).join('')
          : RZUI.empty('Nenhum post ainda', 'Crie o primeiro post com um clique no botão acima.')) +
      '</div>';

    var filter = el.querySelector('#filter-status');
    filter.addEventListener('change', function(){
      var v = filter.value;
      Array.prototype.forEach.call(el.querySelectorAll('.post-card'), function(c){
        c.hidden = v && c.getAttribute('data-status') !== v;
      });
    });

    el.querySelector('#btn-new-post').addEventListener('click', openNewPostModal);
  }

  function postCard(p){
    var chips = (p.platforms || []).map(function(t){
      return RZUI.platformChipLabeled(t.platform, t.status);
    }).join('');
    var sched = p.scheduledFor
      ? '<div class="p-meta">⏰ Agendado para ' + RZUI.fmtDateTime(p.scheduledFor) +
        (p.timezone && p.timezone !== 'UTC' ? ' (' + RZUI.esc(p.timezone) + ')' : '') + '</div>'
      : '';
    return (
      '<div class="post-card" data-status="' + RZUI.esc(p.status) + '">' +
        '<div class="post-content">' + RZUI.esc(p.content) + '</div>' +
        '<div class="post-meta">' +
          RZUI.badge(p.status) +
          '<span class="muted">' + RZUI.fmtDateTime(p.createdAt) + '</span>' +
          '<span class="muted mono">' + RZUI.esc(p.id) + '</span>' +
        '</div>' +
        sched +
        (chips ? '<div class="chips">' + chips + '</div>' : '') +
      '</div>'
    );
  }

  async function openNewPostModal(){
    var profiles, accounts;
    try {
      profiles = (await RZ.getList('profiles', '/v1/profiles?limit=100')).data || [];
      accounts = (await RZ.getList('accounts', '/v1/accounts?limit=100')).data || [];
    } catch (e) {
      RZUI.toast('Não foi possível carregar profiles/contas: ' + e.message, 'error');
      return;
    }
    if (!profiles.length) {
      RZUI.toast('Crie um profile primeiro.', 'error');
      location.hash = '#/profiles';
      return;
    }
    if (!accounts.length) {
      RZUI.toast('Não há contas conectadas. Rode o seed ou conecte uma conta.', 'error');
      location.hash = '#/accounts';
      return;
    }

    var selectedProfile = (RZ.state.profileId && profiles.some(function(p){ return p.id === RZ.state.profileId; }))
      ? RZ.state.profileId : profiles[0].id;

    var modal = RZUI.modal(
      '<div class="modal-head"><h3>Novo post</h3><button class="btn btn-ghost btn-sm" data-close>✕</button></div>' +
      '<div class="modal-body">' +
        '<div class="field">' +
          '<label for="np-profile">Profile</label>' +
          '<select id="np-profile" class="input select">' +
            profiles.map(function(p){
              return '<option value="' + RZUI.esc(p.id) + '"' + (p.id === selectedProfile ? ' selected' : '') + '>' +
                RZUI.esc(p.name) + '</option>';
            }).join('') +
          '</select>' +
        '</div>' +
        '<div class="field">' +
          '<label for="np-content">Conteúdo <span class="muted" id="np-count">0 / 4000</span></label>' +
          '<textarea id="np-content" class="input textarea" maxlength="4000" rows="6" placeholder="Escreva o post que será publicado em todas as redes selecionadas…" required></textarea>' +
        '</div>' +
        '<div class="field">' +
          '<label>Plataformas</label>' +
          '<div id="np-platforms" class="check-grid"></div>' +
          '<div class="hint muted">Plataformas sem conta conectada no profile aparecem desabilitadas.</div>' +
        '</div>' +
        '<div class="field">' +
          '<label for="np-schedule">Agendar para (opcional)</label>' +
          '<input id="np-schedule" class="input" type="datetime-local">' +
        '</div>' +
      '</div>' +
      '<div class="modal-foot">' +
        '<button class="btn btn-ghost" data-close>Cancelar</button>' +
        '<button class="btn btn-primary" id="np-submit">Publicar</button>' +
      '</div>',
      { wide: true }
    );

    var profileSel = modal.el.querySelector('#np-profile');
    var contentEl = modal.el.querySelector('#np-content');
    var countEl = modal.el.querySelector('#np-count');
    var platformsBox = modal.el.querySelector('#np-platforms');
    var scheduleEl = modal.el.querySelector('#np-schedule');
    var submit = modal.el.querySelector('#np-submit');

    contentEl.addEventListener('input', function(){
      countEl.textContent = contentEl.value.length + ' / 4000';
    });

    function renderPlatforms(){
      var pid = profileSel.value;
      var profileAccounts = accounts.filter(function(a){ return a.profileId === pid; });
      platformsBox.innerHTML = window.RZ_PLATFORMS.map(function(p){
        var accs = profileAccounts.filter(function(a){ return a.platform === p.id; });
        var has = accs.length > 0;
        var title = has
          ? accs.map(function(a){ return a.displayName || a.platformUserId || a.id; }).join(', ')
          : 'Sem conta conectada neste profile';
        return (
          '<label class="check' + (has ? ' on' : ' disabled') + '" title="' + RZUI.esc(title) + '">' +
            '<input type="checkbox" value="' + RZUI.esc(has ? accs[0].id : '') + '"' + (has ? ' checked' : ' disabled') + '>' +
            RZUI.platformIco(p.id) + RZUI.esc(p.label) +
          '</label>'
        );
      }).join('');
      Array.prototype.forEach.call(platformsBox.querySelectorAll('input[type=checkbox]'), function(cb){
        cb.addEventListener('change', function(){
          var lbl = cb.closest('.check');
          if (lbl) lbl.classList.toggle('on', cb.checked);
        });
      });
    }

    profileSel.addEventListener('change', renderPlatforms);
    renderPlatforms();

    submit.addEventListener('click', async function(){
      var content = contentEl.value.trim();
      if (!content) { RZUI.toast('Escreva o conteúdo do post.', 'error'); return; }

      var ids = [];
      Array.prototype.forEach.call(platformsBox.querySelectorAll('input[type=checkbox]:checked'), function(cb){
        ids.push(cb.value);
      });
      if (!ids.length) { RZUI.toast('Selecione ao menos uma plataforma com conta conectada.', 'error'); return; }

      var body = {
        content: content,
        platforms: ids.map(function(id){
          var acc = accounts.find(function(a){ return a.id === id; });
          return { platform: acc.platform, accountId: acc.id };
        }),
      };
      var sched = scheduleEl.value;
      if (sched) {
        var d = new Date(sched);
        if (!isNaN(d.getTime())) {
          body.scheduledFor = d.toISOString();
          body.timezone = (Intl.DateTimeFormat().resolvedOptions().timeZone) || 'UTC';
        }
      }

      submit.disabled = true;
      submit.textContent = 'Publicando…';
      try {
        var res = await RZApi.post('/v1/posts', body);
        RZ.invalidate('posts');
        modal.close();
        RZUI.toast(
          'Post criado! Status agregado: ' + (res.data && res.data.status ? res.data.status : 'ok') +
          ' (' + ids.length + ' plataforma(s)).', 'success');
        V.render(document.getElementById('view'));
      } catch (e) {
        if (e instanceof RZApi.ApiError && e.code === 'CONFLICT') {
          RZUI.toast('Post duplicado nos últimos 24h (post existente: ' +
            (e.details && e.details.existingPostId ? e.details.existingPostId : '?') + ').', 'error');
        } else {
          RZUI.toast('Erro: ' + e.message, 'error');
        }
        submit.disabled = false;
        submit.textContent = 'Publicar';
      }
    });
  }

  window.RZViews.posts = V;
})();
