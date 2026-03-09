// A glowing orb that travels along a curved arc between two agents,
// leaving a fading trail of particles behind it.
// Each message type has a distinct visual behavior:
//   question     — pulsating yellow orb, zigzag trail, "?" spark
//   response     — smooth green orb, clean flowing trail
//   notification — purple orb with initial flash burst, ring trail
//   task         — red/orange intense orb, angular sharp trail, fast
//   default      — blue orb, standard trail

const TRAIL_LEN = 18;

// Orb visual config per message type
const ORB_STYLES = {
  question: {
    core: "#ffd740", glow: "#ffab00", trail: [255, 215, 64],
    size: 5, glowR: 14, speed: 0.9, trailMode: "zigzag",
  },
  response: {
    core: "#00FF88", glow: "#00b894", trail: [0, 255, 136],
    size: 4, glowR: 11, speed: 1.0, trailMode: "smooth",
  },
  notification: {
    core: "#B967FF", glow: "#7C3AED", trail: [185, 103, 255],
    size: 4.5, glowR: 16, speed: 1.1, trailMode: "rings",
  },
  task: {
    core: "#FF006E", glow: "#d63031", trail: [255, 0, 110],
    size: 5.5, glowR: 13, speed: 1.4, trailMode: "sharp",
  },
  default: {
    core: "#00FFFF", glow: "#0080FF", trail: [0, 255, 255],
    size: 4, glowR: 10, speed: 1.1, trailMode: "smooth",
  },
};

