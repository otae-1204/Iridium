# 封存：主页小游戏（猫耳 Boss）

**旧代码，已停用，不要拿来用。** 现行唯一的游戏是切水果（`src/splash/slice.ts`，由 `src/splash/engine.ts` 驱动；⋯ 菜单的 Game 也是它）。没有 kumo 明说，不要恢复或搬用这里的任何代码（kumo，2026-11-26）。

2026-11-26 封存（kumo）。不参与构建：`attic/` 不在 `tsconfig.json` 的 include 里，没有任何文件引用它。

- `game.ts`：游戏本体（Boss、子代理、技能、血条与防御、折线火箭），只给已有字符上色，不加字符。
- `engine.ts`、`splash.tsx`：当时接好游戏的引擎与页面层（提交 `456f902`，v0.2.76），与 `game.ts` 配套。

## 恢复

1. `game.ts` 放回 `src/splash/`。
2. 对照这里的 `engine.ts`、`splash.tsx`，把游戏相关的部分并回现行文件（搜 `gm.`、`gst`、`allow`、`onGame`）。
3. 入口（右上角的心）在 `456f902` 里已摘掉，要自己加回：`draw()` 末尾用 `gm.stamp(gm.soul(), …gm.heart())` 画心，`tap()` 里按到心即 `begin()`，再按即 `quit()`。
4. 文案键 `game.move/hold/zig/win/lose/stream/echo/parallel/think/sub/flow/compact` 加回 `src/i18n/zh.ts`、`en.ts`；`.game-say` 动画加回 `src/styles.css`（`git show 456f902:web/src/styles.css`）。

## 实测（2026-11-25）

每帧脚本耗时：电脑约 2.3 ms，CPU 降速 4 倍的手机视口约 5 ms。
