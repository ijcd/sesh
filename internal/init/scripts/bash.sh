# sesh discovery hook (bash)
__sesh_check_project() {
  if [ "$PWD" = "$_SESH_LAST_PWD" ]; then return; fi
  _SESH_LAST_PWD="$PWD"
  if [ -f "$PWD/.sesh.yml" ]; then
    local guard="SESH_ANNOUNCED_$(echo "$PWD" | tr -c 'a-zA-Z0-9_' '_')"
    if [ -z "${!guard}" ]; then
      echo "sesh: project here — \`sesh local\` to launch"
      eval "export $guard=1"
    fi
  fi
}
case "$PROMPT_COMMAND" in
  *__sesh_check_project*) ;;
  *) PROMPT_COMMAND="__sesh_check_project;${PROMPT_COMMAND}" ;;
esac

# sesh: record each typed command (bash flavor via DEBUG trap).
__sesh_record_cmd_bash() {
  # Skip when we're inside PROMPT_COMMAND or shell builtins
  [[ "$BASH_COMMAND" == "$PROMPT_COMMAND" ]] && return
  [[ "$BASH_COMMAND" == __sesh_check_project* ]] && return
  [[ -n "$KITTY_LISTEN_ON" ]] || return
  kitten @ set-user-vars "sesh_cmd=$BASH_COMMAND" 2>/dev/null
}
trap '__sesh_record_cmd_bash' DEBUG
