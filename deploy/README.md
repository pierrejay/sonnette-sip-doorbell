# deploy

Runtime files pushed to the board during application deployment.

This directory contains init scripts, ALSA configuration, example
credentials/settings files, the chime audio asset, logrotate config and
small maintenance scripts used by the Makefile deployment targets.

Do not put real secrets here directly. Run `make prepare-secrets` from
the repository root, then edit files under `deploy/etc/` locally; that
directory is gitignored.

See [../docs/deploy.md](../docs/deploy.md) for the deployment flow and
runtime notes.
