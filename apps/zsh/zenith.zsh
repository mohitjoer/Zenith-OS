# Adapted from Ryoku's apps/zsh/ryoku.zsh (GPL-3.0).
[[ ${__ZENITH_ZSH_LOADED:-0} == 1 ]] && return
__ZENITH_ZSH_LOADED=1

typeset -U path
path=("$HOME/.local/bin" $path)
export EDITOR="${EDITOR:-nvim}" VISUAL="${VISUAL:-nvim}"

[[ -o interactive ]] || return

_zenith_cfg=${XDG_CONFIG_HOME:-$HOME/.config}
# A starship.toml of the user'"'"'s own (at $_zenith_cfg/starship.toml) wins over the packaged one.
# The packaged starship.toml lives at $_zenith_cfg/zenith/apps/starship/starship.toml
[[ -z $STARSHIP_CONFIG && ! -e $_zenith_cfg/starship.toml ]] &&
  export STARSHIP_CONFIG=$_zenith_cfg/zenith/apps/starship/starship.toml
command -v starship >/dev/null 2>&1 && eval "$(starship init zsh)"
command -v zoxide >/dev/null 2>&1 && eval "$(zoxide init zsh --cmd cd)"
command -v mise >/dev/null 2>&1 && eval "$(mise activate zsh)"
command -v fzf >/dev/null 2>&1 && eval "$(fzf --zsh)"

autoload -Uz compinit
_cache=${XDG_CACHE_HOME:-$HOME/.cache}/zsh
mkdir -p "$_cache"
compinit -d "$_cache/zcompdump"

[[ -r /usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh ]] &&
  source /usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE='fg=#8A8A8A'

# History substring search (up/down arrows search history with current buffer)
if [[ -r /usr/share/zsh-history-substring-search/zsh-history-substring-search.zsh ]]; then
  source /usr/share/zsh-history-substring-search/zsh-history-substring-search.zsh
  bindkey '^[[A' history-substring-search-up
  bindkey '^[[B' history-substring-search-down
fi

# Atuin for enhanced history (sqlite-backed, syncable)
if command -v atuin >/dev/null 2>&1; then
  eval "$(atuin init zsh --disable-up-arrow)"
fi

# Sourced last: the highlighter wraps the widgets that exist when it loads.
if [[ -r /usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh ]]; then
  source /usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
  ZSH_HIGHLIGHT_STYLES[default]='fg=#cdd6f4'
  ZSH_HIGHLIGHT_STYLES[command]='fg=#a6e3a1,bold'
  ZSH_HIGHLIGHT_STYLES[hashed-command]='fg=#a6e3a1,bold'
  ZSH_HIGHLIGHT_STYLES[builtin]='fg=#89b4fa,bold'
  ZSH_HIGHLIGHT_STYLES[function]='fg=#89b4fa,bold'
  ZSH_HIGHLIGHT_STYLES[alias]='fg=#94e2d5,bold'
  ZSH_HIGHLIGHT_STYLES[suffix-alias]='fg=#94e2d5,bold'
  ZSH_HIGHLIGHT_STYLES[global-alias]='fg=#94e2d5,bold'
  ZSH_HIGHLIGHT_STYLES[reserved-word]='fg=#cba6f7,bold'
  ZSH_HIGHLIGHT_STYLES[precommand]='fg=#cba6f7,bold'
  ZSH_HIGHLIGHT_STYLES[commandseparator]='fg=#6c7086'
  ZSH_HIGHLIGHT_STYLES[single-hyphen-option]='fg=#fab387'
  ZSH_HIGHLIGHT_STYLES[double-hyphen-option]='fg=#fab387'
  ZSH_HIGHLIGHT_STYLES[single-quoted-argument]='fg=#f9e2af'
  ZSH_HIGHLIGHT_STYLES[double-quoted-argument]='fg=#f9e2af'
  ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument]='fg=#f9e2af'
  ZSH_HIGHLIGHT_STYLES[redirection]='fg=#89b4fa'
  ZSH_HIGHLIGHT_STYLES[named-fd]='fg=#89b4fa'
  ZSH_HIGHLIGHT_STYLES[numeric-fd]='fg=#89b4fa'
  ZSH_HIGHLIGHT_STYLES[globbing]='fg=#f5c2e7'
  ZSH_HIGHLIGHT_STYLES[history-expansion]='fg=#f5c2e7'
  ZSH_HIGHLIGHT_STYLES[comment]='fg=#6c7086'
  ZSH_HIGHLIGHT_STYLES[unknown-token]='fg=#f38ba8'
fi

if command -v eza >/dev/null 2>&1; then
  alias ls='eza -lh --group-directories-first'
  alias lsa='ls -a'
  alias lt='eza --tree --level=2 --long --git'
  alias lta='lt -a'
fi

# Fastfetch banner on interactive shell startup
if [[ -o interactive ]] && command -v fastfetch >/dev/null 2>&1; then
  fastfetch
fi

# Ghostty injects its integration into the shell it starts itself; this covers
# nested shells (exec zsh, tmux, sudo -E). The script guards against a second load.
if [[ -n $GHOSTTY_RESOURCES_DIR ]]; then
  source "$GHOSTTY_RESOURCES_DIR"/shell-integration/zsh/ghostty-integration
fi

unset _zenith_cfg _cache
