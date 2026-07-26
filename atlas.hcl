locals {
  database_password_raw = file(getenv("DATABASE_MIGRATION_PASSWORD_FILE"))
  database_password = endswith(local.database_password_raw, "\r\n") ? trimsuffix(local.database_password_raw, "\r\n") : (
    endswith(local.database_password_raw, "\n") ? trimsuffix(local.database_password_raw, "\n") : local.database_password_raw
  )
  database_url = "postgres://${urlescape(getenv("DATABASE_MIGRATION_USER"))}:${urlescape(local.database_password)}@${getenv("DATABASE_HOST_URL")}:${getenv("DATABASE_PORT")}/${getenv("DATABASE_NAME")}?sslmode=${urlescape(getenv("DATABASE_SSLMODE"))}"
}

env "nexusrelay" {
  url = local.database_url

  migration {
    dir          = "file://migrations"
    exec_order   = "LINEAR"
    lock_timeout = "60s"
  }
}
