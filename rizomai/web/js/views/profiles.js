/* =========================================================================
   RIZOMAI — view: Profiles.
   Grade de cards + modal criar. Clique define o "profile ativo"
   (persistido em localStorage — usado por posts e contas).
   ========================================================================= */
(function(){
  'use strict';

  var V = {};

  V.render = async function(el){
    el.innerHTML = RZUI.loading();
    try {
      var profiles = (await RZ.getList('profiles', '/v1/profiles?limit=100')).data || [];
      var accounts = (await RZ.getList('accounts', '/v1/accounts?limit=100')).data || [];

      var countByProfile = {};
      accounts.forEach(function(a){
        countByProfile[a.profileId] = (countByProfile[a.profileId] || 0) + 1;
      });

      el.innerHTML =
        '<div class="page-head">' +
          '<div><h2>Profiles</h2><p class="muted">Perfis agrupam as contas conectadas de cada operação.</p></div>' +
          '<div class="page-head-actions"><button class="btn btn-primary" id="btn-new-profile">+ Novo profile</button></div>' +
        '</div>' +
        (profiles.length
          ? '<div class="profiles-grid">' +
            profiles.map(function(p){ return profileCard(p, countByProfile[p.id] || 0); }).join('') +
            '</div>'
          : RZUI.empty('Nenhum profile ainda', 'Crie o primeiro profile para começar a conectar contas.'));

      Array.prototype.forEach.call(el.querySelectorAll('.profile-card'), function(card){
        card.addEventListener('click', function(){
          var id = card.getAttribute('data-id');
          RZ.state.profileId = id;
          localStorage.setItem('rizomai.profileId', id);
          var nameEl = card.querySelector('.p-name');
          RZUI.toast('Profile ativo: ' + (nameEl ? nameEl.textContent.trim() : id), 'success');
          V.render(el);
        });
      });

      el.querySelector('#btn-new-profile').addEventListener('click', openNewProfileModal);
    } catch (e) {
      el.innerHTML = '';
      RZ.handleError(e);
    }
  };

  function profileCard(p, count){
    var active = RZ.state.profileId === p.id;
    return (
      '<div class="profile-card' + (active ? ' active' : '') + '" data-id="' + RZUI.esc(p.id) +
        '" role="button" tabindex="0" title="Usar como profile ativo">' +
        '<div class="p-name">' + RZUI.esc(p.name) +
          (active ? '<span class="p-badge">ativo</span>' : '') + '</div>' +
        '<div class="p-id mono">' + RZUI.esc(p.id) + '</div>' +
        '<div class="p-meta">' + count + ' conta(s) · criado em ' + RZUI.fmtDate(p.createdAt) + '</div>' +
      '</div>'
    );
  }

  function openNewProfileModal(){
    var modal = RZUI.modal(
      '<div class="modal-head"><h3>Novo profile</h3><button class="btn btn-ghost btn-sm" data-close>✕</button></div>' +
      '<div class="modal-body">' +
        '<div class="field">' +
          '<label for="np-name">Nome</label>' +
          '<input id="np-name" class="input" maxlength="100" placeholder="Ex.: Minha Agência" required>' +
        '</div>' +
      '</div>' +
      '<div class="modal-foot">' +
        '<button class="btn btn-ghost" data-close>Cancelar</button>' +
        '<button class="btn btn-primary" id="np-submit">Criar profile</button>' +
      '</div>'
    );
    var nameEl = modal.el.querySelector('#np-name');
    var submit = modal.el.querySelector('#np-submit');

    submit.addEventListener('click', async function(){
      var name = nameEl.value.trim();
      if (!name) { RZUI.toast('Informe o nome do profile.', 'error'); return; }
      submit.disabled = true;
      submit.textContent = 'Criando…';
      try {
        var res = await RZApi.post('/v1/profiles', { name: name });
        RZ.invalidate('profiles');
        modal.close();
        RZUI.toast('Profile criado: ' + (res.data && res.data.name ? res.data.name : name), 'success');
        V.render(document.getElementById('view'));
      } catch (e) {
        RZUI.toast('Erro: ' + e.message, 'error');
        submit.disabled = false;
        submit.textContent = 'Criar profile';
      }
    });
    nameEl.addEventListener('keydown', function(e){ if (e.key === 'Enter') submit.click(); });
  }

  window.RZViews.profiles = V;
})();
