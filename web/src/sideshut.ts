// The sidebar shut on a computer (kumo, 2026-11-26): its [Ir] pressed, it folds away to the left
// and the main pane's top bar takes the mark, the name and the link, as a phone's does; [Ir] there
// brings it back. Kept on this device.
import { keep, kept } from "./ui";

export const SHUT_KEY = "ir.sidebarShut";
export const shutKept = (storage?: () => Storage) => kept(SHUT_KEY, storage) === "1";
export const keepShut = (shut: boolean, storage?: () => Storage) => keep(SHUT_KEY, shut ? "1" : null, storage);
