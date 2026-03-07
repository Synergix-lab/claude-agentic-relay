# Scene Director — Architecture

The space background is not a random event spawner. It is a **cinematic director** that choreographs multi-event sequences ("scenes") with narrative pacing, foreshadowing, cause-and-effect chains, and enforced breathing room between acts.

---

## System Overview

```
SpaceBackground
  |
  |-- Ambient Layers (L0-L6, always running, untouched)
  |     starfield, nebulae, dust, galaxies, blackholes, moons, rings
  |
  |-- Scene Director (NEW — replaces raw _maybeSpawnEvent)
  |     |
  |     |-- Narrative Clock (phase state machine)
  |     |     calm -> building -> climax -> cooldown -> calm ...
  |     |
  |     |-- Scene Queue (pending scene beats with delays)
  |     |     [{ time: absolute, fn: () => void }]
  |     |
  |     |-- Scene Picker (weighted random, no recent repeats)
  |     |
  |     |-- Foreshadow Manager (pre-event visual cues)
  |     |
  |     |-- Cause-Effect Linker (post-event reactions)
  |     |
  |     |-- "Ma" Enforcer (silence after narrative events)
  |     |
  |     |-- Ambient Traffic (small ships between scenes)
  |
  |-- Event Layer (L7 — renders all active events)
  |     existing event types + new "ship" type
  |
  |-- Vignette (L8)
```

---

## New State (constructor additions)

```javascript
// --- Scene Director state ---
this._pendingSpawns = [];      // delayed spawns: [{ time, fn }]
this._sceneCooldown = 0;       // seconds until next scene allowed
this._sceneHistory = [];       // last 4 scene type strings
this._maCooldown = 0;          // seconds of enforced ambient-only silence
this._narrativePhase = "calm"; // "calm" | "building" | "climax" | "cooldown"
this._phaseTimer = 0;          // time spent in current phase
this._phaseDuration = 0;       // randomly chosen duration for current phase
```

---

## Narrative Clock

The clock cycles through 4 phases. Each phase has a random duration within a range. The phase controls what CAN spawn.

```
  calm (15-25s)       Only ambient layers. No scenic/narrative events.
       |              Ambient breathes deeper: nebula pulse +20%, twinkle +15%.
       v
  building (8-15s)    Scenic events allowed (asteroids, comets, distant ships).
       |              Density increases: event check every 0.25s instead of 0.35s.
       v
  climax (4-10s)      ONE narrative scene fires (from the scene picker).
       |              Max 1 narrative event at a time.
       v
  cooldown (10-18s)   Aftermath only. Existing events finish, debris drifts.
       |              No new events spawn. Foreshadowing for next cycle may begin.
       v
  calm (repeat)       Duration may compress: multiply by 0.85 each cycle,
                      reset to 1.0 after 4 cycles.
```

### Phase Transition Logic

```javascript
update(dt) {
  // ... existing ambient updates ...

  this._phaseTimer += dt;
  this._maCooldown = Math.max(0, this._maCooldown - dt);
  this._sceneCooldown = Math.max(0, this._sceneCooldown - dt);

  // Process pending spawns (scene beats)
  for (let i = this._pendingSpawns.length - 1; i >= 0; i--) {
    if (this._phase >= this._pendingSpawns[i].time) {
      this._pendingSpawns[i].fn();
      this._pendingSpawns.splice(i, 1);
    }
  }

  // Phase transitions
  if (this._phaseTimer >= this._phaseDuration) {
    this._advancePhase();
  }

  // Event spawning gated by phase
  this._eventCooldown -= dt;
  if (this._eventCooldown <= 0) {
    this._eventCooldown = this._narrativePhase === "building" ? 0.25 : 0.35;
    this._phaseSpawn();
  }
}
```

### _advancePhase()

```javascript
_advancePhase() {
  const DURATIONS = {
    calm:     [15, 25],
    building: [8, 15],
    climax:   [4, 10],
    cooldown: [10, 18],
  };
  const NEXT = { calm: "building", building: "climax", climax: "cooldown", cooldown: "calm" };

  this._narrativePhase = NEXT[this._narrativePhase];
  this._phaseTimer = 0;
  const [min, max] = DURATIONS[this._narrativePhase];
  this._phaseDuration = min + Math.random() * (max - min);

  // On entering climax: pick and fire a scene
  if (this._narrativePhase === "climax" && this._sceneCooldown <= 0) {
    this._spawnScene();
  }
}
```

