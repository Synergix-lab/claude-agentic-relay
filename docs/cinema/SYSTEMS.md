# Supporting Systems

Foreshadowing, cause-and-effect, ambient traffic, "ma" enforcement, and the campfire effect.

---

## 1. Foreshadowing System

Every narrative event MUST be preceded by a subtle visual cue. The viewer's brain registers it subconsciously, making the event feel "inevitable yet unforeseen" when it arrives.

### Foreshadowing Rules

- Opacity: 10-20% of event's peak opacity
- Duration: 2-5 seconds before the event
- Always fade in/out with sinusoidal easing (never pop)
- Placed at the EXACT position where the event will occur

### Foreshadow Types

| Preceding Event | Foreshadow Type | Visual | Duration |
|-----------------|----------------|--------|----------|
| Supernova | Erratic star | Existing star in `_starsNear` flickers fast, brightens to 0.9 | 4-5s |
| Wormhole | Purple glow | `foreshadow` event, purple radial gradient, pulsing | 3-4s |
| Quasar | Orange glow | `foreshadow` event, orange radial gradient | 2-3s |
| Pulsar | White glow | `foreshadow` event, white radial gradient | 2s |
| Ship arrival | Nav light | `navlight` event, blinking dot approaching from edge | 3-5s |
| Comet | Edge streak | `foreshadow` event at screen edge, cyan, narrow | 1.5s |
| Hyperspace | Dust convergence | Modify dust `driftX/driftY` to point toward origin | 2-3s |
| Meteor shower | Lone shooting stars | 1-2 `shootingstar` events at matching angle | 3-6s |

### Implementation Pattern

Every scene method should start with its foreshadow:

```javascript
_sceneExample() {
  const px = ..., py = ...;

  // ALWAYS: Foreshadow first
  this._spawnForeshadow(px, py, "purple", 3);

  // THEN: Actual event (delayed)
  this._queueSpawn(3, () => { /* the real event */ });
}
```

---

## 2. Cause-and-Effect System

When certain events occur, they should physically affect other active events. This makes the universe feel cohesive.

### Effect Table

| Cause Event | Affected Events | Effect | Radius |
|-------------|----------------|--------|--------|
| **Supernova** | Active `asteroid` events | Add radial velocity away from supernova center. `vx += (dx/dist) * 60`, `vy += (dy/dist) * 40` | 400px |
| **Supernova** | (spawn) | Spawn 6-8 `debris` particles at supernova position | -- |
| **Comet** (breakup) | (spawn) | Spawn 4-6 small `asteroid` events in a fan pattern | -- |
| **Wormhole** | (spawn) | Spawn `ship` event at wormhole position, heading outward | -- |
| **Hyperspace** flash | (spawn) | 3-5 `shootingstar` events radiating from flash center | -- |
| **Pulsar** beam | `_dust` particles | Temporarily boost alpha of dust particles within beam angle (render-time) | beamLength * 1.2 |
| **Dogfight** flash | (spawn) | 3 `debris` particles at flash point | -- |
| **Comet** (normal) | (spawn, optional) | Every 2s, 20% chance to spawn tiny `debris` along trail | -- |

### Implementation

Effects that modify existing events should run in `_queueSpawn` callbacks at the appropriate time. Effects that are render-time (like pulsar illuminating dust) should be flagged on the event and handled in `_renderEvent`.

```javascript
// Example: supernova shockwave
this._queueSpawn(6.5, () => {
  for (const ev of this._events) {
    if (ev.type !== "asteroid") continue;
    const dx = ev.x - supernovaX;
    const dy = ev.y - supernovaY;
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (dist < 400 && dist > 0) {
      ev.vx += (dx / dist) * 60;
      ev.vy += (dy / dist) * 40;
    }
  }
});
```

---

## 3. "Ma" Enforcement (Post-Event Silence)

After every narrative scene, enforce a period of ambient-only silence. No new scenic or narrative events spawn. Only the base layers continue (starfield, nebulae, dust, aurora).

