import { createHash } from "node:crypto";
import {
  copyFile,
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  rm,
  writeFile,
  chmod,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const manifest = JSON.parse(
  await readFile(join(root, "build/typst-manifest.json"), "utf8"),
);
const platform = {
  darwin: "apple-darwin",
  linux: "unknown-linux-musl",
  win32: "pc-windows-msvc",
}[process.platform];
const architecture = { arm64: "aarch64", x64: "x86_64" }[process.arch];
if (!platform || !architecture)
  throw new Error("No pinned Typst runtime for this platform.");
const name = `typst-${architecture}-${platform}.${process.platform === "win32" ? "zip" : "tar.xz"}`;
const asset = manifest.assets[name];
if (!asset?.sha256 || asset.sha256.length !== 64)
  throw new Error("Missing pinned runtime checksum.");
const temporary = await mkdtemp(join(tmpdir(), "kiko-typst-"));
try {
  const response = await fetch(asset.url, {
    signal: AbortSignal.timeout(60000),
  });
  if (!response.ok)
    throw new Error("Could not download the pinned Typst runtime.");
  const bytes = Buffer.from(await response.arrayBuffer());
  if (createHash("sha256").update(bytes).digest("hex") !== asset.sha256)
    throw new Error("Typst runtime checksum mismatch.");
  const archive = join(temporary, name);
  await writeFile(archive, bytes);
  const result = spawnSync("tar", ["-xf", archive, "-C", temporary], {
    stdio: "inherit",
  });
  if (result.status !== 0) throw new Error("Could not extract Typst.");
  const folder = join(temporary, `typst-${architecture}-${platform}`);
  const destination =
    process.platform === "darwin"
      ? join(root, "build/bin/Kiko.app/Contents/Resources")
      : join(root, "build/bin");
  await mkdir(destination, { recursive: true });
  const binary = process.platform === "win32" ? "typst.exe" : "typst";
  await copyFile(join(folder, binary), join(destination, binary));
  if (process.platform !== "win32")
    await chmod(join(destination, binary), 0o755);
  const notices = join(destination, "typst-notices");
  await mkdir(notices, { recursive: true });
  for (const file of await readdir(folder)) {
    if (/license|notice/i.test(file))
      await copyFile(join(folder, file), join(notices, file));
  }
  await copyFile(
    join(root, "build/typst-manifest.json"),
    join(notices, "runtime-manifest.json"),
  );
  for (const file of ["LICENSE", "THIRD_PARTY_NOTICES.md"])
    await copyFile(join(root, file), join(destination, file));
  for (const file of ["pdfium-manifest.json", "pdfium-notices.md"])
    await copyFile(join(root, "build", file), join(destination, file));
  if (process.platform === "darwin") {
    const signed = spawnSync(
      "codesign",
      ["--force", "--deep", "--sign", "-", join(root, "build/bin/Kiko.app")],
      { stdio: "inherit" },
    );
    if (signed.status !== 0)
      throw new Error(
        "Could not re-sign the application after bundling its runtime.",
      );
  }
  console.log(
    `Bundled Typst ${manifest.version}: ${join(destination, binary)}`,
  );
} finally {
  await rm(temporary, { recursive: true, force: true });
}
