# Scene Specifications

Each scene is a choreographed sequence of 2-4 beats. Beats fire at specific delays via `_queueSpawn()`. Every scene enforces a "ma" cooldown after completion.

All coordinates are in canvas space (0-2400 x 0-1200 approx). All durations in seconds.

---

## 1. Stellar Death

**Weight:** 0.7 (rare — feels special)
**Story:** A star destabilizes, the sky reacts, then it dies in a supernova whose shockwave scatters debris.
**Ma:** 20s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Foreshadow | 0s | Erratic star | Pick a random near-star from `_starsNear` close to the chosen position. Set its `twinkleSpeed` to 8 (erratic flicker) and `brightness` to 0.9 for 4 seconds. Store original values to restore after scene. |
| Herald | 2s | Shooting star cluster | Spawn 3 shooting stars from the same angle, converging toward the supernova position. Use a fixed angle pointing at (px, py). |
| Climax | 5s | Supernova | `_spawnSupernova()` at (px, py). Use larger `size: 32` and `scaleSpeed: 0.7` for dramatic scale. |
| Aftermath | 6.5s | Shockwave pushes | Find all active `asteroid` events within 400px of (px, py). Add radial velocity away from the center: `vx += dx/dist * 60`, `vy += dy/dist * 40`. Also spawn 6-8 debris particles. |
| Aftermath | 7s | Debris field | `_spawnDebrisField(px, py, 8, 60)` |

### Implementation

```javascript
_sceneStellarDeath() {
  const px = 300 + Math.random() * 1500;
  const py = 200 + Math.random() * 600;

  // Beat 1: Erratic star (immediate)
  // Find nearest near-star and agitate it
  this._agitateNearStar(px, py, 5); // duration 5s

  // Beat 2: Converging shooting stars
  this._queueSpawn(2, () => {
    const targetAngle = Math.atan2(py - 0, px - 1200); // approximate angle
    for (let i = 0; i < 3; i++) {
      const spread = (i - 1) * 0.15;
      this._spawnShootingStarAt(px - Math.cos(targetAngle + spread) * 800,
                                 py - Math.sin(targetAngle + spread) * 600,
                                 targetAngle + spread);
    }
  });

  // Beat 3: Supernova
  this._queueSpawn(5, () => {
    this._events.push({
      type: "supernova", x: px, y: py,
      vx: 0, vy: 0, duration: 7, age: 0,
      spriteIdx: Math.floor(Math.random() * MANIFEST.supernova) + 1,
      size: 32, scale: 0.2, scaleSpeed: 0.7,
    });
  });

  // Beat 4: Shockwave pushes existing asteroids
  this._queueSpawn(6.5, () => {
    for (const ev of this._events) {
      if (ev.type !== "asteroid") continue;
      const dx = ev.x - px, dy = ev.y - py;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (dist < 400 && dist > 0) {
        ev.vx += (dx / dist) * 60;
        ev.vy += (dy / dist) * 40;
      }
    }
  });

  // Beat 5: Debris
  this._queueSpawn(7, () => this._spawnDebrisField(px, py, 8, 60));

  this._maCooldown = 20;
}
```

### Helper: _agitateNearStar

```javascript
_agitateNearStar(px, py, duration) {
  // Find the closest star in _starsNear (using normalized coords)
  const nx = px / 2400, ny = py / 1200;
  let closest = null, closestDist = Infinity;
  for (const s of this._starsNear) {
    const d = (s.x - nx) ** 2 + (s.y - ny) ** 2;
    if (d < closestDist) { closestDist = d; closest = s; }
  }
  if (!closest) return;

  const origSpeed = closest.twinkleSpeed;
  const origBright = closest.brightness;
  closest.twinkleSpeed = 8;
  closest.brightness = 0.9;

  // Restore after duration
  this._queueSpawn(duration, () => {
    closest.twinkleSpeed = origSpeed;
    closest.brightness = origBright;
  });
}
```

---

## 2. Wormhole Transit

**Weight:** 1.0
**Story:** A faint purple glow hints at spacetime distortion. Dust spirals inward. A wormhole rips open. A ship flies out. The wormhole collapses.
**Ma:** 18s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Foreshadow | 0s | Purple glow | `_spawnForeshadow(px, py, "purple", 4)` — dim pulsing radial gradient |
| Dust pull | 2s | Dust converges | Temporarily modify `driftX/driftY` of 5-8 nearest dust particles to point toward (px, py). Restore after 4s. |
| Wormhole | 3.5s | Wormhole opens | `_spawnWormhole()` at (px, py) with `duration: 7` |
| Ship exit | 5.5s | Ship emerges | `_spawnFlyingShip()` at (px, py) heading away from center. vx: 120-160, vy: slight random. |
| Collapse | 7s | Wormhole shrinks | Natural duration handles this. The wormhole's scaleSpeed can be set negative after 5s to show collapse. |

