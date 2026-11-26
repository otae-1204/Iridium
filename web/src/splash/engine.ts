// The splash's drawing (splash.tsx; kumo, 2026-11-25): a band of code in small characters, each
// cell of it as dark and as coloured as the figure behind it is there, so that the characters
// draw the figure: Iridium's letters, an agent's mark, a face. It moves only as something
// changes: the code flows for a moment as the band comes and as its figure changes, the cells
// going over to the new figure from its middle outwards; the characters about the pointer
// change as it passes. Otherwise nothing is drawn again.
import type { Agent } from "../api/types.gen";
import { AGY_DATA_URI, CLAUDE, DSH, DSH_VIEWBOX, OPENAI, OPENAI_VIEWBOX, PI, PI_VIEWBOX } from "../logos";
import { STANDARD, type Fig } from "./figure";
import { slicer, type Cast, type Sprite } from "./slice";

const FONT_PX = 8; // px; a narrow band's (fine): 7
const ROW_PX = 9; // px, a row's height; fine: 8
const SRC = 256; // px, the figure as it is drawn before the cells are read from it
const BASE = 0.12; // a cell's opacity with no figure on it
const TURN = 700; // ms, one cell going over
const SPREAD = 650; // ms, from the middle's first cell to the rim's
const IDLE = 0.4; // the flow's speed with nothing happening, of its speed as something does
const GLYPHS = "abcdefghijklmnopqrstuvwxyz0123456789{}[]()<>=+-*/&|!?:;.,_#$%@~^";
const WORDS = ["mygo", "satori", "iridium"];
const CODE = [
  "func (s *Server) Serve(ctx context.Context) error {",
  "for ev := range ch.events {",
  "if err != nil { return fmt.Errorf(\"open: %w\", err) }",
  "select { case <-ctx.Done(): return ctx.Err() }",
  "msg := &Message{ID: next(), Channel: ch.id, Text: text}",
  "defer mu.Unlock()",
  "go func() { defer wg.Done(); run(ctx, a) }()",
  "type Member struct { Nick string; Agent string; On []Trigger }",
  "const rows = useMemo(() => fold(log.msgs), [log.end])",
  "export function open(id: string): void {",
  "await api.send(id, { text, files })",
  "return list.filter((x) => x.cwd === cwd)",
  "switch ev.Type { case \"message-created\":",
  "w.Header().Set(\"Content-Type\", \"application/json\")",
  "if !ok { http.Error(w, \"not found\", 404); return }",
  "for (const m of members) wake(m, msg)",
  "ch <- Event{Seq: seq, At: time.Now()}",
  "const [open, setOpen] = useState(false)",
  "tx, err := db.BeginTx(ctx, nil)",
  "import { useEffect, useRef } from \"react\"",
  "git commit -m \"channel: one message, said once\"",
  "ir api message.create channel_id=$id content=ok",
  "session.resume(id).then(stream)",
  "func holder(sock string) (*Conn, error) {",
  "$ ir run --secret token -- deploy",
  "case <-time.After(30 * time.Second):",
  "let seq = 0; const heard = new Set()",
  "rows, err := q.Query(ctx, `select seq, text from messages`)",
  "useLayoutEffect(() => measure(ref.current), [])",
  "token := r.Header.Get(\"Authorization\")",
];

/** The flow's speed for an effort (0 to 1), of its own: 1 at the standard one (high), up to
 *  2.2 times at the most, down to half at the least. */
const pace = (p: number) => (p <= STANDARD ? 0.45 + (p / STANDARD) * 0.55 : 1 + Math.pow((p - STANDARD) / (1 - STANDARD), 1.3) * 1.2);

type RGB = [number, number, number];
type Field = { a: Float32Array; r: Float32Array; g: Float32Array; b: Float32Array };
const field = (n: number): Field => ({ a: new Float32Array(n), r: new Float32Array(n), g: new Float32Array(n), b: new Float32Array(n) });
const clamp = (x: number, lo = 0, hi = 1) => (x < lo ? lo : x > hi ? hi : x);

/** One row's code: lines one after another, a word of ours among them now and then (seldom: to
 *  be come upon, not read). */
function line(n: number): string[] {
  let s = "";
  while (s.length < n) s += (Math.random() < 1 / 70 ? WORDS[(Math.random() * WORDS.length) | 0]! : CODE[(Math.random() * CODE.length) | 0]!) + "  ";
  return [...s.slice(0, n)];
}

/** An agent's mark on a square of SRC, in ink where it is drawn in the text's colour. */
function mark(g: CanvasRenderingContext2D, agent: Agent, ink: string, size = SRC): boolean {
  const box = (vb: string) => {
    const [x, y, w, h] = vb.split(" ").map(Number) as [number, number, number, number];
    const k = (size * 0.8) / Math.max(w, h);
    g.translate(size / 2 - (x + w / 2) * k, size / 2 - (y + h / 2) * k);
    g.scale(k, k);
  };
  const paths: [string, string][] = agent === "claude" ? [["#D97757", CLAUDE]] : agent === "codex" ? [[ink, OPENAI]] : agent === "pi" ? PI : agent === "dsh" ? [[ink, DSH]] : [];
  if (!paths.length) return false;
  g.save();
  box(agent === "claude" ? "0 0 24 24" : agent === "codex" ? OPENAI_VIEWBOX : agent === "pi" ? PI_VIEWBOX : DSH_VIEWBOX);
  for (const [fill, d] of paths) {
    g.fillStyle = fill;
    g.fill(new Path2D(d));
  }
  g.restore();
  return true;
}

/** A colour of the rainbow at hue h (degrees), bright. */
function rainbowAt(h: number): RGB {
  const f = (n: number) => {
    const k = (n + h / 30) % 12;
    return 255 * (0.55 - 0.5 * Math.max(-1, Math.min(k - 3, 9 - k, 1)));
  };
  return [f(0), f(8), f(4)];
}

// The rainbow's colours, one for each degree, worked out once: not three numbers made anew for each
// cell each frame.
const HUES = (() => {
  const t = new Uint8Array(360 * 3);
  for (let h = 0; h < 360; h++) t.set(rainbowAt(h).map((v) => Math.min(255, Math.round(v))), h * 3);
  return t;
})();
const hueAt3 = (h: number) => ((((h | 0) % 360) + 360) % 360) * 3;

/** Iridium's letters (ui.tsx IrMark's, with no square about them), in a rainbow. */
function letters(g: CanvasRenderingContext2D, ink: string) {
  const k = (SRC * 0.78) / 28;
  g.save();
  g.translate(SRC / 2 - 32 * k, SRC / 2 - 32 * k);
  g.scale(k, k);
  // A rainbow across them (kumo, 2026-11-25: RGB, every colour), not the text's ink.
  void ink;
  const paint = g.createLinearGradient(20, 18, 44, 46);
  ["#ff3b30", "#ff9500", "#ffcc00", "#34c759", "#32ade6", "#007aff", "#af52de"].forEach((c, i, all) => paint.addColorStop(i / (all.length - 1), c));
  g.fillStyle = paint;
  g.fill(new Path2D("M20 18h6v28h-6zM32 28h5.6v3.2c1.4-2.4 3.6-3.6 6.4-3.6v5.8c-3.8 0-6.2 1.6-6.2 6V46H32z"));
  g.restore();
}

const loaded = new Map<string, Promise<HTMLImageElement | null>>();
function picture(src: string): Promise<HTMLImageElement | null> {
  let p = loaded.get(src);
  if (!p) {
    p = new Promise((done) => {
      const img = new Image();
      // Decoded before it is given: drawn the first time it would be decoded then, the frame held.
      img.onload = () => img.decode().then(() => done(img.naturalWidth ? img : null), () => done(img.naturalWidth ? img : null));
      img.onerror = () => done(null);
      img.src = src;
    });
    loaded.set(src, p);
  }
  return p;
}
/** A figure's picture asked for ahead (a row of the + panel as it is drawn). */
export const ahead = (f: Fig) => void (f.src && picture(f.src));

// The band as one left it, for the next one of its size within a moment: the empty pane's band
// goes on in the channel made from it, the same characters where they were, nothing coming in
// anew (kumo, 2026-11-25: it blinked).
let kept: { at: number; cols: number; rows: number; text: string[][]; off: Float32Array<ArrayBuffer>; speed: Float32Array<ArrayBuffer>; cur: Field; key: string } | null = null;

export type Splash = { leave(done: () => void): void; enter(): void; figure(f: Fig | null): void; place(x: number, side: number, y?: number): void; foot(y: number): void; window(middle: number, height: number): void; recolor(): void; power(f: number): void; resize(): void; near(x: number, y: number): void; tap(x: number, y: number): void; hold(on: boolean): void; stop(): void; edge(on: boolean): void;  cast(who: Cast[]): void; onPlay(h: (on: boolean) => void): void; play(done: () => void): boolean };