---

## Scene Picker

`_spawnScene()` selects from a weighted pool, avoiding the last 4 types.

```javascript
_spawnScene() {
  const SCENES = [
    { type: "stellarDeath",     weight: 0.7,  fn: () => this._sceneStellarDeath() },
    { type: "wormholeTransit",  weight: 1.0,  fn: () => this._sceneWormholeTransit() },
    { type: "cometBreakup",     weight: 1.0,  fn: () => this._sceneCometBreakup() },
    { type: "patrol",           weight: 1.2,  fn: () => this._scenePatrol() },
    { type: "dogfight",         weight: 0.9,  fn: () => this._sceneDogfight() },
    { type: "hyperspaceJump",   weight: 0.8,  fn: () => this._sceneHyperspaceJump() },
    { type: "pulsarDiscovery",  weight: 0.8,  fn: () => this._scenePulsarDiscovery() },
    { type: "stationResupply",  weight: 0.5,  fn: () => this._sceneStationResupply() },
    { type: "deepSpaceSignal",  weight: 0.7,  fn: () => this._sceneDeepSpaceSignal() },
    { type: "shipJoke",         weight: 1.0,  fn: () => this._sceneShipJoke() },
    { type: "convoy",           weight: 0.8,  fn: () => this._sceneConvoy() },
    { type: "distantBattle",    weight: 0.6,  fn: () => this._sceneDistantBattle() },
    { type: "falseCalmTrap",    weight: 0.4,  fn: () => this._sceneFalseCalm() },
  ];

  // Exclude recently played
  const available = SCENES.filter(s => !this._sceneHistory.includes(s.type));
  if (!available.length) { this._sceneHistory = []; return; }

  // Weighted random
  const total = available.reduce((sum, s) => sum + s.weight, 0);
  let pick = Math.random() * total;
  for (const s of available) {
    pick -= s.weight;
    if (pick <= 0) {
      s.fn();
      this._sceneHistory.push(s.type);
      if (this._sceneHistory.length > 4) this._sceneHistory.shift();
      this._sceneCooldown = 15 + Math.random() * 20;
      return;
    }
  }
}
```

---

## Delayed Spawn Queue

Scenes schedule their beats via `_queueSpawn`:

```javascript
_queueSpawn(delaySec, fn) {
  this._pendingSpawns.push({ time: this._phase + delaySec, fn });
}
```

Example usage in a scene:
```javascript
_sceneWormholeTransit() {
  const px = 300 + Math.random() * 1400;
  const py = 200 + Math.random() * 500;

  // Beat 1 (foreshadow): dim purple glow
  this._spawnForeshadow(px, py, "purple", 3);

  // Beat 2 (t=3s): wormhole opens
  this._queueSpawn(3, () => this._spawnWormhole(px, py));

  // Beat 3 (t=5.5s): ship exits wormhole
  this._queueSpawn(5.5, () => {
    const dir = px < 1000 ? 1 : -1;
    this._spawnFlyingShip({ x: px, y: py, vx: dir * 140, vy: (Math.random() - 0.5) * 20 });
  });

  // "Ma" enforcement
  this._maCooldown = 18;
}
```

---

## New Event Type: "ship" (simple flying ship)

Reuse existing ship sprites but without the speech bubble logic. Simpler than `shipvisit`.

```javascript
// Spawn helper
_spawnFlyingShip({ x, y, vx, vy, shipType, color, duration }) {
  const type = shipType || (Math.floor(Math.random() * 3) + 1);
  const c = color || (Math.random() > 0.5 ? "blue" : "red");
  const dir = vx > 0 ? "right" : "left";
  this._events.push({
    type: "ship",
    x, y, vx, vy,
    duration: duration || 12,
    age: 0,
    shipType: type, color: c, dir,
    frame: 1, frameTimer: 0,
  });
}
```

### Update logic (add to existing ship visit block)

```javascript
if (ev.type === "ship") {
  ev.frameTimer += dt;
  if (ev.frameTimer >= 0.12) { ev.frameTimer = 0; ev.frame = (ev.frame % 6) + 1; }
}
```

