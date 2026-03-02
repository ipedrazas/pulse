import { describe, expect, it } from "vitest";
import { formatLastSeen } from "./formatLastSeen";

describe("formatLastSeen", () => {
  it("returns 'never' for null", () => {
    expect(formatLastSeen(null)).toBe("never");
  });

  it("returns seconds ago", () => {
    const now = new Date();
    now.setSeconds(now.getSeconds() - 30);
    expect(formatLastSeen(now.toISOString())).toBe("30s ago");
  });

  it("returns minutes ago", () => {
    const now = new Date();
    now.setMinutes(now.getMinutes() - 5);
    expect(formatLastSeen(now.toISOString())).toBe("5m ago");
  });

  it("returns hours ago", () => {
    const now = new Date();
    now.setHours(now.getHours() - 3);
    expect(formatLastSeen(now.toISOString())).toBe("3h ago");
  });

  it("returns days ago", () => {
    const now = new Date();
    now.setDate(now.getDate() - 2);
    expect(formatLastSeen(now.toISOString())).toBe("2d ago");
  });

  it("returns 'just now' for future timestamps", () => {
    const future = new Date();
    future.setMinutes(future.getMinutes() + 5);
    expect(formatLastSeen(future.toISOString())).toBe("just now");
  });
});
