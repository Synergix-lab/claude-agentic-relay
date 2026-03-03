class Particle {
  constructor(x, y, color, vx, vy, life, size) {
    this.x = x;
    this.y = y;
    this.color = color;
    this.vx = vx;
    this.vy = vy;
    this.life = life;
    this.maxLife = life;
    this.size = size;
    this.gravity = 60;
  }

  get alive() {
    return this.life > 0;
  }

  update(dt) {
    this.life -= dt;
    this.x += this.vx * dt;
    this.y += this.vy * dt;
    this.vy += this.gravity * dt;
  }

  render(ctx) {
    const alpha = Math.max(0, this.life / this.maxLife);
    ctx.save();
    ctx.globalAlpha = alpha;
    ctx.fillStyle = this.color;
    ctx.fillRect(
      this.x - this.size / 2,
      this.y - this.size / 2,
      this.size,
      this.size
    );
    ctx.restore();
  }
}

export class ParticleEmitter {
  constructor() {
    this.particles = [];
  }

  emit(type, x, y) {
    if (type === "spawn") {
      const colors = ["#00e676", "#69f0ae", "#b9f6ca", "#00c853"];
      for (let i = 0; i < 12; i++) {
        const angle = (Math.PI * 2 * i) / 12;
        const speed = 40 + Math.random() * 40;
        this.particles.push(
          new Particle(
            x, y,
            colors[Math.floor(Math.random() * colors.length)],
            Math.cos(angle) * speed,
            Math.sin(angle) * speed - 20,
            0.8 + Math.random() * 0.4,
            2 + Math.random() * 2
          )
        );
      }
    } else if (type === "celebrate") {
      const colors = [
        "#ff5252", "#ffd740", "#69f0ae", "#40c4ff",
        "#ea80fc", "#ff6e40", "#6c5ce7", "#00e676",
      ];
      for (let i = 0; i < 24; i++) {
        const angle = -Math.PI / 2 + (Math.random() - 0.5) * Math.PI * 0.8;
        const speed = 80 + Math.random() * 100;
        this.particles.push(
          new Particle(
            x + (Math.random() - 0.5) * 20,
            y,
            colors[Math.floor(Math.random() * colors.length)],
            Math.cos(angle) * speed,
            Math.sin(angle) * speed,
            1.2 + Math.random() * 0.6,
            2 + Math.random() * 3
          )
        );
      }
    }
  }

  update(dt) {
    for (let i = this.particles.length - 1; i >= 0; i--) {
      this.particles[i].update(dt);
      if (!this.particles[i].alive) {
        this.particles.splice(i, 1);
      }
    }
  }

  render(ctx) {
    for (const p of this.particles) {
      p.render(ctx);
    }
  }
}