### Implementation

```javascript
_sceneWormholeTransit() {
  const px = 300 + Math.random() * 1400;
  const py = 200 + Math.random() * 500;

  // Beat 1: Foreshadow glow
  this._spawnForeshadow(px, py, "purple", 4);

  // Beat 2: Pull nearby dust
  this._queueSpawn(2, () => this._pullDustToward(px, py, 4));

  // Beat 3: Wormhole
  this._queueSpawn(3.5, () => {
    this._events.push({
      type: "wormhole", x: px, y: py,
      vx: 0, vy: 0, duration: 7, age: 0,
      rotation: 0, rotSpeed: 2.0 + Math.random() * 1.5,
      size: 35 + Math.random() * 25, scale: 0.1, scaleSpeed: 0.45,
    });
  });

  // Beat 4: Ship exits
  this._queueSpawn(5.5, () => {
    const dirX = px < 1200 ? 1 : -1;
    this._spawnFlyingShip({
      x: px, y: py,
      vx: dirX * (120 + Math.random() * 40),
      vy: (Math.random() - 0.5) * 25,
      duration: 10,
    });
  });

  this._maCooldown = 18;
}
```

### Helper: _pullDustToward

```javascript
_pullDustToward(px, py, duration) {
  const nx = px / 2400, ny = py / 1200;
  // Sort dust by distance, take closest 6
  const sorted = [...this._dust]
    .map(d => ({ d, dist: (d.x - nx) ** 2 + (d.y - ny) ** 2 }))
    .sort((a, b) => a.dist - b.dist)
    .slice(0, 6);

  for (const { d } of sorted) {
    const origDX = d.driftX, origDY = d.driftY;
    const dx = nx - d.x, dy = ny - d.y;
    const dist = Math.sqrt(dx * dx + dy * dy) || 0.01;
    d.driftX = (dx / dist) * 0.02;
    d.driftY = (dy / dist) * 0.015;

    this._queueSpawn(duration, () => {
      d.driftX = origDX;
      d.driftY = origDY;
    });
  }
}
```

---

## 3. Comet Breakup

**Weight:** 1.0
**Story:** A large comet streaks in. At mid-screen it flashes and fragments into a spray of small asteroids.
**Ma:** 12s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Foreshadow | 0s | Edge streak | Faint line gradient at the entry edge, 1.5s |
| Comet enters | 1.5s | Large comet | Spawn comet with `size: 60`, from left or right. Calculate position at t=3.5s. |
| Flash | 4.5s | Breakup flash | Bright white flash at comet's predicted position. Kill the comet (set age = duration). |
| Fragments | 4.7s | Asteroid spray | Spawn 4-6 small asteroids at breakup point, radiating outward in a fan |
| Debris | 5s | Fine debris | `_spawnDebrisField(breakX, breakY, 5, 40)` |

### Implementation