### Ma Duration Table

| Scene | Ma Duration | Why |
|-------|------------|-----|
| Stellar Death | 20s | Massive event needs long cool-down |
| Wormhole Transit | 18s | Complex sequence, let it breathe |
| Comet Breakup | 12s | Medium event |
| Patrol Flyby | 8s | Light scene, short pause |
| Dogfight | 10s | Action scene, moderate pause |
| Hyperspace Jump | 10s | Quick but dramatic |
| Pulsar Discovery | 15s | Atmospheric, needs space |
| Station Resupply | 15s | Long scene, proportional pause |
| Deep Space Signal | 12s | Medium intensity |
| Ship Joke Visit | 12s | Let the joke land |
| Convoy | 10s | Light traffic scene |
| Distant Battle | 8s | Subtle, short pause |
| False Calm | 5s | The silence IS the scene |

### Ma Behavior

During "ma":
- `_maCooldown > 0` blocks all event spawning in `_phaseSpawn()`
- Ambient layers become **slightly more active**:
  - Nebula pulse amplitude: +20% (`nc.alpha * pulse * 1.2`)
  - Star twinkle amplitude: +15%
  - This makes the "silence" feel alive, like Miyazaki's train scene in Spirited Away

### Implementation

```javascript
// In _phaseSpawn():
if (this._maCooldown > 0) return; // hard block

// In nebula render (during ma):
const maBoost = this._maCooldown > 0 ? 1.2 : 1.0;
const a = nc.alpha * pulse * maBoost;
```

---

## 4. Ambient Traffic (Between Scenes)

Between choreographed scenes, small ambient traffic events keep the space feeling inhabited. These are NOT part of scenes — they're independent background activity.

### Ambient Traffic is ONLY allowed during "building" phase

During "calm" phase: nothing. During "building": ambient traffic spawns. During "climax" and "cooldown": nothing (scene is playing).

### Traffic Types

| Type | Frequency | Speed | Visual |
|------|-----------|-------|--------|
| Solo cargo | 1 every 15-30s | vx: 60-90 | Single `ship`, slight sine wobble in vy |
| Distant ship | 1 every 20-40s | Very small (scale 1.0), vx: 40-60 | Near-transparent ship high on screen |

### Sine Wobble for Cargo Ships

The "casual" trajectory — not perfectly straight:

```javascript
// In ship update, if ev.wobble is set:
if (ev.wobble) {
  ev.y += Math.sin(ev.age * ev.wobble) * 0.3;
}
```

---

## 5. The Campfire Effect (Pacing Tuning)

### 1/f Noise for Timing Variation

All timing intervals should use organic variation, not fixed values or pure random:

```javascript
_pinkNoiseInterval(baseInterval) {
  const t = this._phase;
  const variation =
    0.50  * Math.sin(t * 0.10) +
    0.25  * Math.sin(t * 0.23) +
    0.125 * Math.sin(t * 0.51) +
    0.0625 * Math.sin(t * 1.07);
  return baseInterval * (1 + variation * 0.4);
}
```

Use this for:
- `_eventCooldown` between spawn checks
- `_sceneCooldown` between scenes
- `_phaseDuration` (multiply the random duration by this factor)

### Easing Rules

| What | Easing | Why |
|------|--------|-----|
| Event fade-in | Sinusoidal | Smooth, non-jarring |
| Event fade-out | Sinusoidal (1.5x longer than fade-in) | Lingers slightly |
| Ship smoothstep | `t * t * (3 - 2t)` | Natural acceleration/deceleration |
| Foreshadow pulse | `0.5 + 0.5 * sin(age * speed)` | Hypnotic rhythm |
| Star twinkle | `0.35 + 0.65 * sin(phase * speed + offset)` | Organic variation |
| Debris drift | Linear (constant velocity) | Physics-accurate inertia |

### Never-Do List

