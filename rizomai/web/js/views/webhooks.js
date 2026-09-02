/* =========================================================================
   RIZOMAI — view: Webhooks.
   Lista + modal criar (nome, URL https, eventos). O secret é exibido
   UMA única vez (spec — ADR-009 §1.7) com botão copiar.
   ========================================================================= */
(function(){
  'use strict';

  var V = {};

  V.render = async function(el){
    el.innerHTML = RZUI.loading();
    try {
      var res = await RZ.getList('webhooks', '/v1/webhooks');
      var webhooks = res.data || [];

      el.innerHTML =
        '<div class="page-head">' +
          '<div><h2>Webhooks</h2><p class="muted">Receba eventos assinados com HMAC-SHA256 (ADR-009).</p></div>' +
          '<div class="page-head-actions"><button class="btn btn-primary" id="btn-new-webhook">+ Novo webhook</button></div>' +
        '</div>' +
        (webhooks.length
          ? '<div class="posts-list">' + webhooks.map(whCard).join('') + '</div>'
          : RZUI.empty('Nenhum webhook registrado', 'Crie um webhook para receber eventos de posts e contas.'));

      el.querySelector('#btn-new-webhook').addEventListener('click', openNewWebhookModal);
    } catch (e) {
      el.innerHTML = '';
      RZ.handleError(e);
    }
  };

  function whCard(w){
    var events = (w.events || []).map(function(ev){
      return '<span class="tag">' + RZUI.esc(ev) + '</span>';
    }).join(' ');
    return (
      '<div class="post-card">' +
        '<div class="spread">' +
          '<div class="account-name">' + RZUI.esc(w.name) + '</div>' +
          '<span class="badge b-' + (w.isActive ? 'ok' : 'muted') + '">' + (w.isActive ? 'ativo' : 'inativo') + '</span>' +
        '</div>' +
        '<div class="p-id mono">' + RZUI.esc(w.url) + '</div>' +
        '<div class="chips">' + events + '</div>' +
        '<div class="p-meta">criado em ' + RZUI.fmtDateTime(w.createdAt) + ' · ' + RZUI.esc(w.id) + '</div>' +
      '</div>'
    );
  }

  function openNewWebhookModal(){
    var modal = RZUI.modal(
      '<div class="modal-head"><h3>Novo webhook</h3><button class="btn btn-ghost btn-sm" data-close>✕</button></div>' +
      '<div class="modal-body">' +
        '<div class="field">' +
          '<label for="wh-name">Nome</label>' +
          '<input id="wh-name" class="input" maxlength="100" placeholder="Ex.: Notificações no Slack" required>' +
        '</div>' +
        '<div class="field">' +
          '<label for="wh-url">URL (HTTPS obrigatório — ADR-009 §1.7)</label>' +
          '<input id="wh-url" class="input mono" type="url" placeholder="https://app.exemplo.com.br/hooks/rizomai" required>' +
        '</div>' +
        '<div class="field">' +
          '<label>Eventos</label>' +
          '<div id="wh-events" class="check-grid">' +
            window.RZ_WEBHOOK_EVENTS.map(function(ev){
              var checked = (ev === 'post.published' || ev === 'post.failed') ? ' checked' : '';
              return '<label class="check on"><input type="checkbox" value="' + ev + '"' + checked +
                '><span class="mono">' + ev + '</span></label>';
            }).join('') +
          '</div>' +
          '<div class="hint muted">Default da spec: post.published e post.failed.</div>' +
        '</div>' +
      '</div>' +
      '<div class="modal-foot">' +
        '<button class="btn btn-ghost" data-close>Cancelar</button>' +
        '<button class="btn btn-primary" id="wh-submit">Criar webhook</button>' +
      '</div>'
    );

    var nameEl = modal.el.querySelector('#wh-name');
    var urlEl = modal.el.querySelector('#wh-url');
    var submit = modal.el.querySelector('#wh-submit');

    submit.addEventListener('click', async function(){
      var name = nameEl.value.trim();
      var url = urlEl.value.trim();
      if (!name) { RZUI.toast('Informe o nome.', 'error'); return; }
      if (!/^https:\/\//.test(url)) { RZUI.toast('A URL deve ser HTTPS (ADR-009 §1.7).', 'error'); return; }

      var events = [];
      Array.prototype.forEach.call(modal.el.querySelectorAll('#wh-events input:checked'), function(cb){
        events.push(cb.value);
      });

      submit.disabled = true;
      submit.textContent = 'Criando…';
      try {
        var res = await RZApi.post('/v1/webhooks', { name: name, url: url, events: events });
        RZ.invalidate('webhooks');
        showSecret(modal, (res.data || {}).secret);
      } catch (e) {
        RZUI.toast('Erro: ' + e.message, 'error');
        submit.disabled = false;
        submit.textContent = 'Criar webhook';
      }
    });

    function showSecret(modal, secret){
      var body = modal.el.querySelector('.modal-body');
      var foot = modal.el.querySelector('.modal-foot');
      foot.style.display = 'none';
      body.innerHTML =
        '<div style="padding:4px 0">' +
          '<div class="empty-ico">🔑</div>' +
          '<h3>Webhook criado!</h3>' +
          '<p class="muted">O <b>secret</b> abaixo é exibido <b>uma única vez</b> — copie agora. Use-o para validar o header ' +
          '<span class="mono">X-Rizomai-Signature</span> (HMAC-SHA256) dos deliveries.</p>' +
          (secret
            ? '<div class="secret-box"><span class="mono">' + RZUI.esc(secret) +
              '</span><button class="copy-btn" id="wh-copy">Copiar</button></div>'
            : '<p class="muted">(secret não retornado pela API.)</p>') +
          '<div class="secret-warn">⚠️ Não é possível recuperar o secret depois: ele é armazenado apenas como hash + criptografado.</div>' +
          '<button class="btn btn-primary" data-close style="margin-top:14px">Concluir</button>' +
        '</div>';

      var copy = body.querySelector('#wh-copy');
      if (copy) {
        copy.addEventListener('click', function(){
          function done(){ copy.textContent = 'Copiado ✓'; }
          if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(secret).then(done, done);
          } else {
            var ta = document.createElement('textarea');
            ta.value = secret;
            document.body.appendChild(ta);
            ta.select();
            try { document.execCommand('copy'); } catch (e) {}
            document.body.removeChild(ta);
            done();
          }
        });
      }
      RZUI.toast('Webhook criado!', 'success');
    }
  }

  window.RZViews.webhooks = V;
})();