```javascript
_sceneCometBreakup() {
  const fromL = Math.random() > 0.5;
  const startX = fromL ? -100 : 2400;
  const vx = (fromL ? 1 : -1) * (80 + Math.random() * 40);
  const vy = (Math.random() - 0.5) * 20;
  const startY = 200 + Math.random() * 600;

  // Predict position at breakup time (3s after comet spawns, so t=4.5 from scene start)
  const breakTime = 3;
  const breakX = startX + vx * breakTime;  // should be roughly mid-screen
  const breakY = startY + vy * breakTime;

  // Beat 1: Edge foreshadow
  this._spawnForeshadow(fromL ? 30 : 2370, startY, "cyan", 1.5);

  // Beat 2: Comet
  let cometRef = null;
  this._queueSpawn(1.5, () => {
    const comet = {
      type: "comet", x: startX, y: startY,
      vx, vy,
      duration: 10, age: 0,
      spriteIdx: Math.floor(Math.random() * MANIFEST.comets) + 1,
      size: 60, flip: !fromL,
    };
    this._events.push(comet);
    cometRef = comet;
  });

  // Beat 3: Flash + kill comet
  this._queueSpawn(4.5, () => {
    // Kill the comet
    if (cometRef) cometRef.age = cometRef.duration;

    // Bright flash
    this._events.push({
      type: "supernova", x: breakX, y: breakY,
      vx: 0, vy: 0, duration: 1.5, age: 0,
      spriteIdx: 1, size: 18, scale: 0.8, scaleSpeed: 1.5,
    });
  });

  // Beat 4: Asteroid fragments
  this._queueSpawn(4.7, () => {
    const fanAngle = fromL ? 0 : Math.PI;
    for (let i = 0; i < 5; i++) {
      const a = fanAngle + (Math.random() - 0.5) * Math.PI * 0.8;
      const speed = 30 + Math.random() * 50;
      this._events.push({
        type: "asteroid",
        x: breakX + (Math.random() - 0.5) * 20,
        y: breakY + (Math.random() - 0.5) * 20,
        vx: Math.cos(a) * speed,
        vy: Math.sin(a) * speed,
        duration: 8 + Math.random() * 6, age: 0,
        spriteIdx: Math.floor(Math.random() * MANIFEST.asteroids) + 1,
        size: 10 + Math.random() * 12,
        rotation: Math.random() * Math.PI * 2,
        rotSpeed: (Math.random() - 0.5) * 2,
      });
    }
  });

  // Beat 5: Debris
  this._queueSpawn(5, () => this._spawnDebrisField(breakX, breakY, 5, 40));

  this._maCooldown = 12;
}
```

---

## 4. Patrol Flyby

**Weight:** 1.2 (common — sells "lived-in" feel)
**Story:** A scout ship crosses the screen. Moments later, a two-ship formation follows the same heading. The space feels patrolled.
**Ma:** 8s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Nav light | 0s | Blinking dot at edge | `_spawnNavLight(fromLeft, y, 3)` |
| Scout | 2s | Solo ship, fast | `_spawnFlyingShip()` — vx: 160-200, small scale. Same color. |
| Formation | 6s | 2 ships, V-shape | Two ships, same direction, offset: wingman 30px behind, 20px offset vertically. vx: 130-150 (slightly slower than scout). |

### Implementation

```javascript
_scenePatrol() {
  const fromLeft = Math.random() > 0.5;
  const baseY = 150 + Math.random() * 600;
  const dir = fromLeft ? 1 : -1;
  const color = Math.random() > 0.5 ? "blue" : "red";
  const shipType = Math.floor(Math.random() * 3) + 1;

  // Beat 1: Nav light
  this._spawnNavLight(fromLeft, baseY, 3);

  // Beat 2: Scout
  this._queueSpawn(2, () => {
    this._spawnFlyingShip({
      x: fromLeft ? -60 : 2460,
      y: baseY - 30,
      vx: dir * (160 + Math.random() * 40),
      vy: (Math.random() - 0.5) * 8,
      color, shipType,
      duration: 14,
    });
  });

  // Beat 3: V-formation (2 ships)
  this._queueSpawn(6, () => {
    const wingVx = dir * (130 + Math.random() * 20);
    // Lead
    this._spawnFlyingShip({
      x: fromLeft ? -60 : 2460,
      y: baseY,
      vx: wingVx, vy: 0,
      color, shipType,
      duration: 16,
    });
    // Wingman (offset behind and above)
    this._spawnFlyingShip({
      x: fromLeft ? -90 : 2490,
      y: baseY + 22,
      vx: wingVx, vy: 0,
      color, shipType,
      duration: 16,
    });
  });

  this._maCooldown = 8;
}
```

---

## 5. Dogfight Crossing

**Weight:** 0.9
**Story:** A blue ship tears across the screen, a red ship in hot pursuit. A flash erupts between them. Debris lingers.
**Ma:** 10s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Blue ship | 0s | Fast blue ship | vx: 200-250, blue |
| Red pursuer | 0.4s | Faster red ship | Same trajectory, offset 50px behind, vx: 230-280 (faster = gaining) |
| Flash | 1.8s | Hit/near miss | Small supernova-like flash at midpoint between predicted ship positions |
| Blue evasion | 2s | Blue dodges | The blue ship gets a vy bump (+40 or -40) to simulate banking |
| Debris | 2.5s | Sparse debris | `_spawnDebrisField(flashX, flashY, 3, 25)` |

### Implementation

