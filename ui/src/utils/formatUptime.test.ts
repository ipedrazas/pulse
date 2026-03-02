import { describe, expect, it } from "vitest";
import { formatUptime } from "./formatUptime";

describe("formatUptime", () => {
  it("returns dash for null", () => {
    expect(formatUptime(null)).toBe("—");
  });

  it("returns dash for negative", () => {
    expect(formatUptime(-1)).toBe("—");
  });

  it("formats seconds", () => {
    expect(formatUptime(45)).toBe("45s");
  });

  it("formats minutes", () => {
    expect(formatUptime(300)).toBe("5m");
  });

  it("formats hours and minutes", () => {
    expect(formatUptime(7500)).toBe("2h 5m");
  });

  it("formats days and hours", () => {
    expect(formatUptime(270000)).toBe("3d 3h");
  });

  it("formats zero", () => {
    expect(formatUptime(0)).toBe("0s");
  });
});