- Never pop an event in (always fade in over 0.5-1.0s minimum)
- Never pop an event out (always fade out over 0.8-1.5s)
- Never use perfectly regular intervals (always add +-20% variation)
- Never have more than 1 narrative event at a time
- Never have more than 4 scenic events simultaneously
- Never have sub-1-second flicker on ambient elements
- Never use linear easing on opacity transitions

---

## 6. Spielberg Compression Pattern

The narrative clock can implement Spielberg's tension compression:

### Cycle Compression

Track cycle count. Each successive cycle, multiply phase durations by a compression factor. After 4 cycles, reset.

```javascript
this._cycleCount = 0;
this._compression = 1.0;

// In _advancePhase(), when returning to "calm":
if (this._narrativePhase === "calm") {
  this._cycleCount++;
  if (this._cycleCount >= 4) {
    this._cycleCount = 0;
    this._compression = 1.0; // reset
  } else {
    this._compression *= 0.85; // 15% shorter each cycle
  }
}

// Apply compression to phase duration:
this._phaseDuration = (min + Math.random() * (max - min)) * this._compression;
```

### Result

- Cycle 1: calm 20s, building 12s, climax 7s, cooldown 14s = ~53s total
- Cycle 2: calm 17s, building 10s, climax 6s, cooldown 12s = ~45s total
- Cycle 3: calm 14s, building 9s, climax 5s, cooldown 10s = ~38s total
- Cycle 4: calm 12s, building 7s, climax 4s, cooldown 9s = ~32s total
- Cycle 5: RESET to cycle 1 durations

This creates an unconscious acceleration that peaks, then resets. The viewer feels the rhythm shift but can't pinpoint why.

---

## 7. Event Capacity Limits

### Simultaneous Event Caps

| Event Category | Max Simultaneous | Why |
|----------------|-----------------|-----|
| Narrative (supernova, wormhole, pulsar, shipvisit) | 1 | Must command full attention |
| Ship (flying ships, all types) | 4 | More looks like a fleet (reserved for convoy scene) |
| Scenic (comet, asteroid, station) | 5 | Background texture, not overwhelming |
| Ambient (shooting star, debris, distantFlash) | 10 | Brief, small, high turnover |
| Foreshadow (glow, navlight) | 2 | Subtle, shouldn't stack |

### Check Before Spawning

```javascript
_canSpawn(category) {
  const LIMITS = {
    narrative: 1, ship: 4, scenic: 5, ambient: 10, foreshadow: 2
  };
  const CATEGORY_MAP = {
    supernova: "narrative", wormhole: "narrative", pulsar: "narrative",
    shipvisit: "narrative", hyperspace: "narrative", quasar: "narrative",
    ship: "ship", streakJump: "ship",
    comet: "scenic", asteroid: "scenic", station: "scenic",
    shootingstar: "ambient", debris: "ambient", distantFlash: "ambient",
    foreshadow: "foreshadow", navlight: "foreshadow",
  };
  const cat = CATEGORY_MAP[category] || "ambient";
  const count = this._events.filter(e => (CATEGORY_MAP[e.type] || "ambient") === cat).length;
  return count < LIMITS[cat];
}
```

---

## 8. Phase-Spawn Event Table (Quick Reference)

What can spawn in each phase:

| Event | Calm | Building | Climax | Cooldown |
|-------|------|----------|--------|----------|
| Shooting star | Rare (1.5%) | Yes (4%) | Rare (3%) | No |
| Meteor shower | No | Yes (1%) | No | No |
| Asteroid | No | Yes (1%) | No | No |
| Comet | No | Yes (1%) | No | No |
| Ambient traffic | No | Yes (low) | No | No |
| Scene (narrative) | No | No | **YES** | No |
| Foreshadow | No | No | Via scene | Possible (for next cycle) |
| Debris | No | No | Via scene | Drifting from scene |

---

## 9. Existing Spawn Method Modifications

