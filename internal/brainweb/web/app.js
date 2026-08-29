/* ============================================================
   Cosca — Cérebro Neural (replicação fiel do Neural-Network-Nav)
   Neurônios = vértices do modelo OBJ do cérebro (skip step).
   Axônios  = conexões por proximidade (curvas Bézier) + edges reais.
   Sinais   = partículas que viajam pelos axônios e disparam
              propagação (o cérebro "pensa").
   Agentes/Conhecimento reais do Cosca = marcadores coloridos.
   Shaders AdditiveBlending. OrbitControls. Read-only.
   Three.js self-hostado (CSP 'self'). ES Modules.
   ============================================================ */
"use strict";

import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { OBJLoader } from 'three/addons/loaders/OBJLoader.js';

/* ---------- Estado global ---------- */
const state = {
  graph: null,
  scene: null, camera: null, renderer: null, clock: null,
  controls: null, raycaster: null,
  reducedMotion: false, webglOk: true,
  // Propagação LIGADA, mas ADAPTATIVA: reativa a cascata, porém a intensidade
  // (profundidade signalDepth) é controlada pela taxa de atividade REAL (ver
  // pollActivity). Uso intenso = mais passagens; pouco/parado = quase nada.
  // Nunca "bisca bisca" constante — só acompanha o uso real do Don.
  propagateSignals: true,
  // Sistema neural
  brainVertices: [],       // Vector3 dos vértices do cérebro (amostrados)
  neurons: [],             // objetos Neuron (pos, size, color, connection)
  allSignals: [],
  particlePool: [],        // pool reutilizável
  particleCursor: 0,
  markerMap: new Map(),    // vertexIndex (múltiplo de skipStep) -> agent/skill marker
  neuronPoints: null,
  axonLine: null,
  signalPoints: null,
  collisions: 0,
  brainY: { minY: -70, maxY: 70 },  // extensão real do eixo Y (pós-escala)
  activityFeed: null, activityList: null,
  activitySeen: new Set(),
};

/* Settings (espelham o projeto de referência) */
const SETTINGS = {
  verticesSkipStep: 4,        // amostra dos vértices do OBJ
  maxAxonDist: 28,            // distância máx (unidades do modelo)
  maxConnectionsPerNeuron: 6,
  signalMinSpeed: 1.75,
  signalMaxSpeed: 3.25,
  currentMaxSignals: 1200,
  limitSignals: 3000,
  refractorySeconds: 3.5,    // janela refratária após disparar (mantém a rede viva)
  signalDepth: 2,            // máx de saltos da cascata (foco, não o cérebro todo)
};

/* ---------- Paleta por departamento (dados reais) ---------- */
const HUES = {
  kernel: 220, ai: 160, api: 217, analytics: 190, architecture: 245,
  backend: 215, devops: 38, frontend: 205, database: 160, security: 0,
  qa: 0, testing: 90, memory: 230, monitoring: 90, product: 10, documentation: 285,
  infrastructure: 40, platform: 217, uiux: 330, workflow: 210, semantic: 160,
  governance: 265, performance: 30, compliance: 265, cache: 180,
};

