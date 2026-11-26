// Light toasts (plan "风格": frosted, popping in), one manager for the page: an action's receipt or
// error, a turn done elsewhere with a key to open it.
import { Toast } from "@base-ui/react/toast";
import { CircleAlert, Download, LoaderCircle } from "lucide-react";
import { ApiError, noAnswer } from "./api/client";
import type { Agent } from "./api/types.gen";
import { t } from "./i18n";
import { Logo } from "./ui";

export const toasts = Toast.createToastManager();

/** mark: the icon before the words: busy (a spinner, as a download goes), download (one done).
 *  file: the description is a file's name, cut in its middle (FileName). */
type Data = { agent?: Agent; jump?: string; error?: boolean; undo?: () => void; mark?: "busy" | "download"; file?: boolean };

/** A toast: title, maybe a line after it; error ones stay 8 s. undo: a key that takes back what
 *  was done, the toast staying 6 s. */
export function toast(title: string, opts: { description?: string; error?: boolean; agent?: Agent; jump?: string; timeout?: number; undo?: () => void; mark?: Data["mark"]; file?: boolean } = {}): string {
  return toasts.add({ title, description: opts.description, timeout: opts.timeout ?? (opts.error ? 8000 : opts.undo ? 6000 : 2500), data: { agent: opts.agent, jump: opts.jump, error: opts.error, undo: opts.undo, mark: opts.mark, file: opts.file } satisfies Data });
}

/** An error's words for a toast. */
/** What went wrong, for people: the network gone, the service down a moment (a gateway's 502-504,
 *  as while it restarts), a bare status said as one; else the server's own words. */
export const errText = (e: unknown) => {
  if (e instanceof TypeError) return t("err.offline");
  if (noAnswer(e)) return t("err.noAnswer");
  if (e instanceof ApiError && /^\d+$/.test(e.message)) return e.status >= 502 && e.status <= 504 ? t("err.unavailable") : t("err.status", { code: e.status });
  return e instanceof Error ? e.message : String(e);
};

export function ToastList({ onJump }: { onJump: (id: string) => void }) {
  const { toasts: list } = Toast.useToastManager();
  return list.map((toast) => {
    const d = toast.data as Data | undefined;
    return (
      <Toast.Root key={toast.id} toast={toast} className="g-pop-t glass thick float flex h-10 max-w-full items-center gap-2.5 rounded-full pr-1.5 pl-3.5 text-sm text-ink">
        {d?.agent && <Logo agent={d.agent} size={15} />}
        {d?.error && <CircleAlert size={15} className="shrink-0 text-err" />}
        {d?.mark === "busy" && <LoaderCircle size={15} className="shrink-0 animate-spin text-ink-3" />}
        {d?.mark === "download" && <Download size={15} className="shrink-0 text-ink-3" />}
        {/* The title whole beside a description (已下载, not 已...): only the description gives way. */}
        <Toast.Title className="max-w-full shrink-0 truncate font-medium tabular-nums" />
        {d?.file && typeof toast.description === "string" ? (
          <Toast.Description className="flex min-w-0 shrink text-ink-2">
            <FileName name={toast.description} />
          </Toast.Description>
        ) : (
          <Toast.Description className="min-w-0 shrink truncate text-ink-2" />
        )}
        {d?.undo ? (
          <Toast.Close onClick={d.undo} className="g-press ml-1 h-7 shrink-0 rounded-full bg-fill px-3 text-xs font-medium hover:bg-fill-2">
            {t("toast.undo")}
          </Toast.Close>
        ) : d?.jump ? (
          <Toast.Close onClick={() => onJump(d.jump!)} className="g-press ml-1 h-7 shrink-0 rounded-full bg-fill px-3 text-xs font-medium hover:bg-fill-2">
            {t("toast.open")}
          </Toast.Close>
        ) : (
          <span className="w-2" />
        )}
      </Toast.Root>
    );
  });
}

/** A file's name cut in its middle as room runs out (AI智能问诊…确认事项.docx): its last few
 *  characters and its extension kept whole. */
function FileName({ name }: { name: string }) {
  const chars = Array.from(name);
  const dot = name.lastIndexOf(".");
  const ext = dot > 0 && name.length - dot <= 10 ? Array.from(name.slice(dot)).length : 0;
  const keep = ext + 4;
  if (chars.length <= keep + 2) return <span className="truncate">{name}</span>;
  return (
    <>
      <span className="min-w-0 truncate">{chars.slice(0, -keep).join("")}</span>
      <span className="shrink-0 whitespace-pre">{chars.slice(-keep).join("")}</span>
    </>
  );
}
