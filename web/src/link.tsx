// The link to the daemon: a signal of five bars by its round trip (signal.ts; kumo + Neon,
// 2026-11-26, in place of a dot and its milliseconds), the exact time in its Tip. With the sidebar shown, right of the name at its top (the user,
// 2026-11-08); on a phone, in the chat's top bar left of ⋯ (2026-11-06). The daemon serving a
// newer build than the page's (and the page not loading it by itself yet), in its place ↻ 刷新,
// which loads it (2026-11-08). In the desktop app, with its own update downloaded, in its place
// 更新, which restarts the app on it (selfupdate.go, 2026-11-26).
import { CircleArrowUp, RotateCw } from "lucide-react";
import { t } from "./i18n";
import { ask, useApp } from "./irapp";
import { BARS, signalOf, type Signal } from "./signal";
import { useLink } from "./store";
import { cn, Tip } from "./ui";
import { reloadServed, useStale } from "./upgrade";

export function LinkDot({ className, side = "bottom" }: { className?: string; side?: "top" | "bottom" }) {
  const link = useLink();
  const stale = useStale();
  const update = useApp()?.update;
  if (update && !stale && link.live !== false)
    return (
      <Tip label={t("app.restartTo", { v: update })} side={side}>
        <button type="button" onClick={() => ask("update", "restart")} className={cn("g-pop g-press flex items-center gap-1 rounded-md px-1.5 text-xs font-medium text-ink-2 hover:text-ink", className)}>
          <CircleArrowUp size={11} aria-hidden />
          {t("app.update")}
        </button>
      </Tip>
    );
  if (stale && link.live !== false)
    return (
      <Tip label={t("top.reloadHint")} side={side}>
        <button type="button" onClick={reloadServed} className={cn("g-pop g-press flex items-center gap-1 rounded-md px-1.5 text-xs font-medium text-ink-2 hover:text-ink", className)}>
          <RotateCw size={11} aria-hidden />
          {t("top.reload")}
        </button>
      </Tip>
    );
  const label = link.live === false ? t("top.offline") : link.rtt !== null ? t("top.online", { ms: link.rtt }) : t("top.connecting");
  // No words by it (kumo + Neon, 2026-11-26): the exact round trip in its Tip, pointed at or pressed.
  return (
    <Tip label={label} side={side} tap>
      <button type="button" className={cn("grid h-6 place-items-center rounded-md px-1.5 outline-none", className)} role="status" aria-label={label}>
        <SignalIcon s={signalOf(link.live, link.rtt)} />
      </button>
    </Tip>
  );
}

const TONE = { run: "var(--run)", need: "var(--need)", err: "var(--err)" } as const;

/** Five bars, bottom-aligned, 15×12: as many lit as the signal says in its tone, the rest faint;
 *  cut off, all dark under a thin red slash; not measured yet, all faint, breathing. */
export function SignalIcon({ s }: { s: Signal }) {
  return (
    <svg width={15} height={12} viewBox="0 0 15 12" aria-hidden className={cn("block", s.kind === "wait" && "sig-wait")}>
      {BARS.map((h, i) => {
        const lit = s.kind === "lit" && i < s.bars;
        const fill = lit ? TONE[s.tone] : s.kind === "off" ? "var(--ink-3)" : "var(--ink-4)";
        return <rect key={i} x={i * 3.25} y={12 - h} width={2} height={h} rx={1} fill={fill} opacity={lit || s.kind === "off" ? 1 : s.kind === "wait" ? 0.5 : 0.22} className="transition-[fill,opacity] duration-(--dur-2)" />;
      })}
      {s.kind === "off" && (
        <line x1={1.5} y1={11.5} x2={13.5} y2={0.5} stroke="var(--err)" strokeWidth={1.25} strokeLinecap="round" />
      )}
    </svg>
  );
}