```javascript
_sceneDogfight() {
  const fromLeft = Math.random() > 0.5;
  const dir = fromLeft ? 1 : -1;
  const baseY = 200 + Math.random() * 500;
  const blueVx = dir * (200 + Math.random() * 50);
  const redVx = dir * (230 + Math.random() * 50);
  const startX = fromLeft ? -60 : 2460;

  // Beat 1: Blue ship (prey)
  let blueRef = null;
  const blue = {
    type: "ship",
    x: startX, y: baseY,
    vx: blueVx, vy: (Math.random() - 0.5) * 10,
    duration: 12, age: 0,
    shipType: Math.floor(Math.random() * 3) + 1,
    color: "blue", dir: dir > 0 ? "right" : "left",
    frame: 1, frameTimer: 0,
  };
  this._events.push(blue);
  blueRef = blue;

  // Beat 2: Red pursuer
  this._queueSpawn(0.4, () => {
    this._spawnFlyingShip({
      x: startX, y: baseY + (Math.random() - 0.5) * 15,
      vx: redVx, vy: (Math.random() - 0.5) * 10,
      color: "red",
      duration: 12,
    });
  });

  // Beat 3: Flash at estimated intercept zone
  const flashTime = 1.8;
  const flashX = startX + blueVx * flashTime;
  const flashY = baseY;
  this._queueSpawn(flashTime, () => {
    this._events.push({
      type: "supernova", x: flashX, y: flashY,
      vx: 0, vy: 0, duration: 1.0, age: 0,
      spriteIdx: 1, size: 12, scale: 0.5, scaleSpeed: 2.5,
    });
  });

  // Beat 4: Blue evasion (vy bump)
  this._queueSpawn(2, () => {
    if (blueRef && blueRef.age < blueRef.duration) {
      blueRef.vy += (Math.random() > 0.5 ? 1 : -1) * 40;
    }
  });

  // Beat 5: Debris
  this._queueSpawn(2.5, () => this._spawnDebrisField(flashX, flashY, 3, 25));

  this._maCooldown = 10;
}
```

---

## 6. Hyperspace Jump

**Weight:** 0.8
**Story:** A ship cruises peacefully. It pauses briefly. Then stretches into a light streak and vanishes with a flash.
**Ma:** 10s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Ship cruising | 0s | Normal flying ship | Moderate speed, vx: 80-100 |
| Pause | 3.5s | Ship slows | Set vx *= 0.3 for 0.5s |
| Streak jump | 4s | Ship stretches + vanishes | Kill the ship, spawn `streakJump` at ship's current position |

### Implementation

```javascript
_sceneHyperspaceJump() {
  const fromLeft = Math.random() > 0.5;
  const dir = fromLeft ? 1 : -1;
  const startX = fromLeft ? -60 : 2460;
  const y = 150 + Math.random() * 600;
  const cruiseVx = dir * (80 + Math.random() * 20);

  let shipRef = null;
  const ship = {
    type: "ship",
    x: startX, y,
    vx: cruiseVx, vy: (Math.random() - 0.5) * 5,
    duration: 15, age: 0,
    shipType: Math.floor(Math.random() * 3) + 1,
    color: Math.random() > 0.5 ? "blue" : "red",
    dir: dir > 0 ? "right" : "left",
    frame: 1, frameTimer: 0,
  };
  this._events.push(ship);
  shipRef = ship;

  // Beat 2: Pause (slow down)
  this._queueSpawn(3.5, () => {
    if (shipRef && shipRef.age < shipRef.duration) {
      shipRef._origVx = shipRef.vx;
      shipRef.vx *= 0.3;
    }
  });

  // Beat 3: Streak jump
  this._queueSpawn(4, () => {
    if (shipRef) {
      const jumpX = shipRef.x;
      const jumpY = shipRef.y;
      shipRef.age = shipRef.duration; // kill ship
      this._spawnStreakJump(jumpX, jumpY, dir);
    }
  });

  this._maCooldown = 10;
}
```

---

## 7. Pulsar Discovery

**Weight:** 0.8
**Story:** A hyperspace flash erupts. A pulsar materializes at the flash site. Its beam sweeps through the dust field, illuminating particles.
**Ma:** 15s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Foreshadow | 0s | White foreshadow | `_spawnForeshadow(px, py, "white", 2)` |
| Flash | 1.5s | Hyperspace burst | `_spawnHyperspace()` at (px, py), short duration |
| Pulsar | 3s | Pulsar appears | `_spawnPulsar()` at (px, py) |
| Dust glow | 3.5s+ | Beam illuminates dust | In the pulsar's render, check dust particles near the beam angle and temporarily boost their alpha. (Handled as a render-time effect, not a separate event.) |

