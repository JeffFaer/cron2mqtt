# cron2mqtt
A small wrapper for cron jobs (any executable, really) that publishes their outcome to one or more MQTT brokers.

It's currently setup so that it integrates well with Home Assistant.

## Usage

1. Configure cron2mqtt to tell it which MQTT broker to publish to.

   ```bash
   $ cron2mqtt configure \
     --broker "${BROKER_URI:?}" \
     --server_name "${OPTIONAL_SERVER_NAME_OVERRIDE:?}" \
     --username "${USERNAME:?}" \
     --password
   ```

   Note: You should not (and cannot) enter the password in the command. You will
   be prompted to enter your password after running the command.

2. Attach cron2mqtt to the cron jobs you wish to monitor.

   ```bash
   $ cron2mqtt attach
   ```

3. Your cron jobs will publish events to your MQTT broker the next time they run.

## Installation

```bash
$ go install github.com/JeffreyFalgout/cron2mqtt
```

## Commands

### `help`

Displays a help message, possibly about a particular command. It also displays
which flags a command accepts.

### `attach`

Attaches monitoring to existing cron jobs. The command will walk you through
your local crontab and ask you if you want to attach monitoring or not.

### `configure`

Sets configuration variables used by cron2mqtt to publish events to your MQTT
broker.

### `exec`

Executes a particular command and publishes the result to MQTT. The command is
uniquely identified by the first argument, and the rest of the arguments are the
command itself.

### `prune`

Purges data from your MQTT broker for cron jobs that don't appear to exist
locally anymore.
