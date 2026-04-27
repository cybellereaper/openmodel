// VS Code extension activation entry point.
//
// This extension wires the PureLang `pr lsp` server to VS Code via the
// vscode-languageclient library, gives PureLang files syntax highlighting
// (via the bundled TextMate grammar), and exposes a `purelang.serverPath`
// setting so users can point at their own `pr` binary.

const vscode = require("vscode");

let client = null;

async function activate(context) {
  let LanguageClient;
  let TransportKind;
  try {
    const lsp = require("vscode-languageclient/node");
    LanguageClient = lsp.LanguageClient;
    TransportKind = lsp.TransportKind;
  } catch (e) {
    vscode.window.showWarningMessage(
      "PureLang: install 'vscode-languageclient' to enable the language server."
    );
    return;
  }

  const cfg = vscode.workspace.getConfiguration("purelang");
  const serverPath = cfg.get("serverPath", "pr");

  const serverOptions = {
    run: { command: serverPath, args: ["lsp"], transport: TransportKind.stdio },
    debug: { command: serverPath, args: ["lsp"], transport: TransportKind.stdio },
  };

  const clientOptions = {
    documentSelector: [{ scheme: "file", language: "purelang" }],
  };

  client = new LanguageClient(
    "purelang",
    "PureLang Language Server",
    serverOptions,
    clientOptions
  );
  await client.start();
  context.subscriptions.push({ dispose: () => client && client.stop() });
}

function deactivate() {
  if (!client) return undefined;
  return client.stop();
}

module.exports = { activate, deactivate };