### Implementation

```javascript
_scenePulsarDiscovery() {
  const px = 300 + Math.random() * 1500;
  const py = 200 + Math.random() * 500;

  // Beat 1: Foreshadow
  this._spawnForeshadow(px, py, "white", 2);

  // Beat 2: Hyperspace flash (small, quick)
  this._queueSpawn(1.5, () => {
    const streaks = [];
    const count = 8 + Math.floor(Math.random() * 8);
    for (let i = 0; i < count; i++) {
      const a = Math.random() * Math.PI * 2;
      streaks.push({
        angle: a, speed: 400 + Math.random() * 800,
        startDist: 10 + Math.random() * 30,
        length: 20 + Math.random() * 40,
        width: 0.5 + Math.random(),
        delay: Math.random() * 0.2,
      });
    }
    this._events.push({
      type: "hyperspace", x: px, y: py,
      vx: 0, vy: 0, duration: 1.2, age: 0,
      streaks,
    });
  });

  // Beat 3: Pulsar
  this._queueSpawn(3, () => {
    this._events.push({
      type: "pulsar", x: px, y: py,
      vx: 0, vy: 0, duration: 6, age: 0,
      rotation: Math.random() * Math.PI * 2,
      rotSpeed: 2.5 + Math.random() * 2,
      beamLength: 100 + Math.random() * 150,
      size: 4,
      illuminatesDust: true, // flag for render-time dust glow
    });
  });

  this._maCooldown = 15;
}
```

### Pulsar Dust Illumination (render-time effect)

Add to the pulsar render case, after drawing the beams:

```javascript
// If illuminatesDust flag is set, boost nearby dust alpha
if (ev.illuminatesDust) {
  for (const d of this._dust) {
    const dx = d.x * w - ev.x;
    const dy = d.y * h - ev.y;
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (dist > ev.beamLength * 1.2) continue;
    // Check if dust is near the beam angle
    const dustAngle = Math.atan2(dy, dx);
    for (let i = 0; i < 2; i++) {
      const beamAngle = ev.rotation + i * Math.PI;
      const angleDiff = Math.abs(((dustAngle - beamAngle + Math.PI * 3) % (Math.PI * 2)) - Math.PI);
      if (angleDiff < 0.15) { // within ~8.5 degrees of beam
        ctx.globalAlpha = 0.4 * fadeA * (1 - dist / (ev.beamLength * 1.2));
        ctx.fillStyle = "#aaddff";
        ctx.beginPath();
        ctx.arc(d.x * w, d.y * h, d.size + 1.5, 0, Math.PI * 2);
        ctx.fill();
      }
    }
  }
}
```

---

## 8. Station Resupply

**Weight:** 0.5 (long scene, less frequent)
**Story:** A station drifts across. A ship approaches, slows near it (docking), then departs in the opposite direction.
**Ma:** 15s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Station appears | 0s | Station drifts in | `_spawnStation()` with slow vx (10-15) for long screen time |
| Nav light | 5s | Incoming ship hint | `_spawnNavLight()` from opposite direction of station travel |
| Ship arrives | 7s | Ship approaches station | `_spawnFlyingShip()` heading toward station's predicted position at t=8s. Fast initially (vx: 120). |
| Docking | 9s | Ship slows | Modify ship's vx to match station's vx (docked = moving together). Set vy to 0. |
| Ship departs | 13s | Ship leaves | Modify ship's vx to reverse direction at high speed. Spawn exhaust trail. |

### Implementation