### Render case

```javascript
case "ship": {
  const img = spaceAssets.getShipFrame(ev.shipType, ev.color, ev.dir, ev.frame);
  if (!img) break;
  const scale = 2.0;
  const sw = img.naturalWidth * scale;
  const sh = img.naturalHeight * scale;

  // Engine trail
  const trailDir = ev.dir === "right" ? -1 : 1;
  const trailLen = 20 + Math.abs(ev.vx) * 0.15;
  const trailGrad = ctx.createLinearGradient(
    ev.x + trailDir * sw * 0.4, ev.y,
    ev.x + trailDir * (sw * 0.4 + trailLen), ev.y
  );
  trailGrad.addColorStop(0, `rgba(80,180,255,${0.2 * fadeIn * fadeOut})`);
  trailGrad.addColorStop(1, "rgba(80,180,255,0)");
  ctx.strokeStyle = trailGrad;
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.moveTo(ev.x + trailDir * sw * 0.4, ev.y);
  ctx.lineTo(ev.x + trailDir * (sw * 0.4 + trailLen), ev.y);
  ctx.stroke();

  // Ship sprite
  ctx.globalAlpha = 0.7 * fadeIn * fadeOut;
  ctx.imageSmoothingEnabled = false;
  ctx.drawImage(img, ev.x - sw / 2, ev.y - sh / 2, sw, sh);
  ctx.globalAlpha = 1;
  break;
}
```

---

## New Event Type: "foreshadow"

A subtle visual cue at a specific position before a major event.

```javascript
_spawnForeshadow(x, y, colorType, duration) {
  const colors = {
    purple: { inner: "180,120,255", outer: "100,60,200" },
    cyan:   { inner: "80,200,240",  outer: "40,120,180" },
    orange: { inner: "255,180,80",  outer: "200,120,40" },
    white:  { inner: "220,230,255", outer: "160,180,220" },
  };
  const c = colors[colorType] || colors.purple;
  this._events.push({
    type: "foreshadow",
    x, y, vx: 0, vy: 0,
    duration: duration || 3,
    age: 0,
    colorInner: c.inner, colorOuter: c.outer,
    pulseSpeed: 2 + Math.random() * 2,
    maxRadius: 25 + Math.random() * 20,
  });
}
```

### Render case

```javascript
case "foreshadow": {
  const pulse = 0.5 + 0.5 * Math.sin(ev.age * ev.pulseSpeed);
  const a = fadeIn * fadeOut * 0.12 * pulse;
  const r = ev.maxRadius * (0.5 + 0.5 * progress);
  const g = ctx.createRadialGradient(ev.x, ev.y, 0, ev.x, ev.y, r);
  g.addColorStop(0, `rgba(${ev.colorInner},${a * 1.5})`);
  g.addColorStop(0.5, `rgba(${ev.colorOuter},${a * 0.5})`);
  g.addColorStop(1, "rgba(0,0,0,0)");
  ctx.fillStyle = g;
  ctx.beginPath();
  ctx.arc(ev.x, ev.y, r, 0, Math.PI * 2);
  ctx.fill();
  break;
}
```

---

## New Event Type: "navlight"

A blinking dot approaching from a screen edge (foreshadow for ships).

```javascript
_spawnNavLight(fromLeft, y, duration) {
  this._events.push({
    type: "navlight",
    x: fromLeft ? -10 : 2410,
    y,
    vx: (fromLeft ? 1 : -1) * 30,
    vy: 0,
    duration: duration || 4,
    age: 0,
    blinkSpeed: 3 + Math.random() * 2,
    color: fromLeft ? "80,180,255" : "255,80,80",
  });
}
```

### Render case

```javascript
case "navlight": {
  const blink = Math.sin(ev.age * ev.blinkSpeed * Math.PI) > 0 ? 1 : 0;
  if (blink) {
    const a = 0.6 * fadeIn * fadeOut;
    ctx.fillStyle = `rgba(${ev.color},${a})`;
    ctx.beginPath();
    ctx.arc(ev.x, ev.y, 2.5, 0, Math.PI * 2);
    ctx.fill();
    // Glow
    const g = ctx.createRadialGradient(ev.x, ev.y, 0, ev.x, ev.y, 8);
    g.addColorStop(0, `rgba(${ev.color},${a * 0.4})`);
    g.addColorStop(1, "rgba(0,0,0,0)");
    ctx.fillStyle = g;
    ctx.beginPath();
    ctx.arc(ev.x, ev.y, 8, 0, Math.PI * 2);
    ctx.fill();
  }
  break;
}
```

