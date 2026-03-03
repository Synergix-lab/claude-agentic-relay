const BUBBLE_LIFETIME = 6;
const MAX_TEXT_LEN = 300;
const MAX_LINE_WIDTH = 260;
const LINE_HEIGHT = 16;
const FONT = "11px 'JetBrains Mono', monospace";

export class Bubble {
  constructor(text, type = "speech", pinned = false) {
    this.fullText = text.length > MAX_TEXT_LEN
      ? text.slice(0, MAX_TEXT_LEN - 3) + "..."
      : text;
    this.type = type; // "speech" | "thought"
    this.pinned = pinned;
    this.life = BUBBLE_LIFETIME;
    this.alpha = 0;
    this.lines = null;
  }

  get alive() {
    if (this.pinned) return true;
    return this.life > 0;
  }

  update(dt) {
    if (!this.pinned) {
      this.life -= dt;
    }

    if (this.pinned) {
      this.alpha = Math.min(1, this.alpha + dt * 5);
    } else if (this.life > BUBBLE_LIFETIME - 0.3) {
      this.alpha = Math.min(1, this.alpha + dt * 5);
    } else if (this.life < 0.5) {
      this.alpha = Math.max(0, this.life / 0.5);
    } else {
      this.alpha = 1;
    }
  }

  _wrapText(ctx) {
    if (this.lines) return this.lines;
    ctx.font = FONT;
    const words = this.fullText.split(/\s+/);
    const lines = [];
    let current = "";

    for (const word of words) {
      const test = current ? current + " " + word : word;
      if (ctx.measureText(test).width > MAX_LINE_WIDTH && current) {
        lines.push(current);
        current = word;
      } else {
        current = test;
      }
    }
    if (current) lines.push(current);

    if (lines.length > 8) {
      lines.length = 8;
      lines[7] = lines[7].slice(0, -3) + "...";
    }

    this.lines = lines;
    return lines;
  }

  render(ctx, x, y) {
    if (this.alpha <= 0) return;

    ctx.save();
    ctx.globalAlpha = this.alpha;
    ctx.font = FONT;

    const lines = this._wrapText(ctx);
    const maxW = Math.max(...lines.map((l) => ctx.measureText(l).width));
    const padding = 12;
    const w = maxW + padding * 2;
    const h = lines.length * LINE_HEIGHT + padding * 2 - 4;
    const bx = x - w / 2;
    const by = y - h - 14;

    if (this.type === "speech") {
      ctx.fillStyle = "#fff";
      ctx.strokeStyle = "#2d3436";
      ctx.lineWidth = 1.5;

      this._roundRect(ctx, bx, by, w, h, 8);
      ctx.fill();
      ctx.stroke();

      // Tail
      ctx.beginPath();
      ctx.moveTo(x - 6, by + h);
      ctx.lineTo(x, by + h + 10);
      ctx.lineTo(x + 6, by + h);
      ctx.fillStyle = "#fff";
      ctx.fill();
      ctx.stroke();

      ctx.fillStyle = "#2d3436";
    } else {
      ctx.fillStyle = "rgba(30, 30, 46, 0.94)";
      ctx.strokeStyle = "#6c5ce7";
      ctx.lineWidth = 1;

      this._roundRect(ctx, bx, by, w, h, 8);
      ctx.fill();
      ctx.stroke();

      // Thought circles
      ctx.beginPath();
      ctx.arc(x - 2, by + h + 6, 3.5, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(x + 4, by + h + 12, 2, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();

      ctx.fillStyle = "#e0e0e8";
    }

    for (let i = 0; i < lines.length; i++) {
      ctx.fillText(lines[i], bx + padding, by + padding + 12 + i * LINE_HEIGHT);
    }

    ctx.restore();
  }

  _roundRect(ctx, x, y, w, h, r) {
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.lineTo(x + w - r, y);
    ctx.quadraticCurveTo(x + w, y, x + w, y + r);
    ctx.lineTo(x + w, y + h - r);
    ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
    ctx.lineTo(x + r, y + h);
    ctx.quadraticCurveTo(x, y + h, x, y + h - r);
    ctx.lineTo(x, y + r);
    ctx.quadraticCurveTo(x, y, x + r, y);
    ctx.closePath();
  }
}