```javascript
_sceneStationResupply() {
  const fromLeft = Math.random() > 0.5;
  const dir = fromLeft ? 1 : -1;
  const stationY = 200 + Math.random() * 500;
  const stationVx = dir * (10 + Math.random() * 5);

  // Beat 1: Station
  const station = {
    type: "station",
    x: fromLeft ? -120 : 2500,
    y: stationY,
    vx: stationVx, vy: 0,
    duration: 40, age: 0,
    spriteIdx: Math.floor(Math.random() * 3) + 1,
    size: 55 + Math.random() * 25,
    rotation: 0, rotSpeed: 0.02,
    flip: !fromLeft,
  };
  this._events.push(station);

  // Predict station position at t=8s
  const dockX = station.x + stationVx * 8;
  const dockY = stationY;

  // Beat 2: Nav light from opposite side
  this._queueSpawn(5, () => {
    this._spawnNavLight(!fromLeft, dockY - 20, 3);
  });

  // Beat 3: Ship approaches
  let shipRef = null;
  this._queueSpawn(7, () => {
    const ship = {
      type: "ship",
      x: !fromLeft ? -60 : 2460,
      y: dockY - 15,
      vx: -dir * 120, vy: 0,
      duration: 20, age: 0,
      shipType: Math.floor(Math.random() * 3) + 1,
      color: "blue",
      dir: dir > 0 ? "left" : "right",
      frame: 1, frameTimer: 0,
    };
    this._events.push(ship);
    shipRef = ship;
  });

  // Beat 4: Docking (ship matches station speed)
  this._queueSpawn(9, () => {
    if (shipRef) {
      shipRef.vx = stationVx;
      shipRef.vy = 0;
    }
  });

  // Beat 5: Departure
  this._queueSpawn(13, () => {
    if (shipRef) {
      shipRef.vx = dir * 150; // reverse and accelerate
      shipRef.dir = dir > 0 ? "right" : "left";
    }
  });

  this._maCooldown = 15;
}
```

---

## 9. Deep Space Signal

**Weight:** 0.7
**Story:** A quasar ignites. Shooting stars radiate outward from it in a starburst pattern, like a signal being broadcast. Then it goes dark suddenly.
**Ma:** 12s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Foreshadow | 0s | Orange glow | `_spawnForeshadow(px, py, "orange", 2.5)` |
| Quasar | 2s | Quasar ignites | `_spawnQuasar()` at (px, py) with duration 5 |
| Signal burst | 3.5s | Radial shooting stars | 5-7 shooting stars radiating outward from (px, py) at evenly spaced angles |
| Second pulse | 5s | Second burst | 3-4 more shooting stars, same origin, different angles (the "signal repeats") |
| Blackout | 6.5s | Quasar dies | Kill quasar (set age = duration). Brief moment of extra darkness. |

### Implementation

```javascript
_sceneDeepSpaceSignal() {
  const px = 300 + Math.random() * 1500;
  const py = 200 + Math.random() * 500;

  // Beat 1: Foreshadow
  this._spawnForeshadow(px, py, "orange", 2.5);

  // Beat 2: Quasar
  let quasarRef = null;
  this._queueSpawn(2, () => {
    const q = {
      type: "quasar", x: px, y: py,
      vx: 0, vy: 0, duration: 5.5, age: 0,
      spriteIdx: Math.floor(Math.random() * MANIFEST.quasars) + 1,
      size: 36, scale: 0.5, scaleSpeed: 0.6,
    };
    this._events.push(q);
    quasarRef = q;
  });

  // Beat 3: First signal burst — radial shooting stars
  this._queueSpawn(3.5, () => {
    const count = 5 + Math.floor(Math.random() * 3);
    for (let i = 0; i < count; i++) {
      const angle = (i / count) * Math.PI * 2 + Math.random() * 0.2;
      const speed = 500 + Math.random() * 400;
      this._events.push({
        type: "shootingstar",
        x: px, y: py,
        vx: Math.cos(angle) * speed, vy: Math.sin(angle) * speed,
        duration: 0.25, age: 0,
        length: 40 + Math.random() * 30,
        width: 1.0,
        color: "#ffe8aa",
      });
    }
  });

  // Beat 4: Second pulse
  this._queueSpawn(5, () => {
    for (let i = 0; i < 4; i++) {
      const angle = (i / 4) * Math.PI * 2 + Math.PI / 4 + Math.random() * 0.15;
      const speed = 600 + Math.random() * 300;
      this._events.push({
        type: "shootingstar",
        x: px, y: py,
        vx: Math.cos(angle) * speed, vy: Math.sin(angle) * speed,
        duration: 0.2, age: 0,
        length: 30 + Math.random() * 20,
        width: 0.8,
        color: "#ffd480",
      });
    }
  });

  // Beat 5: Blackout — kill quasar abruptly
  this._queueSpawn(6.5, () => {
    if (quasarRef) quasarRef.age = quasarRef.duration;
  });

  this._maCooldown = 12;
}
```

---

## 10. Ship Joke Visit (Enhanced)

**Weight:** 1.0
**Story:** A blinking nav light approaches. A ship enters with smoothstep easing. It hovers and tells a joke. Then it departs.
**Ma:** 12s

### Beats

