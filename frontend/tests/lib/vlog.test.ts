// LOG-04: toda linha de log do frontend começa com data e hora (com segundos).
import { afterEach, describe, expect, it, vi } from "vitest";
import { carimbo } from "@/lib/vlog";

const FORMATO = /^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}$/;

describe("LOG-04 carimbo", () => {
  it("formata data e hora locais com segundos e zeros à esquerda", () => {
    expect(carimbo(new Date(2026, 0, 5, 7, 3, 9))).toBe("2026/01/05 07:03:09");
    expect(carimbo(new Date(2026, 10, 13, 23, 59, 58))).toBe("2026/11/13 23:59:58");
  });

  it("sem argumento usa o relógio atual", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 10, 13, 14, 5, 9));
    try {
      expect(carimbo()).toBe("2026/11/13 14:05:09");
      expect(carimbo()).toMatch(FORMATO);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("LOG-04 vlog", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
    vi.resetModules();
  });

  it("com NEXT_PUBLIC_VERBOSE=true a linha começa com o carimbo", async () => {
    vi.stubEnv("NEXT_PUBLIC_VERBOSE", "true");
    vi.resetModules();
    const { vlog } = await import("@/lib/vlog");
    const log = vi.spyOn(console, "log").mockImplementation(() => {});
    vlog("x.ts", "F", "msg", 3);
    expect(log).toHaveBeenCalledTimes(1);
    const [linha, extra] = log.mock.calls[0];
    expect(linha).toMatch(/^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2} \[x\.ts\] \[F\] msg$/);
    expect(extra).toBe(3);
  });

  it("desligado não loga", async () => {
    vi.stubEnv("NEXT_PUBLIC_VERBOSE", "false");
    vi.resetModules();
    const { vlog } = await import("@/lib/vlog");
    const log = vi.spyOn(console, "log").mockImplementation(() => {});
    vlog("x.ts", "F", "msg");
    expect(log).not.toHaveBeenCalled();
  });
});
