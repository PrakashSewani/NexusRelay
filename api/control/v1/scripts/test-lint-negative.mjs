import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const contractDirectory = fileURLToPath(new URL("..", import.meta.url));
const redocly = fileURLToPath(
  new URL("../node_modules/@redocly/cli/bin/cli.js", import.meta.url),
);
const result = spawnSync(
  process.execPath,
  [
    redocly,
    "lint",
    "fixtures/missing-operation-id.yaml",
    "--config",
    "redocly.yaml",
    "--lint-config",
    "error",
    "--format",
    "json",
    "--max-problems",
    "100",
  ],
  { cwd: contractDirectory, encoding: "utf8" },
);

if (result.error) {
  throw result.error;
}

if (result.status === 0) {
  throw new Error("Expected the missing operationId fixture to fail linting");
}

let report;
try {
  report = JSON.parse(result.stdout);
} catch {
  process.stderr.write(`${result.stdout}\n${result.stderr}`);
  throw new Error("Redocly did not return the expected JSON lint report");
}

const fatalRules = report.problems
  .filter((problem) => problem.severity === "error")
  .map((problem) => problem.ruleId);
if (
  fatalRules.length !== 1 ||
  fatalRules[0] !== "operation-operationId"
) {
  process.stderr.write(`${result.stdout}\n${result.stderr}`);
  throw new Error("Lint failed without the operation-operationId rule");
}

process.stdout.write(
  "Verified: an operation without operationId fails operation-operationId.\n",
);
