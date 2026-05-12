# sesh discovery hook (zsh)
__sesh_chpwd() {
  if [[ -f "$PWD/.sesh.yml" ]]; then
    local guard="SESH_ANNOUNCED_$(echo "$PWD" | tr -c 'a-zA-Z0-9_' '_')"
    if [[ -z "${(P)guard}" ]]; then
      echo "sesh: project here — \`sesh local\` to launch"
      eval "export $guard=1"
    fi
  fi
}
typeset -ga chpwd_functions
chpwd_functions+=(__sesh_chpwd)
__sesh_chpwd

# sesh: record each typed command to a kitty per-window user variable.
# Lets `sesh capture` recover the exact command you typed, even when it's
# a shell function, alias, or `exec` wrapper that the kernel process tree
# can't surface.
__sesh_record_cmd() {
  [[ -n "$KITTY_LISTEN_ON" ]] || return
  kitten @ set-user-vars "sesh_cmd=$1" 2>/dev/null
}
typeset -ga preexec_functions
preexec_functions+=(__sesh_record_cmd)