const $ = (sel) => document.querySelector(sel);
function hashString(str) {
  let h = 5381;
  for (let i = 0; i < str.length; i++) h = ((h << 5) + h) + str.charCodeAt(i);
  return h >>> 0;
}
function deptHue(d) {
  if (!d) return 220;
  return HUES[d] ?? (hashString(d) % 360);
}
function esc(s) { return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;" }[c])); }

function detectWebGL() {
  const ov = new URLSearchParams(location.search);
  if (ov.has("force2D")) { state.webglOk = false; return false; }
  if (ov.has("forceWebGL")) { state.webglOk = true; return true; }
  try {
    const gl = document.createElement("canvas").getContext("webgl2") || document.createElement("canvas").getContext("webgl");
    if (!gl) { state.webglOk = false; return false; }
    const r = gl.getParameter(gl.RENDERER);
    if (typeof r === "string" && /swiftshader|software|llvmpipe/i.test(r)) { state.webglOk = false; return false; }
    return true;
  } catch (e) { state.webglOk = false; return false; }
}

/* ---------- Init ---------- */
async function init() {
  state.reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  try {
    const res = await fetch("/brain/graph", { headers: { Accept: "application/json" } });
    if (!res.ok) throw new Error("GET /brain/graph " + res.status);
    state.graph = await res.json();
  } catch (e) {
    showFatal("Não foi possível carregar o cérebro: " + e.message);
    return;
  }
  renderHUD(); renderLegend(); initActivityFeed();
  const loading = document.getElementById("loading");
  if (loading) loading.style.display = "none";

  if (!detectWebGL()) { showFallback2D(); return; }
  try {
    initThree();
    await buildBrain();   // carrega OBJ como fonte dos neurônios
    loop();
  } catch (e) { console.error("WebGL falhou:", e); showFallback2D(); }

  // Atividade REAL do Cosca -> pulsos nos neurônios dos agents reais.
  pollActivity();
  setInterval(pollActivity, 4000);
}

function renderHUD() {
  const m = state.graph.meta || {};
  $("#hud-stats").innerHTML = [
    statHTML(m.agents ?? 0, "agentes"),
    statHTML(m.skills ?? 0, "skills"),
    statHTML(state.nodesCount ?? 0, "neurônios"),
  ].join("");
}
function statHTML(v, l) { return `<div class="stat"><b>${v}</b><span>${l}</span></div>`; }
function renderLegend() {
  const rows = [
    { c: "hsl(var(--viz-don))", l: "Don", n: 1 },
    { c: "hsl(var(--viz-kernel))", l: "Kernel", n: 1 },
    { c: "hsl(var(--viz-tenente))", l: "Tenentes", n: countTier(1) },
    { c: "#3B82F6", l: "Capos", n: countTier(2) },
    { c: "#9fe8ff", l: "Sinais", n: 0 },
  ];
  $("#legend-rows").innerHTML = rows.map(r =>
    `<div class="legend-row"><span class="legend-dot" style="background:${r.c};color:${r.c}"></span>
     <span class="legend-label">${r.l}</span><span class="legend-count">${r.n}</span></div>`).join("");
}
function countTier(t) { return (state.graph.nodes || []).filter(n => n.tier === t).length; }

/* ---------- Cena ---------- */
function initThree() {
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(75, innerWidth / innerHeight, 10, 5000);
  camera.position.set(0, 150, 40);
  camera.lookAt(0, 0, 0);
  const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: "high-performance" });
  renderer.setSize(innerWidth, innerHeight);
  renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
  renderer.setClearColor(0x000d0c, 1);
  $("#scene").appendChild(renderer.domElement);
  state.scene = scene; state.camera = camera; state.renderer = renderer; state.clock = new THREE.Clock();
  state.raycaster = new THREE.Raycaster();

  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true; controls.dampingFactor = 0.08;
  controls.enablePan = true; controls.screenSpacePanning = true;
  controls.minDistance = 20; controls.maxDistance = 2000;
  controls.autoRotate = !state.reducedMotion; controls.autoRotateSpeed = 0.25;
  state.controls = controls;

  // Sem grid/eixos — o cérebro flutua limpo no espaço profundo (read-only).
  buildAmbientParticles();
}

function buildAmbientParticles() {
  const count = 400;
  const geo = new THREE.BufferGeometry();
  const arr = new Float32Array(count * 3);
  for (let i = 0; i < count; i++) {
    const r = 300 + ((hashString("a" + i) % 1000) / 1000) * 400;
    const t = (hashString("at" + i) % 360) / 360 * Math.PI * 2;
    const p = Math.acos(((hashString("ap" + i) % 2000) / 2000) * 2 - 1);
    arr[i*3] = r*Math.sin(p)*Math.cos(t); arr[i*3+1] = r*Math.sin(p)*Math.sin(t); arr[i*3+2] = r*Math.cos(p);
  }
  geo.setAttribute("position", new THREE.BufferAttribute(arr, 3));
  const mat = new THREE.PointsMaterial({ color: 0x2a4a5a, size: 0.6, transparent: true, opacity: 0.4, blending: THREE.AdditiveBlending, depthWrite: false });
  state.scene.add(new THREE.Points(geo, mat));
}

