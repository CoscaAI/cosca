/* =========================================================================
   RIZOMAI — view: Login.
   API key (do seed) → valida via GET /v1/profiles → localStorage → entra.
   401/erro → mensagem clara. Caixa "modo demo" com os passos.
   ========================================================================= */
(function(){
  'use strict';

  var V = {};

  V.render = function(el, message){
    document.title = 'RIZOMAI — Entrar';
    el.innerHTML =
      '<div class="login">' +
        '<div class="login-card">' +
          '<div class="brand brand-center">' +
            '<div class="brand-mark">🌱</div>' +
            '<div class="brand-name">RIZOMAI</div>' +
            '<div class="brand-slogan">Um post. Todas as redes. Uma API.</div>' +
          '</div>' +
          '<h1 class="login-title">Acessar o dashboard</h1>' +
          '<p class="muted login-sub">Entre com a <b>API key</b> do seu team (gerada pelo <span class="mono">make seed</span>).</p>' +
          '<form id="login-form" autocomplete="off">' +
            '<div class="field">' +
              '<label for="login-key">API key</label>' +
              '<input id="login-key" class="input mono" type="password" placeholder="sk_live_..." spellcheck="false" required>' +
            '</div>' +
            '<div id="login-error" class="form-error" hidden></div>' +
            '<button class="btn btn-primary btn-block" type="submit" id="login-btn">Entrar</button>' +
          '</form>' +
          '<div class="login-demo">' +
            '<div class="demo-title">Modo demo — passos</div>' +
            '<ol class="demo-steps">' +
              '<li>Suba o Postgres: <span class="mono">make db-up</span></li>' +
              '<li>Gere a chave e copie a <b>API_KEY</b> impressa: <span class="mono">make seed</span></li>' +
              '<li>Suba a API com fila simulada: <span class="mono">make run-api</span> (env <span class="mono">RIZOMAI_QUEUE=simulated</span>)</li>' +
              '<li>Cole a chave acima e clique em <b>Entrar</b></li>' +
            '</ol>' +
          '</div>' +
        '</div>' +
      '</div>';

    var form = el.querySelector('#login-form');
    var input = el.querySelector('#login-key');
    var errEl = el.querySelector('#login-error');
    var btn = el.querySelector('#login-btn');

    function showError(msg){
      errEl.textContent = msg || '';
      errEl.hidden = !msg;
    }
    function busy(b){
      btn.disabled = b;
      btn.textContent = b ? 'Validando…' : 'Entrar';
    }

    if (RZ.state.key) input.value = RZ.state.key;
    if (message) showError(message);

    form.addEventListener('submit', async function(e){
      e.preventDefault();
      var key = input.value.trim();
      if (!key) { showError('Informe a API key.'); return; }
      showError('');
      busy(true);
      try {
        await RZApi.get('/v1/profiles?limit=1'); // valida a chave no gateway
        RZ.setKey(key);
        window.dispatchEvent(new CustomEvent('rizomai:login'));
      } catch (err) {
        showError(err && err.message ? err.message : 'Não foi possível validar a chave.');
      } finally {
        busy(false);
      }
    });
  };

  window.RZViews.login = V;
})();
