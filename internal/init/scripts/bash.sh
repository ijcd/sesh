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
