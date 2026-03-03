export class World {
  constructor() {
    this.y = 0; // For depth sorting — always behind everything
    this.projectName = "default";
    this.hierarchyLinks = []; // [{from: AgentView, to: AgentView}, ...]
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

    // Title
    ctx.save();
    ctx.font = "bold 28px 'JetBrains Mono', monospace";
    ctx.fillStyle = "rgba(108, 92, 231, 0.08)";
    ctx.textAlign = "center";

    const title = this.projectName && this.projectName !== "default"
      ? `AGENT RELAY · ${this.projectName}`
      : "AGENT RELAY";
    ctx.fillText(title, w / 2, 44);
    ctx.restore();

    // Subtle center circle (agent arena boundary)
    ctx.save();
    const cx = w / 2;
    const cy = h / 2;
    const radius = Math.min(w, h) * 0.35;
    ctx.setLineDash([4, 8]);
    ctx.strokeStyle = "rgba(108, 92, 231, 0.08)";
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.arc(cx, cy, radius, 0, Math.PI * 2);
    ctx.stroke();
    ctx.restore();

    // Hierarchy lines between agents
    if (this.hierarchyLinks.length > 0) {
      ctx.save();
      ctx.setLineDash([3, 6]);
      ctx.strokeStyle = "rgba(108, 92, 231, 0.15)";
      ctx.lineWidth = 1;
      for (const link of this.hierarchyLinks) {
        ctx.beginPath();
        ctx.moveTo(link.from.x, link.from.y);
        ctx.lineTo(link.to.x, link.to.y);
        ctx.stroke();
      }
      ctx.restore();
    }
  }
}
