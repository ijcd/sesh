# sesh discovery hook (fish)
function __sesh_chpwd --on-variable PWD
  if test -f "$PWD/.sesh.yml"
    set -l guard "SESH_ANNOUNCED_"(echo "$PWD" | tr -c 'a-zA-Z0-9_' '_')
    if not set -q $guard
      echo "sesh: project here — \`sesh local\` to launch"
      set -gx $guard 1
    end
  end
end
__sesh_chpwd

# sesh: record each typed command (fish flavor).
function __sesh_record_cmd --on-event fish_preexec
  test -n "$KITTY_LISTEN_ON"; or return
  kitten @ set-user-vars "sesh_cmd=$argv" 2>/dev/null
end
