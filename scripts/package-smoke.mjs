import { randomBytes } from "node:crypto";
import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { createServer } from "node:net";
import assert from "node:assert/strict";
import { once } from "node:events";

const binary = process.argv[2];
if (!binary)
  throw new Error(
    "Usage: node scripts/package-smoke.mjs <packaged-app-binary>",
  );
const directory = await mkdtemp(join(tmpdir(), "kiko-package-smoke-"));
const listener = createServer().listen(0, "127.0.0.1");
await once(listener, "listening");
const port = listener.address().port;
await new Promise((resolve) => listener.close(resolve));
const origin = `http://127.0.0.1:${port}`;
const token = randomBytes(32).toString("base64url");
const processHandle = spawn(resolve(binary), ["--web"], {
  env: {
    ...process.env,
    KIKO_WEB_TOKEN: token,
    KIKO_WEB_MASTER_KEY: randomBytes(32).toString("base64"),
    KIKO_WEB_ADDRESS: `127.0.0.1:${port}`,
    KIKO_WEB_DATA_DIR: directory,
  },
  stdio: ["ignore", "ignore", "ignore"],
});
let cookie = "";
async function request(path, body) {
  return fetch(origin + path, {
    method: body === undefined ? "GET" : "POST",
    headers: {
      Origin: origin,
      "Content-Type": "application/json",
      Cookie: cookie,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(20000),
  });
}
async function rpc(method, ...args) {
  const response = await request("/api/rpc", { method, args });
  assert.equal(response.status, 200);
  const result = await response.json();
  if (result.error) throw new Error(result.error);
  return result.result;
}
async function download(kind, id, format) {
  const response = await request(
    "/api/export?" + new URLSearchParams({ kind, id, format }),
  );
  if (!response.ok) throw new Error((await response.text()).slice(0, 200));
  const bytes = Buffer.from(await response.arrayBuffer());
  assert.equal(
    bytes.subarray(0, format === "pdf" ? 5 : 2).toString(),
    format === "pdf" ? "%PDF-" : "PK",
  );
  return bytes;
}
try {
  let ready = false;
  for (let attempt = 0; attempt < 100; attempt++) {
    if (processHandle.exitCode !== null)
      throw new Error("Packaged server exited during startup.");
    try {
      const response = await request("/api/session");
      assert.equal(response.status, 200);
      ready = true;
      break;
    } catch {
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }
  assert.ok(ready, "Packaged server did not start");
  assert.ok((await (await request("/")).text()).includes("/web-runtime.js"));
  assert.equal(
    (await request("/api/rpc", { method: "Bootstrap", args: [] })).status,
    401,
  );
  const login = await request("/api/login", { token });
  assert.equal(login.status, 204);
  cookie = login.headers.get("set-cookie").split(";")[0];
  assert.equal((await rpc("Bootstrap")).preferences.stage, "nickname");
  await rpc("SaveNickname", "Package smoke test");
  const chat = (await rpc("CreateChat", "")).selection.chatId;
  const sheet = (await rpc("CreateSheet", chat, "Budget")).artifactId;
  const saved = await rpc("SaveArtifact", sheet, 1, {
    tabs: [
      {
        name: "Budget",
        cells: { A1: { value: 12 }, A2: { formula: "=A1*2" } },
      },
    ],
  });
  assert.equal(saved.values.Budget.A2.value, "24");
  await download("artifact", sheet, "xlsx");
  const doc = (
    await rpc(
      "CreateDocument",
      chat,
      "Offline plan",
      "# Offline plan\n\nLocal export check.",
    )
  ).documentId;
  await download("document", doc, "docx");
  const pdf = await download("document", doc, "pdf");
  const attachment = await rpc(
    "AttachFile",
    chat,
    "offline-plan.pdf",
    pdf.toString("base64"),
  );
  assert.ok(
    (await rpc("ReadAttachment", chat, attachment.id, 1))[0].text.includes(
      "Offline",
    ),
  );
  const pageImage = await rpc("RenderAttachmentPage", chat, attachment.id, 1);
  assert.ok(
    pageImage.startsWith("data:image/png;base64,iVBOR"),
    "PDF worker did not return an actual PNG page",
  );
  const deck = (await rpc("CreateDeck", chat, "Offline slides")).artifactId;
  const slides = await request(
    "/api/export?" +
      new URLSearchParams({ kind: "artifact", id: deck, format: "pptx" }),
  );
  assert.equal(slides.status, 400);
  assert.match(await slides.text(), /editor preview/);
  await download("artifact", deck, "pdf");
  assert.equal((await request("/api/logout", {})).status, 204);
  assert.equal(
    (await request("/api/rpc", { method: "Bootstrap", args: [] })).status,
    401,
  );
  console.log(
    "Packaged smoke passed: session/RPC, sheets/XLSX, documents/DOCX/PDF, isolated PDF import/render, slides/PDF + PPTX editor contract, logout. No paid requests.",
  );
} finally {
  const exited = once(processHandle, "exit");
  processHandle.kill("SIGTERM");
  const timer = setTimeout(() => processHandle.kill("SIGKILL"), 8000);
  try {
    if (processHandle.exitCode === null) await exited;
  } finally {
    clearTimeout(timer);
    await rm(directory, { recursive: true, force: true });
  }
}