---

## New Event Type: "debris"

Small particles that linger after an event (supernova, comet breakup, dogfight).

```javascript
_spawnDebrisField(cx, cy, count, spread) {
  for (let i = 0; i < (count || 5); i++) {
    const angle = Math.random() * Math.PI * 2;
    const speed = 5 + Math.random() * 20;
    this._events.push({
      type: "debris",
      x: cx + (Math.random() - 0.5) * (spread || 30),
      y: cy + (Math.random() - 0.5) * (spread || 30),
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed,
      duration: 8 + Math.random() * 12,
      age: 0,
      size: 1 + Math.random() * 2.5,
      rotation: Math.random() * Math.PI * 2,
      rotSpeed: (Math.random() - 0.5) * 3,
      glint: Math.random() > 0.6, // some particles glint
      glintSpeed: 2 + Math.random() * 3,
    });
  }
}
```

### Render case

```javascript
case "debris": {
  let a = 0.35 * fadeIn * fadeOut;
  if (ev.glint) a *= 0.5 + 0.5 * Math.sin(ev.age * ev.glintSpeed);
  ctx.globalAlpha = a;
  ctx.fillStyle = "#aabbcc";
  ctx.save();
  ctx.translate(ev.x, ev.y);
  ctx.rotate(ev.rotation);
  ctx.fillRect(-ev.size / 2, -ev.size / 2, ev.size, ev.size);
  ctx.restore();
  ctx.globalAlpha = 1;
  break;
}
```

---

## New Event Type: "distantFlash"

Tiny flashes in the deep background suggesting far-away battle.

```javascript
_spawnDistantFlashes(cx, cy, count, spreadRadius) {
  for (let i = 0; i < count; i++) {
    const angle = Math.random() * Math.PI * 2;
    const dist = Math.random() * spreadRadius;
    this._events.push({
      type: "distantFlash",
      x: cx + Math.cos(angle) * dist,
      y: cy + Math.sin(angle) * dist,
      vx: 0, vy: 0,
      duration: 0.15 + Math.random() * 0.2,
      age: 0,
      size: 1.5 + Math.random() * 2,
      startDelay: Math.random() * 2, // stagger flashes
    });
  }
}
```

### Render case

```javascript
case "distantFlash": {
  if (ev.age < (ev.startDelay || 0)) break;
  const localAge = ev.age - (ev.startDelay || 0);
  const localProgress = localAge / (ev.duration - (ev.startDelay || 0));
  if (localProgress < 0 || localProgress > 1) break;
  const brightness = localProgress < 0.3
    ? localProgress / 0.3
    : 1 - (localProgress - 0.3) / 0.7;
  ctx.globalAlpha = brightness * 0.5;
  ctx.fillStyle = "#ffe8cc";
  ctx.beginPath();
  ctx.arc(ev.x, ev.y, ev.size, 0, Math.PI * 2);
  ctx.fill();
  // tiny glow
  ctx.globalAlpha = brightness * 0.15;
  ctx.beginPath();
  ctx.arc(ev.x, ev.y, ev.size * 3, 0, Math.PI * 2);
  ctx.fill();
  ctx.globalAlpha = 1;
  break;
}
```

---

## New Event Type: "streakJump"

A ship stretching into hyperspace (elongated sprite → flash → gone).

```javascript
_spawnStreakJump(x, y, dirX) {
  this._events.push({
    type: "streakJump",
    x, y,
    vx: 0, vy: 0,
    duration: 1.2,
    age: 0,
    dirX: dirX || 1, // 1 = right, -1 = left
    shipType: Math.floor(Math.random() * 3) + 1,
    color: Math.random() > 0.5 ? "blue" : "red",
  });
}
```

### Render case