/** fine: smaller characters, more of them to a figure (the portrait layout, where the band is
 *  narrow and a figure of few cells is a blur: kumo, 2026-11-25). apart: not the pane's band (the
 *  ⋯ menu's Game, game/overlay.tsx): nothing of it kept for the next one, nor taken. */
export function splash(cv: HTMLCanvasElement, still: boolean, fine = false, apart = false): Splash {
  const FONT = fine ? 7 : FONT_PX, ROW = fine ? 8 : ROW_PX;
  const g = cv.getContext("2d")!;
  const src = document.createElement("canvas");
  src.width = src.height = SRC;
  const sg = src.getContext("2d", { willReadFrequently: true })!;
  const small = document.createElement("canvas");
  const mg = small.getContext("2d", { willReadFrequently: true })!;
  const tiny = document.createElement("canvas");
  const tg = tiny.getContext("2d")!;

  let W = 0, H = 0, dpr = 1, cw = 5.4, cols = 0, rows = 0, len = 0;
  let cur = field(0), from = field(0), to = field(0);
  let busy = new Uint8Array(0), wasTurning = false;
  let wait = new Float32Array(0), off = new Float32Array(0), speed = new Float32Array(0);
  let text: string[][] = [];
  let ink: RGB = [34, 34, 34], dark = false;
  // The effort slider's level (0 to 1), and where the figure's middle is and how large it is:
  // each as asked (pwT; cx, most: -1 the band's own middle) and as drawn now (pw; px, ps), the
  // drawn gliding to the asked, never set over to it in one frame (kumo, 2026-11-25: it twitched).
  let pw = STANDARD, pwT = STANDARD;
  let cx = -1, most = 0, px = -1, ps = 0;
  let cy = -1, py = -1; // its middle's height: asked (-1: the window's own middle) and drawn
  // Where the figure's lowest edge is to stand (-1: none asked): on the composer (kumo, 2026-11-25:
  // Ir's letters stand on its top, as on a line). below: how far under the square's middle what
  // is drawn in it ends, a share of its side (the letters 0.39, a face 0.47, a mark 0.4).
  let footT = -1, below = 0.39;
  const yGoal = (gc: number, gs: number) => (footT >= 0 ? footT - below * gs : cy < 0 ? gc : cy);
  // As the page is laid out (its first second), what is asked of the place is there at once, not
  // glided to: it moved about as the page settled (kumo, 2026-11-25).
  const born = performance.now();
  const settling = () => performance.now() - born < 1200;
  // The window (kumo, 2026-11-25): the code is over the whole canvas, each row where it is, and
  // only the rows in the window are seen, fading out to its upper and lower edge. Moved, the
  // window goes over the rows, those it leaves fading and those it comes to coming up: the
  // characters do not move with it. Its middle and height, asked (-1: the canvas's own) and drawn.
  let wcT = -1, whT = -1, wc = 0, wh = 0;
  // The figure's own cells are seen whole, in the window's fading edges and past them too (kumo,
  // 2026-11-25: the figure was cut off by them): how much of each cell is the figure's (fm), and
  // the most of it in each row (fr).
  let fm = new Float32Array(0), fr = new Float32Array(0);
  // How far the band is let out above and below what is asked (kumo, 2026-11-25: pressed above
  // or below it, or held there, it opens out slowly, as far as the pane goes).
  let up = 0, down = 0;
  const winGoal = (): [number, number] => {
    const [c, h] = wcT < 0 ? [H / 2, H] : [wcT, whT];
    if (!up && !down) return [c, h];
    const top = Math.max(-H * 0.3, c - h / 2 - up), bot = Math.min(H * 1.3, c + h / 2 + down);
    return [(top + bot) / 2, Math.max(h, bot - top)];
  };
  /** How much of a row at height y is seen: all of it in the window's middle half, none at its edges. */
  const seen = (y: number) => {
    const d = Math.abs(y - wc) / (wh / 2 || 1);
    if (d >= 1) return 0;
    if (d <= 0.52) return 1;
    const t = (1 - d) / 0.48;
    return t * t * (3 - 2 * t);
  };
  let vel = 0, wasGliding = false, tick = 0, calm = false, odd = false;
  let leaving: (() => void) | null = null, left = false, spreadNow = SPREAD;
  const goal = (): [number, number] => (cx < 0 ? [W / 2, Math.min((whT < 0 ? H : whT) * 0.92, W * 0.7)] : [cx, most]);
  // Iridium's letters drawn: their cells' colours flow through the rainbow each frame (kumo,
  // 2026-11-25: a gradient that moves).
  // (so before anything is painted too: no code shown about nothing as the page loads; kumo,
  // 2026-11-26: grey code each time the page was loaded)
  let rainbow = true;
  // Whether the ring is to be there: as soon as a figure is asked for, not as its pictures have
  // come (kumo, 2026-11-26: the ring stood on while a face loaded, then was gone at once).
  let ringAsk = false;
  let fig: Fig | null = null, photo = false, drawnKey = "", asked = 0;
  let t0 = -1e9, flowTo = 0, last = 0, raf = 0, first = true, dead = false;

  const inkNow = () => {
    const c = getComputedStyle(cv).color;
    sg.clearRect(0, 0, 1, 1);
    sg.fillStyle = c;
    sg.fillRect(0, 0, 1, 1);
    const d = sg.getImageData(0, 0, 1, 1).data;
    ink = [d[0]!, d[1]!, d[2]!];
    dark = 0.3 * ink[0] + 0.59 * ink[1] + 0.11 * ink[2] > 128;
    return `rgb(${ink.join(",")})`;
  };

  /** The pictures a figure is drawn from, loaded. */
  const load = async (f: Fig | null) => {
    const face = (f?.src ? await picture(f.src) : null) ?? (f?.alt ? await picture(f.alt) : null);
    const img = face ?? (f?.agent === "agy" ? await picture(AGY_DATA_URI) : null);
    return { face, img };
  };
  /** The figure on the square (src), at once: what is read from it is never half the last one and
   *  half this (kumo, 2026-11-26: the letters were read again while a face loaded, and came out in
   *  colour). True: a picture, its tones kept, not a flat mark. */
  const paint = (f: Fig | null, face: HTMLImageElement | null, img: HTMLImageElement | null): boolean => {
    const css = inkNow();
    rainbow = false;
    sg.clearRect(0, 0, SRC, SRC);
    if (img && face) {
      const s = Math.min(img.naturalWidth, img.naturalHeight);
      sg.save();
      sg.beginPath();
      sg.arc(SRC / 2, SRC / 2, SRC * 0.47, 0, 7);
      sg.clip();
      sg.drawImage(img, (img.naturalWidth - s) / 2, (img.naturalHeight - s) / 2, s, s, SRC * 0.03, SRC * 0.03, SRC * 0.94, SRC * 0.94);
      sg.restore();
      below = 0.47;
      return true;
    }
    below = 0.4;
    if (img) sg.drawImage(img, SRC * 0.1, SRC * 0.1, SRC * 0.8, SRC * 0.8);
    else if (!f?.agent || !mark(sg, f.agent, css)) (letters(sg, css), (below = 0.39), (rainbow = true));
    return false;
  };

  /** Each cell's opacity and colour for the square as it is painted. */
  const read = (): Field => {
    const n = cols * rows, out = field(n);
    if (!n) return out;
    const mid = px, side = ps, fx = mid - side / 2, fy = py - side / 2;
    small.width = cols;
    small.height = rows;
    mg.imageSmoothingQuality = "high";
    // Through a half-size step: one step down from SRC skipped pixels, a mark's thin rays gone.
    tiny.width = cols * 3;
    tiny.height = rows * 3;
    tg.imageSmoothingQuality = "high";
    tg.drawImage(src, (fx / cw) * 3, (fy / ROW) * 3, (side / cw) * 3, (side / ROW) * 3);
    mg.drawImage(tiny, 0, 0, cols, rows);
    const sharp = mg.getImageData(0, 0, cols, rows).data;
    // No halo about the figure (kumo, 2026-11-25: the colours spread about it, and the code there
    // lighter): the code about it as all the rest is.
    const vivid = (r: number, g: number, b: number, k: number, gain: number): RGB => {
      const m = (r + g + b) / 3;
      return [clamp((m + (r - m) * k) * gain, 0, 255), clamp((m + (g - m) * k) * gain, 0, 255), clamp((m + (b - m) * k) * gain, 0, 255)];
    };
    if (fm.length !== n) fm = new Float32Array(n);
    if (fr.length !== rows) fr = new Float32Array(rows);
    fr.fill(0);
    for (let i = 0; i < n; i++) {
      const o = i * 4, m = sharp[o + 3]! / 255;
      fm[i] = m;
      if (m > fr[(i / cols) | 0]!) fr[(i / cols) | 0] = m;
      let a = BASE, c: RGB = ink;
      // A picture (a face): the code about it goes out from its rim to none, the face alone
      // (kumo, 2026-11-25); about letters or a mark the code is as everywhere.
      if (photo && m <= 0.5) {
        const d = Math.hypot(((i % cols) + 0.5) * cw - mid, (((i / cols) | 0) + 0.5) * ROW - py) - side * 0.47;
        a = BASE * clamp(1 - d / (side * 0.09));
      }
      if (photo && m > 0.5) {
        const lum = (0.3 * sharp[o]! + 0.59 * sharp[o + 1]! + 0.11 * sharp[o + 2]!) / 255;
        a = (0.3 + 0.7 * Math.pow(dark ? lum : 1 - lum, 0.75)) * (0.7 + 0.3 * pw);
        c = vivid(sharp[o]!, sharp[o + 1]!, sharp[o + 2]!, 1.5, dark ? 1.2 : 0.85);
      } else if (!photo && m > 0) {
        c = [ink[0] + (sharp[o]! - ink[0]) * m, ink[1] + (sharp[o + 1]! - ink[1]) * m, ink[2] + (sharp[o + 2]! - ink[2]) * m];
        a = BASE + m * (0.42 + 0.4 * pw);
      }
      out.a[i] = a;
      out.r[i] = c[0];
      out.g[i] = c[1];
      out.b[i] = c[2];
    }
    // Iridium's letters are not shown for now (kumo, 2026-11-25: no channel open, and a channel
    // with no agent, are blank as they come; an agent's face comes out of it as one is added).
    // The cells are all there still, so what a finger lights up is seen: the characters, in their
    // colours.
    if (rainbow) out.a.fill(0);
    return out;
  };

  const FONTS = (z: number) => `${FONT * z}px ui-monospace, "SF Mono", Menlo, "JetBrains Mono", "Cascadia Mono", "DejaVu Sans Mono", monospace`;
  // What the stars, rings and the rocket light this frame, each cell's most (lit) in its hue (litHue):
  // laid down shape by shape over the cells under their outlines alone, a row at a time (a ring's
  // two spans, a stroke's band), then read as the rows are drawn; the cells lit are kept (touched)
  // to be put back to none, not the whole field gone over (kumo, 2026-11-25: lighter work).
  let lit = new Float32Array(0), litHue = new Float32Array(0), rowLit = new Uint8Array(0), touched = new Int32Array(0), nTouched = 0;
  const light = (r: number, k: number, v: number, h: number) => {
    if (v <= 0.02) return;
    const i = r * cols + k;
    if (v <= lit[i]!) return;
    if (lit[i] === 0) touched[nTouched++] = i;
    lit[i] = v;
    litHue[i] = h;
    rowLit[r] = 1;
  };
  /** Each cell whose middle is in the band from (ax, ay) to (bx, by), w either side of it: told its
   *  middle, how far it is from the stroke and where along it (0 to 1). */
  const stroke = (ax: number, ay: number, bx: number, by: number, w: number, each: (r: number, k: number, x: number, y: number, d: number, u: number) => void) => {
    const dx = bx - ax, dy = by - ay, L2 = dx * dx + dy * dy;
    const r0 = Math.max(0, Math.ceil((Math.min(ay, by) - w - 0.5 - ROW / 2) / ROW)), r1 = Math.min(rows - 1, Math.floor((Math.max(ay, by) + w - 0.5 - ROW / 2) / ROW));
    for (let r = r0; r <= r1; r++) {
      const ym = r * ROW + 0.5 + ROW / 2;
      // The part of the stroke within w of this row, up and down: the cells to look at are by it.
      let xa = Math.min(ax, bx), xb = Math.max(ax, bx);
      if (Math.abs(dy) > 1) {
        const ta = clamp((ym - w - ay) / dy), tb = clamp((ym + w - ay) / dy);
        (xa = ax + dx * Math.min(ta, tb)), (xb = ax + dx * Math.max(ta, tb));
        if (xa > xb) [xa, xb] = [xb, xa];
      }
      const f = off[r]! - Math.floor(off[r]!);
      const k0 = Math.max(0, Math.ceil((xa - w) / cw - 0.5 + f)), k1 = Math.min(cols - 1, Math.floor((xb + w) / cw - 0.5 + f));
      for (let k = k0; k <= k1; k++) {
        const xm = (k + 0.5 - f) * cw;
        const u = L2 ? clamp(((xm - ax) * dx + (ym - ay) * dy) / L2) : 0;
        const d = Math.hypot(xm - ax - dx * u, ym - ay - dy * u);
        if (d < w) each(r, k, xm, ym, d, u);
      }
    }
  };
  /** A ring: the cells in it, a row's two spans of it (none of its inside or outside gone over). */
  const ringOf = (q: Shape, now: number) => {
    const o = q.rad + q.w, n = Math.max(0, q.rad - q.w);
    const r0 = Math.max(0, Math.ceil((q.y - o - 0.5 - ROW / 2) / ROW)), r1 = Math.min(rows - 1, Math.floor((q.y + o - 0.5 - ROW / 2) / ROW));
    for (let r = r0; r <= r1; r++) {
      const ym = r * ROW + 0.5 + ROW / 2, dy = ym - q.y;
      const xo = Math.sqrt(Math.max(0, o * o - dy * dy)), xi = Math.abs(dy) < n ? Math.sqrt(n * n - dy * dy) : 0;
      const f = off[r]! - Math.floor(off[r]!);
      for (const [a, b] of [[q.x - xo, q.x - xi], [q.x + xi, q.x + xo]] as const) {
        const k0 = Math.max(0, Math.ceil(a / cw - 0.5 + f)), k1 = Math.min(cols - 1, Math.floor(b / cw - 0.5 + f));
        for (let k = k0; k <= k1; k++) {
          const dx = (k + 0.5 - f) * cw - q.x;
          light(r, k, q.fade * clamp(1 - Math.abs(Math.hypot(dx, dy) - q.rad) / q.w), q.hue + Math.atan2(dy, dx) * 57.3 + now * 0.12);
        }
      }
    }
  };
  /** A star: its ten edges, each a stroke. */
  const starOf = (q: Shape, now: number) => {
    const tip = (j: number): [number, number] => {
      const a = q.turn + (j * Math.PI) / 5 - Math.PI / 2, d = j % 2 ? q.rad * 0.45 : q.rad;
      return [q.x + Math.cos(a) * d, q.y + Math.sin(a) * d];
    };
    for (let j = 0; j < 10; j++) {
      const [ax, ay] = tip(j), [bx, by] = tip(j + 1);
      stroke(ax, ay, bx, by, q.w, (r, k, x, y, d) => light(r, k, q.fade * (1 - d / q.w), q.hue === GLINT ? GLINT : q.hue + Math.atan2(y - q.y, x - q.x) * 57.3 + now * 0.12));
    }
  };
  /** A hue that is no hue: the light itself (white on a dark page, gold on a light one). */
  const GLINT = 9999;
  /** The rocket (kumo, 2026-11-26: finer): its way ahead a faint thin line; the rocket a point of
   *  light with four rays; behind it a tail as of a comet, thick and light at the head, thinner
   *  and into the rainbow and out, a few hundred px long. */
  const rocketOf = (rn: RocketNow, now: number) => {
    const { k } = rn, w = ROW * 0.9, TAIL = Math.min(360, k.total * 0.6);
    for (let i = 1; i < k.pts.length; i++) {
      const [ax, ay] = k.pts[i - 1]!, [bx, by] = k.pts[i]!, s0 = k.cum[i - 1]!, L = k.cum[i]! - s0;
      if (s0 > rn.head) {
        stroke(ax, ay, bx, by, cw * 0.7, (r, c, _x, _y, _d, u) => light(r, c, 0.3, (s0 + L * u) * 0.9 - now * 0.25));
        continue;
      }
      stroke(ax, ay, bx, by, w * 1.9, (r, c, _x, _y, d, u) => {
        const at = s0 + L * u, back = rn.head - at;
        if (back < 0) return d < cw * 0.7 ? light(r, c, 0.3, at * 0.9 - now * 0.25) : undefined;
        if (back > TAIL) return;
        const q = 1 - back / TAIL, wide = w * (0.25 + 1.6 * q * q);
        if (d < wide) light(r, c, q * (1 - (d / wide) ** 2), back < 46 && d < wide * 0.5 ? GLINT : at * 0.9 - now * 0.3);
      });
    }
    // Its head: a disc of light, and four rays that turn and flicker.
    const { hx, hy } = rn, ray = ROW * (3.2 + 0.8 * Math.sin(now / 37)), a = now / 300;
    stroke(hx, hy, hx, hy, ROW * 1.5, (r, c, _x, _y, d) => light(r, c, 1 - (d / (ROW * 1.5)) ** 2, GLINT));
    for (let j = 0; j < 2; j++) {
      const dx = Math.cos(a + j * 1.5708) * ray, dy = Math.sin(a + j * 1.5708) * ray;
      stroke(hx - dx, hy - dy, hx + dx, hy + dy, cw * 0.8, (r, c, _x, _y, _d, u) => light(r, c, 1 - Math.abs(u * 2 - 1) * 0.7, GLINT));
    }
  };

  // The canvas's left and right tenth go to none (a CSS mask before: done in the canvas, what is
  // drawn in it is whole there).
  let edgeOn = true;
  const edges = (s: number) => {
    if (!edgeOn || s < 0.01) return;
    const w = W * 0.1;
    g.globalCompositeOperation = "destination-out";
    for (const x of [0, W]) {
      const gr = g.createLinearGradient(x, 0, x ? W - w : w, 0);
      gr.addColorStop(0, `rgba(0,0,0,${s})`);
      gr.addColorStop(1, "rgba(0,0,0,0)");
      g.fillStyle = gr;
      g.fillRect(x ? W - w : 0, 0, w, H);
    }
    g.globalCompositeOperation = "source-over";
  };


  // The figure (kumo, 2026-11-26): where no agent is to be drawn, a band of the code's own
  // characters in the pane's middle: the track of something going round a ball, as Iridium's
  // satellites go round theirs (named for the element's 77 electrons): only the part of it lately
  // gone over is drawn, thick at its head and thin to its tail, the rest empty. Its place on the
  // ball: up and down by an angle that goes on evenly, round the ball by `turn` times as much.
  // With turn 1 the track is a figure of eight (Viviani's curve), gone over again and again; turn
  // drifts away from 1 of itself now and then, and the track no longer closes: each round falls
  // beside the last, and it draws stars of five and seven points; drifting back, the eight again.
  // One line, one rule, nothing staged. The ball turns to the left and the right; the rainbow
  // flows along the band; the code flows through it. Laid down each frame as points of the band
  // (RN along it, a few across), each on the cell it falls in, the nearest kept; no cell asked
  // anything. A star over a part of it (draw) stirs that part (stir): another run of colours
  // there, wider, going out along the band both ways and dying away.
  const RN = 640;
  const PX = new Float32Array(RN + 1), PY = new Float32Array(RN + 1), PZ = new Float32Array(RN + 1);
  // Where its head is: the angle up and down, and round the ball.
  let headU = 0, headL = 0;
  // Its flare (Iridium's satellites were known for theirs: sunlight off a panel, a point brighter
  // than any star for a second): when the last one was, when the next is due; and how far each
  // cell is that light, not the rainbow, for it and for the satellite itself at the head.
  let flareAt = -1e9, flareNext = 0, rw = new Float32Array(0);
  let ra = new Float32Array(0), rh = new Float32Array(0), rz = new Float32Array(0), ru = new Int16Array(0), rowRing = new Uint8Array(0), ringCells = new Int32Array(0), nRing = 0;
  let stir = new Float32Array(RN), stir2 = new Float32Array(RN);
  let ringK = 0, ringAt = 0;
  const ringLay = (now: number) => {
    for (let j = 0; j < nRing; j++) (ra[ringCells[j]!] = 0), (rw[ringCells[j]!] = 0);
    nRing = 0;
    rowRing.fill(0);
    const dt = Math.min(50, now - ringAt);
    ringAt = now;
    if (ringK < 0.01 || !rows) return;
    const ringIn = ringK * ringK * (3 - 2 * ringK);
    // What is stirred goes on to its neighbours, less each time, and dies away.
    const dec = Math.exp(-dt / 700);
    for (let j = 0; j < RN; j++) stir2[j] = Math.max(stir[j]!, (stir[(j + 1) % RN]! + stir[(j + RN - 1) % RN]!) * 0.49) * dec;
    [stir, stir2] = [stir2, stir];
    const top = footT >= 0 ? footT : H, t = still ? 5000 : now;
    const R = Math.min(W * 0.34, top * 0.3, 220), wv = Math.max(7, R * 0.085), xc = W / 2, yc = cy >= 0 ? cy : top * 0.5;
    // How many times round for once up and down: 1 most of the time, away from it (as far as 7/4,
    // sevenfold) for a while now and then, there and back smoothly.
    const far = Math.max(0, Math.sin(t / 8300) * Math.sin(t / 19700 + 1.3)) ** 2, turn = 1 + 0.75 * far;
    const du = still ? 0 : (dt / 1000) * 0.6;
    headU += du;
    headL += du * turn;
    // At rest (turn 1) the eight is brought round to face the eye again, slowly: its crossing in
    // front, or behind.
    if (far < 0.02) {
      const d = headL - headU, to = Math.round(d / Math.PI) * Math.PI;
      headL += (to - d) * (1 - Math.exp(-dt / 1500));
    }
    // The part drawn: so far back from its head, a little short of the whole eight.
    const span = 6.2832 * 0.84;
    for (let j = 0; j <= RN; j++) {
      const sb = (j / RN) * span, u = headU - sb, l = headL - sb * turn, cu = Math.cos(u);
      (PX[j] = Math.sin(u)), (PY[j] = cu * Math.sin(l)), (PZ[j] = cu * Math.cos(l));
    }
    // The ball turns about its upright, to the left and back to the right, nods a little, and lies
    // aslant in the pane.
    const a1 = 0.9 * Math.sin(t / 2600), a2 = 0.3 + 0.12 * Math.sin(t / 3700), a3 = 0.46;
    const c1 = Math.cos(a1), s1 = Math.sin(a1), c2 = Math.cos(a2), s2 = Math.sin(a2), c3 = Math.cos(a3), s3 = Math.sin(a3);
    const flow = t * 0.09;
    if (!flareNext) flareNext = now + 6000;
    if (!still && now > flareNext) (flareAt = now), (flareNext = now + 9000 + Math.random() * 12000);
    // Up at once, gone in a second.
    const fl = now - flareAt < 90 ? (now - flareAt) / 90 : Math.exp(-(now - flareAt - 90) / 380);
    for (let iu = 0; iu < RN; iu++) {
      const q = iu / RN, e = stir[iu]!;
      // The satellite itself at the very head; the flare on it and a little way back.
      const white = Math.max(q < 0.012 ? 1 : 0, fl > 0.02 && q < 0.14 ? fl * (1 - q / 0.14) : 0);
      // Thick just behind its head, thinner all the way to its tail.
      const wide = wv * (q < 0.03 ? 0.3 + (q / 0.03) * 0.7 : Math.pow(1 - q, 0.6) * 0.92 + 0.08) * (1 + 0.6 * e) * (1 + 0.8 * white);
      const hue0 = q * 620 - flow + e * 150, mx = PX[iu]!, my = PY[iu]!, mz = PZ[iu]!;
      // Across it: along the ball, square to its way.
      const tx = PX[iu + 1]! - mx, ty = PY[iu + 1]! - my, tz = PZ[iu + 1]! - mz;
      let ax = ty * mz - tz * my, ay = tz * mx - tx * mz, az = tx * my - ty * mx;
      const al = Math.hypot(ax, ay, az) || 1;
      (ax /= al), (ay /= al), (az /= al);
      const RM = Math.max(2, Math.ceil((wide * 2.6) / cw) + 1), lum = 1 - 0.35 * q;
      for (let iv = 0; iv < RM; iv++) {
        const vn = (iv / (RM - 1)) * 2 - 1, v = vn * wide;
        const x = mx * R + ax * v, y = my * R + ay * v, z = mz * R + az * v;
        const x1 = x * c1 + z * s1, z1 = z * c1 - x * s1;
        const y2 = y * c2 - z1 * s2, z2 = y * s2 + z1 * c2;
        const k = 1 + z2 / (R * 3);
        const row = ((yc + (x1 * s3 + y2 * c3) * k) / ROW) | 0;
        if (row < 0 || row >= rows) continue;
        const col = Math.round((xc + (x1 * c3 - y2 * s3) * k) / cw - 0.5 + off[row]! - Math.floor(off[row]!));
        if (col < 0 || col >= cols) continue;
        const i = row * cols + col;
        if (ra[i]! > 0 && rz[i]! >= z2) continue;
        if (ra[i] === 0) ringCells[nRing++] = i;
        rowRing[row] = 1;
        rz[i] = z2;
        ru[i] = iu;
        // In front, strong; behind, faint.
        // Coming and going, all of it by as much: not its strongest cells full till the last.
        ra[i] = Math.min(1, (0.62 + 0.38 * (z2 / R)) * lum + e * 0.4 + white) * ringIn;
        rw[i] = white;
        rh[i] = hue0 + vn * 30;
      }
    }
  };

  // The little game (slice.ts; kumo, 2026-11-26): its faces are cells in colours of their own,
  // laid over the code each frame (sp, 0xAABBGGRR a cell; 0: none) and put back to none the next.
  let sp = new Uint32Array(0), spCells = new Int32Array(0), nSp = 0;
  // The page told as the game begins and is over: its bars go for it and come back (splash.tsx).
  let told: ((on: boolean) => void) | null = null, playing = false, done: (() => void) | null = null;
  const tell = (on: boolean) => void (on !== playing && ((playing = on), told?.(on)));
  const sl = slicer({
    W: () => W,
    H: () => H,
    // The whole pane is its own while it is on (the bars gone for it).
    top: () => 14,
    bottom: () => H - 10,
    cw: () => cw,
    row: ROW,
    cols: () => cols,
    rows: () => rows,
    frac: (r) => off[r]! - Math.floor(off[r]!),
    put(i, c) {
      if (sp[i] === 0) spCells[nSp++] = i;
      sp[i] = c;
      rowLit[(i / cols) | 0] = 1;
    },
    line: (ax, ay, bx, by, w, v) => stroke(ax, ay, bx, by, w, (r, c, _x, _y, d) => light(r, c, v * (1 - (d / w) ** 2), GLINT)),
    burst(x, y, n, speed, size, lightOnly) {
      const a0 = Math.random() * 6.28;
      for (let j = 0; j < n; j++) pop(x, y, false, speed * (0.7 + Math.random() * 0.6), size * (0.7 + Math.random() * 0.6), a0 + (j / n) * 6.2832, 260, 700 + Math.random() * 400, lightOnly && j % 2 ? GLINT : Math.random() * 360);
      kick();
    },
    async sprite(who, size): Promise<Sprite | null> {
      const face = who?.src ? await picture(who.src) : null;
      const img = face ?? (who?.agent === "agy" ? await picture(AGY_DATA_URI) : null);
      const c = document.createElement("canvas"), cg = c.getContext("2d")!;
      c.width = c.height = SRC;
      const css = inkNow();
      if (!who) {
        // Iridium's own mark, on its rounded square (ui.tsx IrMark): the one not to be cut.
        // A block of one colour, white on a dark page and black on a light one, the letters cut
        // out of it: what is not to be cut is to be plain to see (kumo, 2026-11-26).
        cg.fillStyle = dark ? "#fff" : "#000";
        cg.beginPath();
        cg.roundRect(SRC * 0.03, SRC * 0.03, SRC * 0.94, SRC * 0.94, SRC * 0.2);
        cg.fill();
        cg.globalCompositeOperation = "destination-out";
        letters(cg, css);
        cg.globalCompositeOperation = "source-over";
      } else if (face) {
        const m = Math.min(face.naturalWidth, face.naturalHeight);
        cg.beginPath();
        cg.arc(SRC / 2, SRC / 2, SRC * 0.48, 0, 7);
        cg.clip();
        cg.drawImage(face, (face.naturalWidth - m) / 2, (face.naturalHeight - m) / 2, m, m, 0, 0, SRC, SRC);
      } else {
        // A mark as it is: nothing under it, no line round it (kumo, 2026-11-26: the real one).
        if (img) cg.drawImage(img, SRC * 0.1, SRC * 0.1, SRC * 0.8, SRC * 0.8);
        else if (!who.agent || !mark(cg, who.agent, css)) return null;
      }
      const w = Math.max(6, Math.round(size / cw)), h = Math.max(4, Math.round(size / ROW));
      // Down by halves: one step from SRC skips pixels.
      const mid = document.createElement("canvas"), mc = mid.getContext("2d")!;
      mid.width = w * 3;
      mid.height = h * 3;
      mc.imageSmoothingQuality = "high";
      mc.drawImage(c, 0, 0, w * 3, h * 3);
      const out = document.createElement("canvas"), oc = out.getContext("2d", { willReadFrequently: true })!;
      out.width = w;
      out.height = h;
      oc.imageSmoothingQuality = "high";
      oc.drawImage(mid, 0, 0, w, h);
      let d: Uint8ClampedArray;
      try {
        d = oc.getImageData(0, 0, w, h).data;
      } catch {
        return null; // a picture not ours to read
      } finally {
        // Let go at once: WebKit holds a canvas's memory till its size is 0.
        c.width = c.height = mid.width = mid.height = out.width = out.height = 0;
      }
      const px = new Uint32Array(w * h);
      for (let j = 0; j < w * h; j++) {
        const a = d[j * 4 + 3]!;
        if (a < 24) continue;
        let r = d[j * 4]!, g2 = d[j * 4 + 1]!, b = d[j * 4 + 2]!;
        if (!dark && who) {
          // On a light page (kumo, 2026-11-26: faint there): its colours deeper and stronger, a
          // light one on white being hardly seen.
          const m = (r + g2 + b) / 3;
          (r = clamp((m + (r - m) * 1.45) * 0.72, 0, 255)), (g2 = clamp((m + (g2 - m) * 1.45) * 0.72, 0, 255)), (b = clamp((m + (b - m) * 1.45) * 0.72, 0, 255));
        }
        px[j] = ((Math.max(dark ? 70 : 110, a) << 24) | (b << 16) | (g2 << 8) | r) >>> 0;
      }
      return { w, h, px };
    },
    // The pane's bars come back as its count comes (kumo, 2026-11-26), not once that is gone.
    ended: () => tell(false),
    over: () => (tell(false), done?.(), (done = null), kick()),
    dark: () => dark,
  });

  const draw = () => {
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    g.clearRect(0, 0, W, H);
    g.font = FONTS(1);
    g.textBaseline = "top";
    const now = performance.now();
    // A face at rest: only the rows and columns about it are drawn, the code beyond its rim being
    // none (kumo, 2026-11-25: the whole pane was gone over each frame for it, and it lagged).
    const rim = photo && !leaving && now - t0 > spreadNow + TURN + 400 ? ps * 0.6 : 0;
    for (let j = 0; j < nTouched; j++) lit[touched[j]!] = 0;
    nTouched = 0;
    rowLit.fill(0);
    for (let j = 0; j < nSp; j++) sp[spCells[j]!] = 0;
    nSp = 0;
    let heavy = 0;
    ringLay(now);
    for (const q of shapes(now)) (q.ring ? ringOf : starOf)(q, now);
    const rs = rocketNow(now);
    if (rs) rocketOf(rs, now);
    sl.paint();
    // Time stood still: the rest of the code dimmed for it.
    const dim = rs && rs.stop ? 1 - 0.45 * Math.sin(Math.PI * Math.min(1, (now - rs.k.at) / STOP)) ** 0.5 : 1;
    for (let r = 0; r < rows; r++) {
      const o = Math.floor(off[r]!), f = off[r]! - o, row = text[r]!, y = r * ROW + 0.5, ym = y + ROW / 2;
      const env = seen(ym);
      const rr = rowRing[r] === 1;
      const lr = rowLit[r] === 1 || rr;
      if (!lr && (env < 0.01 && fr[r]! < 0.01)) continue;
      if (!lr && rim && Math.abs(ym - py) > rim) continue;
      const k0 = rim && !lr ? Math.max(0, Math.floor((px - rim) / cw)) : 0;
      const k1 = rim && !lr ? Math.min(cols, Math.ceil((px + rim) / cw) + 1) : cols;
      let run = "", at = 0, key = -1;
      const flush = () => {
        if (!run || !(key & 31)) return;
        // (one of the game's faces: in the heavier weight, a colour on a thin stroke being faint)
        if (key >> 20 !== heavy) g.font = (heavy = key >> 20) ? `bold ${FONTS(1)}` : FONTS(1);
        g.fillStyle = `rgba(${((key >> 5) & 31) << 3},${((key >> 10) & 31) << 3},${((key >> 15) & 31) << 3},${(key & 31) / 24})`;
        g.fillText(run, (at - f) * cw, y);
      };
      for (let k = k0; k < k1; k++) {
        const i = r * cols + k;
        let cr = cur.r[i]!, cg = cur.g[i]!, cb = cur.b[i]!;
        // A star, ring or the rocket over this cell: the cell's own character in its colour.
        let lv = lr ? lit[i]! : 0;
        const rv = rr ? ra[i]! : 0;
        let ca = clamp(cur.a[i]!) * (env + (1 - env) * (fm[i] ?? 0)) * dim;
        if (rv > 0) {
          // The ring's cell: in the ring's colour; a star over it stirs the ring there, and is not
          // drawn over it.
          if (lv > 0.3 && ru[i]! >= 0) stir[ru[i]!] = 1;
          lv = 0;
          const h3 = hueAt3(rh[i]!), wq = rw[i]!;
          // (white on a dark page; on a light one the sun's own gold: black there was ugly, kumo 2026-11-26)
          const g0 = dark ? 255 : 255, g1 = dark ? 255 : 160, g2 = dark ? 255 : 0;
          const qr = HUES[h3]! + (g0 - HUES[h3]!) * wq, qg = HUES[h3 + 1]! + (g1 - HUES[h3 + 1]!) * wq, qb = HUES[h3 + 2]! + (g2 - HUES[h3 + 2]!) * wq;
          // Over what is there (a face coming or going), by its own strength: the cell's own
          // opacity was its before, and the ring stood full on a face till it was gone, and came
          // full at once (kumo, 2026-11-26).
          const a = rv + ca * (1 - rv), u = rv / (a || 1);
          (cr = cr + (qr - cr) * u), (cg = cg + (qg - cg) * u), (cb = cb + (qb - cb) * u);
          ca = a;
        }
        // One of the game's faces over this cell: its colour, and nothing else's.
        const sv = lr ? sp[i]! : 0;
        if (sv) (cr = sv & 255), (cg = (sv >> 8) & 255), (cb = (sv >> 16) & 255), (lv = lv > 0 && litHue[i] === GLINT ? lv : 0);
        if (lv > 0) {
          const m = Math.min(1, lv * 1.4);
          if (litHue[i]! === GLINT) (cr += (255 - cr) * m), (cg += ((dark ? 255 : 170) - cg) * m), (cb += ((dark ? 255 : 0) - cb) * m);
          else {
            const h3 = hueAt3(litHue[i]!);
            (cr += (HUES[h3]! - cr) * m), (cg += (HUES[h3 + 1]! - cg) * m), (cb += (HUES[h3 + 2]! - cb) * m);
          }
        }
        const k2 = Math.min(24, Math.round(Math.max(ca, lv * 0.95, sv ? (sv >>> 24) / 255 : 0) * 24)) | ((cr >> 3) << 5) | ((cg >> 3) << 10) | ((cb >> 3) << 15) | (sv ? 1 << 20 : 0);
        if (k2 !== key) {
          flush();
          run = "";
          at = k;
          key = k2;
        }
        // On the way a cell is another character, one cell in four, changed eight times a second:
        // not a new one each frame.
        const h = busy[i] ? Math.imul(i ^ Math.imul(tick, 0x9e3779b1), 0x85ebca6b) >>> 0 : 1;
        let ch = h % 4 === 0 ? GLYPHS[(h >>> 8) % GLYPHS.length]! : (row[(k + o) % len] ?? " ");
        // Under one of the game's faces no cell is empty: where the code has a space, a character of
        // the code's own, the same one as the row flows (a face full of gaps is no face).
        if (sv && ch === " ") ch = GLYPHS[(Math.imul((k + o) % len ^ (r * 131), 0x9e3779b1) >>> 8) % GLYPHS.length]!;
        run += ch;
      }
      flush();
    }
    edges(1);
  };

  // What the pointer leaves (kumo, 2026-11-25): stars and rings, growing out from a point, drawn
  // in no character of their own: the code's rows go on as they are, and those of their characters
  // a shape's outline lies over are in the rainbow (kumo: not a new character put in).
  type Spark = { x: number; y: number; vx: number; vy: number; at: number; life: number; ring: boolean; size: number; hue: number; spin: number; fall: number };
  type Shape = { x: number; y: number; rad: number; turn: number; ring: boolean; fade: number; hue: number; w: number };
  let pops: Spark[] = [];
  // The second ring of a press, on its way.
  let lx = 0, ly = 0, lensOn = false;
  /** way: which way it goes (none: any); fall: how fast it comes to fall, px a second each second;
   *  hue GLINT: of light, not a colour. */
  const pop = (x: number, y: number, ring: boolean, speed: number, size: number, way = Math.random() * Math.PI * 2, fall = 0, life = 0, hue = Math.random() * 360) => {
    pops.push({ x, y, vx: Math.cos(way) * speed, vy: Math.sin(way) * speed - (ring || fall ? 0 : 10), at: performance.now(), life: life || (ring ? 1100 + Math.random() * 500 : 900 + Math.random() * 500), ring, size, hue, spin: (Math.random() - 0.5) * 1.5, fall });
    if (pops.length > 56) pops = pops.slice(-56);
  };
  /** The shapes now: where each is, how far out it has grown, how much of it is left. */
  const shapes = (now: number): Shape[] => {
    if (!pops.length) return [];
    pops = pops.filter((p) => now - p.at < p.life);
    return pops.map((p) => {
      const t = (now - p.at) / p.life, s2 = (now - p.at) / 1000;
      // Out from a point to its size, fast at first, slowing; going in its last part.
      const rad = Math.max(cw, p.size * (1 - (1 - t) ** 3)), w = ROW * 0.8;
      return { x: p.x + p.vx * s2, y: p.y + p.vy * s2 + 0.5 * p.fall * s2 * s2, rad, turn: p.spin * s2, ring: p.ring, fade: t < 0.55 ? 1 : 1 - (t - 0.55) / 0.45, hue: p.hue, w };
    });
  };
  // A zigzag drawn with the pointer (kumo, 2026-11-25): time stands still (the flow and the stars
  // stop, the rest of the code dims) as the line drawn lights up along the code, then a rocket
  // goes along it, faster and faster, its trail behind it, and bursts at its end.
  type Rocket = { pts: [number, number][]; cum: number[]; total: number; at: number; flown: boolean; puff: number; fly: number };
  type RocketNow = { k: Rocket; stop: boolean; drawn: number; head: number; hx: number; hy: number };
  // No standing still before it goes (kumo, 2026-11-26: the page seemed stuck).
  const STOP = 0;
  /** How far along its way at u of its flight: off slowly, faster and faster, a little slower in. */
  const sped = (u: number) => (u < 0.85 ? Math.pow(u / 0.85, 2.4) * 0.8 : 0.8 + 0.2 * (1 - (1 - (u - 0.85) / 0.15) ** 2));
  let rk: Rocket | null = null;
  let path: { x: number; y: number; t: number }[] = [];
  let looked = 0, held = false, heldAt = 0, lastHeld = 0;
  /** The pointer's way, its corners only (Ramer–Douglas–Peucker, eps px). */
  const corners = (pts: { x: number; y: number }[], eps: number): [number, number][] => {
    const keep = new Uint8Array(pts.length);
    keep[0] = keep[pts.length - 1] = 1;
    const go = (a: number, b: number) => {
      const A = pts[a]!, B = pts[b]!, L = Math.hypot(B.x - A.x, B.y - A.y) || 1;
      let far = 0, at = -1;
      for (let i = a + 1; i < b; i++) {
        const d = Math.abs((B.x - A.x) * (A.y - pts[i]!.y) - (A.x - pts[i]!.x) * (B.y - A.y)) / L;
        if (d > far) (far = d), (at = i);
      }
      if (far > eps && at > 0) (keep[at] = 1), go(a, at), go(at, b);
    };
    go(0, pts.length - 1);
    return pts.filter((_, i) => keep[i]).map((p) => [p.x, p.y]);
  };
  /** A zigzag in the pointer's way: two sharp turns or more, between strokes long enough. */
  const zigzag = (): [number, number][] | null => {
    if (path.length < 8) return null;
    const v = corners(path, 14);
    if (v.length < 4 || v.length > 9) return null;
    let sharp = 0, total = 0;
    for (let i = 1; i < v.length; i++) {
      const L = Math.hypot(v[i]![0] - v[i - 1]![0], v[i]![1] - v[i - 1]![1]);
      total += L;
      if (L < 50) return null;
      if (i < v.length - 1) {
        const ax = v[i]![0] - v[i - 1]![0], ay = v[i]![1] - v[i - 1]![1], bx = v[i + 1]![0] - v[i]![0], by = v[i + 1]![1] - v[i]![1];
        if ((ax * bx + ay * by) / (Math.hypot(ax, ay) * Math.hypot(bx, by) || 1) < -0.2) sharp++;
      }
    }
    return sharp >= 2 && total > 260 ? v : null;
  };
  const launch = (pts: [number, number][]) => {
    const cum = [0];
    for (let i = 1; i < pts.length; i++) cum.push(cum[i - 1]! + Math.hypot(pts[i]![0] - pts[i - 1]![0], pts[i]![1] - pts[i - 1]![1]));
    rk = { pts, cum, total: cum.at(-1)!, at: performance.now(), flown: false, puff: 0, fly: clamp(cum.at(-1)! * 1.5, 700, 1500) };
    path = [];
    kick();
  };
  /** The point s along the rocket's way. */
  const along = (k: Rocket, s: number): [number, number] => {
    let i = 1;
    while (i < k.cum.length - 1 && k.cum[i]! < s) i++;
    const a = k.pts[i - 1]!, b = k.pts[i]!, L = k.cum[i]! - k.cum[i - 1]! || 1, u = clamp((s - k.cum[i - 1]!) / L);
    return [a[0] + (b[0] - a[0]) * u, a[1] + (b[1] - a[1]) * u];
  };
  const rocketNow = (now: number): RocketNow | null => {
    if (!rk) return null;
    const t = now - rk.at;
    const stop = t < STOP;
    const drawn = STOP ? clamp(t / (STOP * 0.7)) : 1;
    const head = stop ? -1 : rk.total * sped(clamp((t - STOP) / rk.fly));
    const [hx, hy] = head < 0 ? [-1e9, -1e9] : along(rk, head);
    return { k: rk, stop, drawn: drawn * (2 - drawn), head, hx, hy };
  };

  const frame = (now: number) => {
    raf = 0;
    if (dead) return;
    // Only flowing: every other frame is enough, half the work.
    if (calm && (odd = !odd)) return kick();
    // The time since the last frame drawn, from the frames' own clock alone: set from outside (a
    // pointer's move, later than the frame it is in) it came to nothing, and the flow stood still
    // under a pointer (kumo, 2026-11-25).
    const dt = Math.max(0, Math.min(50, now - last)) / 1000;
    last = now;
    tick = Math.floor(now / 125);
    // The flow gathers and dies away: its speed goes to what is asked of it, by a part each frame.
    // It flows the whole time it is there (kumo, 2026-11-25), slowly; faster for a moment as it
    // comes, as its figure changes, as the effort is moved.
    const burst = now < flowTo;
    const want = still ? 0 : burst ? 1 : IDLE;
    vel += (want - vel) * (1 - Math.exp(-dt / (burst ? 0.4 : 0.7)));
    if (!(vel >= 0 && vel <= 1.5)) vel = want; // never out of what a speed is, whatever a frame's time was
    const flow = vel > 0.004 ? vel : 0;
    const frozen = !!rk && now - rk.at < STOP;
    if (frozen) for (const p of pops) p.at += dt * 1000;
    if (rk && !frozen) {
      const t = now - rk.at;
      if (!rk.flown) (rk.flown = true), (flowTo = now + rk.fly + 600);
      if (t < rk.fly && now - rk.puff > 42) {
        // Sparks off its tail: small, to either side of its way, falling.
        rk.puff = now;
        const s0 = rk.total * sped(t / rk.fly), [hx, hy] = along(rk, s0), [bx, by] = along(rk, Math.max(0, s0 - 12));
        const back = Math.atan2(by - hy, bx - hx) + (Math.random() - 0.5) * 1.6;
        pop(hx, hy, false, 40 + Math.random() * 70, 7 + Math.random() * 9, back, 260, 520 + Math.random() * 380, Math.random() < 0.3 ? GLINT : Math.random() * 360);
      }
      if (t >= rk.fly) {
        // Its end: a star of light out from the point at once; a round of stars flung out evenly
        // and falling as they go; small ones after them.
        const [ex, ey] = rk.pts.at(-1)!;
        rk = null;
        // And the game begins (kumo, 2026-11-26), where the figure is.
        if (rainbow && !leaving && !left) sl.start(), tell(true);
        pop(ex, ey, false, 0, 78, 0, 0, 520, GLINT);
        const n = 11, a0 = Math.random() * 6.28, hue0 = Math.random() * 360;
        for (let j = 0; j < n; j++) pop(ex, ey, false, 190 + (j % 2) * 110, 20 + (j % 2) * 14, a0 + (j / n) * 6.2832, 170, 1100 + Math.random() * 400, hue0 + (j / n) * 360);
        for (let j = 0; j < 8; j++) pop(ex, ey, false, 60 + Math.random() * 120, 8 + Math.random() * 8, Math.random() * 6.28, 240, 700 + Math.random() * 500, GLINT);
      }
    }
    // Held down (kumo, 2026-11-25): more of them all the while, and the band let out that way.
    if (sl.on()) sl.step(dt * 1000);
    // Held: stars flung out all round it, a few at a time, evenly, for as long as it is.
    else if (held && lensOn && now - heldAt > 260 && now - lastHeld > 150) {
      lastHeld = now;
      const a0 = Math.random() * 6.2832;
      for (let j = 0; j < 3; j++) pop(lx, ly, false, 150 + Math.random() * 130, 24 + Math.random() * 16, a0 + j * 2.0944 + (Math.random() - 0.5) * 0.6, 120);
    }
    if (flow > 0 && !frozen) for (let r = 0; r < rows; r++) off[r] = (((off[r]! + speed[r]! * flow * pace(pw) * dt) % len) + len) % len;
    // The figure's place, size and strength glide to what is asked: the cells read anew each frame.
    const [gx, gs] = goal();
    const [gc, gh] = winGoal();
    const gy = yGoal(gc, gs);
    const gliding = Math.abs(gc - wc) > 0.4 || Math.abs(gh - wh) > 0.4 || Math.abs(gx - px) > 0.4 || Math.abs(gy - py) > 0.4 || Math.abs(gs - ps) > 0.4 || Math.abs(pwT - pw) > 0.004;
    if (gliding) {
      const k = 1 - Math.exp(-dt / 0.3), kw = up || down ? 1 - Math.exp(-dt / 0.9) : k;
      wc += (gc - wc) * kw;
      wh += (gh - wh) * kw;
      px += (gx - px) * k;
      py += (gy - py) * k;
      ps += (gs - ps) * k;
      pw += (pwT - pw) * k;
    } else if (wasGliding) (wc = gc), (wh = gh), (px = gx), (py = gy), (ps = gs), (pw = pwT);
    if ((gliding || wasGliding) && !leaving) to = read();
    const span = now - t0;
    const turning = span < spreadNow + TURN + 400;
    if (!turning && !leaving && (gliding || wasGliding)) (cur.a.set(to.a), cur.r.set(to.r), cur.g.set(to.g), cur.b.set(to.b));
    wasGliding = gliding;
    if (turning) {
      for (let i = 0; i < cur.a.length; i++) {
        const p = clamp((span - wait[i]!) / TURN);
        const e = p * p * (3 - 2 * p);
        cur.a[i] = from.a[i]! + (to.a[i]! - from.a[i]!) * e;
        cur.r[i] = from.r[i]! + (to.r[i]! - from.r[i]!) * e;
        cur.g[i] = from.g[i]! + (to.g[i]! - from.g[i]!) * e;
        cur.b[i] = from.b[i]! + (to.b[i]! - from.b[i]!) * e;
        // A cell the figure comes to or leaves is other characters on the way, its own again after.
        busy[i] = p > 0 && p < 1 && Math.abs(to.a[i]! - from.a[i]!) > 0.2 ? 1 : 0;
      }
    } else if (wasTurning) {
      busy.fill(0);
      // Gone to the last cell: told, once.
      if (leaving) {
        const done = leaving;
        leaving = null;
        left = true;
        draw();
        return done();
      }
    }
    wasTurning = turning;
    // The ring there where Iridium's letters would be (no agent to draw), coming and going.
    const ringOn = ringAsk && !leaving && !left && (!sl.on() || sl.going()) ? 1 : 0;
    ringK += (ringOn - ringK) * (1 - Math.exp(-dt / (sl.on() ? 0.9 : 0.4)));
    if (Math.abs(ringOn - ringK) < 0.005) ringK = ringOn;
    draw();
    calm = ringK < 0.01 && !sl.on() && !burst && !turning && !gliding && !pops.length && !lensOn && !rk && !held && Math.abs(vel - want) < 0.02;
    if (flow > 0 || want || turning || gliding || pops.length || rk || held || sl.on()) kick();
  };
  const kick = () => {
    if (!raf && !dead) raf = requestAnimationFrame(frame);
  };

  /** Goes over to the figure as painted: at once (snap), or cell by cell from the middle. */
  const turn = (snap: boolean) => {
    to = read();
    if (snap || still) {
      cur = { a: to.a.slice(), r: to.r.slice(), g: to.g.slice(), b: to.b.slice() };
      from = cur;
      t0 = -1e9;
      draw();
      if (!still) kick();
      return;
    }
    from = { a: cur.a.slice(), r: cur.r.slice(), g: cur.g.slice(), b: cur.b.slice() };
    const far = Math.hypot(W / 2, H / 2) || 1;
    for (let i = 0; i < wait.length; i++) wait[i] = (Math.hypot(((i % cols) + 0.5) * cw - px, (((i / cols) | 0) + 0.5) * ROW - py) / far) * SPREAD * (0.92 + Math.random() * 0.16);
    t0 = performance.now();
    spreadNow = SPREAD;
    // Faster for a moment as it first comes; a figure changing leaves the flow as it is.
    if (first) flowTo = t0 + 2600;
    first = false;
    kick();
  };

  const show = (f: Fig | null, snap: boolean) => {
    const n = ++asked;
    ringAsk = !f?.src && !f?.alt && !(f?.agent && ["claude", "codex", "pi", "dsh", "agy"].includes(f.agent));
    kick();
    void load(f).then(({ face, img }) => {
      if (n !== asked || dead || leaving || left) return;
      photo = paint(f, face, img);
      ringAsk = rainbow;
      drawnKey = f?.key ?? "";
      turn(snap);
    });
  };

  /** What is asked, at once (no motion wanted, or nothing drawn yet to move from). */
  const settle = () => {
    [px, ps] = goal();
    [wc, wh] = winGoal();
    py = yGoal(wc, ps);
    pw = pwT;
    if (!cur.a.length || drawnKey !== (fig?.key ?? "")) return;
    to = read();
    cur = { a: to.a.slice(), r: to.r.slice(), g: to.g.slice(), b: to.b.slice() };
    from = cur;
    draw();
  };

  const resize = () => {
    const w = cv.clientWidth, h = cv.clientHeight;
    if (!w || !h || (w === W && h === H)) return;
    const grown = W > 0;
    const was = grown ? { cols, rows, text, off, speed, cur } : null;
    W = w;
    H = h;
    dpr = Math.min(2, window.devicePixelRatio || 1);
    cv.width = Math.round(W * dpr);
    cv.height = Math.round(H * dpr);
    g.font = `${FONT}px ui-monospace, "SF Mono", Menlo, "JetBrains Mono", "Cascadia Mono", "DejaVu Sans Mono", monospace`;
    cw = g.measureText("M").width || 5.4;
    [px, ps] = goal();
    [wc, wh] = winGoal();
    py = yGoal(wc, ps);
    pw = pwT;
    cols = Math.ceil(W / cw) + 1;
    rows = Math.floor(H / ROW);
    len = cols * 2 + 40;
    const n = cols * rows;
    cur = field(n);
    for (let i = 0; i < n; i++) (cur.r[i] = ink[0]), (cur.g[i] = ink[1]), (cur.b[i] = ink[2]);
    from = cur;
    wait = new Float32Array(n);
    lit = new Float32Array(n);
    litHue = new Float32Array(n);
    touched = new Int32Array(n);
    nTouched = 0;
    rowLit = new Uint8Array(rows);
    sp = new Uint32Array(n);
    spCells = new Int32Array(n);
    nSp = 0;
    ra = new Float32Array(n);
    rw = new Float32Array(n);
    rh = new Float32Array(n);
    rz = new Float32Array(n);
    ru = new Int16Array(n);
    ringCells = new Int32Array(n);
    nRing = 0;
    rowRing = new Uint8Array(rows);
    busy = new Uint8Array(n);
    off = new Float32Array(rows);
    speed = new Float32Array(rows);
    text = [];
    for (let r = 0; r < rows; r++) {
      // Made another size (the page laid out, the window sized): the rows it had go on as they
      // were, each where it was; only new ones are made (kumo, 2026-11-25: all of it was made anew
      // each time, and it jumped about as the page came).
      const old = was && r < was.rows ? was.text[r]! : null;
      text.push(old ? (old.length >= len ? old.slice(0, len) : [...old, ...line(len - old.length)]) : line(len));
      off[r] = old ? was!.off[r]! % len : Math.random() * len;
      speed[r] = old ? was!.speed[r]! : (r % 2 ? -1 : 1) * (1.8 + Math.random() * 4.2); // cells a second
    }
    if (was)
      for (let r = 0; r < Math.min(rows, was.rows); r++)
        for (let k = 0; k < Math.min(cols, was.cols); k++) {
          const i = r * cols + k, j = r * was.cols + k;
          (cur.a[i] = was.cur.a[j]!), (cur.r[i] = was.cur.r[j]!), (cur.g[i] = was.cur.g[j]!), (cur.b[i] = was.cur.b[j]!);
        }
    if (!apart && !grown && kept && kept.cols === cols && kept.rows === rows && performance.now() - kept.at < 2000) {
      ({ text, off, speed, cur } = kept);
      from = cur;
      first = false;
      draw();
      const same = kept.key === (fig?.key ?? "");
      kept = null;
      return show(fig, same);
    }
    show(fig, grown);
  };

  return {
    figure(f) {
      if ((f?.key ?? "") === (fig?.key ?? "") && drawnKey === (f?.key ?? "")) return;
      fig = f;
      if (W) show(f, false);
    },
    resize,
    // Going (kumo, 2026-11-25: played whole, not cut): the code flows off and the cells go out
    // from the band's rim inwards, the figure's last; done as the last is out.
    leave(done) {
      if (leaving) return void (leaving = done);
      sl.stop();
      tell(false);
      if (still || !W || left) return done();
      asked++;
      from = { a: cur.a.slice(), r: cur.r.slice(), g: cur.g.slice(), b: cur.b.slice() };
      to = { a: new Float32Array(cur.a.length), r: cur.r.slice(), g: cur.g.slice(), b: cur.b.slice() };
      const far = Math.hypot(Math.max(px, W - px), H / 2) || 1;
      for (let i = 0; i < wait.length; i++) wait[i] = (1 - Math.hypot(((i % cols) + 0.5) * cw - px, (((i / cols) | 0) + 0.5) * ROW - py) / far) * SPREAD * 1.5 * (0.92 + Math.random() * 0.16);
      leaving = done;
      t0 = performance.now();
      spreadNow = SPREAD * 1.5;
      flowTo = t0 + 1500;
      kick();
    },
    /** Asked for again while going, or after: there again, its figure coming as at first. */
    enter() {
      if (!leaving && !left) return;
      leaving = null;
      left = false;
      if (W) show(fig, false);
    },
    power(f) {
      if (Math.abs(f - pwT) < 0.01) return;
      pwT = f;
      if (!W) return void (pw = f);
      if (still) return settle();
      flowTo = performance.now() + 1400;
      kick();
    },
    window(middle, height) {
      if (Math.abs(middle - wcT) < 1 && Math.abs(height - whT) < 1) return;
      const first = wcT < 0;
      wcT = middle;
      whT = height;
      if (!W) return;
      if (still || first || settling()) return settle();
      kick();
    },
    place(x, side, y = -1) {
      if (Math.abs(x - cx) < 1 && Math.abs(side - most) < 1 && Math.abs(y - cy) < 1) return;
      cy = y;
      cx = x;
      most = side;
      if (!W) return;
      if (still || px < 0 || settling()) return settle();
      kick();
    },
    // y -1: none (the figure in the window's middle).
    foot(y) {
      if (Math.abs(y - footT) < 1) return;
      footT = y;
      if (!W) return;
      if (still || px < 0 || settling()) return settle();
      kick();
    },
    // The page's colours changed (day and night): the figure and the code drawn again in the ink
    // now, at once (kumo, 2026-11-25: Ir kept the old one).
    recolor() {
      if (!W || leaving || left) return;
      show(fig, true);
    },
    near(x, y) {
      if (still || !W) return;
      const inside = x >= 0 && y >= 0 && x <= W && y <= H;
      if (!inside) {
        if (lensOn) (lensOn = false), kick();
        return;
      }
      const moved = Math.hypot(x - lx, y - ly);
      // The game's: a stroke pressed cuts what it goes through; no stars left, no zigzag looked for.
      if (sl.on()) {
        if (held && lensOn) sl.move(lx, ly, x, y);
        (lx = x), (ly = y), (lensOn = true);
        return kick();
      }
      lx = x;
      ly = y;
      lensOn = true;
      const now = performance.now();
      // A line is drawn pressed (a mouse's button down, a finger on the pane), not by a pointer
      // only passing (kumo, 2026-11-26: rockets went off unasked).
      if (moved > 3 && held) path.push({ x, y, t: now });
      path = path.filter((p) => now - p.t < 1400);
      if (!rk && now - looked > 60) {
        looked = now;
        const z = zigzag();
        if (z) launch(z);
      }
      // A pointer only passing leaves nothing (kumo, 2026-11-26): stars come of a press, and more
      // of them while it is held.
      kick();
    },
    tap(x, y) {
      if (still || !W) return;
      (lx = x), (ly = y), (lensOn = true);
      // A press: a few stars out from it (kumo, 2026-11-26). Not in the game, where it is a cut.
      if (!sl.on()) for (let j = 0; j < 4; j++) pop(x, y, false, 90 + Math.random() * 110, 26 + Math.random() * 16);
      heldAt = lastHeld = performance.now();
      kick();
    },
    hold(on) {
      held = on && !still;
      path = [];
      if (!on) sl.lift();
      if (held) kick();
    },
    edge(on) {
      if (on === edgeOn) return;
      edgeOn = on;
      if (W && still) draw();
      kick();
    },
    cast: (who) => sl.cast(who),
    onPlay: (h) => void (told = h),
    // The game begun at once, no zigzag drawn first (the ⋯ menu's Game): false while the letters
    // are not drawn yet. done: when it is over, its count gone.
    play(then) {
      if (sl.on()) return true;
      if (still || !W || !rainbow || leaving || left) return false;
      done = then;
      sl.start();
      tell(true);
      kick();
      return true;
    },
    stop() {
      sl.stop();
      tell(false);
      done = null;
      if (!apart && cols && rows && !leaving && !left) kept = { at: performance.now(), cols, rows, text, off, speed, cur, key: drawnKey };
      dead = true;
      cancelAnimationFrame(raf);
      if (apart) for (const c of [src, small, tiny]) c.width = c.height = 0;
    },
  };
}
