Output "demo.gif"
Set PlaybackSpeed 0.75
Set TypingSpeed 100ms
Set Shell "bash"

# Display the Taskfile
Type "cat Taskfile.yml" Enter
Sleep 2s
Ctrl+L # Clear screen

# Run the view command
Type "tmcp view Taskfile.yml" Enter
Sleep 2s

# Navigate in the TUI
Type "Down" # Select 'weather' task
Sleep 1s
Type "Enter" # View details of 'weather' task
Sleep 3s
Type "Escape" # Go back to the task list
Sleep 1s
Type "Up" # Select 'news' task
Sleep 1s
Type "Enter" # View details of 'news' task
Sleep 3s
Type "q" # Quit the TUI
Sleep 1s

# End of demo
Hide
Type "exit" Enter
