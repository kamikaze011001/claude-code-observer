# VHS tape for claude-code-observer demo GIF
# Record with: vhs scripts/demo.tape
# Output:      docs/demo.gif
#
# DATA SOURCE: this tape records against your REAL ~/.claude-code-observer
# store. `cco` must be installed and on PATH in the recording shell.
#
# BEFORE RECORDING:
#   1. Confirm the store has a representative set of sessions, INCLUDING at
#      least one prompt that spawned subagents (needed for the waterfall).
#   2. SCRUB FOR SENSITIVE DATA — this GIF is public. Check that visible
#      project names, prompt text, and paths are safe to publish.
#   3. Tune the `j` counts marked RECORD-TIME-TUNABLE below so the cursor
#      lands on a session, then a prompt, that has a subagent waterfall.

Output docs/demo.gif

Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 80ms
Set Framerate 24

# Scene 1 — launch into the dashboard
Type "cco"
Enter
Sleep 2.5s

# Scene 2 — sessions list
Type "s"
Sleep 1.5s

# Scene 3 — move the cursor (RECORD-TIME-TUNABLE: adjust count)
Type "j"
Sleep 300ms
Type "j"
Sleep 1s

# Scene 4 — session detail (event timeline)
Enter
Sleep 2.5s

# Scene 5 — move to a user_prompt row (RECORD-TIME-TUNABLE: adjust count)
Type "j"
Sleep 300ms
Type "j"
Sleep 1s

# Scene 6 — prompt detail
Enter
Sleep 2.5s

# Scene 7 — subagent waterfall (the headline feature)
Type "w"
Sleep 3s

# Scene 8 — unwind back to the dashboard
Type "b"
Sleep 400ms
Type "b"
Sleep 400ms
Type "b"
Sleep 400ms
Type "b"
Sleep 400ms

# Scene 9 — about card
Type "?"
Sleep 2s

# Scene 10 — quit
Type "b"
Sleep 400ms
Type "q"
Sleep 500ms