export class MessageOrb {
  constructor(fromX, fromY, toX, toY, kind = "default", onArrive, priority) {
    this.fromX = fromX;
    this.fromY = fromY;
    this.toX = toX;
    this.toY = toY;
    this.onArrive = onArrive;
    this.kind = kind;
    this.priority = priority || "P2";

    this.t = 0;
    this.done = false;

    this._curY = fromY;
    this.x = fromX;

    this.trail = [];
    this._phase = 0;
    this._flashAlpha = kind === "notification" || this.priority === "P0" ? 1.0 : 0;

    const style = ORB_STYLES[kind] || ORB_STYLES.default;
    this.coreColor = style.core;
    this.glowColor = style.glow;
    this.trailColor = style.trail;
    this.orbSize = style.size;
    this.glowR = style.glowR;
    this.speed = style.speed;
    this.trailMode = style.trailMode;

    // P0 override: bigger, faster, red glow
    if (this.priority === "P0") {
      this.coreColor = "#ff4444";
      this.glowColor = "#ff0000";
      this.trailColor = [255, 68, 68];
      this.orbSize = style.size * 1.5;
      this.glowR = style.glowR * 1.4;
      this.speed = style.speed * 1.5;
    }
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

    this._phase += dt;
    this.t += dt * this.speed;

    // Decay flash
    if (this._flashAlpha > 0) {
      this._flashAlpha = Math.max(0, this._flashAlpha - dt * 3);
    }

    if (this.t >= 1) {
      this.t = 1;
      this.done = true;
      if (this.onArrive) this.onArrive();
    }

    const pos = this._bezier(this.t);
    this.x = pos.x;
    this._curY = pos.y;

    // Trail point with mode-specific offset
    let tx = pos.x, ty = pos.y;
    if (this.trailMode === "zigzag") {
      tx += Math.sin(this._phase * 20) * 6;
      ty += Math.cos(this._phase * 15) * 3;
    } else if (this.trailMode === "sharp") {
      tx += (Math.random() - 0.5) * 4;
      ty += (Math.random() - 0.5) * 4;
    }

    this.trail.push({ x: tx, y: ty, alpha: 1 });
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

    // --- Notification flash burst on spawn ---
    if (this._flashAlpha > 0) {
      ctx.globalAlpha = this._flashAlpha * 0.6;
      const flashGrad = ctx.createRadialGradient(
        this.fromX, this.fromY, 0,
        this.fromX, this.fromY, 40 + (1 - this._flashAlpha) * 30
      );
      flashGrad.addColorStop(0, `rgba(${r},${g},${b}, 0.8)`);
      flashGrad.addColorStop(0.5, `rgba(${r},${g},${b}, 0.2)`);
      flashGrad.addColorStop(1, `rgba(${r},${g},${b}, 0)`);
      ctx.fillStyle = flashGrad;
      ctx.beginPath();
      ctx.arc(this.fromX, this.fromY, 70, 0, Math.PI * 2);
      ctx.fill();
    }

    // --- Trail ---
    if (this.trailMode === "rings") {
      // Notification: expanding ring markers along trail
      for (let i = 0; i < this.trail.length; i += 3) {
        const p = this.trail[i];
        const ringR = 3 + (i / this.trail.length) * 6;
        ctx.globalAlpha = p.alpha * 0.25;
        ctx.strokeStyle = `rgb(${r},${g},${b})`;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.arc(p.x, p.y, ringR, 0, Math.PI * 2);
        ctx.stroke();
      }
    } else if (this.trailMode === "sharp") {
      // Task: sharp angular segments
      for (let i = 0; i < this.trail.length; i++) {
        const p = this.trail[i];
        const size = 1.5 + (i / this.trail.length) * 3;
        ctx.globalAlpha = p.alpha * 0.6;
        ctx.fillStyle = `rgb(${r},${g},${b})`;
        // Diamond shape
        ctx.beginPath();
        ctx.moveTo(p.x, p.y - size);
        ctx.lineTo(p.x + size, p.y);
        ctx.lineTo(p.x, p.y + size);
        ctx.lineTo(p.x - size, p.y);
        ctx.closePath();
        ctx.fill();
      }
    } else {
      // Smooth / zigzag: standard dot trail
      for (let i = 0; i < this.trail.length; i++) {
        const p = this.trail[i];
        const size = 2 + (i / this.trail.length) * 3;
        ctx.globalAlpha = p.alpha * 0.5;
        ctx.fillStyle = `rgb(${r},${g},${b})`;
        ctx.beginPath();
        ctx.arc(p.x, p.y, size, 0, Math.PI * 2);
        ctx.fill();
      }
    }

    // Trail connecting line
    if (this.trail.length > 1) {
      ctx.globalAlpha = 0.2;
      ctx.strokeStyle = this.glowColor;
      ctx.lineWidth = this.trailMode === "sharp" ? 2 : 1.5;
      ctx.beginPath();
      ctx.moveTo(this.trail[0].x, this.trail[0].y);
      for (let i = 1; i < this.trail.length; i++) {
        ctx.lineTo(this.trail[i].x, this.trail[i].y);
      }
      ctx.stroke();
    }

    // --- Main orb ---
    if (!this.done) {
      // Outer glow
      ctx.globalAlpha = 0.35;
      const outerGrad = ctx.createRadialGradient(
        this.x, this._curY, 0,
        this.x, this._curY, this.glowR
      );
      outerGrad.addColorStop(0, `rgba(${r},${g},${b}, 0.5)`);
      outerGrad.addColorStop(1, `rgba(${r},${g},${b}, 0)`);
      ctx.fillStyle = outerGrad;
      ctx.beginPath();
      ctx.arc(this.x, this._curY, this.glowR, 0, Math.PI * 2);
      ctx.fill();

      // Core with neon glow
      ctx.globalAlpha = 1;
      ctx.fillStyle = this.coreColor;
      ctx.shadowColor = this.glowColor;
      ctx.shadowBlur = 15;
      ctx.beginPath();

      if (this.trailMode === "sharp") {
        // Task: diamond core
        const s = this.orbSize;
        ctx.moveTo(this.x, this._curY - s);
        ctx.lineTo(this.x + s, this._curY);
        ctx.lineTo(this.x, this._curY + s);
        ctx.lineTo(this.x - s, this._curY);
        ctx.closePath();
      } else if (this.trailMode === "zigzag") {
        // Question: pulsating circle
        const pulse = this.orbSize + Math.sin(this._phase * 8) * 1.5;
        ctx.arc(this.x, this._curY, pulse, 0, Math.PI * 2);
      } else {
        ctx.arc(this.x, this._curY, this.orbSize, 0, Math.PI * 2);
      }
      ctx.fill();

      // Inner bright point
      ctx.fillStyle = "#fff";
      ctx.shadowBlur = 0;
      ctx.beginPath();
      ctx.arc(this.x, this._curY, 1.5, 0, Math.PI * 2);
      ctx.fill();
    }

    ctx.restore();
  }
}
