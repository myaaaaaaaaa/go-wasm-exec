#!/usr/bin/env -S deno run -A

const goEnvRoot = new Deno.Command("go", { args: ["env", "GOROOT"] }).outputSync().stdout
const goroot = new TextDecoder().decode(goEnvRoot).trim();

const wasmExecPath = `${goroot}/lib/wasm/wasm_exec.js`;
await import(`file://${wasmExecPath}`);

const go = new globalThis.Go();
go.argv = Deno.args;
go.env = Object.assign({ TMPDIR: Deno.env.get("TMPDIR") || "/tmp" }, Deno.env.toObject());
go.exit = Deno.exit;

const wasmBytes = await Deno.readFile(Deno.args[0]);
const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
await go.run(instance);
