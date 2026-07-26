import { render, screen } from "@testing-library/react";
import axe from "axe-core";
import { describe, expect, it } from "vitest";

import { contrastRatio } from "../test/contrast";

import Home from "./page";

describe("admin shell", () => {
  it("identifies itself as an unimplemented repository scaffold", () => {
    render(<Home />);

    expect(
      screen.getByRole("heading", { name: /admin interface is not implemented yet/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/repository scaffold/i);
    expect(screen.getByText(/does not observe a running control plane/i)).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: /administration areas/i })).toBeInTheDocument();
    expect(screen.getAllByText("Unavailable")).toHaveLength(12);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("provides one set of page landmarks and a skip link to main content", () => {
    render(<Home />);

    expect(screen.getByRole("banner")).toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /skip to content/i })).toHaveAttribute(
      "href",
      "#main-content",
    );
  });

  it("keeps dim text comfortably above WCAG AA contrast", () => {
    expect(contrastRatio("#8f9f98", "#09100e")).toBeGreaterThanOrEqual(5.5);
  });

  it("has no automated accessibility violations", async () => {
    const { container } = render(<Home />);
    const results = await axe.run(container, {
      rules: {
        // jsdom does not implement canvas pixel reads; deterministic token contrast is asserted above.
        "color-contrast": { enabled: false },
      },
    });

    expect(results.violations).toEqual([]);
  });
});
