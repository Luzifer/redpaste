![license](https://badges.fyi/github/license/Luzifer/redpaste)
[![download](https://badges.fyi/static/download/on GoBuilder.me)](https://gobuilder.me/github.com/Luzifer/redpaste)

# Luzifer / redpaste

`redpaste` is a small utility to improve my work with a remote server I'm working on permanentely. On the remote server I'm working with a tmux session all the time but quite often I need to copy something from my terminal and paste it to the browser.

As long as it's just a few lines its fine to just resize the pane (<kbd>ctrl</kbd>+<kbd>b</kbd>, <kbd>z</kbd>) and then mark it, copy it using the copy function of my system and paste it to the browser. But sometimes there are several hundres of lines to copy. Now the fuckup begins: Scrolling in tmux buffer, selecting a part of the whole, copy it, scroll again, select another part, repeat...

To work around this I got me `redpaste` which just takes stuff from `stdin` and stores it to a Redis server. On my local computer I can fetch the data to the clipboard and paste it at once.

## Usage

- Use `redpaste --create-config` to write a default configuration
- Set your redis connect string inside the configuration  
(Format: `tcp://auth:<password>@<your machine>:6379/0`)
- To test everything just `echo "hi" | redpaste set` and `redpaste get`, this should work as expected

### Use for tmux copying

Add this sniplet to your `.tmux.conf`:

```
bind C-y run "tmux save-buffer - | redpaste set"
```

- Enter selection mode: <kbd>ctrl</kbd>+<kbd>b</kbd>, <kbd>[</kbd>
- Move to the part your want to copy, start selecting: <kbd>v</kbd>
- Select everything you need to copy and copy the selection to the buffer: <kbd>y</kbd>
- Execute the sniplet above: <kbd>ctrl</kbd>+<kbd>b</kbd>, <kbd>ctrl</kbd>+<kbd>y</kbd>

### Use to fetch the data to your local clipbpard (OSX)

Just execute this command:

```
redpaste get | pbcopy
```
