import { SpriteGenerator, PALETTE_COLORS, ACTIVITY_GLOW, ACTIVITY_FRAME_SPEED } from "./sprite.js";
import { Bubble } from "./bubble.js";
import { ParticleEmitter } from "./particles.js";
import { spaceAssets, nameHash } from "./space-assets.js";
import { roboSprite } from "./robo-sprite.js";
import { mechSprite } from "./mech-sprite.js";

// Message type → color for arrival bursts
const ARRIVAL_COLORS = {
  question:     "#ffd740",
  response:     "#55efc4",
  notification: "#a29bfe",
  task:         "#ff7675",
  default:      "#74b9ff",
};

const SPRITE_SIZE = 64; // 32px * 2 scale
const HALF = SPRITE_SIZE / 2;

export class AgentView {
  constructor(name, role, description, paletteIndex, online, project) {
    this.name = name;
    this.role = role;
    this.description = description;
    this.paletteIndex = paletteIndex;
    this.online = online;
    this.project = project || "default";
    this.isExecutive = false;
    this.currentTaskLabel = null;
    this.isBlocked = false;
    this._teams = [];
    this.fileLocks = []; // active file locks for this agent

    this.x = 0;
    this.y = 0;
    this.targetX = 0;
    this.targetY = 0;

    this.spawnAlpha = 0;
    this.spawning = true;
    this.frameIndex = 0;
    this.frameTimer = 0;
    this.highlighted = false;

    // New sprite system: archetype + golden
    const spriteData = SpriteGenerator.generate(paletteIndex, name);
    this.frames = spriteData.frames;
    this.archetype = spriteData.archetype;
    this.isGolden = spriteData.isGolden;
    this.color = PALETTE_COLORS[spriteData.paletteIndex % PALETTE_COLORS.length];

    this.bubble = null;
    this.particles = new ParticleEmitter();

    // Presence & aura
    this.glowPhase = Math.random() * Math.PI * 2;
    this.breathPhase = Math.random() * Math.PI * 2;
    this.hovered = false;
    this.selected = false;       // Canvas selection ring
    this._selectPhase = 0;       // Animated rotation for selection ring
    this.ripples = [];
    this._blockedPhase = 0;
    this._workingPhase = 0;
    this._shakeX = 0;
    this.sleeping = false;
    this._sleepPhase = 0;

    // Orbital animation (used in global view)
    this.orbit = null; // { cx, cy, rx, ry, angle, speed, tilt }

    // Animated planet (48x48, 60 frames) — planetType set from API data
    this.planetType = null; // e.g. "terran/1", set by main.js from agent API
    this.solarPlanet = null; // e.g. "earth", set by layoutAgents in focused view
    this._planetFrameIndex = nameHash(name) % 60; // offset so planets don't all sync
    this._planetFrameTimer = 0;
    this._planetSpeed = 0.8 + (nameHash(name) % 50) / 100; // 0.8-1.3s per frame
    this._solarFrameIndex = nameHash(name) % 8;
    this._solarFrameTimer = 0;
    this._solarFrameSpeed = 0.12 + (nameHash(name) % 40) / 1000;

    // Colony mode (project focused view)
    this.colony = false;
    this._roboAnim = "idle";
    this._roboFrame = 0;
    this._roboTimer = 0;
    this._roboFlip = nameHash(name) % 2 === 0; // random face direction
    this._mechColor = mechSprite.colorForAgent(name);
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
    const r = this.minimal ? 12 : HALF;
    return Math.abs(px - this.x) < r && Math.abs(py - this.y) < r;
  }

  spawnEffect() {
    this.particles.emit("spawn", this.x, this.y + 24);
  }

  triggerRipple(color) {
    this.ripples.push({ radius: 12, life: 0.7, maxLife: 0.7, color: color || this.color });
  }

  arrivalBurst(msgType) {
    const color = ARRIVAL_COLORS[msgType] || ARRIVAL_COLORS.default;
    this.particles.emit("arrival", this.x, this.y, color);
    this.triggerRipple(color);
  }