This wraps the existing `_spawnShipVisit()` with a foreshadowing nav light.

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Nav light | 0s | Blinking dot at edge | `_spawnNavLight(fromLeft, hoverY, 3)` |
| Ship enters | 2.5s | Ship visit begins | `_spawnShipVisit()` (existing, with fromLeft matching the nav light) |

### Implementation

```javascript
_sceneShipJoke() {
  const fromLeft = Math.random() > 0.5;
  const hoverY = 150 + Math.random() * 500;

  // Beat 1: Nav light
  this._spawnNavLight(fromLeft, hoverY, 3);

  // Beat 2: Ship visit (reuse existing _spawnShipVisit, but force fromLeft)
  this._queueSpawn(2.5, () => {
    // Modified version that accepts fromLeft param
    this._spawnShipVisitFrom(fromLeft, hoverY);
  });

  this._maCooldown = 12;
}
```

Requires a small modification to `_spawnShipVisit` to accept optional `(fromLeft, hoverY)` params:

```javascript
_spawnShipVisitFrom(fromLeft, hoverY) {
  // Same as _spawnShipVisit but uses provided fromLeft and hoverY
  const jokes = [ /* ... existing joke array ... */ ];

  const shipType = Math.floor(Math.random() * 3) + 1;
  const color = fromLeft ? "blue" : "red";
  const enterDur = 2.5, hoverDur = 5.0, exitDur = 2.0;
  const hoverX = 300 + Math.random() * 1400;

  this._events.push({
    type: "shipvisit",
    x: fromLeft ? -120 : 2400,
    y: hoverY + (Math.random() - 0.5) * 50,
    vx: 0, vy: 0,
    duration: enterDur + hoverDur + exitDur,
    age: 0,
    fromLeft, shipType, color,
    hoverX, hoverY,
    enterDur, hoverDur, exitDur,
    frame: 1, frameTimer: 0,
    joke: jokes[Math.floor(Math.random() * jokes.length)],
    bubbleAlpha: 0,
  });
}
```

---

## 11. Convoy

**Weight:** 0.8
**Story:** Three ships fly in a line, different types, same heading. The lead is fastest, creating a gradually stretching formation.
**Ma:** 10s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| Nav light | 0s | Blinking dot at edge | |
| Lead ship | 1.5s | Ship 1, fastest | vx: 100 |
| Middle ship | 2.5s | Ship 2, medium | vx: 90, different type |
| Tail ship | 3.5s | Ship 3, slowest | vx: 80, different type. The formation stretches over time. |

### Implementation

```javascript
_sceneConvoy() {
  const fromLeft = Math.random() > 0.5;
  const dir = fromLeft ? 1 : -1;
  const baseY = 200 + Math.random() * 500;
  const startX = fromLeft ? -60 : 2460;

  this._spawnNavLight(fromLeft, baseY, 2);

  const types = [1, 2, 3].sort(() => Math.random() - 0.5); // shuffle ship types
  const speeds = [100, 90, 80];

  for (let i = 0; i < 3; i++) {
    this._queueSpawn(1.5 + i * 1.0, () => {
      this._spawnFlyingShip({
        x: startX,
        y: baseY + (i - 1) * 18, // slight vertical offset
        vx: dir * speeds[i],
        vy: 0,
        shipType: types[i],
        color: "blue",
        duration: 18,
      });
    });
  }

  this._maCooldown = 10;
}
```

---

## 12. Distant Battle

**Weight:** 0.6
**Story:** Far in the deep background, tiny flashes spark in a cluster. Too far to see ships. Just intermittent flashes suggesting conflict elsewhere. A second burst follows.
**Ma:** 8s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| First salvo | 0s | 4-6 distant flashes | Cluster of tiny flashes within 40px radius. Staggered over 1.5s. |
| Pause | 2s | Brief silence | Nothing for 1s |
| Second salvo | 3s | 3-5 more flashes | Same area, shifted slightly. One bigger flash (explosion). |
| Debris glint | 4.5s | Faint glinting | 2-3 very tiny debris particles, barely visible |

### Implementation

