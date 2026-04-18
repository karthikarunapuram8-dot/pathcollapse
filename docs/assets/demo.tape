# PathCollapse demo — renders docs/assets/demo.gif
#
# Regenerate with:   vhs docs/assets/demo.tape
#
# Requirements: vhs (charmbracelet), ttyd, ffmpeg — `scoop install vhs` on
# Windows brings all three. On macOS: `brew install vhs`.
#
# Uses cmd as the shell on Windows because:
#   - vhs's ttyd cannot reliably spawn MSYS2/Git-Bash (the System32 `bash`
#     shim routes through WSL and produces path-translation spam).
#   - PowerShell's PSReadLine echoes typed characters locally during Hide
#     blocks, leaking setup commands into the visible portion of the GIF.
#   - cmd has no readline, no banner, no local echo — setup stays hidden
#     and the visible output is pure pathcollapse stdout.
#
# The demo walks through ingest → analyze → breakpoints (with calibrated
# confidence and --quiet to suppress the cold-start note) → HTML report.

Output docs/assets/demo.gif

Require pathcollapse

Set Shell       "cmd"
Set Theme       "Catppuccin Mocha"
Set FontFamily  "JetBrains Mono"
Set FontSize    20
Set Width       1280
Set Height      720
Set Padding     24
Set TypingSpeed 35ms

# ── Hidden setup ─────────────────────────────────────────────────────────
# Minimal prompt (just "$ "), clean working dir, fixture in place.
Hide
Type "prompt $$ "
Enter
Type "mkdir C:\tmp\pc-demo 2>nul"
Enter
Type "cd /d C:\tmp\pc-demo"
Enter
Type "del /q * 2>nul"
Enter
Type "copy C:\Users\RDPUser\OneDrive\PathCollapse\internal\testdata\enterprise_ad.json ad.json >nul"
Enter
Sleep 200ms
Type "cls"
Enter
Show

# ── 1. Ingest ─────────────────────────────────────────────────────────────
Type "pathcollapse ingest --input ad.json --type json --output snap.json"
Sleep 500ms
Enter
Sleep 2s

# ── 2. Analyze — find the top attack paths ────────────────────────────────
Type `pathcollapse analyze --graph snap.json --query "FIND PATHS FROM user:alice TO privilege:tier0 LIMIT 3"`
Sleep 500ms
Enter
Sleep 3500ms

# ── 3. Breakpoints with calibrated confidence (quiet → no cold-start note) ─
Type "pathcollapse breakpoints --graph snap.json --top 3 --confidence on --quiet"
Sleep 500ms
Enter
Sleep 4s

# ── 4. Render the HTML report ─────────────────────────────────────────────
Type "pathcollapse report --graph snap.json --format html --output report.html"
Sleep 500ms
Enter
Sleep 1500ms

# ── 5. A quick glimpse of the artifact ────────────────────────────────────
Type "dir report.html"
Sleep 300ms
Enter
Sleep 2500ms
