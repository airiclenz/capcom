// Package clipboard hands copied text to the host's clipboard programs: the native clipboard
// (pbcopy, wl-copy, xclip, xsel, termux-clipboard-set, WSL's clip.exe, or the Win32 clipboard on
// Windows) and tmux's paste buffer. `capcom ui` calls it when a selection in the detail pane is
// copied, beside the OSC 52 sequence it writes to the terminal itself.
//
// These writes run on every copy with no prompt, so which file a program name stands for must not
// be a question a directory of the working tree answers. Every program is resolved on PATH and
// refused when it resolves inside the working tree, or through a relative PATH entry; a refused
// program runs nothing. Every input is a field of Writer, so tests swap the PATH lookup, the
// runner and the Windows writer and no real program runs.
package clipboard