/* ---------- Construir cérebro a partir do modelo OBJ ---------- */
async function buildBrain() {
  const loader = new OBJLoader();
  const text = await fetch("/brain/models/brain.obj").then(r => r.text());
  const obj = loader.parse(text);
  const brainMesh = obj.getObjectByName("brain") || obj.children[0];
  const positions = brainMesh.geometry.attributes.position.array;
  const verts = [];
  for (let i = 0; i < positions.length; i += 3) {
    verts.push(new THREE.Vector3(positions[i], positions[i+1], positions[i+2]));
  }
  state.brainVertices = verts;

  // We combine: centro do cérebro = origem. Normaliza p/ escala agradável.
  const box = new THREE.Box3().setFromPoints(verts);
  const center = box.getCenter(new THREE.Vector3());
  const scale = 140 / box.getSize(new THREE.Vector3()).length(); // cabe à câmera
  verts.forEach(v => v.sub(center).multiplyScalar(scale));

  // Extensão REAL do eixo Y pós-escala (~ ±33.5): usada p/ normalizar o
  // gradiente de cor com precisão (não hardcode 70/140).
  let minY = Infinity, maxY = -Infinity;
  for (const v of verts) { if (v.y < minY) minY = v.y; if (v.y > maxY) maxY = v.y; }
  if (!(maxY > minY)) { minY = -70; maxY = 70; }
  state.brainY.minY = minY; state.brainY.maxY = maxY;

  // Mapeia agentes reais (Don/Kernel/capos) para vértices amostrados.
  buildMarkerMap();

  // Neurônios = vértices amostrados (skip step), como no projeto ref.
  const positionsArr = [];
  const colorsArr = [];
  const sizesArr = [];
  const markers = [];

  for (let i = 0; i < verts.length; i += SETTINGS.verticesSkipStep) {
    const pos = verts[i];
    const marker = state.markerMap.get(i);
    // Cor base: gradiente por posição (volume do cérebro) — ciano/azul frio
    // com brilho variando pela altura, para dar profundidade (não branco liso).
    const baseColor = neuronColorByPosition(pos, state.brainY.minY, state.brainY.maxY);
    const color = marker ? marker.color : baseColor;
    const size = marker ? marker.size : THREE.MathUtils.randFloat(0.75, 2.2);
    positionsArr.push(pos.x, pos.y, pos.z);
    colorsArr.push(color.r, color.g, color.b);
    sizesArr.push(size);
    const neuron = {
      pos: pos.clone(), size, color,
      connection: [], receivedSignal: false, firedCount: 0, prevAxon: null, marker,
      refractoryUntil: 0, // periodo refratario depois de disparar
    };
    state.neurons.push(neuron);
  }
  state.nodesCount = state.neurons.length;

  const geom = new THREE.BufferGeometry();
  geom.setAttribute("position", new THREE.Float32BufferAttribute(positionsArr, 3));
  geom.setAttribute("color", new THREE.Float32BufferAttribute(colorsArr, 3));
  geom.setAttribute("size", new THREE.Float32BufferAttribute(sizesArr, 1));
  buildNeuronShader(geom);

  buildAxons();
  buildParticlePool();
  buildSignalMesh();

  // re-HUD com contagem de neurônios.
  renderHUD();
}

/* Mapeia agentes reais para os vértices do cérebro (marcadores acesos).
   CRÍTICO: cada índice gravado DEVE ser múltiplo de verticesSkipStep, pois o
   loop de buildBrain itera i += skipStep — um índice fora do grid amostrado
   nunca é encontrado (bug que fazia Don/Kernel/capos sumirem). */
function buildMarkerMap() {
  const g = state.graph;
  const nodes = g.nodes || [];
  const verts = state.brainVertices;
  if (!verts.length) return;
  const skip = SETTINGS.verticesSkipStep;                 // grid de amostragem
  const sampledCount = Math.floor(verts.length / skip);   // nº de vértices amostrados
  const align = (ordinal) => Math.max(0, Math.min(sampledCount - 1, ordinal)) * skip;

  // Distribui os principais nós (Don, kernel, capos) por vértices espalhados.
  const roots = nodes.filter(n => n.is_root || n.id === "kernel");
  const capos = nodes.filter(n => n.tier === 1 || n.tier === 2);

  // Don e kernel no centro.
  const centerOrd = Math.round((sampledCount - 1) / 2);
  roots.forEach((n, k) => {
    const ordinal = k === 0 ? centerOrd
      : Math.min(sampledCount - 1, centerOrd + Math.round(sampledCount * 0.05));
    const idx = align(ordinal);
    if (idx < verts.length) state.markerMap.set(idx, { color: nodeColor(n), size: 8.0, node: n });
  });

  // Capos espalhados uniformemente pelos vértices (sempre múltiplo de skip).
  capos.forEach((n, k) => {
    const ordinal = Math.floor((sampledCount * (k + 1)) / (capos.length + 1));
    const idx = align(ordinal);
    if (idx < verts.length && !state.markerMap.has(idx)) {
      state.markerMap.set(idx, { color: nodeColor(n), size: 6.0, node: n });
    }
  });
}

function nodeColor(n) {
  if (n.is_root) return new THREE.Color(0xfbbf24);
  if (n.id === "kernel") return new THREE.Color(0xe5e7eb);
  // Marcadores de agentes: mais saturados e claros => destacam mesmo em
  // repouso, mantendo a identidade de cor do departamento.
  return new THREE.Color().setHSL(deptHue(n.department) / 360, 0.68, n.tier === 1 ? 0.82 : 0.72);
}

