import { describe, expect, it } from "vitest";

import { resolveBuildRevision } from "../next.config";

describe("build revision", () => {
  it("uses a deterministic local fallback only when the value is absent", () => {
    expect(resolveBuildRevision(undefined)).toBe("dev");
  });

  it("preserves a valid release revision", () => {
    expect(resolveBuildRevision("release-2026.07.26_a1b2c3d")).toBe(
      "release-2026.07.26_a1b2c3d",
    );
  });

  it.each(["", " release", "release ", "release/1", "release:1", "-release"])(
    "rejects an unsafe explicit revision: %j",
    (value) => {
      expect(() => resolveBuildRevision(value)).toThrow(/NEXUSRELAY_BUILD_REVISION/);
    },
  );
});
