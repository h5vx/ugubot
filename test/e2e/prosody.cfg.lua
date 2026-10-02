admins = {}
modules_enabled = { "roster"; "saslauth"; "tls"; "disco"; "ping"; "version"; "offline" }
authentication = "internal_hashed"
storage = "internal"
c2s_require_encryption = true
allow_registration = false
log = { { levels = { min = "info" }, to = "console" } }
certificates = "/certs"
VirtualHost "localhost"
  ssl = { key = "/certs/localhost.key"; certificate = "/certs/localhost.crt" }
Component "conference.localhost" "muc"
  muc_room_locking = false
  muc_room_default_change_subject = true
  ssl = { key = "/certs/localhost.key"; certificate = "/certs/localhost.crt" }