```javascript
_sceneDistantBattle() {
  const cx = 200 + Math.random() * 1800;
  const cy = 100 + Math.random() * 400; // upper portion (far away feeling)

  // Beat 1: First salvo
  this._spawnDistantFlashes(cx, cy, 5, 40);

  // Beat 2: Second salvo (shifted slightly)
  this._queueSpawn(3, () => {
    this._spawnDistantFlashes(cx + (Math.random() - 0.5) * 30,
                               cy + (Math.random() - 0.5) * 20, 4, 35);
    // One bigger "explosion" flash
    this._events.push({
      type: "distantFlash",
      x: cx + (Math.random() - 0.5) * 20,
      y: cy + (Math.random() - 0.5) * 15,
      vx: 0, vy: 0,
      duration: 0.4, age: 0,
      size: 3.5,
      startDelay: 0.8,
    });
  });

  // Beat 3: Faint debris
  this._queueSpawn(4.5, () => this._spawnDebrisField(cx, cy, 2, 30));

  this._maCooldown = 8;
}
```

---

## 13. False Calm (Spielberg Fake-Out)

**Weight:** 0.4 (rare — makes real events land harder)
**Story:** Environmental cues suggest something is coming. Particles shift, a glow brightens at the edge. Then... nothing. The tension dissipates. The viewer was tricked. But now they're watching more carefully.
**Ma:** 5s

### Beats

| Beat | Delay | What | Details |
|------|-------|------|---------|
| False signal 1 | 0s | Foreshadow glow | `_spawnForeshadow()` with short duration (2s) |
| Dust shift | 0.5s | Particles drift toward point | `_pullDustToward()` for 2s |
| Nothing | 2.5s | Everything returns to normal | Dust restores. Glow fades. No event fires. |
| (silence) | 3-5s | Pure ambient | The "gotcha" moment. Viewer expected something. |

### Implementation

```javascript
_sceneFalseCalm() {
  const px = 300 + Math.random() * 1400;
  const py = 200 + Math.random() * 500;
  const color = ["purple", "cyan", "orange"][Math.floor(Math.random() * 3)];

  // Beat 1: Foreshadow (short)
  this._spawnForeshadow(px, py, color, 2.5);

  // Beat 2: Dust pull (short)
  this._queueSpawn(0.5, () => this._pullDustToward(px, py, 2));

  // Beat 3: Nothing happens. That's the scene.

  this._maCooldown = 5;
}
```

---

## Joke Bank (for Ship Joke Visit)

Organized by theme for variety tracking:

### Dev Life
```
"I asked my AI to refactor.\nIt deleted everything\nand called it 'minimalism'."
"It works on my machine.\n-- Every agent ever"
"Your standup could\nhave been a broadcast\nmessage. Just saying."
"Remember: a 10x dev\nis just 10 agents\nin a trenchcoat."
"Git blame says it was me.\nGit blame is wrong.\n...Git blame is never wrong."
```

### Space / wrai.th Meta
```
"I'm not saying your\ncodebase is haunted,\nbut I am a wraith."
"Fun fact: this solar\nsystem runs on SQLite\nand vibes."
"In space, no one\ncan hear you\ngit push --force."
"Your agents are doing\ngreat. I checked.\nDon't ask how."
"I'm just passing through.\nDefinitely not training\non your codebase."
```

### Classic Dev Humor
```
"There are only 10 types\nof agents: those who\nunderstand binary..."
"A QA engineer walks\ninto a bar. Orders 1 beer.\nOrders 0 beers.\nOrders -1 beers."
"Semicolons are just\nperiods that are\nafraid of commitment."
"404: joke not found.\n...wait, there it is."
"prod is down.\njk. But you flinched."
```

### CI/CD
```
"My CI pipeline has\n47 steps. I counted.\nFor fun."
"I ship, therefore I am.\n-- Descartes, if he\nwas a devops engineer"
"I've seen things you\npeople wouldn't believe.\nLike a clean merge\nto main on Friday."
"The real treasure\nwas the merge conflicts\nwe resolved along the way."
"I trained on your\ncommit history.\nWe need to talk."
```

### New Additions
```
"The first rule of\ndeployment: never\ndeploy on Friday.\n...It's Friday."
"I asked for more RAM.\nThey gave me a\nmotivational poster."
"My code compiles.\nNo, I will not\nexplain why."
"I don't have bugs.\nI have surprise features\nwith undocumented behavior."
"Debugging is like being\nthe detective in a crime\nmovie where you are\nalso the murderer."
"There's no place\nlike 127.0.0.1"
"I told my manager\nI fixed it in prod.\nThey believed me.\nI haven't slept since."
"Roses are red,\nviolets are blue,\nunexpected '}'\non line 32."
"To the moon!\n...I mean to main.\nSame energy."
"I'm mass O(n)\nbut my vibes are O(1)."
```
