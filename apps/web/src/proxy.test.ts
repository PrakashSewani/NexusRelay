import { describe, expect, it } from "vitest";

import { createContentSecurityPolicy } from "./proxy";

describe("content security policy", () => {
  it("uses nonce-only scripts and styles in production", () => {
    const policy = createContentSecurityPolicy("test-nonce", false);

    expect(policy).toContain("script-src 'self' 'nonce-test-nonce' 'strict-dynamic'");
    expect(policy).toContain("style-src 'self' 'nonce-test-nonce'");
    expect(policy).not.toContain("'unsafe-inline'");
    expect(policy).not.toContain("'unsafe-eval'");
  });

  it("limits development relaxations to framework debugging needs", () => {
    const policy = createContentSecurityPolicy("test-nonce", true);

    expect(policy).toContain("script-src 'self' 'nonce-test-nonce' 'strict-dynamic' 'unsafe-eval'");
    expect(policy).toContain("style-src 'self' 'unsafe-inline'");
  });
});
