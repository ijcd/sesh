-- Lua port of internal/plugins/emacs/{emacs,elisp}.go.
-- Mechanism-not-policy: shells out to `emacsclient -e <form>`; the form
-- is a call to a user-defined hook (sesh-open-project / sesh-close-project).
-- Registers as "emacs-lua" during the spike so the Go "emacs" plugin can
-- still be loaded alongside for comparison.

local function quote(s)
  -- Wrap s in a double-quoted elisp literal, escaping backslash and quote.
  -- Order matters: backslash before quote.
  s = s:gsub("\\", "\\\\")
  s = s:gsub('"', '\\"')
  return '"' .. s .. '"'
end

local function isabs(p)
  return p:sub(1, 1) == "/"
end

local function join_path(cwd, rel)
  if cwd:sub(-1) == "/" then return cwd .. rel end
  return cwd .. "/" .. rel
end

local function build_open_form(hook, name, cwd, files)
  if not files or #files == 0 then
    return "(" .. hook .. " " .. quote(name) .. " " .. quote(cwd) .. ")"
  end
  local parts = {}
  for i, f in ipairs(files) do
    if not isabs(f) then f = join_path(cwd, f) end
    parts[i] = quote(f)
  end
  return "(" .. hook .. " " .. quote(name) .. " " .. quote(cwd)
    .. " '(" .. table.concat(parts, " ") .. "))"
end

local function build_close_form(hook, name)
  return "(" .. hook .. " " .. quote(name) .. ")"
end

local function defaults(cfg)
  cfg = cfg or {}
  return {
    hook = cfg.hook or "sesh-open-project",
    close_hook = cfg.close_hook or "sesh-close-project",
    daemon = cfg.daemon or "sesh",
    files = cfg.files or {},
  }
end

local ELISP_SYMBOL = "^[^%s%(%)'\"`,]+$"

local function ec_run(socket, form)
  return sesh.exec("emacsclient", { socket, "-e", form })
end

sesh.register("emacs-lua", {
  up = function(env, cfg)
    local c = defaults(cfg)
    local form = build_open_form(c.hook, env.name, env.cwd, c.files)
    local socket = "--socket-name=" .. c.daemon

    -- Probe daemon: cheap (emacs-version) call.
    local probe = ec_run(socket, "(emacs-version)")
    if probe.code ~= 0 then
      -- No daemon — spawn and wait for readiness.
      local spawn = sesh.exec("emacs", { "--daemon=" .. c.daemon })
      if spawn.code ~= 0 then
        return "spawn daemon: " .. (spawn.stderr or spawn.error or "exit " .. tostring(spawn.code))
      end
      local ok = sesh.wait_for(function()
        return ec_run(socket, "(emacs-version)").code == 0
      end, 5000)
      if not ok then
        return "daemon not ready after 5s"
      end
    end

    local r = ec_run(socket, form)
    if r.code ~= 0 then
      return "dispatch open hook: " .. (r.stderr or "exit " .. tostring(r.code))
    end
    return nil
  end,

  down = function(env, cfg)
    local c = defaults(cfg)
    local form = build_close_form(c.close_hook, env.name)
    local r = ec_run("--socket-name=" .. c.daemon, form)
    if r.code ~= 0 then
      sesh.log.warn("emacs-lua: close hook (best-effort): " .. (r.stderr or "exit " .. tostring(r.code)))
    end
    return nil -- best-effort: never propagate
  end,

  validate = function(env, cfg)
    local errs = {}
    local c = defaults(cfg)
    if not sesh.path_lookup("emacsclient") then
      table.insert(errs, "emacs-lua: emacsclient not found in PATH")
    end
    if not c.hook:match(ELISP_SYMBOL) then
      table.insert(errs, "emacs-lua: hook '" .. c.hook .. "' is not a valid elisp symbol")
    end
    if not c.close_hook:match(ELISP_SYMBOL) then
      table.insert(errs, "emacs-lua: close_hook '" .. c.close_hook .. "' is not a valid elisp symbol")
    end
    return errs
  end,

  dry_run = function(env, cfg)
    local c = defaults(cfg)
    local form = build_open_form(c.hook, env.name, env.cwd, c.files)
    return {
      { "emacsclient", "--socket-name=" .. c.daemon, "-e", form },
    }
  end,
})
