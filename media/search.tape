# search — / as you type, matches highlighted, n jumps.
Output out/search.gif
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
Type "/"
Sleep 400ms
Type "braille"
Sleep 1.5s
Type "n"
Sleep 600ms
Type "n"
Sleep 600ms
Hide
Type "q"
Enter
