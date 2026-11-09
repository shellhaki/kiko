import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const env = {
  ...process.env,
  GOFLAGS: [process.env.GOFLAGS, "-mod=readonly"].filter(Boolean).join(" "),
};

function run(command, args, capture = false) {
  const result = spawnSync(command, args, {
    cwd: root,
    env,
    encoding: "utf8",
    stdio: capture ? "pipe" : "inherit",
    shell: process.platform === "win32" && command === "npm",
  });
  if (result.error || result.status !== 0) {
    console.error(`Failed: ${command} ${args.join(" ")}`);
    if (result.error) console.error(result.error.message);
    if (capture && result.stderr) console.error(result.stderr);
    process.exit(result.status || 1);
  }
  return result.stdout?.trim() || "";
}

run("npm", ["--prefix", "frontend", "run", "check"]);

const directories = run("go", ["list", "-f", "{{.Dir}}", "./..."], true).split(
  /\r?\n/,
);
const files = directories.flatMap((directory) =>
  readdirSync(directory)
    .filter((name) => name.endsWith(".go"))
    .map((name) => join(directory, name)),
);
const unformatted = run("gofmt", ["-l", ...files], true);
if (unformatted) {
  console.error(
    `Go formatting differs. Run gofmt -w on these files:\n${unformatted}`,
  );
  process.exit(1);
}

const pdfium = JSON.parse(
  readFileSync(join(root, "build/pdfium-manifest.json"), "utf8"),
);
const module = JSON.parse(
  run("go", ["list", "-m", "-json", pdfium.goModule], true),
);
const wasm = readFileSync(join(module.Dir, "internal/pdfium_wasm/pdfium.wasm"));
if (
  module.Version !== pdfium.version ||
  createHash("sha256").update(wasm).digest("hex") !== pdfium.wasmSHA256
)
  throw new Error(
    "Embedded PDFium differs from its pinned notice/provenance manifest.",
  );
readFileSync(join(root, "build/pdfium-notices.md"));

run("go", ["vet", "./..."]);
run("go", ["test", "-count=1", "-timeout=2m", "./core/attachment"]);
run("go", ["test", "-race", "-count=1", "-timeout=5m", "./..."]);
console.log("All local quality checks passed.");
