# ui — reading an article (media/demo.html, pinned config).
Output out/ui.gif
Set Shell "bash"
Set FontSize 15
Set Width 1080
Set Height 660
Set Padding 14
Set TypingSpeed 40ms

Hide
Type "export HOME=$(mktemp -d) && mkdir -p $HOME/.config/wr && cp media/demo.toml $HOME/.config/wr/config.toml"
Enter
Type "clear"
Enter
Show
Sleep 300ms
Type "wr media/demo.html"
Enter
Sleep 2s
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "t"
Sleep 1s
Type "\x1b"
Sleep 500ms
Hide
Type "q"
Enter
