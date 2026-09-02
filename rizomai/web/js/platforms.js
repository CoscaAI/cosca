/* =========================================================================
   RIZOMAI — catálogo das 13 plataformas (spec: Platform enum).
   Cada plataforma: id (contrato), label, icon (glyph/emoji, sem CDN),
   color (marca), kind (oauth | credentials — ADR-006 §4).
   ========================================================================= */
(function(){
  'use strict';

  // Plataformas SEM browser OAuth: recebem credenciais diretas via
  // POST /v1/connect/{platform}/credentials (handlers.ConnectCredentials).
  var CREDENTIALS = ['telegram', 'bluesky', 'reddit'];

  var PLATFORMS = [
    { id:'x',              label:'X',              icon:'𝕏', color:'#111111' },
    { id:'linkedin',       label:'LinkedIn',       icon:'in',color:'#0a66c2' },
    { id:'telegram',       label:'Telegram',       icon:'✈', color:'#229ed9' },
    { id:'instagram',      label:'Instagram',      icon:'◎', color:'#e1306c' },
    { id:'facebook',       label:'Facebook',       icon:'f', color:'#1877f2' },
    { id:'threads',        label:'Threads',        icon:'≡', color:'#111111' },
    { id:'youtube',        label:'YouTube',        icon:'▶', color:'#ff0000' },
    { id:'tiktok',         label:'TikTok',         icon:'♪', color:'#010101' },
    { id:'bluesky',        label:'Bluesky',        icon:'☁', color:'#0285ff' },
    { id:'reddit',         label:'Reddit',         icon:'r', color:'#ff4500' },
    { id:'pinterest',      label:'Pinterest',      icon:'p', color:'#e60023' },
    { id:'snapchat',       label:'Snapchat',       icon:'👻',color:'#fffc00', dark:true },
    { id:'googlebusiness', label:'Google Business', icon:'G', color:'#4285f4' },
  ].map(function(p){
    return Object.assign({}, p, {
      kind: CREDENTIALS.indexOf(p.id) >= 0 ? 'credentials' : 'oauth',
    });
  });

  var MAP = {};
  PLATFORMS.forEach(function(p){ MAP[p.id] = p; });

  // Eventos de webhook (catálogo ADR-009 — spec WebhookEvent).
  var EVENTS = [
    'post.scheduled', 'post.published', 'post.partial', 'post.failed',
    'account.connected', 'account.disconnected', 'webhook.test',
  ];

  window.RZ_PLATFORMS = PLATFORMS;
  window.RZ_PLATFORMS_MAP = MAP;
  window.RZ_PLATFORM_IDS = PLATFORMS.map(function(p){ return p.id; });
  window.RZ_WEBHOOK_EVENTS = EVENTS;
})();
