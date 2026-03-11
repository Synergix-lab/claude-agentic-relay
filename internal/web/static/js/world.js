/**
 * World — Central solar system renderer.
 *
 * Global view: ONE sun (center) + project-planets orbiting.
 * Focused view: hidden (agents shown via AgentView tree layout).
 * Hierarchy links drawn in focused/single-project views.
 */
import { spaceAssets, SOLAR_PLANETS } from "./space-assets.js";

export { SpaceBackground } from "./space-bg.js";

function _fmtTokens(n) {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + "M";
  if (n >= 1000) return (n / 1000).toFixed(1) + "K";
  return String(n);
}

// Slight elliptical tilt for 3/4 perspective
const ORBIT_TILT = 0.55;

export class World {
  constructor() {
    this.y = -99999;
    this.isBackground = false;

    // Global view: sun + orbiting project planets
    this.sunCenter = null;          // { cx, cy }
    this.projectPlanets = [];       // [{ project, planetType, orbitRadius, angle, speed, cx, cy, size, agentCount }] — planetType from DB e.g. "terran/1"
    this.hoveredPlanet = null;      // project name or null

    // Colony view (focused project): planet surface
    this.colony = null;             // { project, solarPlanet, surfaceY }

    // Focused / single-project view: old cluster system
    this.clusters = [];             // [{project, cx, cy, radius, hidden?}]

    // Hierarchy links (focused view only)
    this.hierarchyLinks = [];

    // Animation
    this._phase = 0;
    this._sunFrameIndex = 0;
    this._sunFrameTimer = 0;
    this._sunFrameSpeed = 0.08;
    this._solarFrameIndex = 0;
    this._solarFrameTimer = 0;
    this._solarFrameSpeed = 0.12; // 8-frame cycle for solar planets

    // Dyson sphere animation
    this._dysonFrameIndex = 0;
    this._dysonFrameTimer = 0;
    this._dysonAngle = 0;

    // Orbit dust particles
    this._orbitDust = []; // { angle, orbitRadius, size, alpha, speed, drift }

    // Asteroid belt
    this._asteroids = []; // { angle, radius, speed, spriteIdx, size, alpha }
    this._asteroidImgs = [];
    this._asteroidsReady = false;
    this._initAsteroidSprites();

    // Stats for tooltip
    this._projectStats = {};
  }

  _initAsteroidSprites() {
    let loaded = 0;
    const total = 16;
    for (let i = 1; i <= total; i++) {
      const img = new Image();
      img.src = `img/space/asteroids/${i}.png`;
      img.onload = () => { loaded++; if (loaded >= total) this._asteroidsReady = true; };
      img.onerror = () => { loaded++; if (loaded >= total) this._asteroidsReady = true; };
      this._asteroidImgs.push(img);
    }
  }

  generateAsteroidBelt(centerX, centerY, radius) {
    this._asteroids = [];
    const count = 60 + Math.floor(Math.random() * 30);
    for (let i = 0; i < count; i++) {
      this._asteroids.push({
        angle: Math.random() * Math.PI * 2,
        radius: radius + (Math.random() - 0.5) * radius * 0.18, // ±9% spread
        speed: 0.003 + Math.random() * 0.004, // slow drift
        spriteIdx: Math.floor(Math.random() * 16),
        size: 6 + Math.random() * 12,
        alpha: 0.3 + Math.random() * 0.5,
      });
    }
  }

