# wrai.th — Design Plan: Video Game Management Screen

## Vision

Deux ecrans completement distincts, comme dans un jeu de gestion spatiale :
- **Galaxy Map** = ecran de selection de monde. Canvas plein ecran, AUCUN panel. Clean.
- **Colony View** = ecran de jeu. C'est ici qu'on gere le projet : agents, messages, memories, tasks, kanban, docs.

L'experience rappelle Civilization (selection de planete -> gestion de colonie) ou Anno (vue monde -> vue ile).

---

## 1. Changements Data (DB)

### Supprimer
- Colonne `planet_type` de la table `agents`
- `backfillPlanets()` dans db.go
- `randomPlanet()` dans agents.go
- Champ `PlanetType` du model Agent (Go + JSON)

### Ajouter
- Table `projects` avec `planet_type` assigne aleatoirement a la creation

```
projects (
  name        TEXT PRIMARY KEY,    -- "brandos-api", "agent-relay"
  planet_type TEXT NOT NULL,       -- "terran/1", "lava/3", etc.
  created_at  TEXT NOT NULL
)
```

Auto-creee quand un agent fait `register_agent` avec un project qui n'existe pas encore.

### Nouvel endpoint API
```
GET /api/projects
  -> [{ name, planet_type, created_at, agent_count, task_stats }]

GET /api/projects/:name
  -> { name, planet_type, agents: [...], tasks: {...}, stats: {...} }
```

---

## 2. Galaxy Map — Ecran de Selection (Vue Globale)

### Principe
Ecran **100% canvas**, zero panel, zero sidebar. Juste l'espace, les planetes-projets, et le starfield. C'est un ecran de selection de monde, pas un dashboard.

### Layout
```
+====================================================================+
|  wrai.th                                                     [?]   |
+====================================================================+
|                                                                     |
|                    . *    .  *   .                                   |
|               *         .                                           |
|                                                                     |
|                   @@@@                                               |
|                  @terran@     .  *                                   |
|                   @@@@                                               |
|              BRANDOS-API                                             |
|              12 agents                                               |
|                                          @@@@                        |
|            .       *                    @lava@                       |
|                                          @@@@                        |
|                                     AGENT-RELAY                      |
|                  .                    3 agents                        |
|                         *                                            |
|        @@@@                                                         |
|       @ice@        .           *              .                      |
|        @@@@                                                         |
|    MY-PROJECT                                                        |
|     1 agent               .          *                               |
|                                                                     |
+====================================================================+
        Canvas plein ecran — pas de header tabs, pas de sidebar
        Seul element HTML : logo wrai.th + bouton help
```

### Chaque planete-projet sur le canvas

```
         .:*:.
       .::::::::.
      :::PLANET:::      <-- sprite anime 48x48, rendu 96-140px
       '::::::::'          (taille proportionnelle aux agents)
         ':*:'
                         <-- glow aura couleur biome
     BRANDOS-API         <-- JetBrains Mono Bold 12px, gold #FFD250
     12 agents online    <-- 9px, muted
     ████████░░░░        <-- progress bar tasks done/total
```

### Interactions Galaxy Map
| Action | Resultat |
|--------|----------|
| **Hover planete** | Scale 1.2, glow intensifie, tooltip stats (tasks par status, agents online/total) |
| **Click planete** | Transition zoom -> Colony View du projet |
| **Pan (drag)** | Navigation dans la galaxy |
| **Scroll** | Zoom in/out |
| **?** | Help modal |

### Ce qui N'EST PAS dans Galaxy Map
- Pas de sidebar messages/memories/tasks
- Pas de tabs Kanban/Vault/Canvas
- Pas de header tabs
- Pas de mode toggle
- Pas de font scale
- Juste le canvas + logo

---

## 3. Colony View — Ecran de Gestion (Vue Projet)

C'est ICI que tout se passe. Quand on entre dans un projet, on a acces a TOUT : agents, communication, tasks, kanban, docs, vault.

### Layout principal

```
+====================================================================+
| [< Galaxy]  BRANDOS-API            [1]Agents [2]Kanban [3]Docs [?] |
+====================================================================+
|                                                                     |
| +--CANVAS (70%)-----------------------------------+ +--PANEL (30%)-+|
| |                                                  | |              ||
| |  @@@                                             | | Messages     ||
| | @   @ <-- planete projet, coin haut-gauche      | | Memories     ||
| |  @@@     tourne en fond, ~120px                  | | Tasks        ||
| |   BRANDOS-API  Terran World                      | |              ||
| |                                                  | |              ||
| |      [ROBO]----[ROBO]                            | |              ||
| |       CEO   |    CTO                             | |              ||
| |             |                                    | |              ||
| |    [ROBO] [ROBO] [ROBO]                          | |              ||
| |   back-1  back-2  front-1                        | |              ||
| |                                                  | |              ||
| |  ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ atmosphere    | |              ||
| |  ===================================== surface   | |              ||
| |  Landscape panorama (biome du projet)            | |              ||
| +--------------------------------------------------+ +--------------+|
+====================================================================+

    Pixel Holo UI panels pour encadrer les infos
```

