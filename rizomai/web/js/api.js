/* =========================================================================
   RIZOMAI — cliente HTTP do dashboard.
   - Envelope de sucesso { data, page?, limit?, total? } (ADR-005)
   - Envelope de erro { code, error, details } → ApiError tipado
   - Header Idempotency-Key (UUID) automático em mutações (ADR-005 §1.3)
   - 401 → dispara evento 'rizomai:unauthorized' (logout automático)
   ========================================================================= */
(function(){
  'use strict';

  class ApiError extends Error {
    constructor(status, code, message, details){
      super(message || code || 'Erro');
      this.name = 'ApiError';
      this.status = status;
      this.code = code || 'ERROR';
      this.details = details || {};
    }
  }

  function getKey(){
    return (window.RZ && RZ.state && RZ.state.key) || localStorage.getItem('rizomai.key') || '';
  }

  function uuid(){
    if (window.crypto && crypto.randomUUID) return crypto.randomUUID();
    return 'idem-' + Date.now() + '-' + Math.random().toString(36).slice(2);
  }

  async function request(path, opts){
    opts = opts || {};
    const method = (opts.method || 'GET').toUpperCase();
    const headers = { 'Accept': 'application/json' };
    const key = getKey();
    if (key) headers['Authorization'] = 'Bearer ' + key;
    if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
    if (method !== 'GET' && !headers['Idempotency-Key']) {
      headers['Idempotency-Key'] = uuid();
    }

    let res;
    try {
      res = await fetch(path, {
        method: method,
        headers: headers,
        body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      });
    } catch (e) {
      throw new ApiError(0, 'NETWORK_ERROR',
        'Não foi possível conectar à API em "' + path + '". Verifique se o gateway está no ar.', {});
    }

    let payload = null;
    const ct = res.headers.get('content-type') || '';
    if (ct.indexOf('application/json') !== -1) {
      try { payload = await res.json(); } catch (e) { payload = null; }
    }

    if (!res.ok) {
      const err = (payload && payload.code)
        ? payload
        : { code: 'BAD_REQUEST', error: 'Erro inesperado (' + res.status + ')', details: {} };
      if (res.status === 401) {
        window.dispatchEvent(new CustomEvent('rizomai:unauthorized', {
          detail: err.error || 'Sessão expirada — entre novamente.',
        }));
      }
      throw new ApiError(res.status, err.code, err.error, err.details);
    }

    return payload !== null ? payload : { data: null };
  }

  window.RZApi = {
    request: request,
    get: function(path, opts){ return request(path, Object.assign({ method: 'GET' }, opts)); },
    post: function(path, body, opts){ return request(path, Object.assign({ method: 'POST', body: body }, opts)); },
    ApiError: ApiError,
  };
})();