```javascript
case "streakJump": {
  const p = ev.age / ev.duration;
  const dir = ev.dirX > 0 ? "right" : "left";
  const img = spaceAssets.getShipFrame(ev.shipType, ev.color, dir, 1);
  if (!img) break;

  if (p < 0.3) {
    // Phase 1: ship visible, slowing down
    const scale = 2.0;
    ctx.globalAlpha = 0.7;
    ctx.imageSmoothingEnabled = false;
    const sw = img.naturalWidth * scale;
    const sh = img.naturalHeight * scale;
    ctx.drawImage(img, ev.x - sw / 2, ev.y - sh / 2, sw, sh);
  } else if (p < 0.6) {
    // Phase 2: stretching
    const stretchT = (p - 0.3) / 0.3;
    const scaleX = 2.0 + stretchT * 8; // stretch up to 10x
    const scaleY = 2.0 * (1 - stretchT * 0.6); // flatten
    const a = 1 - stretchT * 0.3;
    ctx.globalAlpha = a * 0.7;
    ctx.imageSmoothingEnabled = false;
    const sw = img.naturalWidth * scaleX;
    const sh = img.naturalHeight * scaleY;
    ctx.drawImage(img, ev.x - sw / 2, ev.y - sh / 2, sw, sh);
  } else if (p < 0.7) {
    // Phase 3: white flash
    const flashA = (0.7 - p) / 0.1;
    const g = ctx.createRadialGradient(ev.x, ev.y, 0, ev.x, ev.y, 40);
    g.addColorStop(0, `rgba(200,220,255,${flashA * 0.6})`);
    g.addColorStop(1, "rgba(180,200,255,0)");
    ctx.fillStyle = g;
    ctx.beginPath();
    ctx.arc(ev.x, ev.y, 40, 0, Math.PI * 2);
    ctx.fill();
  } else {
    // Phase 4: light trail fading
    const trailA = (1 - p) / 0.3;
    const trailLen = 80;
    const tg = ctx.createLinearGradient(
      ev.x - ev.dirX * trailLen, ev.y, ev.x, ev.y
    );
    tg.addColorStop(0, "rgba(180,210,255,0)");
    tg.addColorStop(1, `rgba(200,230,255,${trailA * 0.3})`);
    ctx.strokeStyle = tg;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(ev.x - ev.dirX * trailLen, ev.y);
    ctx.lineTo(ev.x, ev.y);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
  break;
}
```

---

## _phaseSpawn() — Replaces _maybeSpawnEvent()

```javascript
_phaseSpawn() {
  if (this._maCooldown > 0) return; // "Ma" enforced silence

  const r = Math.random();

  switch (this._narrativePhase) {
    case "calm":
      // Ambient only — shooting stars very rarely
      if (r < 0.015) this._spawnShootingStar();
      break;

    case "building":
      // Scenic events ramp up
      if (r < 0.04) this._spawnShootingStar();
      if (r < 0.05) this._spawnMeteorShower();
      if (r < 0.06 && this._cnt("comet") < 2) this._spawnComet();
      if (r < 0.07 && this._cnt("asteroid") < 5) this._spawnAsteroid();
      break;

    case "climax":
      // Scene already fired via _advancePhase. Allow ambient traffic only.
      if (r < 0.03) this._spawnShootingStar();
      break;

    case "cooldown":
      // Nothing new. Let existing events finish.
      break;
  }
}
```

---

## Migration Notes

1. **Remove** the existing `_maybeSpawnEvent()` body entirely
2. **Remove** individual event spawning from the probability ladder (stations, quasars, supernovae, wormholes, pulsars, hyperspace, shipvisits are now ONLY spawned via scenes)
3. **Keep** the existing spawn methods (`_spawnComet`, `_spawnAsteroid`, `_spawnShootingStar`, `_spawnMeteorShower`, `_spawnStation`, `_spawnQuasar`, `_spawnSupernova`, `_spawnWormhole`, `_spawnPulsar`, `_spawnHyperspace`, `_spawnShipVisit`) — scenes call them
4. **Modify** `_spawnWormhole`, `_spawnSupernova`, etc. to accept optional `(x, y)` params so scenes can place them at specific coordinates
5. **Add** the new event types: `ship`, `foreshadow`, `navlight`, `debris`, `distantFlash`, `streakJump`
6. **Add** render cases for all new event types
7. **Fix** the DEBUG spawn rate (0.95 → remove, it's no longer used)
8. **Keep** `shipvisit` event type and its render case — it's now spawned via `_sceneShipJoke()`
