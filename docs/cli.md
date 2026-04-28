# CLI Reference

The `oni` CLI is your gateway to scaffolding, running, and managing OniWorks applications.

## Project

| Command | Description |
|---------|-------------|
| `oni new <name>` | Create a new OniWorks app |
| `oni new <name> --frontend` | Create with Vite + TS + Tailwind |
| `oni serve` | Start dev server (uses Air if installed) |
| `oni serve -p 3000` | Dev server on port 3000 |
| `oni build` | Compile production binary |
| `oni build -o myapp` | Custom output path |
| `oni deploy` | Start Caddy + Let's Encrypt proxy |

## Generators

```bash
oni make:controller User          # app/http/controllers/user_controller.go
oni make:model Post               # app/models/post.go
oni make:model Post -m            # + migration
oni make:resource Comment         # controller + model + migration
oni make:migration create_tags    # database/migrations/TIMESTAMP_create_tags.go
oni make:middleware Auth          # app/http/middleware/auth.go
oni make:job SendEmail            # app/jobs/send_email_job.go
oni make:mail WelcomeEmail        # app/mail/welcome_email_mail.go
oni make:seeder UserSeeder        # database/seeders/user_seeder.go
oni make:policy PostPolicy        # app/policies/post_policy.go
oni make:channel Chat             # app/channels/chat_channel.go
oni make:test PostTest            # tests/post_test.go
```

## Database

```bash
oni migrate                # run pending migrations
oni migrate:rollback       # roll back last batch
oni migrate:fresh          # drop all + re-run (prompts for confirmation)
oni migrate:status         # show migration status table
oni db:seed                # run all seeders
oni db:seed --class User   # run specific seeder
```

## Queue

```bash
oni queue:work                    # start workers (default queue)
oni queue:work --queue=mail       # specific queue
oni queue:work --tries=5          # max retry attempts
oni queue:restart                 # graceful restart
```

## Scheduler

```bash
oni schedule:run    # run all due scheduled tasks (use in cron: * * * * * oni schedule:run)
```

## Security

```bash
oni key:generate           # generate APP_KEY, write to .env
oni secrets:set DB_PASS s3cr3t   # encrypt and store a secret
oni secrets:get DB_PASS          # decrypt and print a secret
```

## Backup & Restore

```bash
oni backup                 # dump DB → storage/backups/
oni restore backup.sql.gz  # restore from file
```

## Operations

```bash
oni health          # run health checks
oni route:list      # list all HTTP routes
oni docs:serve      # serve OniWorks docs at localhost:4000
oni docs:serve --port 5000
oni admin:install   # set up the Oni Admin panel
```
