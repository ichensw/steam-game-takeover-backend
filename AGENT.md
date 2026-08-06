# Agent Instructions

## Deployment

- Production deployment must use the repository deployment script:
  `./scripts/deploy_backend.sh deploy`
- Status checks should use:
  `./scripts/deploy_backend.sh status`
- If the deployment script fails because of environment, compatibility, test, build, upload, migration, service restart, or health-check issues, fix or make the script compatible first, then rerun the script.
- Do not bypass the script with ad hoc production steps such as manually copying binaries, running remote `systemctl restart`, applying migrations by hand, or calling remote deployment commands directly.
- Database migrations, when needed, must be passed through the script's supported `MIGRATIONS` / `REMOTE_MYSQL_CMD` flow.
