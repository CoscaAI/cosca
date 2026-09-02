/* =========================================================================
   RIZOMAI — view: Contas.
   Cards de contas (plataforma, handle, health por token-status).
   Modal "Conectar": OAuth (abre authUrl em nova aba) ou credenciais
   diretas (telegram/bluesky/reddit → POST /v1/connect/{platform}/credentials).
   ========================================================================= */
(function(){
  'use strict';

  var V = {};

  V.render = async function(el){
    el.innerHTML = RZUI.loading();
    try {
      var accounts = (await RZ.getList('accounts', '/v1/accounts?limit=100')).data || [];
      var profiles = (await RZ.getList('profiles', '/v1/profiles?limit=100')).data || [];
      var nameById = {};
      profiles.forEach(function(p){ nameById[p.id] = p.name; });

      el.innerHTML =
        '<div class="page-head">' +
          '<div><h2>Contas</h2><p class="muted">Contas de redes sociais conectadas ao RIZOMAI (health por token).</p></div>' +
          '<div class="page-head-actions"><button class="btn btn-primary" id="btn-connect">+ Conectar conta</button></div>' +
        '</div>' +
        (accounts.length
          ? '<div class="accounts-grid">' + accounts.map(accountCard(nameById)).join('') + '</div>'
          : RZUI.empty('Nenhuma conta conectada',
            'Conecte a primeira conta — ou rode o seed, que cria 13 contas fictícias para o modo demo.'));

      el.querySelector('#btn-connect').addEventListener('click', openConnectModal);
    } catch (e) {
      el.innerHTML = '';
      RZ.handleError(e);
    }
  };

  function accountCard(nameById){
    return function(a){
      var p = RZUI.platform(a.platform);
      var handle = a.displayName || a.platformUserId || a.externalIdentifier || a.id;
      return (
        '<div class="account-card">' +
          '<div class="account-top">' +
            RZUI.platformIco(a.platform, 'lg') +
            '<div>' +
              '<div class="account-name">' + RZUI.esc(p ? p.label : a.platform) + '</div>' +
              '<div class="account-handle">' + RZUI.esc(handle) + '</div>' +
            '</div>' +
          '</div>' +
          '<div class="account-meta">' +
            '<span>' + RZUI.tokenBadge(a.tokenStatus) + '</span>' +
            '<span>' + RZUI.esc(nameById[a.profileId] || a.profileId) + '</span>' +
            '<span class="muted">' + RZUI.fmtDate(a.connectedAt) + '</span>' +
          '</div>' +
        '</div>'
      );
    };
  }

  function openConnectModal(){
    RZ.getList('profiles', '/v1/profiles?limit=100').then(function(res){
      var profiles = res.data || [];
      if (!profiles.length) {
        RZUI.toast('Crie um profile antes de conectar contas.', 'error');
        location.hash = '#/profiles';
        return;
      }
      var selected = (RZ.state.profileId && profiles.some(function(p){ return p.id === RZ.state.profileId; }))
        ? RZ.state.profileId : profiles[0].id;

      var modal = RZUI.modal(
        '<div class="modal-head"><h3>Conectar conta</h3><button class="btn btn-ghost btn-sm" data-close>✕</button></div>' +
        '<div class="modal-body">' +
          '<div class="field">' +
            '<label for="cn-profile">Profile</label>' +
            '<select id="cn-profile" class="input select">' +
              profiles.map(function(p){
                return '<option value="' + RZUI.esc(p.id) + '"' + (p.id === selected ? ' selected' : '') + '>' +
                  RZUI.esc(p.name) + '</option>';
              }).join('') +
            '</select>' +
          '</div>' +
          '<div class="field">' +
            '<label for="cn-platform">Plataforma</label>' +
            '<select id="cn-platform" class="input select">' +
              window.RZ_PLATFORMS.map(function(p){
                return '<option value="' + p.id + '">' + p.label +
                  (p.kind === 'credentials' ? ' (credenciais)' : '') + '</option>';
              }).join('') +
            '</select>' +
          '</div>' +
          '<div id="cn-body"></div>' +
        '</div>' +
        '<div class="modal-foot">' +
          '<button class="btn btn-ghost" data-close>Cancelar</button>' +
          '<div id="cn-foot"></div>' +
        '</div>'
      );

      var profileSel = modal.el.querySelector('#cn-profile');
      var platformSel = modal.el.querySelector('#cn-platform');
      var bodyBox = modal.el.querySelector('#cn-body');
      var footBox = modal.el.querySelector('#cn-foot');

      function render(){
        var p = window.RZ_PLATFORMS_MAP[platformSel.value];
        if (!p) return;
        if (p.kind === 'credentials') renderCredentials(p);
        else renderOAuth(p);
      }

      function renderCredentials(p){
        var fields = {
          telegram: [
            { id: 'botToken',    label: 'Bot token',     type: 'password', ph: '123456:ABC-DEF…' },
            { id: 'chatId',      label: 'Chat ID',       type: 'text',     ph: '-100123456789' },
          ],
          bluesky: [
            { id: 'identifier',  label: 'Identificador', type: 'text',     ph: 'user.bsky.social' },
            { id: 'appPassword', label: 'App password',  type: 'password', ph: 'xxxx-xxxx-xxxx-xxxx' },
          ],
          reddit: [
            { id: 'username',    label: 'Username',      type: 'text',     ph: 'user' },
            { id: 'password',    label: 'Password',      type: 'password', ph: '••••••••' },
          ],
        }[p.id] || [];

        bodyBox.innerHTML =
          fields.map(function(f){
            return (
              '<div class="field">' +
                '<label for="cn-' + f.id + '">' + RZUI.esc(f.label) + '</label>' +
                '<input id="cn-' + f.id + '" class="input" type="' + f.type + '" placeholder="' +
                RZUI.esc(f.ph || '') + '" autocomplete="off">' +
              '</div>'
            );
          }).join('') +
          '<p class="muted hint">Credenciais são criptografadas em repouso (AES-256-GCM) e validadas contra a plataforma antes de salvar.</p>';

        footBox.innerHTML = '<button class="btn btn-primary" id="cn-submit">Conectar</button>';
        var submit = footBox.querySelector('#cn-submit');
        submit.addEventListener('click', async function(){
          var payload = { profileId: profileSel.value };
          var ok = true;
          fields.forEach(function(f){
            var v = (bodyBox.querySelector('#cn-' + f.id).value || '').trim();
            payload[f.id] = v;
            if (!v) ok = false;
          });
          if (!ok) { RZUI.toast('Preencha todos os campos.', 'error'); return; }
          submit.disabled = true;
          submit.textContent = 'Conectando…';
          try {
            await RZApi.post('/v1/connect/' + p.id + '/credentials', payload);
            RZ.invalidate('accounts');
            modal.close();
            RZUI.toast('Conta ' + p.label + ' conectada!', 'success');
            V.render(document.getElementById('view'));
          } catch (e) {
            RZUI.toast('Erro: ' + e.message, 'error');
            submit.disabled = false;
            submit.textContent = 'Conectar';
          }
        });
      }

      function renderOAuth(p){
        bodyBox.innerHTML =
          '<p class="muted">Este fluxo abre o OAuth da ' + RZUI.esc(p.label) +
            ' em uma nova aba. Complete o login lá — quando voltar, a conta aparecerá na lista.</p>' +
          '<p class="hint">Em modo demo (sem client_id/secret reais), o OAuth externo pode não concluir — use o seed para contas fictícias.</p>';
        footBox.innerHTML = '<button class="btn btn-primary" id="cn-oauth">Iniciar OAuth</button>';
        footBox.querySelector('#cn-oauth').addEventListener('click', async function(ev){
          var btn = ev.currentTarget;
          btn.disabled = true;
          btn.textContent = 'Gerando URL…';
          try {
            var res = await RZApi.get('/v1/connect/' + p.id + '?profileId=' + encodeURIComponent(profileSel.value));
            var data = res.data || {};
            if (data.authUrl) {
              window.open(data.authUrl, '_blank', 'noopener');
              RZUI.toast('OAuth de ' + p.label + ' aberto em nova aba.', 'success');
            } else {
              RZUI.toast('A API não devolveu URL de autorização.', 'error');
            }
            modal.close();
          } catch (e) {
            RZUI.toast('Erro: ' + e.message, 'error');
            btn.disabled = false;
            btn.textContent = 'Iniciar OAuth';
          }
        });
      }

      profileSel.addEventListener('change', render);
      platformSel.addEventListener('change', render);
      render();
    }).catch(function(e){
      RZUI.toast('Erro: ' + e.message, 'error');
    });
  }

  window.RZViews.accounts = V;
})();