/* Cor base dos neurônios: gradiente ciano→azul pela altura/profundidade do
   cérebro, dando volume e profundidade em vez de branco uniforme. */
function neuronColorByPosition(pos, minY, maxY) {
  // Normaliza pela extensão REAL do eixo Y (pós-escala: ~±33.5), não 70/140.
  const range = (maxY - minY) || 1;
  const h = THREE.MathUtils.clamp((pos.y - minY) / range, 0, 1);
  const hue = 190 + h * 35;             // ciano -> azul
  const sat = 0.55 + h * 0.25;          // mais saturado no topo
  const light = 0.40 + (1 - Math.abs(h - 0.5)) * 0.20; // repouso contido (escuro) p/ destacar o pulso
  return new THREE.Color().setHSL(hue / 360, sat, light);
}

function buildNeuronShader(geom) {
  const uniforms = { sizeMultiplier: { value: 1.8 }, opacity: { value: 0.6 }, uTex: { value: makeElectricTexture() } };
  const vert = `
    uniform float sizeMultiplier;
    attribute float size;
    attribute vec3 color;
    varying vec3 vColor;
    void main() {
      vColor = color;
      vec4 mv = modelViewMatrix * vec4(position, 1.0);
      gl_PointSize = size * sizeMultiplier * (220.0 / length(mv.xyz));
      gl_Position = projectionMatrix * mv;
    }`;
  const frag = `
    uniform sampler2D uTex;
    uniform float opacity;
    varying vec3 vColor;
    void main() {
      vec4 t = texture2D(uTex, gl_PointCoord);
      gl_FragColor = vec4(vColor, opacity) * t;
      if (gl_FragColor.a < 0.01) discard;
    }`;
  const mat = new THREE.ShaderMaterial({
    uniforms, vertexShader: vert, fragmentShader: frag,
    blending: THREE.AdditiveBlending, transparent: true, depthTest: false,
  });
  state.neuronPoints = new THREE.Points(geom, mat);
  state.neuronShaderMat = mat;
  state.scene.add(state.neuronPoints);
}

/* Axônios: conecta neurônios por proximidade (maxAxonDist), Bézier.
   Otimizado com SPATIAL HASH — só compara neurônios em células vizinhas,
   evitando o loop O(n²) que travava o navegador com milhares de neurônios. */
function buildAxons() {
  const positionsArr = [];
  const opacityArr = [];
  const indicesArr = [];
  let nextIndex = 0;
  const neurons = state.neurons;
  const threshold = SETTINGS.maxAxonDist;
  const cell = threshold; // célula = distância de conexão

  // Grid espacial: célula -> lista de índices de neurônios.
  const grid = new Map();
  const keyOf = (v) => Math.floor(v.x / cell) + "," + Math.floor(v.y / cell) + "," + Math.floor(v.z / cell);
  neurons.forEach((n, idx) => {
    const k = keyOf(n.pos);
    if (!grid.has(k)) grid.set(k, []);
    grid.get(k).push(idx);
  });

  for (let j = 0; j < neurons.length; j++) {
    const n1 = neurons[j];
    const jk = keyOf(n1.pos);
    const [jx, jy, jz] = jk.split(",").map(Number);
    // Checa as células vizinhas (27) em -1..1.
    for (let dx = -1; dx <= 1; dx++) for (let dy = -1; dy <= 1; dy++) for (let dz = -1; dz <= 1; dz++) {
      const ck = (jx+dx) + "," + (jy+dy) + "," + (jz+dz);
      const buckets = grid.get(ck);
      if (!buckets) continue;
      for (const k of buckets) {
        if (k <= j) continue;
        const n2 = neurons[k];
        if (n1.pos.distanceTo(n2.pos) < threshold &&
            n1.connection.length < SETTINGS.maxConnectionsPerNeuron &&
            n2.connection.length < SETTINGS.maxConnectionsPerNeuron) {
          const axon = makeAxon(n1, n2);
          n1.connection.push({ axon, end: "A" });
          n2.connection.push({ axon, end: "B" });
          const verts = axon.vertices;
          const o = THREE.MathUtils.randFloat(0.02, 0.12); // opacidade por axônio (teia mais discreta)
          for (let i = 0; i < verts.length; i++) {
            positionsArr.push(verts[i].x, verts[i].y, verts[i].z);
            opacityArr.push(o);                            // 1 por vértice (== positionsArr)
            if (i < verts.length - 1) {
              const idx = nextIndex;
              indicesArr.push(idx, idx + 1);
            }
            nextIndex += 1;
          }
        }
      }
    }
  }

  if (!indicesArr.length) return;
  const geom = new THREE.BufferGeometry();
  geom.setAttribute("position", new THREE.Float32BufferAttribute(positionsArr, 3));
  geom.setAttribute("opacity", new THREE.Float32BufferAttribute(opacityArr, 1));
  geom.setIndex(new THREE.BufferAttribute(new Uint32Array(indicesArr), 1));
  geom.computeBoundingSphere();
  const uniforms = { opacityMultiplier: { value: 0.5 } }; // teia sináptica visível, mas discreta
  const vert = `
    attribute float opacity;
    uniform float opacityMultiplier;
    varying float vOpacity;
    void main() {
      vOpacity = opacity * opacityMultiplier;
      gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
    }`;
  const frag = `
    varying float vOpacity;
    void main() {
      gl_FragColor = vec4(0.5, 0.75, 1.0, vOpacity);
    }`;
  const mat = new THREE.ShaderMaterial({
    uniforms, vertexShader: vert, fragmentShader: frag,
    blending: THREE.AdditiveBlending, transparent: true, depthTest: false,
  });
  state.axonLine = new THREE.LineSegments(geom, mat);
  state.axonMat = mat;
  state.axonGeom = geom;
  state.scene.add(state.axonLine);
}

