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
