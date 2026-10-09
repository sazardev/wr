# shortcuts — the menu (m), a row of every bound key.
Output out/shortcuts.gif
Set Shell "bash"
Set FontSize 15
Set Width 1080
Set Height 660
Set Padding 14
Set TypingSpeed 30ms

Hide
Type "export HOME=$(mktemp -d) && mkdir -p $HOME/.config/wr && cp media/demo.toml $HOME/.config/wr/config.toml"
Enter
Type "clear"
Enter
Show
Type "wr media/demo.html"
Enter
Sleep 2s
Type "m"
Sleep 2.5s
Hide
Type "q"
Enter
