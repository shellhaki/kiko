import { cp, mkdir } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const destination = join(root, "frontend/public/pdfjs");
await mkdir(destination, { recursive: true });
for (const name of ["cmaps", "standard_fonts", "wasm"])
  await cp(
    join(root, "frontend/node_modules/pdfjs-dist", name),
    join(destination, name),
    { recursive: true },
  );