  update(dt) {
    // Animated planet frame advance (48x48 pool)
    this._planetFrameTimer += dt;
    if (this._planetFrameTimer >= this._planetSpeed) {
      this._planetFrameTimer = 0;
      this._planetFrameIndex = (this._planetFrameIndex + 1) % 60;
    }
    // Solar system planet frame advance
    this._solarFrameTimer += dt;
    if (this._solarFrameTimer >= this._solarFrameSpeed) {
      this._solarFrameTimer = 0;
      this._solarFrameIndex = (this._solarFrameIndex + 1) % 8;
    }

    // Orbital animation (moon mode)
    if (this.orbit) {
      this.orbit.angle += this.orbit.speed * dt;
      this.targetX = this.orbit.cx + Math.cos(this.orbit.angle) * this.orbit.rx;
      this.targetY = this.orbit.cy + Math.sin(this.orbit.angle) * this.orbit.ry;
      // Y-sort: agents behind the planet (top half) are smaller/dimmer
      this._orbitDepth = Math.sin(this.orbit.angle); // -1 (behind) to +1 (front)
    } else {
      this._orbitDepth = 0;
    }

    // Smooth position lerp
    this.x += (this.targetX - this.x) * 4 * dt;
    this.y += (this.targetY - this.y) * 4 * dt;

    // Spawn fade-in
    if (this.spawning) {
      this.spawnAlpha = Math.min(1, this.spawnAlpha + dt * 3);
      if (this.spawnAlpha >= 1) this.spawning = false;
    }

    // Animation speed: activity-driven, or fallback to task/blocked/sleeping
    const activitySpeed = this.activity ? (ACTIVITY_FRAME_SPEED[this.activity] || 0.5) : null;
    const animSpeed = this.sleeping ? 2.0 : activitySpeed || (this.isBlocked ? 1.0 : (this.currentTaskLabel ? 0.3 : 0.5));
    this.frameTimer += dt;
    if (this.frameTimer > animSpeed) {
      this.frameTimer = 0;
      this.frameIndex = (this.frameIndex + 1) % this.frames.length;
    }

    // Presence phases
    this.glowPhase += dt * 2;
    this.breathPhase += dt * (this.sleeping ? 0.6 : 1.5);
    if (this.sleeping) this._sleepPhase += dt;
    if (this.isBlocked) {
      this._blockedPhase += dt * 4;
      this._shakeX = Math.sin(this._blockedPhase * 8) * 2; // horizontal shake
    } else {
      this._shakeX *= 0.9; // decay shake
    }
    if (this.currentTaskLabel && !this.isBlocked) {
      this._workingPhase += dt * 3;
    }
    // Activity phase for ring animation (no particles — clean)
    if (this.activity && this.activity !== "idle") {
      this._activityPhase = (this._activityPhase || 0) + dt * 3;
    }
    // Selection ring rotation
    if (this.selected) {
      this._selectPhase += dt * 1.2;
    }

    // Ripples
    for (let i = this.ripples.length - 1; i >= 0; i--) {
      this.ripples[i].life -= dt;
      this.ripples[i].radius += dt * 90;
      if (this.ripples[i].life <= 0) this.ripples.splice(i, 1);
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

    // --- Colony mode: mech character in project view ---
    if (this.colony) {
      const dx = this.x;
      const dy = this.y;

      // Advance mech idle animation
      this._roboTimer += 0.016; // ~60fps
      if (this._roboTimer >= 0.12) { // 120ms per frame
        this._roboTimer = 0;
        this._roboFrame++;
      }

      ctx.save();
      ctx.globalAlpha = dimmed ? alpha * 0.25 : alpha;
      ctx.imageSmoothingEnabled = false;

      // --- Executive aura (golden halo) ---
      if (this.isExecutive && !this.isGolden && !dimmed) {
        const auraPhase = this.glowPhase * 0.8;
        const baseAlpha = 0.25 + 0.1 * Math.sin(auraPhase);
        const outerR = 44 + 3 * Math.sin(auraPhase * 0.7);
        const halo = ctx.createRadialGradient(dx, dy, 20, dx, dy, outerR);
        halo.addColorStop(0, `rgba(255, 215, 0, ${baseAlpha * 0.5})`);
        halo.addColorStop(0.5, `rgba(255, 200, 0, ${baseAlpha * 0.2})`);
        halo.addColorStop(1, "rgba(255, 170, 0, 0)");
        ctx.fillStyle = halo;
        ctx.beginPath();
        ctx.arc(dx, dy, outerR, 0, Math.PI * 2);
        ctx.fill();
        for (let i = 0; i < 3; i++) {
          const sa = auraPhase * 1.2 + (i * Math.PI * 2 / 3);
          const sr = 36 + 2 * Math.sin(auraPhase * 2 + i);
          const sparkA = 0.5 + 0.4 * Math.sin(auraPhase * 3 + i * 2);
          ctx.fillStyle = `rgba(255, 230, 100, ${sparkA})`;
          ctx.beginPath();
          ctx.arc(dx + Math.cos(sa) * sr, dy + Math.sin(sa) * sr * 0.6, 1.5, 0, Math.PI * 2);
          ctx.fill();
        }
      }

      // --- Golden agent aura ---
      if (this.isGolden && !dimmed) {
        const goldenPhase = this.glowPhase * 0.6;
        const goldenAlpha = 0.3 + 0.15 * Math.sin(goldenPhase);
        const goldenR = 48 + 4 * Math.sin(goldenPhase * 0.5);
        const goldenGrad = ctx.createRadialGradient(dx, dy, 10, dx, dy, goldenR);
        goldenGrad.addColorStop(0, `rgba(255, 215, 0, ${goldenAlpha * 0.7})`);
        goldenGrad.addColorStop(0.3, `rgba(255, 200, 0, ${goldenAlpha * 0.4})`);
        goldenGrad.addColorStop(0.6, `rgba(255, 170, 0, ${goldenAlpha * 0.15})`);
        goldenGrad.addColorStop(1, "rgba(255, 170, 0, 0)");
        ctx.fillStyle = goldenGrad;
        ctx.beginPath();
        ctx.arc(dx, dy, goldenR, 0, Math.PI * 2);
        ctx.fill();
        for (let i = 0; i < 5; i++) {
          const sa = goldenPhase * 1.5 + (i * Math.PI * 2 / 5);
          const sr = 38 + 3 * Math.sin(goldenPhase * 2 + i);
          const sparkA = 0.5 + 0.45 * Math.sin(goldenPhase * 4 + i * 1.7);
          ctx.fillStyle = `rgba(255, 240, 120, ${sparkA})`;
          ctx.beginPath();
          ctx.arc(dx + Math.cos(sa) * sr, dy + Math.sin(sa) * sr * 0.55, 1.8, 0, Math.PI * 2);
          ctx.fill();
        }
      }

      // --- Activity ring ---
      if (this.activity && this.activity !== "idle" && !dimmed && !this.sleeping) {
        const glowColor = ACTIVITY_GLOW[this.activity];
        if (glowColor) {
          const phase = this._activityPhase || 0;
          const pulseAlpha = 0.3 + 0.2 * Math.sin(phase);
          ctx.strokeStyle = glowColor;
          ctx.globalAlpha = pulseAlpha;
          ctx.lineWidth = 1;
          ctx.beginPath();
          ctx.arc(dx, dy, 40, 0, Math.PI * 2);
          ctx.stroke();
          ctx.globalAlpha = alpha;
        }
      }

      // Working green glow
      if (this.currentTaskLabel && !this.isBlocked && !dimmed) {
        const workAlpha = 0.06 + 0.03 * Math.sin(this._workingPhase);
        const grad = ctx.createRadialGradient(dx, dy, 10, dx, dy, 44);
        grad.addColorStop(0, `rgba(0, 230, 118, ${workAlpha})`);
        grad.addColorStop(1, "rgba(0, 230, 118, 0)");
        ctx.fillStyle = grad;
        ctx.beginPath();
        ctx.arc(dx, dy, 44, 0, Math.PI * 2);
        ctx.fill();
      }

      // Blocked red glow
      if (this.isBlocked && !dimmed) {
        const bAlpha = 0.15 + 0.1 * Math.sin(this._blockedPhase);
        const grad = ctx.createRadialGradient(dx, dy, 10, dx, dy, 44);
        grad.addColorStop(0, `rgba(255,107,107,${bAlpha})`);
        grad.addColorStop(1, "rgba(255,107,107,0)");
        ctx.fillStyle = grad;
        ctx.beginPath();
        ctx.arc(dx, dy, 44, 0, Math.PI * 2);
        ctx.fill();
      }

      // Shadow under mech
      const shadowImg = mechSprite.getShadow(3);
      if (shadowImg) {
        ctx.globalAlpha = alpha * 0.4;
        ctx.drawImage(shadowImg, dx - 20, dy + 12, 40, 12);
        ctx.globalAlpha = alpha;
      } else {
        ctx.fillStyle = "rgba(0,0,0,0.25)";
        ctx.beginPath();
        ctx.ellipse(dx, dy + 16, 18, 5, 0, 0, Math.PI * 2);
        ctx.fill();
      }

      // --- Selection ring (game-style rotating dashed circle) ---
      if (this.selected && !dimmed) {
        const selR = 38;
        const pulse = 0.6 + 0.4 * Math.sin(this._selectPhase * 2.5);
        const segments = 8;
        const gap = Math.PI / 12;

        ctx.save();
        ctx.translate(dx, dy - 2);
        ctx.rotate(this._selectPhase * 0.5);

        // Outer glow
        ctx.strokeStyle = `rgba(108, 92, 231, ${0.25 * pulse})`;
        ctx.lineWidth = 5;
        ctx.beginPath();
        ctx.arc(0, 0, selR + 2, 0, Math.PI * 2);
        ctx.stroke();

        // Main dashed ring
        ctx.lineWidth = 2;
        for (let i = 0; i < segments; i++) {
          const a0 = (i / segments) * Math.PI * 2 + gap / 2;
          const a1 = ((i + 1) / segments) * Math.PI * 2 - gap / 2;
          ctx.strokeStyle = i % 2 === 0
            ? `rgba(162, 155, 254, ${0.9 * pulse})`
            : `rgba(108, 92, 231, ${0.6 * pulse})`;
          ctx.beginPath();
          ctx.arc(0, 0, selR, a0, a1);
          ctx.stroke();
        }

        // Corner chevrons (4 small arrows at cardinal points)
        ctx.strokeStyle = `rgba(162, 155, 254, ${0.8 * pulse})`;
        ctx.lineWidth = 1.5;
        for (let i = 0; i < 4; i++) {
          const angle = (i / 4) * Math.PI * 2;
          const cx = Math.cos(angle) * (selR + 8);
          const cy = Math.sin(angle) * (selR + 8);
          const inward = angle + Math.PI;
          const chevLen = 4;
          ctx.beginPath();
          ctx.moveTo(cx + Math.cos(inward + 0.5) * chevLen, cy + Math.sin(inward + 0.5) * chevLen);
          ctx.lineTo(cx, cy);
          ctx.lineTo(cx + Math.cos(inward - 0.5) * chevLen, cy + Math.sin(inward - 0.5) * chevLen);
          ctx.stroke();
        }

        ctx.restore();
      }

      // Draw mech sprite
      const mechFrame = mechSprite.getIdleFrame(this._mechColor, this._roboFrame);
      const size = this.hovered ? 80 : 64;
      if (this.sleeping) ctx.globalAlpha = alpha * 0.5;
      if (mechFrame) {
        if (this._roboFlip) {
          ctx.save();
          ctx.scale(-1, 1);
          ctx.drawImage(mechFrame, -dx - size / 2, dy - size / 2 - 8, size, size);
          ctx.restore();
        } else {
          ctx.drawImage(mechFrame, dx - size / 2, dy - size / 2 - 8, size, size);
        }
      }
      ctx.globalAlpha = alpha;

      // --- Sleeping Zzz ---
      if (this.sleeping && !dimmed) {
        const t = this._sleepPhase;
        for (let i = 0; i < 3; i++) {
          const phase = t * 0.8 + i * 0.7;
          const rise = (phase % 3) / 3;
          const zx = dx + 20 + i * 6;
          const zy = dy - 20 - rise * 30;
          const za = (1 - rise) * 0.8;
          const zSize = 8 + i * 3;
          ctx.font = `bold ${zSize}px 'JetBrains Mono', monospace`;
          ctx.textAlign = "center";
          ctx.fillStyle = `rgba(150, 130, 255, ${za})`;
          ctx.fillText("z", zx, zy);
        }
      }

      // --- Status dot (activity-aware) ---
      const dotX = dx + size / 2 - 6;
      const dotY = dy - size / 2 - 6;
      ctx.beginPath();
      ctx.arc(dotX, dotY, 4, 0, Math.PI * 2);
      const actColor = this.activity && this.activity !== "idle" ? ACTIVITY_GLOW[this.activity] : null;
      if (this.sleeping) {
        ctx.fillStyle = "#9b59b6"; ctx.shadowColor = "#9b59b6"; ctx.shadowBlur = 4;
      } else if (actColor) {
        ctx.fillStyle = actColor; ctx.shadowColor = actColor; ctx.shadowBlur = 5;
      } else if (this.online) {
        ctx.fillStyle = "#00e676"; ctx.shadowColor = "#00e676"; ctx.shadowBlur = 4;
      } else {
        ctx.fillStyle = "#636e72"; ctx.shadowBlur = 0;
      }
      ctx.fill();
      ctx.shadowBlur = 0;

      // --- Name tag + team dots ---
      const labelY = dy + size / 2 - 2;
      ctx.font = `bold 10px 'JetBrains Mono', monospace`;
      ctx.textAlign = "center";
      const displayName = this.isGolden ? `★ ${this.name}` : this.name;
      ctx.fillStyle = this.isGolden ? "#ffd700" : (this.hovered ? "#fff" : "rgba(220,220,230,0.85)");
      ctx.fillText(displayName, dx, labelY);

      // Team dots
      const TEAM_DOT_COLORS = { admin: "#ffd700", regular: "#00FFFF", bot: "#00FF88" };
      if (this._teams && this._teams.length > 0 && !dimmed) {
        const nameW = ctx.measureText(displayName).width;
        let dotStartX = dx + nameW / 2 + 5;
        for (let i = 0; i < Math.min(this._teams.length, 3); i++) {
          const dotColor = TEAM_DOT_COLORS[this._teams[i].type] || TEAM_DOT_COLORS.regular;
          ctx.fillStyle = dotColor;
          ctx.globalAlpha = 0.7;
          ctx.beginPath();
          ctx.arc(dotStartX + i * 7, labelY - 4, 2.5, 0, Math.PI * 2);
          ctx.fill();
        }
        ctx.globalAlpha = alpha;
      }

      // --- Hover detail panel ---
      if (this.hovered && !dimmed) {
        let infoY = labelY + 12;
        ctx.textAlign = "center";

        if (this.role) {
          ctx.font = "9px 'JetBrains Mono', monospace";
          ctx.fillStyle = "rgba(224,224,232,0.6)";
          ctx.globalAlpha = 1;
          const shortRole = this.role.length > 35 ? this.role.slice(0, 33) + "..." : this.role;
          ctx.fillText(shortRole, dx, infoY);
          infoY += 11;
        }
        if (this.activity && this.activity !== "idle") {
          const actLabel = this.activityTool || this.activity;
          ctx.font = "9px 'JetBrains Mono', monospace";
          ctx.fillStyle = ACTIVITY_GLOW[this.activity] || "#888";
          ctx.globalAlpha = 0.7;
          ctx.fillText(actLabel, dx, infoY);
          infoY += 11;
        }
        if (this._teams && this._teams.length > 0) {
          ctx.font = "9px 'JetBrains Mono', monospace";
          const teamStr = this._teams.map(t => t.name).join(" · ");
          const shortTeams = teamStr.length > 35 ? teamStr.slice(0, 33) + "..." : teamStr;
          ctx.fillStyle = "rgba(0, 255, 255, 0.45)";
          ctx.globalAlpha = 1;
          ctx.fillText(shortTeams, dx, infoY);
          infoY += 11;
        }
        if (this.currentTaskLabel) {
          ctx.font = "9px 'JetBrains Mono', monospace";
          const taskLabel = this.currentTaskLabel.length > 30
            ? this.currentTaskLabel.slice(0, 28) + "..." : this.currentTaskLabel;
          ctx.fillStyle = this.isBlocked ? "rgba(255, 107, 107, 0.7)" : "rgba(0, 230, 118, 0.55)";
          ctx.fillText(taskLabel, dx, infoY);
        }
      } else if (!dimmed && this.currentTaskLabel) {
        ctx.font = "9px 'JetBrains Mono', monospace";
        ctx.textAlign = "center";
        const taskLabel = this.currentTaskLabel.length > 22
          ? this.currentTaskLabel.slice(0, 20) + "..." : this.currentTaskLabel;
        ctx.fillStyle = this.isBlocked ? "rgba(255, 107, 107, 0.5)" : "rgba(0, 230, 118, 0.4)";
        ctx.globalAlpha = 0.8;
        ctx.fillText(taskLabel, dx, labelY + 12);
      }

      ctx.restore();

      // Bubble
      if (this.bubble && !dimmed) this.bubble.render(ctx, dx, dy - size / 2 - 20);

      // Particles
      this.particles.render(ctx);

      return;
    }

    // --- Minimal mode: orbiting planet sprite with 3D depth ---
    if (this.minimal) {
      const depth = this._orbitDepth || 0;
      const depthScale = 0.65 + 0.35 * ((depth + 1) / 2); // 0.65 to 1.0
      const depthAlpha = 0.45 + 0.55 * ((depth + 1) / 2); // 0.45 to 1.0

      const dx = this.x, dy = this.y;

      // In focused view use solar planet, in global view use animated 48x48
      const planetImg = this.solarPlanet
        ? spaceAssets.getSolarPlanetFrame(this.solarPlanet, this._solarFrameIndex)
        : spaceAssets.getPlanetFrame(this.planetType || spaceAssets.fallbackPlanetType(this.name), this._planetFrameIndex);
      const baseSize = this.solarPlanet
        ? (this.hovered ? 56 : 42)   // solar planets bigger in focused view
        : (this.hovered ? 36 : 24);  // animated planets smaller in global view
      const size = baseSize * depthScale;

      ctx.save();
      ctx.imageSmoothingEnabled = false;
      ctx.globalAlpha = (dimmed ? 0.15 : alpha) * depthAlpha;

      // Subtle glow behind planet
      const glowR = size * 1.2;
      const actColor = this.activity && this.activity !== "idle" ? ACTIVITY_GLOW[this.activity] : null;
      const glowColor = actColor || (this.isBlocked ? "#ff6b6b" : (this.online ? this.color : "#444"));
      const glowHex = glowColor.startsWith("#") ? glowColor : "#888";
      const grad = ctx.createRadialGradient(dx, dy, size * 0.3, dx, dy, glowR);
      grad.addColorStop(0, this._rgba(glowHex, 0.25));
      grad.addColorStop(1, this._rgba(glowHex, 0));
      ctx.fillStyle = grad;
      ctx.beginPath();
      ctx.arc(dx, dy, glowR, 0, Math.PI * 2);
      ctx.fill();

      // Animated planet sprite
      if (planetImg) {
        ctx.drawImage(planetImg, dx - size / 2, dy - size / 2, size, size);
      } else {
        // Fallback dot while loading
        ctx.fillStyle = glowColor;
        ctx.beginPath();
        ctx.arc(dx, dy, size / 3, 0, Math.PI * 2);
        ctx.fill();
      }

      // Show name only on hover (global view) or always in solar/focused view
      if (this.hovered || this.solarPlanet) {
      ctx.globalAlpha = this.hovered ? 0.95 : 0.7;
      ctx.font = `bold ${this.solarPlanet ? 11 : 9}px 'JetBrains Mono', monospace`;
      ctx.textAlign = "center";
      ctx.fillStyle = "#e0e0e8";
      ctx.fillText(this.name, dx, dy + size / 2 + 14);
      if (this.hovered) {
          if (this.solarPlanet) {
            ctx.font = "9px 'JetBrains Mono', monospace";
            ctx.fillStyle = "rgba(255,200,60,0.7)";
            ctx.fillText(this.solarPlanet.toUpperCase(), dx, dy + size / 2 + 26);
          }
          if (this.role) {
            ctx.font = "9px 'JetBrains Mono', monospace";
            ctx.fillStyle = "rgba(224,224,232,0.5)";
            const shortRole = this.role.length > 30 ? this.role.slice(0, 28) + "..." : this.role;
            ctx.fillText(shortRole, dx, dy + size / 2 + (this.solarPlanet ? 38 : 26));
          }
        }
      } // end name label if

      ctx.restore();
      return;
    }

    // Breathing offset + blocked shake
    const breathY = Math.sin(this.breathPhase) * 2.5;
    const drawX = this.x + this._shakeX;
    const drawY = this.y + breathY;

    ctx.save();

    // --- Golden agent: outer luminescent aura ---
    if (this.isGolden && !dimmed) {
      const goldenPhase = this.glowPhase * 0.6;
      const goldenAlpha = 0.3 + 0.15 * Math.sin(goldenPhase);
      const goldenR = 44 + 4 * Math.sin(goldenPhase * 0.5);

      const goldenGrad = ctx.createRadialGradient(drawX, drawY, 10, drawX, drawY, goldenR);
      goldenGrad.addColorStop(0, `rgba(255, 215, 0, ${goldenAlpha * 0.7})`);
      goldenGrad.addColorStop(0.3, `rgba(255, 200, 0, ${goldenAlpha * 0.4})`);
      goldenGrad.addColorStop(0.6, `rgba(255, 170, 0, ${goldenAlpha * 0.15})`);
      goldenGrad.addColorStop(1, "rgba(255, 170, 0, 0)");
      ctx.fillStyle = goldenGrad;
      ctx.beginPath();
      ctx.arc(drawX, drawY, goldenR, 0, Math.PI * 2);
      ctx.fill();

      // Orbiting golden sparkles (5 particles)
      for (let i = 0; i < 5; i++) {
        const sa = goldenPhase * 1.5 + (i * Math.PI * 2 / 5);
        const sr = 34 + 3 * Math.sin(goldenPhase * 2 + i);
        const sx = drawX + Math.cos(sa) * sr;
        const sy = drawY + Math.sin(sa) * sr * 0.55;
        const sparkA = 0.5 + 0.45 * Math.sin(goldenPhase * 4 + i * 1.7);
        ctx.fillStyle = `rgba(255, 240, 120, ${sparkA})`;
        ctx.beginPath();
        ctx.arc(sx, sy, 1.8, 0, Math.PI * 2);
        ctx.fill();
      }
    }

    // --- Glow ring (online agents, subtle) ---
    if (this.online && !dimmed && !this.isGolden && !this.activity) {
      const basePulse = 0.12 + 0.06 * Math.sin(this.glowPhase);
      const hoverBoost = this.hovered ? 0.15 : 0;
      const intensity = basePulse + hoverBoost;
      const glowRadius = 34 + 2 * Math.sin(this.glowPhase * 0.7);

      const grad = ctx.createRadialGradient(drawX, drawY, 12, drawX, drawY, glowRadius);
      grad.addColorStop(0, this._rgba(this.color, intensity * 0.4));
      grad.addColorStop(0.6, this._rgba(this.color, intensity * 0.08));
      grad.addColorStop(1, this._rgba(this.color, 0));
      ctx.fillStyle = grad;
      ctx.beginPath();
      ctx.arc(drawX, drawY, glowRadius, 0, Math.PI * 2);
      ctx.fill();
    }

    // --- Ripples ---
    for (const ripple of this.ripples) {
      const progress = 1 - ripple.life / ripple.maxLife;
      const rippleAlpha = (1 - progress) * 0.5;
      ctx.globalAlpha = rippleAlpha * alpha;
      ctx.strokeStyle = ripple.color;
      ctx.lineWidth = 2 * (1 - progress);
      ctx.beginPath();
      ctx.arc(drawX, drawY, ripple.radius, 0, Math.PI * 2);
      ctx.stroke();
    }

    ctx.globalAlpha = dimmed ? alpha * 0.3 : alpha;

    // --- Working state: green tint underlay ---
    if (this.currentTaskLabel && !this.isBlocked && !dimmed) {
      const workAlpha = 0.08 + 0.04 * Math.sin(this._workingPhase);
      const workGrad = ctx.createRadialGradient(drawX, drawY, 8, drawX, drawY, 38);
      workGrad.addColorStop(0, `rgba(0, 230, 118, ${workAlpha})`);
      workGrad.addColorStop(1, "rgba(0, 230, 118, 0)");
      ctx.fillStyle = workGrad;
      ctx.beginPath();
      ctx.arc(drawX, drawY, 38, 0, Math.PI * 2);
      ctx.fill();
    }

    // --- Activity ring (thin, tight around sprite) ---
    if (this.activity && this.activity !== "idle" && !dimmed && !this.sleeping) {
      const glowColor = ACTIVITY_GLOW[this.activity];
      if (glowColor) {
        const phase = this._activityPhase || 0;
        const pulseAlpha = 0.3 + 0.2 * Math.sin(phase);
        ctx.strokeStyle = glowColor;
        ctx.globalAlpha = pulseAlpha;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.arc(drawX, drawY, HALF + 8, 0, Math.PI * 2);
        ctx.stroke();
        ctx.globalAlpha = dimmed ? 0.3 : alpha;
      }
    }

    // --- Animated planet sprite (48x48 rendered at 64x64) ---
    const planetImg = spaceAssets.getPlanetFrame(this.planetType || spaceAssets.fallbackPlanetType(this.name), this._planetFrameIndex);
    if (planetImg) {
      if (this.sleeping) ctx.globalAlpha *= 0.5;
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(planetImg, drawX - HALF, drawY - HALF, SPRITE_SIZE, SPRITE_SIZE);
      if (this.sleeping) ctx.globalAlpha = dimmed ? 0.3 : alpha;
    } else {
      // Fallback to old sprite while loading
      const frame = this.frames[this.frameIndex];
      if (frame) {
        if (this.sleeping) ctx.globalAlpha *= 0.5;
        ctx.drawImage(frame, drawX - HALF, drawY - HALF, SPRITE_SIZE, SPRITE_SIZE);
        if (this.sleeping) ctx.globalAlpha = dimmed ? 0.3 : alpha;
      }
    }

    ctx.globalAlpha = dimmed ? 0.3 : 1;

    // --- Sleeping Zzz ---
    if (this.sleeping && !dimmed) {
      const t = this._sleepPhase;
      ctx.font = "bold 14px 'JetBrains Mono', monospace";
      ctx.textAlign = "center";
      for (let i = 0; i < 3; i++) {
        const phase = t * 0.8 + i * 0.7;
        const rise = (phase % 3) / 3; // 0→1 cycle
        const zx = drawX + 20 + i * 6;
        const zy = drawY - 20 - rise * 30;
        const za = (1 - rise) * 0.8;
        const size = 8 + i * 3;
        ctx.font = `bold ${size}px 'JetBrains Mono', monospace`;
        ctx.fillStyle = `rgba(150, 130, 255, ${za})`;
        ctx.fillText("z", zx, zy);
      }
    }

    // --- Blocked agent glow ---
    if (this.isBlocked && !dimmed) {
      const blockedPulse = 0.3 + 0.2 * Math.sin(this._blockedPhase);
      const blockedGrad = ctx.createRadialGradient(drawX, drawY, 10, drawX, drawY, 44);
      blockedGrad.addColorStop(0, `rgba(255,107,107,${blockedPulse})`);
      blockedGrad.addColorStop(1, `rgba(255,107,107,0)`);
      ctx.fillStyle = blockedGrad;
      ctx.beginPath();
      ctx.arc(drawX, drawY, 44, 0, Math.PI * 2);
      ctx.fill();
    }

    // --- Executive aura (luminescent golden halo) ---
    if (this.isExecutive && !this.isGolden && !dimmed) {
      const auraPhase = this.glowPhase * 0.8;
      const baseAlpha = 0.25 + 0.1 * Math.sin(auraPhase);
      const outerR = 40 + 3 * Math.sin(auraPhase * 0.7);
      const innerR = 20;

      const halo = ctx.createRadialGradient(drawX, drawY, innerR, drawX, drawY, outerR);
      halo.addColorStop(0, `rgba(255, 215, 0, ${baseAlpha * 0.5})`);
      halo.addColorStop(0.5, `rgba(255, 200, 0, ${baseAlpha * 0.2})`);
      halo.addColorStop(1, "rgba(255, 170, 0, 0)");
      ctx.fillStyle = halo;
      ctx.beginPath();
      ctx.arc(drawX, drawY, outerR, 0, Math.PI * 2);
      ctx.fill();

      for (let i = 0; i < 3; i++) {
        const sa = auraPhase * 1.2 + (i * Math.PI * 2 / 3);
        const sr = 32 + 2 * Math.sin(auraPhase * 2 + i);
        const sx = drawX + Math.cos(sa) * sr;
        const sy = drawY + Math.sin(sa) * sr * 0.6;
        const sparkA = 0.5 + 0.4 * Math.sin(auraPhase * 3 + i * 2);
        ctx.fillStyle = `rgba(255, 230, 100, ${sparkA})`;
        ctx.beginPath();
        ctx.arc(sx, sy, 1.5, 0, Math.PI * 2);
        ctx.fill();
      }
    }

    // --- Status dot (activity-aware) ---
    const dotX = drawX + 26;
    const dotY = drawY - 24;
    ctx.beginPath();
    ctx.arc(dotX, dotY, 4, 0, Math.PI * 2);
    const actColor = this.activity && this.activity !== "idle" ? ACTIVITY_GLOW[this.activity] : null;
    if (this.sleeping) {
      ctx.fillStyle = "#9b59b6";
      ctx.shadowColor = "#9b59b6";
      ctx.shadowBlur = 4;
    } else if (actColor) {
      ctx.fillStyle = actColor;
      ctx.shadowColor = actColor;
      ctx.shadowBlur = 5;
    } else if (this.online) {
      ctx.fillStyle = "#00e676";
      ctx.shadowColor = "#00e676";
      ctx.shadowBlur = 4;
    } else {
      ctx.fillStyle = "#636e72";
      ctx.shadowBlur = 0;
    }
    ctx.fill();
    ctx.shadowBlur = 0;

    // --- Name tag + team dots (always visible) ---
    const labelY = drawY + 40;
    const nameAlpha = this.hovered ? 1 : 0.85;
    ctx.font = "bold 12px 'JetBrains Mono', monospace";
    ctx.textAlign = "center";
    ctx.fillStyle = this.isGolden ? "#ffd700" : this.color;
    ctx.globalAlpha = (dimmed ? 0.3 : nameAlpha);

    const displayName = this.isGolden ? `★ ${this.name}` : this.name;
    ctx.fillText(displayName, drawX, labelY);

    // Team dots (small colored circles next to name)
    const TEAM_DOT_COLORS = { admin: "#ffd700", regular: "#00FFFF", bot: "#00FF88" };
    if (this._teams && this._teams.length > 0 && !dimmed) {
      const nameW = ctx.measureText(displayName).width;
      let dotStartX = drawX + nameW / 2 + 6;
      for (let i = 0; i < Math.min(this._teams.length, 3); i++) {
        const dotColor = TEAM_DOT_COLORS[this._teams[i].type] || TEAM_DOT_COLORS.regular;
        ctx.fillStyle = dotColor;
        ctx.globalAlpha = 0.7;
        ctx.beginPath();
        ctx.arc(dotStartX + i * 7, labelY - 4, 2.5, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.globalAlpha = dimmed ? 0.3 : 1;
    }

    // --- Hover detail panel (role + activity + task) ---
    if (this.hovered && !dimmed) {
      let detailY = labelY + 13;
      ctx.textAlign = "center";

      // Role
      if (this.role) {
        ctx.font = "10px 'JetBrains Mono', monospace";
        ctx.fillStyle = "rgba(224,224,232,0.6)";
        ctx.globalAlpha = 1;
        const shortRole = this.role.length > 28 ? this.role.slice(0, 26) + "..." : this.role;
        ctx.fillText(shortRole, drawX, detailY);
        detailY += 12;
      }

      // Activity
      if (this.activity && this.activity !== "idle") {
        const actLabel = this.activityTool || this.activity;
        ctx.font = "9px 'JetBrains Mono', monospace";
        ctx.fillStyle = ACTIVITY_GLOW[this.activity] || "#888";
        ctx.globalAlpha = 0.7;
        ctx.fillText(actLabel, drawX, detailY);
        detailY += 11;
      }

      // Team names (compact list)
      if (this._teams && this._teams.length > 0) {
        ctx.font = "9px 'JetBrains Mono', monospace";
        const teamStr = this._teams.map(t => t.name).join(" · ");
        const shortTeams = teamStr.length > 35 ? teamStr.slice(0, 33) + "..." : teamStr;
        ctx.fillStyle = "rgba(0, 255, 255, 0.45)";
        ctx.globalAlpha = 1;
        ctx.fillText(shortTeams, drawX, detailY);
        detailY += 11;
      }

      // Project tag (only multi-project)
      if (this.showProjectTag && this.project) {
        ctx.font = "9px 'JetBrains Mono', monospace";
        ctx.fillStyle = "rgba(108, 92, 231, 0.5)";
        ctx.fillText(this.project, drawX, detailY);
        detailY += 11;
      }

      // Current task
      if (this.currentTaskLabel) {
        ctx.font = "9px 'JetBrains Mono', monospace";
        const taskLabel = this.currentTaskLabel.length > 30
          ? this.currentTaskLabel.slice(0, 28) + "..."
          : this.currentTaskLabel;
        ctx.fillStyle = this.isBlocked ? "rgba(255, 107, 107, 0.7)" : "rgba(0, 230, 118, 0.55)";
        ctx.fillText(taskLabel, drawX, detailY);
      }
    } else if (!dimmed && this.currentTaskLabel) {
      // Non-hovered: show task label only (compact, one line under name)
      ctx.font = "9px 'JetBrains Mono', monospace";
      ctx.textAlign = "center";
      const taskLabel = this.currentTaskLabel.length > 22
        ? this.currentTaskLabel.slice(0, 20) + "..."
        : this.currentTaskLabel;
      ctx.fillStyle = this.isBlocked ? "rgba(255, 107, 107, 0.5)" : "rgba(0, 230, 118, 0.4)";
      ctx.globalAlpha = 0.8;
      ctx.fillText(taskLabel, drawX, labelY + 12);
    }

    // --- File lock icon ---
    if (this.fileLocks.length > 0 && !dimmed) {
      const lockX = drawX + HALF - 4;
      const lockY = drawY - HALF + 4;
      // Lock body
      ctx.globalAlpha = 0.85;
      ctx.fillStyle = "#ffd700";
      ctx.fillRect(lockX - 4, lockY, 8, 6);
      // Lock shackle
      ctx.strokeStyle = "#ffd700";
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.arc(lockX, lockY, 3, Math.PI, 0);
      ctx.stroke();
      // Count badge
      if (this.fileLocks.length > 1) {
        ctx.font = "bold 7px 'JetBrains Mono', monospace";
        ctx.fillStyle = "#000";
        ctx.textAlign = "center";
        ctx.fillText(String(this.fileLocks.length), lockX, lockY + 5);
      }
      ctx.globalAlpha = 1;
    }

    ctx.restore();

    // --- Bubble ---
    if (this.bubble && !dimmed) {
      this.bubble.render(ctx, drawX, drawY - 44);
    }

    // --- Particles ---
    this.particles.render(ctx);
  }

  _rgba(hex, a) {
    let h = hex.startsWith("#") ? hex.slice(1) : hex;
    if (h.length === 3) h = h[0]+h[0]+h[1]+h[1]+h[2]+h[2];
    const r = parseInt(h.slice(0, 2), 16);
    const g = parseInt(h.slice(2, 4), 16);
    const b = parseInt(h.slice(4, 6), 16);
    return `rgba(${r},${g},${b},${a})`;
  }

  set dimMode(v) { this._dimMode = v; }
  get dimMode() { return this._dimMode; }
}
