# sonnette

Go daemon running on the doorbell board.

It watches the GPIO button, plays the local chime, asks Twilio to start
the phone call, supervises `pjsua`, exposes the health endpoint, and emits
optional webhook events.

Build it from the repository root:

```bash
make build-go
```

See [../README.md](../README.md#sonnette-go-daemon) for the architecture
overview and [../docs/deploy.md](../docs/deploy.md) for configuration,
deployment and runtime notes.
