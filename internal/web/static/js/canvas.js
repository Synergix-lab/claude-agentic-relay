export class CanvasEngine {
  constructor(canvasEl) {
    this.canvas = canvasEl;
    this.ctx = canvasEl.getContext("2d");
    this.renderables = [];
    this.running = false;
    this.lastTime = 0;

    this.resize();
    window.addEventListener("resize", () => this.resize());
  }

  resize() {
    const container = this.canvas.parentElement;
    const dpr = window.devicePixelRatio || 1;
    this.canvas.width = container.clientWidth * dpr;
    this.canvas.height = container.clientHeight * dpr;
    this.ctx.scale(dpr, dpr);
    this.width = container.clientWidth;
    this.height = container.clientHeight;
  }

  add(renderable) {
    this.renderables.push(renderable);
  }

  remove(renderable) {
    const idx = this.renderables.indexOf(renderable);
    if (idx !== -1) this.renderables.splice(idx, 1);
  }

  start() {
    if (this.running) return;
    this.running = true;
    this.lastTime = performance.now();
    this._frame();
  }

  _frame() {
    if (!this.running) return;
    const now = performance.now();
    const dt = (now - this.lastTime) / 1000;
    this.lastTime = now;

    this.ctx.clearRect(0, 0, this.width, this.height);

    // Sort by y for depth
    this.renderables.sort((a, b) => (a.y ?? 0) - (b.y ?? 0));

    for (const r of this.renderables) {
      if (r.update) r.update(dt);
      if (r.render) r.render(this.ctx, this.width, this.height);
    }

    requestAnimationFrame(() => this._frame());
  }

  stop() {
    this.running = false;
  }
}
