# links — label every link on screen and follow one by its hint.
Output out/links.gif
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
Type "f"
Sleep 1.5s
Type "a"
Sleep 800ms
Type "1"
Sleep 1.5s
Hide
Type "q"
Enter
