// Making a channel (kumo, 2026-11-25): no popover to line agents up in. The "+" of a group's or a
// folder's row makes a channel there with no agent yet and opens it, the + beside its members'
// key open on it (chat/channel/top.tsx): its agents are added there. The row's menu, or the "+"
// at the right button: 新建子文件夹, the file browser kept in the row's folder. A folder no
// listed group holds: the file browser too; a channel is made in the folder it shows by its
// 在此新建 (filebrowser.tsx).
import { cloneElement, useEffect, useLayoutEffect, useRef, useState, type MouseEvent, type ReactElement, type ReactNode } from "react";
import type { Agent, Session, Workspace } from "./api/types.gen";
import { making } from "./pending";
import { api } from "./api/client";
import { askAdd, keepList } from "./chat/channel/adding";
import { useMachines } from "./data";
import { t } from "./i18n";
import { agentsAt, pageAt, type Machine } from "./machines";
import type { BrowseAt } from "./places";
import { errText, toast } from "./toast";
import { cn } from "./ui";

/** One folder's listing; prefetched before it is shown (the popover opening, a row hovered), so the
 *  picker opens and steps at its full height instead of a spinner springing open into the list. */
export const foldersQ = (path: string, machine?: string) => ({ queryKey: ["folders", machine ?? "", path], queryFn: () => api.folders(path, machine), retry: false, staleTime: 10_000 });

/** Its children's height, glided to as it changes (a folder of more or fewer folders): the popover
 *  grows and shrinks instead of jumping (the user, 2026-11-06). */
export function Tall({ children, className }: { children: ReactNode; className?: string }) {
  const inner = useRef<HTMLDivElement>(null);
  const [h, setH] = useState<number | undefined>(undefined);
  useLayoutEffect(() => {
    const el = inner.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setH(el.offsetHeight));
    ro.observe(el);
    setH(el.offsetHeight);
    return () => ro.disconnect();
  }, []);
  return (
    <div className={cn("overflow-hidden transition-[height] duration-(--dur-3) ease-(--ease-out) motion-reduce:transition-none", className)} style={{ height: h }}>
      <div ref={inner}>{children}</div>
    </div>
  );
}

/** A channel made in cwd (at machine: a path there; none: here) with no agent yet: its id, the +
 *  beside its members' key asked to open on it; null (and said) when it was not made. */
export async function newChannelAt(cwd: string, machine?: string, workspace?: number): Promise<string | null> {
  try {
    // Every new chat is a channel: no agent of its own, its agents members of it.
    const { id } = await api.create({ agent: "" as Agent, cwd, ...(workspace ? { workspace } : {}), ...(machine ? { machine } : {}) });
    if (workspace) keepList(id);
    askAdd(id);
    return id;
  } catch (e) {
    toast(t("err.newSession"), { error: true, description: errText(e) });
    return null;
  }
}

/** Where a folder as the list writes it ("<machine>:<path>" at another) is: the path there and
 *  its machine; null (and said) when its machine is gone or not connected. */
function whereOf(path: string, ms: Machine[]): { cwd: string; machine?: string } | null {
  const r = agentsAt(path, ms);
  if (r === "gone") return toast(t("new.gone"), { error: true }), null;
  if (r === "here") return { cwd: path };
  if (!r.m.online) return toast(t("new.offline", { name: r.m.name }), { error: true }), null;
  return { cwd: r.cwd, machine: r.m.id };
}

/** A channel made in a folder of the list (newChannelAt), in its group. */
export async function newChannel(path: string, ms: Machine[], workspace?: number): Promise<string | null> {
  const at = whereOf(path, ms);
  return at && newChannelAt(at.cwd, at.machine, workspace);
}

/** The "+" of a group's or a folder's row. Pressed: a channel there, opened (opened). At the right
 *  button, or open set by the row's menu: 新建子文件夹, the file browser opened in the row's
 *  folder and kept in it (sub; kumo, 2026-11-25): a folder there gone into or made, 在此新建. */
export function NewHere({ w, trigger, open, setOpen, opened, sub }: { w: Workspace; trigger: ReactElement; open: boolean; setOpen: (o: boolean) => void; opened: (id: string) => void; sub?: (at: BrowseAt) => void }) {
  const machines = useMachines();
  const [busy, setBusy] = useState(false);
  const here = async () => {
    if (busy) return;
    const at = whereOf(w.path, machines);
    if (!at) return;
    setBusy(true);
    try {
      // Its row at the top of the folder at once (pending.ts making; kumo, 2026-11-26: the "+"
      // pressed, nothing till the server had made it, a second or more), opened once made.
      const now = Date.now();
      const row = { agent: "" as Agent, cwd: at.cwd, machine: at.machine, workspace: w.id || undefined, bare: true, status: "idle", hosted: false, live: false, updatedAt: now, createdAt: now } as Omit<Session, "id">;
      const id = await making(row, () => newChannelAt(at.cwd, at.machine, w.id));
      if (id) opened(id);
    } finally {
      setBusy(false);
    }
  };
  const browse = () => {
    const there = pageAt(w.path, machines);
    if (!whereOf(w.path, machines)) return;
    const path = there ? there.path : w.path;
    sub?.({ machine: there?.m.id, path, within: path, workspace: w.id });
  };
  // The row's menu asks by open: answered at once, nothing of this key's stays open.
  useEffect(() => {
    if (!open) return;
    setOpen(false);
    browse();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
  return cloneElement(trigger as ReactElement<{ onClick?: (e: MouseEvent) => void; onContextMenu?: (e: MouseEvent) => void; "aria-busy"?: boolean }>, {
    onClick: (e: MouseEvent) => (e.stopPropagation(), void here()),
    "aria-busy": busy || undefined,
    onContextMenu: (e: MouseEvent) => (e.preventDefault(), e.stopPropagation(), browse()),
  });
}
