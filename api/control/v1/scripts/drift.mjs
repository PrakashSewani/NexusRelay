import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const contractDirectory = fileURLToPath(new URL("..", import.meta.url));
const repositoryRoot = fileURLToPath(new URL("../../../..", import.meta.url));
const generatedPaths = [
  "internal/transport/controlplane/generated/control_plane.gen.go",
  "apps/web/src/generated/control-plane/schema.d.ts",
];
const before = new Map(
  generatedPaths.map((path) => [
    path,
    readFileSync(new URL(path, `file://${repositoryRoot}/`)),
  ]),
);

execFileSync("corepack", ["pnpm", "--ignore-workspace", "run", "generate"], {
  cwd: contractDirectory,
  stdio: "inherit",
});

const drifted = generatedPaths.filter((path) => {
  const after = readFileSync(new URL(path, `file://${repositoryRoot}/`));
  return !before.get(path).equals(after);
});

if (drifted.length > 0) {
  process.stderr.write("Generated control-plane artifacts have drifted:\n");
  process.stderr.write(`${drifted.join("\n")}\n`);
  process.exitCode = 1;
}
