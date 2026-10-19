import type { MemoryStore } from "memory/MemoryStore";
export const open = (revision: string): MemoryStore => ({ revision });