function makeAxon(nA, nB) {
  const subdiv = 8;
  const cpLength = nA.pos.distanceTo(nB.pos) / (1.5 + Math.random() * 2.5);
  const cpA = controlPoint(nA.pos, nB.pos, cpLength);
  const cpB = controlPoint(nB.pos, nA.pos, cpLength);
  const curve = new THREE.CubicBezierCurve3(nA.pos.clone(), cpA, cpB, nB.pos.clone());
  // Guarda os neurônios nas pontas (usados pelo updateSignals p/ propagar).
  return { curve, vertices: curve.getSpacedPoints(subdiv), neuronA: nA, neuronB: nB };
}

function controlPoint(v1, v2, len) {
  const dir = v2.clone().sub(v1).normalize();
  const north = new THREE.Vector3(0, 0, 1);
  const axis = new THREE.Vector3().crossVectors(north, dir).normalize();
  const theta = dir.angleTo(north);
  const rot = new THREE.Matrix4().makeRotationAxis(axis, theta);
  const z = Math.cos(Math.PI / 4);
  const ang = Math.random() * Math.PI * 2;
  const r = Math.sqrt(1 - z * z);
  const cp = new THREE.Vector3(r * Math.cos(ang), r * Math.sin(ang), z);
  cp.multiplyScalar(len); cp.applyMatrix4(rot); cp.add(v1);
  return cp;
}

/* ---------- Pool de partículas (sinais) ---------- */
function buildParticlePool() {
  state.particlePool = [];
  for (let i = 0; i < SETTINGS.limitSignals; i++) {
    state.particlePool.push(new THREE.Vector3(0, -9999, 0));
  }
  state.particleCursor = 0;
}
function getParticle() {
  const p = state.particlePool[state.particleCursor];
  state.particleCursor = (state.particleCursor + 1) % state.particlePool.length;
  return p;
}

function buildSignalMesh() {
  const geo = new THREE.BufferGeometry();
  const arr = new Float32Array(SETTINGS.limitSignals * 3);
  geo.setAttribute("position", new THREE.BufferAttribute(arr, 3));
  // Shader: pontos grandes, brilhantes, com textura radial — sempre visíveis
  // independente da distância (pixel size), para o "pulso" saltar aos olhos.
  const mat = new THREE.ShaderMaterial({
    uniforms: { uTex: { value: makeElectricTexture() }, opacity: { value: 1.0 } },
    vertexShader: `
      void main() {
        vec4 mv = modelViewMatrix * vec4(position, 1.0);
        gl_PointSize = 11.0;
        gl_Position = projectionMatrix * mv;
      }`,
    fragmentShader: `
      uniform sampler2D uTex;
      uniform float opacity;
      void main() {
        vec4 t = texture2D(uTex, gl_PointCoord);
        gl_FragColor = vec4(0.8, 0.99, 1.0, opacity) * t;
        if (gl_FragColor.a < 0.01) discard;
      }`,
    transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, depthTest: false,
  });
  state.signalPoints = new THREE.Points(geo, mat);
  state.scene.add(state.signalPoints);
}

