/**
 * MechSprite — Preloads and serves mech pixel art sprites.
 *
 * 7 colors: blue, cyan, green, grey, orange, red, yellow
 * Animations: idle (13 frames, green=15), trans (3), revert (3)
 * Frame size: 80x80px
 */

const MECH_COLORS = ["blue", "cyan", "grey", "orange", "red", "yellow"];
const IDLE_FRAMES = { blue: 13, cyan: 13, grey: 13, orange: 13, red: 13, yellow: 13 };
const TRANS_FRAMES = 3;
const REVERT_FRAMES = 3;

function nameHash(str) {
  let h = 0;
  for (let i = 0; i < str.length; i++) h = ((h << 5) - h + str.charCodeAt(i)) | 0;
  return Math.abs(h);
}

class MechSpriteManager {
  constructor() {
    this._cache = new Map();
  }

  preload() {
    for (const color of MECH_COLORS) {
      const idleCount = IDLE_FRAMES[color];
      for (let i = 1; i <= idleCount; i++) this._load(`/img/mechs/${color}/idle/${i}.png`);
      for (let i = 1; i <= TRANS_FRAMES; i++) this._load(`/img/mechs/${color}/trans/${i}.png`);
      for (let i = 1; i <= REVERT_FRAMES; i++) this._load(`/img/mechs/${color}/revert/${i}.png`);
    }
    // Shadows
    for (let i = 1; i <= 5; i++) this._load(`/img/mechs/shadow/${i}.png`);
  }

  _load(path) {
    if (this._cache.has(path)) return;
    const img = new Image();
    img.src = path;
    this._cache.set(path, img);
  }

  _get(path) {
    const img = this._cache.get(path);
    return (img && img.complete && img.naturalWidth > 0) ? img : null;
  }

  /** Get a deterministic mech color from an agent name. */
  colorForAgent(agentName) {
    return MECH_COLORS[nameHash(agentName) % MECH_COLORS.length];
  }

  /** Get idle frame. frameIndex will be modulo'd. */
  getIdleFrame(color, frameIndex) {
    const count = IDLE_FRAMES[color] || 13;
    const f = (frameIndex % count) + 1;
    return this._get(`/img/mechs/${color}/idle/${f}.png`);
  }

  /** Get transform frame (1-3). */
  getTransFrame(color, frameIndex) {
    const f = (frameIndex % TRANS_FRAMES) + 1;
    return this._get(`/img/mechs/${color}/trans/${f}.png`);
  }

  /** Get shadow image (size 1-5, 5 = largest). */
  getShadow(size) {
    const s = Math.max(1, Math.min(5, size));
    return this._get(`/img/mechs/shadow/${s}.png`);
  }
}

export const mechSprite = new MechSpriteManager();
export { MECH_COLORS, IDLE_FRAMES };
