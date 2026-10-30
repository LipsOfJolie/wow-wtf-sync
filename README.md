# WoW WTF Sync

A Windows desktop launcher for synchronizing World of Warcraft Account data
with a GitHub repository. The interface uses Russian on Russian Windows
installations and English on other systems.

## Requirements

- Go 1.27.3 or later
- A CGO-enabled C compiler for Windows, such as MinGW-w64

## Build

From the project directory, run:

```powershell
go test ./...
go build -ldflags="-H=windowsgui -linkmode=internal" -o .\wow-sync-launcher.exe .
```

The Windows resource files embed the application icon in the executable.

## First launch

Enter the game's `WTF\Account` folder, the GitHub repository URL, and a
Personal Access Token with permission to read and write repository contents.
Save the settings before syncing.

The launcher stores its settings, including the token, in `config.json` next
to the executable as plain text. Keep this file private and never commit or
share it. The local clone and backups are also private user data.
