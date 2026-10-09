# sections — the heading index.
Output out/sections.gif
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
Type "t"
Sleep 2s
Type "]"
Sleep 800ms
Hide
Type "q"
Enter