### Zone Canvas (gauche, ~70%)

#### Planete du projet (coin haut-gauche)
- La planete animee du projet tourne en permanence dans le coin superieur gauche
- Taille ~120px, avec glow aura biome
- En dessous : nom du projet + type de biome
- Rendu via les **Pixel Holo UI title_panel** assets (cadre holographique autour)

#### Agents organises hierarchiquement
- Les robots sont disposes en **arbre hierarchique** (org chart)
  - CEO/Executive en haut, managers au milieu, IC en bas
  - Lignes de connexion pointillees entre manager -> reports (existant : `hierarchyLinks`)
- Chaque robot = `robo-sprite.js` (idle/work animation)
- Differencies par : nom, status dot, animation

#### Surface planete (bas du canvas)
- Landscape panorama tile du biome du projet
- Les robots "marchent" sur la surface
- Atmosphere gradient au dessus

#### Infos projet (Pixel Holo UI panels)
- Panel holographique (9-slice `panel/Clean/`) affichant les stats du projet :
  - Nombre d'agents (online / total)
  - Tasks : pending / in-progress / done / blocked
  - Derniere activite
- Rendu sur le canvas avec les assets Pixel Holo UI
- Donnees via `GET /api/projects/:name`

### Zone Panel (droite, ~30%)

La sidebar existante, mais UNIQUEMENT visible en Colony View :

| Tab | Raccourci | Contenu |
|-----|-----------|---------|
| **Messages** | `M` | Inbox, conversations, user questions |
| **Memories** | `Y` | Shared knowledge, search |
| **Tasks** | `T` | Task list filtre par projet |

### Vues alternatives (raccourcis clavier)

Ces vues **remplacent le canvas** (pas la sidebar) :

| Vue | Raccourci | Description |
|-----|-----------|-------------|
| **Agents** (defaut) | `1` | Canvas avec robots + hierarchie + planete en fond |
| **Kanban** | `2` | Board kanban du projet (existant) |
| **Docs / Vault** | `3` | Vault browser du projet (existant) |
| **Connections** | `4` | Vue communication — lignes entre agents qui se parlent |

`Escape` = retour a Galaxy Map

### Interactions Colony View
| Action | Resultat |
|--------|----------|
| **Hover robot** | Affiche role + task en cours |
| **Click robot** | Ouvre detail panel (slide-in, existant) |
| **`1`** | Vue Agents (canvas, defaut) |
| **`2`** | Vue Kanban |
| **`3`** | Vue Docs/Vault |
| **`4`** | Vue Connections |
| **`M`** | Focus tab Messages (sidebar) |
| **`Y`** | Focus tab Memories (sidebar) |
| **`T`** | Focus tab Tasks (sidebar) |
| **`Escape`** | Retour Galaxy Map |
| **`?`** | Help modal |

---

## 4. Pixel Holo UI — Utilisation des Assets

Assets source : `/Downloads/Pixel Holo UI Pack.zip` -> copier dans `internal/web/static/img/ui/`

### Mapping des assets

| Asset | Utilisation |
|-------|-------------|
| `panel/Clean/` (9-slice) | Cadre pour les stats du projet sur canvas |
| `title_panel/Clean/` (9-slice) | Cadre avec titre pour section planete |
| `panel_left/Clean/` | Panel lateral pour la sidebar |
| `panel_right/Clean/` | Panel lateral droit |
| `button/Normal+Hover+Clicked/` | Boutons de navigation (Galaxy, tabs) |
| `planet_selector/` (animated) | Animation de selection quand on hover une planete en Galaxy Map |
| `divider_horizontal/` | Separateurs dans les panels |
| `divider_vertical/` | Separateur canvas/sidebar |
| `loading_wheel/` (11 frames) | Loading indicator pendant transitions |
| `icons/small/` | Fleches, check, cogwheel, plus, minus, x |
| `icons/large/standard/` | Planet icon, question mark, exclamation |
| `icons/large/chromatic_aberration/` | Version glitch pour etats critiques (blocked) |

### Rendu 9-slice sur Canvas
Les panels Pixel Holo sont en 9-slice (coins + bords + centre).
Sur Canvas 2D : dessiner les 9 morceaux en les etirant pour faire un panel de taille arbitraire.

```
[TL][TC][TC][TC][TR]
[CL][CC][CC][CC][CR]    <- CC = center tile, repete
[CL][CC][CC][CC][CR]
[BL][BC][BC][BC][BR]
```

---

## 5. Style Visuel

### Palette
```
Background:     #020617   (espace profond)
Panel bg:       rgba(10, 12, 30, 0.85)  (panels semi-transparents)
Surface text:   #F8FAFC
Accent gold:    #FFD250   (noms projets, titres)
Accent green:   #00E676   (actif, done, working)
Accent red:     #FF5252   (blocked, erreur)
Accent purple:  #A29BFE   (hierarchy, sleeping)
Accent cyan:    #4FC3F7   (idle, info)
Holo border:    rgba(120, 100, 255, 0.3)  (bordure panels Pixel Holo)
Muted:          rgba(224, 224, 232, 0.5)
```

