// 16x16 pixel-art sprites defined as string grids
// Each character maps to a color in the palette

const SPRITE_DATA = {
  agent: [
    // Frame 0
    [
      "................",
      "......HHHH......",
      ".....HHHHHH.....",
      "....HSSSSSH.....",
      "....SEEEESH.....",
      "....SEEEESH.....",
      "....SSMMSS......",
      ".....SSSS.......",
      "....CCCCCC......",
      "...CCCCCCCC.....",
      "...CCCCCCCC.....",
      "...CC.CC.CC.....",
      "....CCCCCC......",
      "....LL..LL......",
      "....LL..LL......",
      "...BBB..BBB.....",
    ],
    // Frame 1 (bob up 1px)
    [
      "................",
      "................",
      "......HHHH......",
      ".....HHHHHH.....",
      "....HSSSSSH.....",
      "....SEEEESH.....",
      "....SEEEESH.....",
      "....SSMMSS......",
      ".....SSSS.......",
      "....CCCCCC......",
      "...CCCCCCCC.....",
      "...CCCCCCCC.....",
      "...CC.CC.CC.....",
      "....CCCCCC......",
      "....LL..LL......",
      "...BBB..BBB.....",
    ],
  ],
};

// Color palettes — each agent gets a unique palette based on index
const PALETTES = [
  { // 0 — teal
    H: "#00cec9", S: "#ffeaa7", E: "#2d3436", M: "#e17055",
    C: "#00b894", L: "#2d3436", B: "#636e72",
  },
  { // 1 — orange
    H: "#fdcb6e", S: "#ffeaa7", E: "#2d3436", M: "#e17055",
    C: "#e17055", L: "#2d3436", B: "#636e72",
  },
  { // 2 — blue
    H: "#74b9ff", S: "#ffeaa7", E: "#2d3436", M: "#e17055",
    C: "#0984e3", L: "#2d3436", B: "#636e72",
  },
  { // 3 — red
    H: "#ff7675", S: "#ffeaa7", E: "#2d3436", M: "#e17055",
    C: "#d63031", L: "#2d3436", B: "#636e72",
  },
  { // 4 — purple
    H: "#a29bfe", S: "#ffeaa7", E: "#2d3436", M: "#e17055",
    C: "#6c5ce7", L: "#2d3436", B: "#636e72",
  },
  { // 5 — green
    H: "#55efc4", S: "#ffeaa7", E: "#2d3436", M: "#e17055",
    C: "#00b894", L: "#2d3436", B: "#636e72",
  },
];

// Palette color for name tags (matches the coat color)
export const PALETTE_COLORS = PALETTES.map(p => p.C);

const cache = new Map();

export class SpriteGenerator {
  static generate(paletteIndex = 0) {
    const key = `agent_${paletteIndex}`;
    if (cache.has(key)) return cache.get(key);

    const palette = PALETTES[paletteIndex % PALETTES.length];
    const frames = SPRITE_DATA.agent;

    const rendered = frames.map((frame) => {
      const size = 48; // 16 * 3
      const canvas = new OffscreenCanvas(size, size);
      const ctx = canvas.getContext("2d");
      const px = 3;

      for (let y = 0; y < 16; y++) {
        for (let x = 0; x < 16; x++) {
          const ch = frame[y][x];
          if (ch === ".") continue;
          ctx.fillStyle = palette[ch] ?? "#fff";
          ctx.fillRect(x * px, y * px, px, px);
        }
      }

      return canvas;
    });

    cache.set(key, rendered);
    return rendered;
  }
}