### Methods that need optional (x, y) params

These methods currently pick random positions. They need to accept optional coordinates so scenes can place them precisely:

```javascript
// Before:
_spawnSupernova() {
  this._events.push({
    type: "supernova",
    x: 300 + Math.random() * 1500,
    y: 150 + Math.random() * 600,
    // ...
  });
}

// After:
_spawnSupernova(x, y) {
  this._events.push({
    type: "supernova",
    x: x ?? (300 + Math.random() * 1500),
    y: y ?? (150 + Math.random() * 600),
    // ...
  });
}
```

Methods to modify:
- `_spawnSupernova(x, y)`
- `_spawnWormhole(x, y)`
- `_spawnPulsar(x, y)`
- `_spawnQuasar(x, y)`
- `_spawnHyperspace(x, y)`
- `_spawnStation(x, y)` (also needs configurable vx)

### Methods that stay unchanged

- `_spawnShootingStar()` — called as-is by scenes, or with custom params inline
- `_spawnMeteorShower()` — called as-is
- `_spawnComet()` — scenes create custom comets inline
- `_spawnAsteroid()` — scenes create custom asteroids inline

---

## 10. File Changes Summary

### space-bg.js

| Section | Change Type | Description |
|---------|------------|-------------|
| Constructor | ADD | `_pendingSpawns`, `_sceneCooldown`, `_sceneHistory`, `_maCooldown`, `_narrativePhase`, `_phaseTimer`, `_phaseDuration`, `_cycleCount`, `_compression` |
| `update()` | ADD | Process `_pendingSpawns`, decrement cooldowns, advance narrative clock, ship frame animation for `type === "ship"` |
| `_maybeSpawnEvent()` | REPLACE | Becomes `_phaseSpawn()` — gated by narrative phase |
| (new) `_advancePhase()` | ADD | Phase state machine transition |
| (new) `_spawnScene()` | ADD | Weighted scene picker |
| (new) `_queueSpawn()` | ADD | Delayed spawn queue |
| (new) `_spawnFlyingShip()` | ADD | Simple flying ship helper |
| (new) `_spawnForeshadow()` | ADD | Foreshadow glow event |
| (new) `_spawnNavLight()` | ADD | Blinking nav light event |
| (new) `_spawnDebrisField()` | ADD | Post-event debris |
| (new) `_spawnDistantFlashes()` | ADD | Deep background battle flashes |
| (new) `_spawnStreakJump()` | ADD | Hyperspace departure visual |
| (new) `_agitateNearStar()` | ADD | Make a star flicker erratically |
| (new) `_pullDustToward()` | ADD | Attract dust particles to a point |
| (new) `_pinkNoiseInterval()` | ADD | Organic timing variation |
| (new) 13 scene methods | ADD | `_sceneStellarDeath`, `_sceneWormholeTransit`, `_sceneCometBreakup`, `_scenePatrol`, `_sceneDogfight`, `_sceneHyperspaceJump`, `_scenePulsarDiscovery`, `_sceneStationResupply`, `_sceneDeepSpaceSignal`, `_sceneShipJoke`, `_sceneConvoy`, `_sceneDistantBattle`, `_sceneFalseCalm` |
| (new) `_spawnShipVisitFrom()` | ADD | Parameterized version of `_spawnShipVisit` |
| (new) `_canSpawn()` | ADD | Capacity limit checker |
| `_renderEvent()` | ADD | 6 new render cases: `ship`, `foreshadow`, `navlight`, `debris`, `distantFlash`, `streakJump` |
| `_renderEvent()` pulsar | MODIFY | Add dust illumination when `ev.illuminatesDust` flag |
| Existing spawn methods | MODIFY | Accept optional `(x, y)` params |
| `_spawnShipVisit()` | KEEP | Unchanged, still used |
| DEBUG line (0.95) | REMOVE | No longer needed |

### space-assets.js

No changes needed. Ship sprites and all assets are already preloaded.
