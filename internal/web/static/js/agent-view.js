import { SpriteGenerator, PALETTE_COLORS } from "./sprite.js";
import { Bubble } from "./bubble.js";
import { ParticleEmitter } from "./particles.js";

export class AgentView {
  constructor(name, role, description, paletteIndex, online, project) {
    this.name = name;
    this.role = role;
    this.description = description;
    this.paletteIndex = paletteIndex;
    this.online = online;
    this.project = project || "default";

    this.x = 0;
    this.y = 0;
    this.targetX = 0;
    this.targetY = 0;

    this.spawnAlpha = 0;
    this.spawning = true;
    this.frameIndex = 0;
    this.frameTimer = 0;
    this.highlighted = false; // true when part of selected conversation

    this.frames = SpriteGenerator.generate(paletteIndex);
    this.color = PALETTE_COLORS[paletteIndex % PALETTE_COLORS.length];
    this.bubble = null;
    this.particles = new ParticleEmitter();
  }

  setPosition(x, y) {
    this.x = x;
    this.y = y;
    this.targetX = x;
    this.targetY = y;
  }

  showBubble(text, type = "speech") {
    this.bubble = new Bubble(text, type);
  }

  hitTest(px, py) {
    return Math.abs(px - this.x) < 30 && Math.abs(py - this.y) < 30;
  }

  spawnEffect() {
    this.particles.emit("spawn", this.x, this.y + 24);
  }

  update(dt) {
    // Smooth position lerp
    this.x += (this.targetX - this.x) * 4 * dt;
    this.y += (this.targetY - this.y) * 4 * dt;

    // Spawn fade-in
    if (this.spawning) {
      this.spawnAlpha = Math.min(1, this.spawnAlpha + dt * 3);
      if (this.spawnAlpha >= 1) this.spawning = false;
    }

    // Animation frames
    this.frameTimer += dt;
    if (this.frameTimer > 0.5) {
      this.frameTimer = 0;
      this.frameIndex = (this.frameIndex + 1) % this.frames.length;
    }

    // Bubble
    if (this.bubble) {
      this.bubble.update(dt);
      if (!this.bubble.alive) this.bubble = null;
    }

    // Particles
    this.particles.update(dt);
  }

  render(ctx) {
    const alpha = this.spawning ? this.spawnAlpha : 1;
    const dimmed = this.highlighted === false && this._dimMode;

    ctx.save();
    ctx.globalAlpha = dimmed ? alpha * 0.3 : alpha;

    // Draw sprite
    const frame = this.frames[this.frameIndex];
    if (frame) {
      ctx.drawImage(frame, this.x - 24, this.y - 24, 48, 48);
    }

    ctx.globalAlpha = dimmed ? 0.3 : 1;

    // Online/offline dot
    const dotX = this.x + 22;
    const dotY = this.y - 20;
    ctx.beginPath();
    ctx.arc(dotX, dotY, 5, 0, Math.PI * 2);
    if (this.online) {
      ctx.fillStyle = "#00e676";
      ctx.shadowColor = "#00e676";
      ctx.shadowBlur = 6;
    } else {
      ctx.fillStyle = "#636e72";
      ctx.shadowBlur = 0;
    }
    ctx.fill();
    ctx.shadowBlur = 0;

    // Name tag
    ctx.font = "bold 10px 'JetBrains Mono', monospace";
    ctx.textAlign = "center";
    ctx.fillStyle = this.color;
    ctx.fillText(this.name, this.x, this.y + 36);

    // Role (small, below name)
    if (this.role) {
      ctx.font = "9px 'JetBrains Mono', monospace";
      ctx.fillStyle = "rgba(224,224,232,0.4)";
      const shortRole = this.role.length > 25 ? this.role.slice(0, 23) + "..." : this.role;
      ctx.fillText(shortRole, this.x, this.y + 48);
    }

    // Project tag (below role, only when multiple projects exist)
    if (this.showProjectTag && this.project) {
      ctx.font = "8px 'JetBrains Mono', monospace";
      ctx.fillStyle = "rgba(108, 92, 231, 0.45)";
      ctx.fillText(this.project, this.x, this.y + 58);
    }

    ctx.restore();

    // Bubble
    if (this.bubble && !dimmed) {
      this.bubble.render(ctx, this.x, this.y - 40);
    }

    // Particles
    this.particles.render(ctx);
  }

  // Set by main.js when a conversation is selected
  set dimMode(v) { this._dimMode = v; }
  get dimMode() { return this._dimMode; }
}
