# Desktop release checklist

## Build (Windows)
1. Bump `productVersion` in `wails.json` and `server/releases/latest.json`
2. Run `build-windows.ps1`
3. Outputs in `build/bin/`:
   - `mscale-desktop-vX.Y.Z.exe` — portable
   - `MscaleSetup-vX.Y.Z.exe` — installer (requires [NSIS](https://nsis.sourceforge.io/Download) / `makensis` on PATH)

## Publish update for customers
1. Upload to server `/var/www/mscale/downloads/`:
   - `mscale-desktop-vX.Y.Z.exe`
   - `MscaleSetup-vX.Y.Z.exe` (when NSIS build succeeds)
   - `wintun.dll` (optional; app embeds it)
2. Update `server/releases/latest.json` (version, URLs, notes)
3. Copy `latest.json` to hub: `/home/ubuntu/mscale-server/releases/latest.json`
4. Restart `mscale-server` (PM2) so `/api/app/latest` serves the new manifest

Customers on an older app version see **Update available** on next launch.