  update(dt) {
    this._phase += dt;

    // Sun animation
    this._sunFrameTimer += dt;
    if (this._sunFrameTimer >= this._sunFrameSpeed) {
      this._sunFrameTimer = 0;
      this._sunFrameIndex = (this._sunFrameIndex + 1) % 30;
    }

    // Solar planet animation (8 frames) for colony biome rendering
    this._solarFrameTimer += dt;
    if (this._solarFrameTimer >= this._solarFrameSpeed) {
      this._solarFrameTimer = 0;
      this._solarFrameIndex = (this._solarFrameIndex + 1) % 8;
    }

    // Animated planet frame (60 frames) for galaxy view project planets
    this._animPlanetTimer = (this._animPlanetTimer || 0) + dt;
    if (this._animPlanetTimer >= 0.1) {
      this._animPlanetTimer = 0;
      this._animPlanetFrame = ((this._animPlanetFrame || 0) + 1) % 60;
    }

    // Dyson sphere animation (slow color cycle + rotation)
    this._dysonFrameTimer += dt;
    if (this._dysonFrameTimer >= 0.4) {
      this._dysonFrameTimer = 0;
      this._dysonFrameIndex = (this._dysonFrameIndex + 1) % 7;
    }
    this._dysonAngle += dt * 0.15;

    // Advance planet orbits
    if (this.sunCenter && this.projectPlanets.length > 0) {
      for (const p of this.projectPlanets) {
        p.angle += p.speed * dt;
        p.cx = this.sunCenter.cx + Math.cos(p.angle) * p.orbitRadius;
        p.cy = this.sunCenter.cy + Math.sin(p.angle) * (p.orbitRadius * ORBIT_TILT);
      }
    }

    // Advance asteroid belt
    for (const a of this._asteroids) {
      a.angle += a.speed * dt;
    }
  }

