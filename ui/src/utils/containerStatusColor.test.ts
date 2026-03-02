import { describe, expect, it } from "vitest";
import { containerStatusColor, containerStatusTextColor } from "./containerStatusColor";

describe("containerStatusColor", () => {
  it("returns green for running", () => {
    expect(containerStatusColor("running")).toBe("bg-green-500");
  });

  it("returns red for exited", () => {
    expect(containerStatusColor("exited")).toBe("bg-red-500");
  });

  it("returns red for dead", () => {
    expect(containerStatusColor("dead")).toBe("bg-red-500");
  });

  it("returns yellow for paused", () => {
    expect(containerStatusColor("paused")).toBe("bg-yellow-500");
  });

  it("returns blue for restarting", () => {
    expect(containerStatusColor("restarting")).toBe("bg-blue-500");
  });

  it("returns cyan for created", () => {
    expect(containerStatusColor("created")).toBe("bg-cyan-500");
  });

  it("returns gray for null", () => {
    expect(containerStatusColor(null)).toBe("bg-gray-500");
  });

  it("returns gray for unknown status", () => {
    expect(containerStatusColor("something")).toBe("bg-gray-500");
  });
});

describe("containerStatusTextColor", () => {
  it("returns green text for running", () => {
    expect(containerStatusTextColor("running")).toBe("text-green-400");
  });

  it("returns red text for exited", () => {
    expect(containerStatusTextColor("exited")).toBe("text-red-400");
  });

  it("returns gray text for null", () => {
    expect(containerStatusTextColor(null)).toBe("text-gray-400");
  });
});
