-- firefox.lua -- sesh plugin: launch Firefox under a per-project profile.
--
-- Tab state is handled by Firefox's own session restore (opt-in once per
-- profile under Settings -> "Open previous windows and tabs"). Sesh sets up
-- the profile and launches it; the browser handles the rest.

local function resolve_binary(cfg)
  if cfg.binary then return cfg.binary end
  local p = sesh.path_lookup("firefox")
  if p then return p end
  local app = "/Applications/Firefox.app/Contents/MacOS/firefox"
  if sesh.file_exists(app) then return app end
  return app -- exec will fail clearly if missing
end

sesh.register("firefox", {
  up = function(env, cfg)
    local bin = resolve_binary(cfg)
    local profile = cfg.profile or env.name
    local create = sesh.exec(bin, { "-CreateProfile", profile })
    if create.code ~= 0 and (create.error == nil or create.error == "") then
      -- non-zero without spawn-failure is acceptable (profile may exist)
      sesh.log.info("firefox: -CreateProfile returned " .. create.code .. " (often benign)")
    end
    local args = { "-P", profile, "--new-instance" }
    if cfg.url then table.insert(args, cfg.url) end
    local launch = sesh.exec_detach(bin, args)
    if launch.error and launch.error ~= "" then
      return "firefox: launch failed: " .. launch.error
    end
    return nil
  end,

  down = function(env, cfg)
    if not cfg.kill_on_down then return nil end
    local profile = cfg.profile or env.name
    sesh.applescript(string.format(
      [[tell application "Firefox" to close (every window whose name contains "%s")]],
      profile))
    return nil
  end,

  validate = function(env, cfg)
    local bin = cfg.binary
    if bin then
      if not sesh.file_exists(bin) then
        return { "firefox: configured binary not found: " .. bin }
      end
      return {}
    end
    if sesh.path_lookup("firefox") then return {} end
    local app = "/Applications/Firefox.app/Contents/MacOS/firefox"
    if sesh.file_exists(app) then return {} end
    return { "firefox: binary not found on PATH or at " .. app }
  end,

  dry_run = function(env, cfg)
    local bin = resolve_binary(cfg)
    local profile = cfg.profile or env.name
    local launch = { bin, "-P", profile, "--new-instance" }
    if cfg.url then table.insert(launch, cfg.url) end
    return {
      { bin, "-CreateProfile", profile },
      launch,
    }
  end,
})
