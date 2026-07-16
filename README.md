# xAI 403 Fixer

CLIProxyAPI native plugin for the xAI authentication change announced on July 16. It scans xAI OAuth auth files and repairs these fields without touching tokens or unrelated metadata:

```json
{
  "base_url": "https://api.x.ai/v1",
  "using_api": true
}
```

Before each write, the complete original JSON is copied to:

```text
data/xai-403-fixer/backups/<timestamp>/
```

Backups contain credentials and must be protected like the original auth directory.

## Publish once

1. Push a tag matching the plugin version, for example `v0.1.0`.
2. GitHub Actions builds and publishes the exact archive names and `checksums.txt` required by the CPA plugin store.

## Import into cloud CPA

Add this repository's raw registry URL as a plugin store source:

```text
https://raw.githubusercontent.com/Sakuralaaa/x-ai-change/main/registry.json
```

The plugin will then appear in the CPA web management plugin store and can be installed from the latest GitHub Release.

## Manual install

Download the shared library for your platform from GitHub Actions or Releases, place it in the CLIProxyAPI plugin directory, and enable it:

```yaml
plugins:
  enabled: true
  configs:
    xai-403-fixer:
      enabled: true
      priority: 1
```

Restart CLIProxyAPI and open **xAI 403 修复** in the management page. Enter the CPA Management Key, scan, review the detected files, and repair selected files.

## Build

The plugin uses Go c-shared mode and requires CGO. Local helper scripts are included, but the repository workflow is the recommended build path.

```powershell
./build.ps1
```

```bash
./build.sh
```

## Safety

- Only credentials whose JSON `type` is exactly `xai` are modified.
- Existing fields, tokens, custom headers, and disabled state are preserved.
- A repair is offered only when the base URL differs, `using_api` is missing, or it is not `true`.
- The UI and status API never return token values.
