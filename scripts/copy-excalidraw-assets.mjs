import { cp, mkdir } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fonts = join(
  root,
  "frontend/node_modules/@excalidraw/excalidraw/dist/prod/fonts",
);
const destination = join(root, "frontend/public/excalidraw/fonts");
await mkdir(destination, { recursive: true });
await cp(fonts, destination, {
  recursive: true,
  filter: (source) => !source.includes("Xiaolai"),
});
