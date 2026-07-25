# Getting Started Guide (Windows)

First time using MeowFlixEmby? Follow this guide step by step — **no command line required**, just double-click.

> In a nutshell: click a video on the Emby / Jellyfin web UI → **Play On / Cast** → select your PC,
> and it opens right in your local player (mpv / PotPlayer / VLC / MPC-HC), with playback progress synced back to the server.

---

## Prerequisites

| You'll Need | Details |
|---|---|
| A Windows PC | The one you normally watch videos on |
| Emby or Jellyfin server URL | Something like `http://192.168.1.10:8096` — the same address you open in your browser |
| Server **username + password** | ⚠️ Must be username & password — **API Key alone won't work** (cannot be discovered as a cast target) |
| A local media player | [mpv](https://mpv.io/installation/) is recommended (fullest feature support). PotPlayer / VLC / MPC-HC also work |

---

## Step 1: Download & Extract

1. Go to the project's **Releases** page and download `meowflix_<version>_windows_amd64.zip`.
   - If your PC uses an ARM processor (e.g., Snapdragon Surface), download `windows_arm64` instead. The vast majority of users should pick **amd64**.
2. **Right-click → Extract All**, and extract to a permanent folder, e.g. `C:\meowflix`.

> ⚠️ **Do not double-click and run directly from inside the zip file.** Windows extracts files to a temporary folder,
> your configuration will be lost, and you may run into errors. Always extract first.

After extracting, you should see these files:

```
meowflix.exe            Main program
meowflix.example.yaml   Configuration template
1-首次设置.bat          ← Start here
2-启动.bat
3-开机自启-安装.bat
3-开机自启-卸载.bat
deploy\                 Advanced scripts
```

---

## Step 2: Double-Click `1-首次设置.bat` (First-Time Setup)

This script does two things:

1. Copies `meowflix.example.yaml` to `meowflix.yaml` (your actual config file).
2. Opens it in Notepad for you to edit.

### At Minimum, Change These 3 Settings

In the Notepad window, find the `server:` section near the top:

```yaml
server:
  type: emby                              # Change to jellyfin if using Jellyfin
  address: https://emby.example.com       # ← Replace with your server address
  username: alice                         # ← Replace with your username
  password: ""                            # ← Put your password inside the quotes
```

After editing, it should look something like:

```yaml
server:
  type: emby
  address: http://192.168.1.10:8096
  username: rick
  password: "my-password"
```

**Important notes:**

- The address **must start with `http://` or `https://`** and should not end with a `/`.
- Put your password between the two double-quotes `"`. If it contains special characters, the quotes are required.
- YAML is **indentation-sensitive**: do not change the leading spaces, and keep a single space after each colon.
- If your server uses a self-signed HTTPS certificate (browser shows "Not Secure"), change `skip_tls_verify: false` to `true`.

### Also Change: Player Executable Path

Scroll down to the `players:` section:

```yaml
players:
  default: mpv                            # Default player
  exe:
    mpv: C:\Green\mpv\mpv.exe             # ← Change to the actual path of mpv.exe on your PC
    potplayer: C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe
    vlc: C:\Green\vlc\vlc.exe
    mpc-hc: C:\Program Files\MPC-HC\mpc-hc64.exe
```

- **Only fill in the player you actually have installed.** Unused entries can be left as-is.
- To find a player's path: Start Menu → right-click the player → More → Open file location →
  right-click the shortcut → Properties → the "Target" field is the full path.
- If mpv is in your system PATH, you can leave the `mpv:` line empty.

**Remember to press Ctrl+S to save, then close Notepad.**

---

## Step 3: Double-Click `2-启动.bat` (Start)

A black console window will open and print logs similar to:

```
level=INFO msg="MeowFlixEmby starting" server_type=emby device_name="MeowFlix (MY-PC)"
level=INFO msg=authenticated device_id=xxxxxxxx
```

If you see **`authenticated`**, it's connected to your server — success!

> **Keep this window open.** Closing it stops the program. For background & auto-start, see Step 5.

---

## Step 4: Cast from the Web UI

1. Open your Emby / Jellyfin web UI in a browser (**log in with the same account used in the config**).
2. Play any video.
3. Find the **cast icon** near the top-right of the player (a screen-with-signal icon; called "Play On" in Emby).
4. You should see **`MeowFlix (your-pc-name)`** in the list — click it.
5. Your local player will launch and start playing.

From then on, pausing, seeking, or switching to the next episode **on the web UI** will control the local player.
Playback progress in the player is also synced back to the server (Continue Watching and watched status work as usual).

---

## Step 5 (Optional): Auto-Start on Login

Don't want to manually double-click every time? Double-click **`3-开机自启-安装.bat`**.

Once installed:

- The program will run automatically in the background on next login — **no black window**.
- No administrator privileges required.
- To remove it, double-click `3-开机自启-卸载.bat`.

> Since the console is hidden, check `meowflix.log` in the program folder if something goes wrong.
> File logging (`log.file`) is enabled by default in the config template.

---

## Troubleshooting

### `.bat` window flashes and disappears instantly

Normally the script pauses before closing. If it doesn't:

1. Make sure you've **extracted** the zip (not running from inside it).
2. Hold <kbd>Shift</kbd> and right-click in the folder → "Open PowerShell window here",
   then type `.\2-启动.bat` and press Enter — this way errors won't disappear.

### Error: `load config: read config "...meowflix.yaml": The system cannot find the file specified`

You haven't run `1-首次设置.bat` yet, or the config file name is wrong.
The file must be called **`meowflix.yaml`** and placed in the **same folder** as `meowflix.exe`.

> Tip: Windows hides file extensions by default. What looks like `meowflix.yaml` might actually be `meowflix.yaml.txt`.
> In File Explorer, go to the "View" tab and check "File name extensions" to verify.

### Error: `invalid config ...: server.address must start with http:// or https://`

You forgot the `http://` prefix. Change it to something like `http://192.168.1.10:8096`.

### Error: `invalid config ...: provide server.api_key, or both server.username and server.password`

Username or password is empty. Fill in both — the password goes inside double-quotes.

### Error: `parse config ...: yaml: ...`

The YAML syntax got broken — usually from **tampering with indentation** or **using Chinese colons/quotes**.
Easiest fix: delete `meowflix.yaml`, double-click `1-首次设置.bat` again to start from the template,
and this time only change the values after each colon.

### Error: `authenticate: ...` / 401

Wrong username/password, or the address doesn't point to the right server.
Open the configured address in a browser and confirm you can log in with those credentials.

### My PC doesn't appear in the casting list on the web UI

Check in order:

1. Does the console show `authenticated`? If not, login failed — see the item above.
2. **Is the web UI logged into the same account as the config file?** Different accounts can't see each other's cast targets.
3. Did you only put `api_key` without username and password? API Key alone cannot register as a cast target.
4. Refresh the web page (F5).

### Cast works but the player doesn't launch

The player path is incorrect. Verify the path under `players.exe` actually exists —
copy it into the File Explorer address bar and press Enter; it should launch the player.

### Pausing/seeking on the web UI doesn't control the player

Remote control capabilities vary by player:

| Player | Remote Control Support |
|---|---|
| **mpv** | Full support (pause, seek, volume, subtitle/audio track switching, auto-next) — **Recommended** |
| VLC | Mostly supported |
| MPC-HC | Mostly supported; must first enable the Web UI in MPC's Options |
| PotPlayer | **Start/stop only** — no remote control from web UI |

Use mpv for the best experience.

### Video stutters / goes through server relay

MeowFlixEmby picks the playback path in this order: **locally mounted drive → cloud drive direct link → server HTTP stream**.
If your media is on a NAS and you've mapped it as a network drive locally, configure `playback.path_maps`
to map the server path to your local drive — the player will read directly from disk, bypassing network relay:

```yaml
playback:
  path_maps:
    - src: /mnt/disk1          # Path prefix on the server
      dst: E:\                 # Corresponding drive letter on your PC
```

If using openlist / AList for cloud drives, see the comments in the `openlist:` section of `meowflix.yaml`.

---

## Want to Go Further?

- Full config reference: read `meowflix.example.yaml` — every option has inline comments.
- Windows Service (runs without login, requires admin), Linux / macOS auto-start, or embedding as a shared library:
  see the [README](../README.md).
- CLI options: `meowflix.exe -version`, `meowflix.exe -config custom-path.yaml`.