  render(ctx, w, h) {
    ctx.imageSmoothingEnabled = false;

    // ── Global view: sun + project planets ──────────────────────────────────
    if (this.sunCenter && this.projectPlanets.length > 0) {
      const { cx, cy } = this.sunCenter;

      // --- Orbit rings (one per distinct radius) + arc trails ---
      // Deduplicate orbit radii to draw one ring per orbital lane
      const drawnRadii = new Set();
      for (const p of this.projectPlanets) {
        const rKey = Math.round(p.orbitRadius);
        if (!drawnRadii.has(rKey)) {
          drawnRadii.add(rKey);
          ctx.save();
          ctx.lineWidth = 0.6;
          ctx.strokeStyle = "rgba(255, 200, 60, 0.06)";
          ctx.setLineDash([4, 8]);
          ctx.beginPath();
          ctx.ellipse(cx, cy, p.orbitRadius, p.orbitRadius * ORBIT_TILT, 0, 0, Math.PI * 2);
          ctx.stroke();
          ctx.setLineDash([]);
          ctx.restore();
        }
      }

      // Arc glow trail behind each planet
      for (const p of this.projectPlanets) {
        const trailLen = 0.4;
        const steps = 16;
        ctx.save();
        ctx.lineCap = "round";
        for (let s = 0; s < steps; s++) {
          const t0 = p.angle - trailLen + (trailLen * s) / steps;
          const t1 = p.angle - trailLen + (trailLen * (s + 1)) / steps;
          const progress = (s + 1) / steps;
          ctx.beginPath();
          ctx.moveTo(
            cx + Math.cos(t0) * p.orbitRadius,
            cy + Math.sin(t0) * (p.orbitRadius * ORBIT_TILT)
          );
          ctx.lineTo(
            cx + Math.cos(t1) * p.orbitRadius,
            cy + Math.sin(t1) * (p.orbitRadius * ORBIT_TILT)
          );
          ctx.strokeStyle = `rgba(255, 210, 80, ${progress * 0.15})`;
          ctx.lineWidth = 1.5 + progress * 1.5;
          ctx.stroke();
        }
        ctx.restore();
      }

      // --- Asteroid belt ---
      if (this._asteroidsReady && this._asteroids.length > 0) {
        for (const a of this._asteroids) {
          const ax = cx + Math.cos(a.angle) * a.radius;
          const ay = cy + Math.sin(a.angle) * (a.radius * ORBIT_TILT);
          const img = this._asteroidImgs[a.spriteIdx];
          if (img && img.complete) {
            ctx.save();
            ctx.globalAlpha = a.alpha;
            ctx.drawImage(img, ax - a.size / 2, ay - a.size / 2, a.size, a.size);
            ctx.restore();
          }
        }
      }

      // --- Sun (founder) at center ---
      const sunSize = 80;
      const glowAlpha = 0.15 + 0.06 * Math.sin(this._phase * 0.7);
      const glowR = sunSize * 2.2;
      const grad = ctx.createRadialGradient(cx, cy, sunSize * 0.3, cx, cy, glowR);
      grad.addColorStop(0, `rgba(255, 200, 60, ${glowAlpha})`);
      grad.addColorStop(0.4, `rgba(255, 150, 30, ${glowAlpha * 0.5})`);
      grad.addColorStop(1, "rgba(255, 100, 0, 0)");
      ctx.save();
      ctx.fillStyle = grad;
      ctx.beginPath();
      ctx.arc(cx, cy, glowR, 0, Math.PI * 2);
      ctx.fill();
      ctx.restore();

      // Sun sprite (always animated)
      const sunImg = spaceAssets.getSunFrame(this._sunFrameIndex);
      if (sunImg) {
        ctx.save();
        ctx.imageSmoothingEnabled = false;
        ctx.drawImage(sunImg, cx - sunSize / 2, cy - sunSize / 2, sunSize, sunSize);
        ctx.restore();
      }

      // --- Dyson sphere overlay on sun (subtle orbiting particles) ---
      const dysonIdx = this._dysonType === "off" ? null
        : this._dysonType ? parseInt(this._dysonType) - 1
        : this._dysonFrameIndex;
      const dysonImg = dysonIdx != null ? spaceAssets.getDysonFrame(dysonIdx) : null;
      if (dysonImg) {
        const dysonSize = sunSize * 1.15;
        ctx.save();
        ctx.imageSmoothingEnabled = false;
        ctx.translate(cx, cy);
        ctx.rotate(this._dysonAngle);
        ctx.globalAlpha = 0.5;
        ctx.drawImage(dysonImg, -dysonSize / 2, -dysonSize / 2, dysonSize, dysonSize);
        ctx.restore();
      }

      // --- Project planets ---
      // Sort by cy so planets further "back" (smaller cy) draw first (depth sort)
      const sorted = [...this.projectPlanets].sort((a, b) => a.cy - b.cy);

      for (const p of sorted) {
        const isHovered = this.hoveredPlanet === p.project;
        const size = isHovered ? p.size * 1.15 : p.size;

        // Planet glow
        const pGlowA = isHovered ? 0.18 : 0.08;
        const pGlowR = size * 1.6;
        const pGrad = ctx.createRadialGradient(p.cx, p.cy, size * 0.2, p.cx, p.cy, pGlowR);
        pGrad.addColorStop(0, `rgba(180, 200, 255, ${pGlowA})`);
        pGrad.addColorStop(0.5, `rgba(100, 140, 200, ${pGlowA * 0.3})`);
        pGrad.addColorStop(1, "rgba(60, 80, 140, 0)");
        ctx.save();
        ctx.fillStyle = pGrad;
        ctx.beginPath();
        ctx.arc(p.cx, p.cy, pGlowR, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();

        // Planet sprite — use animated 48x48 planet from DB planetType
        const planetImg = p.planetType
          ? spaceAssets.getPlanetFrame(p.planetType, this._animPlanetFrame || 0)
          : spaceAssets.getSolarPlanetFrame(p.solarPlanet, this._solarFrameIndex);
        if (planetImg) {
          ctx.save();
          ctx.imageSmoothingEnabled = false;
          ctx.drawImage(planetImg, p.cx - size / 2, p.cy - size / 2, size, size);
          ctx.restore();
        }

        // Mini moons orbiting planet (1 per 4 agents, max 4) — with occlusion
        const moonCount = Math.min(Math.floor(p.agentCount / 4), 4);
        const moonOrbitR = size * 0.75;
        const moonSize = 10;
        const _drawMoon = (mi) => {
          const moonAngle = this._phase * (0.5 + mi * 0.15) + (mi * Math.PI * 2) / moonCount;
          const mx = p.cx + Math.cos(moonAngle) * moonOrbitR;
          const my = p.cy + Math.sin(moonAngle) * (moonOrbitR * 0.5);
          const behind = Math.sin(moonAngle) < 0; // behind planet if y < center
          const moonIdx = ((Math.abs(p.project.charCodeAt(0) * 7 + mi * 3)) % spaceAssets.miniMoonCount) + 1;
          return { mx, my, behind, moonIdx };
        };
        // Draw moons BEHIND planet first (before planet sprite was drawn above)
        // Actually we need to split: draw behind moons, then planet, then front moons
        // Since planet is already drawn, we just skip behind moons here
        if (moonCount > 0) {
          for (let mi = 0; mi < moonCount; mi++) {
            const m = _drawMoon(mi);
            if (m.behind) continue; // behind planet = hidden
            const moonImg = spaceAssets.getMiniMoon(m.moonIdx);
            if (moonImg) {
              ctx.save();
              ctx.imageSmoothingEnabled = false;
              ctx.globalAlpha = 0.85;
              ctx.drawImage(moonImg, m.mx - moonSize / 2, m.my - moonSize / 2, moonSize, moonSize);
              ctx.restore();
            }
          }
        }

        // Project label (always visible, below planet)
        ctx.save();
        ctx.font = `bold ${isHovered ? 12 : 10}px 'JetBrains Mono', monospace`;
        ctx.textAlign = "center";
        ctx.fillStyle = isHovered ? "rgba(255, 220, 100, 0.9)" : "rgba(255, 210, 80, 0.55)";
        ctx.shadowColor = "rgba(0, 0, 0, 0.6)";
        ctx.shadowBlur = 4;
        ctx.fillText(p.project.toUpperCase(), p.cx, p.cy + size / 2 + 16);
        ctx.shadowBlur = 0;

        // Agent count badge
        if (p.agentCount > 0) {
          ctx.font = "9px 'JetBrains Mono', monospace";
          ctx.fillStyle = isHovered ? "rgba(200, 220, 255, 0.8)" : "rgba(180, 200, 230, 0.4)";
          ctx.fillText(`${p.agentCount} agent${p.agentCount > 1 ? "s" : ""}`, p.cx, p.cy + size / 2 + 28);
        }

        // Task progress bar (if stats available)
        const stats = this._projectStats[p.project];
        if (stats && stats.tasks > 0) {
          const barW = Math.min(size * 1.2, 80);
          const barH = 4;
          const barX = p.cx - barW / 2;
          const barY = p.cy + size / 2 + 33;
          const progress = stats.done / stats.tasks;
          // Background
          ctx.fillStyle = "rgba(255, 255, 255, 0.08)";
          ctx.fillRect(barX, barY, barW, barH);
          // Fill
          if (progress > 0) {
            ctx.fillStyle = isHovered ? "rgba(0, 230, 118, 0.7)" : "rgba(0, 230, 118, 0.4)";
            ctx.fillRect(barX, barY, barW * progress, barH);
          }
        }

        // Token usage badge — only shown on hover to keep galaxy clean
        if (isHovered && stats && stats.tokens_24h > 0) {
          ctx.font = "bold 9px 'JetBrains Mono', monospace";
          ctx.fillStyle = "rgba(255, 217, 61, 0.8)";
          ctx.fillText(_fmtTokens(stats.tokens_24h) + " tk", p.cx, p.cy + size / 2 + 44);
        }
        ctx.restore();
      }

      // --- Hovered planet tooltip ---
      if (this.hoveredPlanet && this._projectStats) {
        const planet = this.projectPlanets.find(p => p.project === this.hoveredPlanet);
        const stats = this._projectStats[this.hoveredPlanet];
        if (planet && stats) {
          const size = planet.size;
          const tx = planet.cx;
          const ty = planet.cy - size / 2 - 46;

          const lines = [
            planet.project.toUpperCase(),
            `${stats.total} agents (${stats.online} online)`,
            `${stats.tasks} tasks (${stats.active} active, ${stats.done} done)`,
          ];
          if (stats.tokens_24h > 0) {
            lines.push(`${_fmtTokens(stats.tokens_24h)} tokens (24h)`);
          }
          ctx.save();
          ctx.font = "bold 11px 'JetBrains Mono', monospace";
          const maxW = Math.max(...lines.map(l => ctx.measureText(l).width));
          const padX = 12, padY = 8, lineH = 16;
          const boxW = maxW + padX * 2;
          const boxH = lines.length * lineH + padY * 2;
          const bx = tx - boxW / 2;
          const by = ty - boxH;

          ctx.fillStyle = "rgba(6, 6, 17, 0.92)";
          ctx.strokeStyle = "rgba(255, 200, 60, 0.3)";
          ctx.lineWidth = 1;
          ctx.beginPath();
          ctx.roundRect(bx, by, boxW, boxH, 4);
          ctx.fill();
          ctx.stroke();

          ctx.textAlign = "left";
          for (let i = 0; i < lines.length; i++) {
            ctx.font = i === 0 ? "bold 11px 'JetBrains Mono', monospace" : "10px 'JetBrains Mono', monospace";
            const isTokenLine = lines[i].includes("tokens (24h)");
            ctx.fillStyle = i === 0 ? "#ffd250" : isTokenLine ? "rgba(255, 217, 61, 0.7)" : "rgba(224, 224, 232, 0.7)";
            ctx.fillText(lines[i], bx + padX, by + padY + (i + 1) * lineH - 3);
          }
          ctx.restore();
        }
      }
    }

    // ── Colony view: planet in corner + project label ──────────────────────
    if (this.colony) {
      const { solarPlanet } = this.colony;

      // Planet rotating in top-left corner
      const cornerSize = 100;
      const cornerX = 70;
      const cornerY = 70;
      const cornerPlanetType = this.colony.planetType || null;
      const cornerPlanetImg = cornerPlanetType
        ? spaceAssets.getPlanetFrame(cornerPlanetType, this._animPlanetFrame || 0)
        : null;
      if (cornerPlanetImg) {
        // Glow behind planet
        const cpGlow = ctx.createRadialGradient(cornerX, cornerY, cornerSize * 0.15, cornerX, cornerY, cornerSize * 0.8);
        cpGlow.addColorStop(0, "rgba(180, 200, 255, 0.15)");
        cpGlow.addColorStop(1, "rgba(60, 80, 140, 0)");
        ctx.save();
        ctx.fillStyle = cpGlow;
        ctx.beginPath();
        ctx.arc(cornerX, cornerY, cornerSize * 0.8, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();

        ctx.save();
        ctx.imageSmoothingEnabled = false;
        ctx.drawImage(cornerPlanetImg, cornerX - cornerSize / 2, cornerY - cornerSize / 2, cornerSize, cornerSize);
        ctx.restore();
      }

      // Project label under corner planet
      ctx.save();
      ctx.font = "bold 11px 'JetBrains Mono', monospace";
      ctx.textAlign = "center";
      ctx.fillStyle = "rgba(255, 220, 100, 0.7)";
      ctx.shadowColor = "rgba(0,0,0,0.5)";
      ctx.shadowBlur = 6;
      ctx.fillText(this.colony.project.toUpperCase(), cornerX, cornerY + cornerSize / 2 + 14);
      ctx.font = "9px 'JetBrains Mono', monospace";
      ctx.fillStyle = "rgba(200, 210, 230, 0.4)";
      const biomeLabel = (cornerPlanetType || "").split("/")[0] || solarPlanet;
      ctx.fillText(biomeLabel.charAt(0).toUpperCase() + biomeLabel.slice(1) + " World", cornerX, cornerY + cornerSize / 2 + 27);
      ctx.shadowBlur = 0;
      ctx.restore();
    }

    // ── Focused/single-project view: clusters (sun hidden) ──────────────────
    for (const cluster of this.clusters) {
      if (cluster.hidden) continue;

      const sunSize = Math.min(cluster.radius * 0.45, 96);

      // Sun glow
      const glowAlpha = 0.15 + 0.06 * Math.sin(this._phase * 0.7);
      const glowR = sunSize * 2.2;
      const grad = ctx.createRadialGradient(cluster.cx, cluster.cy, sunSize * 0.3, cluster.cx, cluster.cy, glowR);
      grad.addColorStop(0, `rgba(255, 200, 60, ${glowAlpha})`);
      grad.addColorStop(0.4, `rgba(255, 150, 30, ${glowAlpha * 0.5})`);
      grad.addColorStop(1, "rgba(255, 100, 0, 0)");
      ctx.save();
      ctx.fillStyle = grad;
      ctx.beginPath();
      ctx.arc(cluster.cx, cluster.cy, glowR, 0, Math.PI * 2);
      ctx.fill();
      ctx.restore();

      // Sun sprite
      const sunImg = spaceAssets.getSunFrame(this._sunFrameIndex);
      if (sunImg) {
        ctx.save();
        ctx.imageSmoothingEnabled = false;
        ctx.drawImage(sunImg, cluster.cx - sunSize / 2, cluster.cy - sunSize / 2, sunSize, sunSize);
        ctx.restore();
      }

      // Label
      ctx.save();
      ctx.font = "bold 11px 'JetBrains Mono', monospace";
      ctx.textAlign = "center";
      ctx.shadowColor = "rgba(255, 180, 40, 0.4)";
      ctx.shadowBlur = 6;
      ctx.fillStyle = "rgba(255, 210, 80, 0.6)";
      ctx.fillText(cluster.project.toUpperCase(), cluster.cx, cluster.cy - sunSize / 2 - 14);
      ctx.shadowBlur = 0;
      ctx.restore();
    }

    // ── Hierarchy lines ─────────────────────────────────────────────────────
    if (this.hierarchyLinks.length > 0) {
      ctx.save();
      for (const link of this.hierarchyLinks) {
        const mx = link.from.x, my = link.from.y;
        const rx = link.to.x, ry = link.to.y;
        const midX = (mx + rx) / 2, midY = (my + ry) / 2;
        const dx = rx - mx, dy = ry - my;
        const len = Math.sqrt(dx * dx + dy * dy);
        if (len < 1) continue;

        let anchorX = 0, anchorY = 0;
        if (this.clusters.length > 0) {
          anchorX = this.clusters[0].cx;
          anchorY = this.clusters[0].cy;
          for (const c of this.clusters) {
            if (Math.hypot(mx - c.cx, my - c.cy) < Math.hypot(mx - anchorX, my - anchorY)) {
              anchorX = c.cx; anchorY = c.cy;
            }
          }
        }

        const perpX = -dy / len, perpY = dx / len;
        const dot = perpX * (anchorX - midX) + perpY * (anchorY - midY);
        const sign = dot > 0 ? 1 : -1;
        const bulge = Math.min(len * 0.2, 40);
        const cpx = midX + perpX * bulge * sign;
        const cpy = midY + perpY * bulge * sign;

        ctx.setLineDash([5, 5]);
        ctx.strokeStyle = "rgba(162, 155, 254, 0.3)";
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.moveTo(mx, my);
        ctx.quadraticCurveTo(cpx, cpy, rx, ry);
        ctx.stroke();

        // Arrow
        const t = 0.92;
        const nearX = (1 - t) ** 2 * mx + 2 * (1 - t) * t * cpx + t * t * rx;
        const nearY = (1 - t) ** 2 * my + 2 * (1 - t) * t * cpy + t * t * ry;
        const arrAngle = Math.atan2(ry - nearY, rx - nearX);
        ctx.setLineDash([]);
        ctx.fillStyle = "rgba(108, 92, 231, 0.45)";
        ctx.beginPath();
        const ax = rx - Math.cos(arrAngle) * 24;
        const ay = ry - Math.sin(arrAngle) * 24;
        ctx.moveTo(ax, ay);
        ctx.lineTo(ax - Math.cos(arrAngle - 0.5) * 8, ay - Math.sin(arrAngle - 0.5) * 8);
        ctx.lineTo(ax - Math.cos(arrAngle + 0.5) * 8, ay - Math.sin(arrAngle + 0.5) * 8);
        ctx.closePath();
        ctx.fill();
      }
      ctx.restore();
    }
  }
}
