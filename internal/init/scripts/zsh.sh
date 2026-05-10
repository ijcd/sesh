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