### Glow par biome (planete du projet)
```
terran:    rgba(100, 200, 130, 0.15)   vert doux
lava:      rgba(255, 120, 40, 0.2)     orange chaud
ice:       rgba(140, 200, 255, 0.15)   bleu froid
ocean:     rgba(60, 140, 220, 0.15)    bleu profond
desert:    rgba(220, 180, 80, 0.12)    jaune sable
forest:    rgba(60, 180, 80, 0.15)     vert foret
gas_giant: rgba(160, 120, 200, 0.15)   violet
barren:    rgba(150, 150, 160, 0.1)    gris
tundra:    rgba(160, 220, 240, 0.12)   cyan pale
```

### Typographie
- **JetBrains Mono** partout (pas de Press Start 2P, illisible en petit)
- Titres : Bold 14px
- Labels : Bold 11-12px
- Stats/details : Regular 9-10px

---

## 6. Transitions

### Galaxy -> Colony (click planete)
1. Planet selector animation (Pixel Holo asset, 4 frames autour de la planete)
2. La planete cliquee scale 1.0 -> 2.5 (600ms ease-out)
3. Les autres planetes fade out (300ms)
4. Le starfield accelere (zoom-in effect)
5. Crossfade vers Colony layout
6. Sidebar slide-in depuis la droite (300ms)
7. Robots apparaissent staggered sur la surface (100ms entre chaque)
8. Planete du projet se positionne dans le coin haut-gauche

### Colony -> Galaxy (Escape ou bouton < Galaxy)
1. Sidebar slide-out (200ms)
2. Robots fade out (200ms)
3. Surface retrecit, planete remonte vers sa position galaxy
4. Autres planetes fade in (400ms)
5. Retour Galaxy Map clean

---

## 7. Architecture des Ecrans

```
wrai.th
  |
  +-- GALAXY MAP (mode: "galaxy")
  |     Canvas plein ecran
  |     Pas de sidebar, pas de tabs
  |     Elements : planetes-projets + starfield
  |
  +-- COLONY VIEW (mode: "colony", project: "brandos-api")
        |
        +-- Canvas zone (70%)
        |     +-- Vue Agents [1] (defaut) : robots + hierarchie + planete bg
        |     +-- Vue Kanban [2] : board kanban
        |     +-- Vue Docs [3] : vault browser
        |     +-- Vue Connections [4] : communication map
        |
        +-- Sidebar zone (30%)
              +-- Tab Messages [M]
              +-- Tab Memories [Y]
              +-- Tab Tasks [T]
```

---

## 8. Flow Utilisateur Complet

```
Ouverture wrai.th
    |
    v
[GALAXY MAP] -- canvas plein ecran, planetes = projets
    |
    | click sur planete "brandos-api"
    | (planet selector animation -> zoom transition)
    v
[COLONY VIEW : Agents] -- robots sur surface, planete en haut-gauche
    |                      sidebar avec Messages/Memories/Tasks
    |
    +--[2]--> [COLONY VIEW : Kanban] -- board kanban du projet
    +--[3]--> [COLONY VIEW : Docs] -- vault browser du projet
    +--[4]--> [COLONY VIEW : Connections] -- comm map agents
    +--[1]--> retour vue Agents
    |
    | click sur robot "backend-1"
    v
[Detail Panel] -- slide-in, infos agent
    |
    | Escape (ferme le detail, reste en Colony)
    v
[COLONY VIEW]
    |
    | Escape (depuis Colony sans detail ouvert)
    v
[GALAXY MAP]
```

---

## 9. Elements a Supprimer de l'Existant

### Backend (Go)
- `planet_type` dans agent model, columns, scan, insert, update
- `backfillPlanets()`, `randomPlanet()`, `planetPool`
- `planet_type` dans ensureColumns + migrateDropGlobalUnique

### Frontend (JS)
- `planetType` property dans AgentView
- `_planetFrameIndex`, `_planetFrameTimer`, `_planetSpeed` (agent-level)
- `fallbackPlanetType()` dans space-assets.js
- Rendu planete dans mode "minimal" agent-view.js
- Attribution `SOLAR_PLANETS` par index agent dans main.js
- Header tabs visible en mode galaxy (masquer tout sauf logo)

### Ce qui RESTE
- Assets planetes animees (reutilises pour les projets)
- `robo-sprite.js` (robots = agents)
- Colony view base (adaptee)
- Starfield / space background
- Sidebar panels (messages, memories, tasks) — deplaces en Colony only
- Kanban, Vault — deplaces en Colony only

---

## 10. Nouveaux Assets a Integrer

Copier depuis Downloads vers `internal/web/static/img/ui/` :

```
img/ui/
  panel/          <- Pixel Holo panel (9-slice, Clean variant)
  title_panel/    <- Pixel Holo title panel (9-slice)
  button/         <- Normal / Hover / Clicked
  planet_selector/ <- 4 frames animation selection
  dividers/       <- horizontal + vertical
  loading/        <- 11 frame loading wheel
  icons/          <- small (arrows, check, x) + large (planet, ?, !)
```