function releaseSignalAt(neuron, depth) {
  if (state.allSignals.length >= SETTINGS.currentMaxSignals) return;
  neuron.firedCount += 1;
  // Período refratário: durante ~3.5s este neurônio não re-dispara em cadeia.
  neuron.refractoryUntil = state.clock.getElapsedTime() + SETTINGS.refractorySeconds;
  neuron.receivedSignal = false;
  // depth = nº de saltos desde a ação real. Limita a cascata a um FOCO (raios
  // próximos), não o cérebro inteiro — senão vira "tudo piscando" (ruído).
  if (depth === undefined) depth = 0;
  const maxDepth = SETTINGS.signalDepth ?? 2;
  for (const c of neuron.connection) {
    if (c.axon !== neuron.prevAxon && state.allSignals.length < SETTINGS.limitSignals) {
      const speed = SETTINGS.signalMinSpeed + Math.random() * (SETTINGS.signalMaxSpeed - SETTINGS.signalMinSpeed);
      state.allSignals.push({
        t: c.end === "A" ? 0 : 1, speed, alive: true,
        axon: c.axon, end: c.end, particle: getParticle(),
        depth: depth + 1,
      });
    }
  }
  neuron._flash = 1;
}

/* Atividade REAL do Cosca: busca /brain/activity e dispara pulsos nos
   neurônios-marcadores dos agents que realmente agiram. Read-only.
   Deduplica por ID (activitySeen): o cérebro SÓ pisca quando há atividade
   NOVA — quando o Don está fazendo algo. Sem novidade, fica parado. */
async function pollActivity() {
  try {
    const res = await fetch("/brain/activity", { headers: { Accept: "application/json" } });
    if (!res.ok) return;
    const data = await res.json();
    const acts = data.activities || [];

    // Separa só as atividades NOVAS (ainda não vistas).
    const seen = state.activitySeen;
    const fresh = [];
    for (const a of acts) {
      const id = a.id || (a.at + "-" + a.agent);
      if (seen.has(id)) continue;
      seen.add(id);
      fresh.push(a);
    }
    // Limita o set p/ não crescer indefinidamente.
    if (seen.size > 300) {
      const arr = [...seen];
      state.activitySeen = new Set(arr.slice(arr.length - 300));
    }

    // Acende apenas o que é NOVO (atividade real no momento).
    fresh.forEach((a) => {
      state._actSeed = (a.description || a.action || a.agent || "") + "|" + (a.at || "");
      const neuron = findNeuronForAgent(a.agent);
      if (neuron && neuron.connection.length) releaseSignalAt(neuron);
    });

    // PROPAGAÇÃO ADAPTATIVA: a intensidade das passagens pelos neurônios
    // acompanha a taxa de atividade REAL no momento. Pouco uso = pouca
    // cascata (depth baixo); uso intenso = mais passagens (depth alto).
    // Nunca é fake: é proporcional a quantas ações novas chegaram agora.
    const rate = fresh.length;
    SETTINGS.signalDepth = rate >= 5 ? 3 : rate >= 2 ? 2 : rate >= 1 ? 1 : 0;

    if (fresh.length) renderActivityFeed(fresh);
  } catch (e) { /* best-effort */ }
}

/* Feed textual de atividade recente (DOM, fora do canvas — read-only).
   Painel teal/semitransparente com as últimas ações reais do cérebro. */
function initActivityFeed() {
  const feed = document.createElement("div");
  feed.className = "activity-feed";
  feed.setAttribute("aria-live", "polite");
  feed.innerHTML = '<div class="activity-title">Atividade ao vivo</div><ul class="activity-list"></ul>';
  document.body.appendChild(feed);
  state.activityFeed = feed;
  state.activityList = feed.querySelector(".activity-list");
}

function fmtTime(ms) {
  if (!ms) return "";
  const d = new Date(ms);
  return [d.getHours(), d.getMinutes(), d.getSeconds()].map(x => String(x).padStart(2, "0")).join(":");
}

function statusClass(st) {
  const s = (st || "").toLowerCase();
  if (/fail|error|timeout|panic/.test(s)) return "act-status is-error";
  if (/ok|done|complete|success|successful/.test(s)) return "act-status is-ok";
  return "act-status";
}

function renderActivityFeed(fresh) {
  if (!state.activityList || !fresh || !fresh.length) return;
  const rows = fresh.map(a => {
    const agent = a.agent || "agente";
    const action = a.description || a.action || "";  // descrição breve (nome do comando)
    const status = a.status || "";
    return `<li class="act-item">
      <span class="act-time">${fmtTime(a.at)}</span>
      <span class="act-agent">${esc(agent)}</span>
      <span class="act-action">${esc(action)}</span>
      <span class="${statusClass(status)}">${esc(status)}</span>
    </li>`;
  }).join("");
  state.activityList.insertAdjacentHTML("afterbegin", rows);
  // Feed enxuto: mostra só os ÚLTIMOS 3.
  while (state.activityList.children.length > 3) state.activityList.lastElementChild.remove();
}

