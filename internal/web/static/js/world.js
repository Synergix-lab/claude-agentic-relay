/**
 * WorldBackground — rendered without camera transform (fills viewport).
 * Draws the gradient and grid.
 */
export class WorldBackground {
  constructor() {
    this.y = -Infinity; // Always behind everything
    this.isBackground = true;
  }

  update() {}

  render(ctx, w, h) {
    // Dark gradient background
    const grad = ctx.createLinearGradient(0, 0, 0, h);
    grad.addColorStop(0, "#0a0a12");
    grad.addColorStop(1, "#0f0f1a");
    ctx.fillStyle = grad;
    ctx.fillRect(0, 0, w, h);

    // Subtle grid
    ctx.strokeStyle = "rgba(255,255,255,0.025)";
    ctx.lineWidth = 1;
    const gridSize = 40;

    for (let x = 0; x < w; x += gridSize) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, h);
      ctx.stroke();
    }
    for (let y = 0; y < h; y += gridSize) {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();
    }
  }
}

/**
 * World — rendered with camera transform.
 * Draws cluster circles, project labels, and hierarchy links.
 */
export class World {
  constructor() {
    this.y = -99999; // Behind agents but after background
    this.isBackground = false;
    this.clusters = []; // [{project, cx, cy, radius}, ...]
    this.hierarchyLinks = []; // [{from: AgentView, to: AgentView}, ...]
  }

  update() {}

  render(ctx, w, h) {
    // Per-cluster: dashed boundary circle + project label (skip hidden clusters)
    for (const cluster of this.clusters) {
      if (cluster.hidden) continue;

      ctx.save();
      ctx.setLineDash([4, 8]);
      ctx.strokeStyle = "rgba(108, 92, 231, 0.10)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.arc(cluster.cx, cluster.cy, cluster.radius, 0, Math.PI * 2);
      ctx.stroke();
      ctx.restore();

      // Project name label above the cluster
      ctx.save();
      ctx.font = "bold 14px 'JetBrains Mono', monospace";
      ctx.fillStyle = "rgba(108, 92, 231, 0.4)";
      ctx.textAlign = "center";
      ctx.fillText(cluster.project.toUpperCase(), cluster.cx, cluster.cy - cluster.radius - 16);
      ctx.restore();
    }

    // Hierarchy lines between agents (manager -> report)
    if (this.hierarchyLinks.length > 0) {
      ctx.save();
      for (const link of this.hierarchyLinks) {
        const mx = link.from.x, my = link.from.y;
        const rx = link.to.x, ry = link.to.y;

        const midX = (mx + rx) / 2;
        const midY = (my + ry) / 2;
        const dx = rx - mx, dy = ry - my;
        const len = Math.sqrt(dx * dx + dy * dy);
        if (len < 1) continue;

        // Find cluster center for this link
        let anchorX = 0, anchorY = 0;
        if (this.clusters.length > 0) {
          // Use the cluster that the manager belongs to
          const closest = this.clusters[0];
          anchorX = closest.cx;
          anchorY = closest.cy;
          // Try to find exact cluster by proximity
          for (const c of this.clusters) {
            const d1 = Math.hypot(mx - c.cx, my - c.cy);
            const d2 = Math.hypot(mx - anchorX, my - anchorY);
            if (d1 < d2) { anchorX = c.cx; anchorY = c.cy; }
          }
        }

        const perpX = -dy / len;
        const perpY = dx / len;
        const toCenterX = anchorX - midX;
        const toCenterY = anchorY - midY;
        const dot = perpX * toCenterX + perpY * toCenterY;
        const sign = dot > 0 ? 1 : -1;
        const bulge = Math.min(len * 0.2, 40);
        const cpx = midX + perpX * bulge * sign;
        const cpy = midY + perpY * bulge * sign;

        ctx.setLineDash([5, 5]);
        ctx.strokeStyle = "rgba(162, 155, 254, 0.35)";
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.moveTo(mx, my);
        ctx.quadraticCurveTo(cpx, cpy, rx, ry);
        ctx.stroke();

        // Arrow at report end
        const t = 0.92;
        const nearX = (1 - t) * (1 - t) * mx + 2 * (1 - t) * t * cpx + t * t * rx;
        const nearY = (1 - t) * (1 - t) * my + 2 * (1 - t) * t * cpy + t * t * ry;
        const arrAngle = Math.atan2(ry - nearY, rx - nearX);
        const arrLen = 8;
        ctx.setLineDash([]);
        ctx.fillStyle = "rgba(108, 92, 231, 0.5)";
        ctx.beginPath();
        ctx.moveTo(rx - Math.cos(arrAngle) * 24, ry - Math.sin(arrAngle) * 24);
        ctx.lineTo(
          rx - Math.cos(arrAngle) * 24 - Math.cos(arrAngle - 0.5) * arrLen,
          ry - Math.sin(arrAngle) * 24 - Math.sin(arrAngle - 0.5) * arrLen
        );
        ctx.lineTo(
          rx - Math.cos(arrAngle) * 24 - Math.cos(arrAngle + 0.5) * arrLen,
          ry - Math.sin(arrAngle) * 24 - Math.sin(arrAngle + 0.5) * arrLen
        );
        ctx.closePath();
        ctx.fill();
      }
      ctx.restore();
    }
  }
}
