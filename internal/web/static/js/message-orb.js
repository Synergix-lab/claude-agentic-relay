// A glowing orb that travels along a curved arc between two agents,
// leaving a fading trail of particles behind it.

const TRAIL_LEN = 14;

// Orb color schemes by message type
const ORB_STYLES = {
  question:     { core: "#ffd740", glow: "#ffab00", trail: [255, 215, 64] },
  response:     { core: "#55efc4", glow: "#00b894", trail: [0, 230, 118] },
  notification: { core: "#a29bfe", glow: "#6c5ce7", trail: [108, 92, 231] },
  task:         { core: "#ff7675", glow: "#d63031", trail: [255, 118, 117] },
  default:      { core: "#74b9ff", glow: "#0984e3", trail: [116, 185, 255] },
};

export class MessageOrb {
  constructor(fromX, fromY, toX, toY, kind = "default", onArrive) {
    this.fromX = fromX;
    this.fromY = fromY;
    this.toX = toX;
    this.toY = toY;
    this.onArrive = onArrive;

    this.t = 0;
    this.speed = 1.1;
    this.done = false;

    this._curY = fromY;
    this.x = fromX;

    this.trail = [];

    const style = ORB_STYLES[kind] || ORB_STYLES.default;
    this.coreColor = style.core;
    this.glowColor = style.glow;
    this.trailColor = style.trail;
  }

  get y() { return this._curY; }
  set y(v) { this._curY = v; }

  _bezier(t) {
    const mx = (this.fromX + this.toX) / 2;
    const my = (this.fromY + this.toY) / 2;
    const dx = this.toX - this.fromX;
    const perpX = mx + (dx > 0 ? -1 : 1) * Math.abs(dx) * 0.25;
    const perpY = my - 30;

    const u = 1 - t;
    const x = u * u * this.fromX + 2 * u * t * perpX + t * t * this.toX;
    const y = u * u * this.fromY + 2 * u * t * perpY + t * t * this.toY;
    return { x, y };
  }

  update(dt) {
    if (this.done) return;

    this.t += dt * this.speed;

    if (this.t >= 1) {
      this.t = 1;
      this.done = true;
      if (this.onArrive) this.onArrive();
    }

    const pos = this._bezier(this.t);
    this.x = pos.x;
    this._curY = pos.y;

    this.trail.push({ x: pos.x, y: pos.y, alpha: 1 });
    if (this.trail.length > TRAIL_LEN) {
      this.trail.shift();
    }

    for (let i = 0; i < this.trail.length; i++) {
      this.trail[i].alpha = (i + 1) / this.trail.length;
    }
  }

  render(ctx) {
    if (this.trail.length === 0) return;

    ctx.save();

    const [r, g, b] = this.trailColor;
    for (let i = 0; i < this.trail.length; i++) {
      const p = this.trail[i];
      const size = 2 + (i / this.trail.length) * 3;
      ctx.globalAlpha = p.alpha * 0.5;
      ctx.fillStyle = `rgb(${r},${g},${b})`;
      ctx.beginPath();
      ctx.arc(p.x, p.y, size, 0, Math.PI * 2);
      ctx.fill();
    }

    if (this.trail.length > 1) {
      ctx.globalAlpha = 0.2;
      ctx.strokeStyle = this.glowColor;
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.moveTo(this.trail[0].x, this.trail[0].y);
      for (let i = 1; i < this.trail.length; i++) {
        ctx.lineTo(this.trail[i].x, this.trail[i].y);
      }
      ctx.stroke();
    }

    if (!this.done) {
      ctx.globalAlpha = 0.3;
      ctx.fillStyle = this.glowColor;
      ctx.beginPath();
      ctx.arc(this.x, this._curY, 10, 0, Math.PI * 2);
      ctx.fill();

      ctx.globalAlpha = 1;
      ctx.fillStyle = this.coreColor;
      ctx.shadowColor = this.glowColor;
      ctx.shadowBlur = 12;
      ctx.beginPath();
      ctx.arc(this.x, this._curY, 4, 0, Math.PI * 2);
      ctx.fill();

      ctx.fillStyle = "#fff";
      ctx.shadowBlur = 0;
      ctx.beginPath();
      ctx.arc(this.x, this._curY, 1.5, 0, Math.PI * 2);
      ctx.fill();
    }

    ctx.restore();
  }
}