/* Associa um nome de agente real a um neurônio-marcador do cérebro.
   Para agentes genéricos (ex.: "kernel"/"don", que o activity log usa para TODO
   comando), escolhemos um neurônio DENTRO DA REDE conectada (não só os poucos
   marcadores fixos) — variando o ponto pelo seed do comando/timestamp, para o
   flash se MOVER pelo volume do cérebro (nunca fixo no mesmo lugar relativo).
   Stay honest: são neurônios com conexões reais, atividade real, só espalhados. */
let agentCursor = 0;
function findNeuronForAgent(agentName) {
  if (!agentName) return null;
  const name = agentName.toLowerCase();

  // 1) Casamento exato com um agente real específico (capo com nome único).
  for (const n of state.neurons) {
    const node = n.marker && n.marker.node;
    if (!node) continue;
    if (node.id.toLowerCase() === name) return n; // exato
  }

  // 2) Agente genérico (kernel/don/opencode que aparece em todo comando):
  //    escolhe dentro de TODOS os neurônios conectados (pool grande), variando
  //    pelo seed do comando. O flash se move pelo volume do cérebro.
  const isGeneric = name === "kernel" || name === "don" || name === "opencode";
  if (isGeneric) {
    const pool = state.neurons.filter((n) => n.connection.length > 0);
    if (pool.length === 0) return null;
    let off = 0;
    if (state._actSeed) { off = hashString(state._actSeed) % pool.length; }
    agentCursor = (agentCursor + 1 + off) % pool.length;
    return pool[agentCursor];
  }

  // 3) Capo buscado por nome aproximado (fuzzy) — usa o marcador dele.
  for (const n of state.neurons) {
    const node = n.marker && n.marker.node;
    if (!node) continue;
    if (node.name.toLowerCase().includes(name) || name.includes(node.name.toLowerCase())) return n;
  }
  return null;
}

function updateSignals(dt) {
  const maxDepth = SETTINGS.signalDepth ?? 2;
  for (let i = state.allSignals.length - 1; i >= 0; i--) {
    const s = state.allSignals[i];
    if (s.end === "A") {
      s.t += s.speed * dt;
      if (s.t >= 1) { s.t = 1; s.alive = false; if (s.depth <= maxDepth) s.axon.neuronB.receivedSignal = true; s.axon.neuronB.prevAxon = s.axon; }
    } else {
      s.t -= s.speed * dt;
      if (s.t <= 0) { s.t = 0; s.alive = false; if (s.depth <= maxDepth) s.axon.neuronA.receivedSignal = true; s.axon.neuronA.prevAxon = s.axon; }
    }
    const pos = s.axon.curve.getPoint(s.t);
    s.particle.set(pos.x, pos.y, pos.z);
    if (!s.alive) state.allSignals.splice(i, 1);
  }
}

function fireNeurons(t) {
  // Toggle: se propagateSignals=false (regime PURO), NÃO repassa o sinal aos
  // vizinhos — só acende quem REALMENTE agiu (via pollActivity). A cascata
  // ("rede pensando") fica disponível quando propagateSignals=true.
  if (state.propagateSignals) {
    for (const n of state.neurons) {
      if (state.allSignals.length < SETTINGS.currentMaxSignals - SETTINGS.maxConnectionsPerNeuron) {
        // Refratário (dt em segundos): dispara só após esfriar. Antes um teto
        // fixo (firedCount<8) que NUNCA decaía congelava a rede após 8 disparos.
        if (n.receivedSignal && t > n.refractoryUntil) { n.fired = true; releaseSignalAt(n); }
      }
      n.receivedSignal = false;
    }
  } else {
    // Regime puro: limpa a flag receivedSignal sem repassar (sem cascata).
    for (const n of state.neurons) n.receivedSignal = false;
  }
  // Sem fallback aleatório: a atividade só entra via dados reais (pollActivity).
}

function updateSignalMesh() {
  const arr = state.signalPoints.geometry.attributes.position.array;
  state.particlePool.forEach((p, i) => {
    arr[i*3] = p.x; arr[i*3+1] = p.y; arr[i*3+2] = p.z;
  });
  state.signalPoints.geometry.attributes.position.needsUpdate = true;
}

