# VHS tape for claude-code-observer demo GIF
# Record with: vhs scripts/demo.tape
# Output: docs/demo.gif

Output docs/demo.gif

Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 20

Type "cco init"
Enter
Sleep 2s

Type "claude --print 'list three files in this repo'"
Enter
Sleep 6s

Type "cco"
Enter
Sleep 3s

# Drill into top session
Down
Enter
Sleep 3s

# Back out
Type "b"
Sleep 1s

# Quit
Type "q"
Sleep 500ms
