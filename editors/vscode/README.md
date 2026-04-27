# PureLang for Visual Studio Code

This extension adds:

- Syntax highlighting for `.pure` files (via `syntaxes/pure.tmLanguage.json`).
- Smart bracket and quote handling and basic indentation rules
  (`language-configuration.json`).
- Integration with the `pr lsp` language server bundled with the PureLang
  compiler. The server provides diagnostics, formatting, and hover
  documentation for built-in standard library functions.

## Install

This extension is not yet published to the marketplace. To use it locally:

1. Build the PureLang compiler so that `pr` is available in your `PATH`:
   ```sh
   go build -o pr ./cmd/pr
   ```
2. Symlink or copy `editors/vscode` into your `~/.vscode/extensions/` directory:
   ```sh
   ln -s "$(pwd)/editors/vscode" ~/.vscode/extensions/purelang
   ```
3. Restart VS Code. Open any `.pure` file to activate the extension.

By default the extension uses the `pr` binary on your `PATH`. Override this
with the `purelang.serverPath` setting if you keep `pr` elsewhere.