function updateNeuronColors(dt, t) {
  if (!state.neuronPoints) return;
  const colAttr = state.neuronPoints.geometry.attributes.color;
  const sizeAttr = state.neuronPoints.geometry.attributes.size;
  state.neurons.forEach((n, i) => {
    const base = n.color;
    if (n._flash > 0) {
      // Flash GRAVE e gradual: dispara em branco-azulado ofuscante e esfria até
      // a cor. Decaimento mais lento (dt*1.2) => o pulso permanece visível por
      // mais tempo, tornando cada ação real inconfundível.
      n._flash -= dt * 1.2;
      const f = Math.max(n._flash, 0);
      const b = 1 + f * 4.0; // brilho até ~5x no pico do disparo (explode do repouso escuro)
      colAttr.array[i*3] = Math.min(1, 0.4 + base.r * b * 0.6 + f * 1.2);
      colAttr.array[i*3+1] = Math.min(1, 0.5 + base.g * b * 0.6 + f * 1.2);
      colAttr.array[i*3+2] = Math.min(1, 0.7 + base.b * b * 0.6 + f * 1.2);
      // Halo: o ponto também EXPANDE durante o pulso (orbe de glow cresce e
      // encolhe com o flash), reforçando a "explosão" do agente que agiu.
      sizeAttr.array[i] = n.size * (1 + f * 3.0);
    } else {
      // REPOUSO ESTÁTICO (sem breathing): a cor fica fixa na base. O cérebro
      // NÃO pisca sozinho — só acende quando há ATIVIDADE REAL (flash _flash).
      // Movimento sem evento real é decoração; parado = parado de verdade.
      colAttr.array[i*3] = base.r;
      colAttr.array[i*3+1] = base.g;
      colAttr.array[i*3+2] = base.b;
      sizeAttr.array[i] = n.size;
    }
  });
  colAttr.needsUpdate = true;
  sizeAttr.needsUpdate = true;
}

/* ---------- Loop ---------- */
function loop() {
  requestAnimationFrame(loop);
  const dt = state.clock.getDelta();
  const t = state.clock.getElapsedTime();

  // SEM heartbeat aleatório — movimento SEM evento real é decoração, não
  // informação. Os sinais só aparecem quando há ATIVIDADE REAL do Cosca
  // (actions/traces). Quando o sistema está parado, o cérebro fica calmo.

  updateSignals(dt);
  fireNeurons(t);
  updateSignalMesh();
  updateNeuronColors(dt, t);

  if (state.controls) {
    state.controls.autoRotate = !state.reducedMotion;
    state.controls.update();
  }
  if (state.renderer) state.renderer.render(state.scene, state.camera);
}

/* ---------- Textura elétrica ---------- */
let _tex = null;
function makeElectricTexture() {
  if (_tex) return _tex;
  const c = document.createElement("canvas"); c.width = c.height = 64;
  const ctx = c.getContext("2d");
  const g = ctx.createRadialGradient(32, 32, 0, 32, 32, 32);
  g.addColorStop(0, "rgba(255,255,255,1)");
  g.addColorStop(0.3, "rgba(180,220,255,0.7)");
  g.addColorStop(1, "rgba(80,120,255,0)");
  ctx.fillStyle = g; ctx.fillRect(0, 0, 64, 64);
  _tex = new THREE.CanvasTexture(c);
  return _tex;
}

/* ---------- Fallback 2D ---------- */
function showFallback2D() {
  $("#scene").style.display = "none";
  const canvas = $("#fallback-canvas"); canvas.style.display = "block";
  drawFallback(canvas);
}
function drawFallback(canvas) {
  const dpr = Math.min(devicePixelRatio, 2);
  canvas.width = innerWidth * dpr; canvas.height = innerHeight * dpr;
  const ctx = canvas.getContext("2d"); ctx.scale(dpr, dpr);
  const W = innerWidth, H = innerHeight;
  ctx.fillStyle = "#012927"; ctx.fillRect(0, 0, W, H);
  ctx.fillStyle = "rgba(255,255,255,0.5)"; ctx.font = "12px Inter"; ctx.textAlign = "center";
  ctx.fillText("Visão 2D (WebGL indisponível) — cérebro neural em modo simplificado", W/2, H-28);
}

function showFatal(msg) {
  $("#scene").innerHTML = `<div style="display:grid;place-items:center;height:100vh;text-align:center;color:#fff"><div>
    <h2 style="color:hsl(var(--viz-don))">Cérebro indisponível</h2>
    <p style="opacity:0.75;max-width:520px">${esc(msg)}</p></div></div>`;
}

window.addEventListener("DOMContentLoaded", init);
