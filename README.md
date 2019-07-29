# Integrad - The Tiniest CI/CD Server

Most CI/CD servers are meant to integrate with a managed Git instance, deploy
to lots of servers at once, and have lots of resources dedicated to them.
Integrad is intended for low-resource, single-user-or-group servers that need to
deploy to themselves.

## Commands

- `integrad deploy --git <git ref> <source directory>`: Create a new
  deployment job.
- `integrad status [-j <job id>]`: View the status of a single or all jobs.
- `integrad logs <job id>`: View the logs of a single job.
- `integrad restart <job id>`: Start a new job with the parameters of the
  specified job.
- `integrad server`: Run the server in the local directory.
- `integrad shutdown`: Shutdown the Integrad server.

## Deployment Configuration

All configuration for a deployment is held in the `deploy.yaml` file in the top
directory of a project.  Integrad uses Go's `text/template` package to provide
build variables `{{ .Source }}` and `{{ .Build }}`, which are absolute paths to
the source and build directories respectively.  There are four possible
top-level sections in the configuration:

- `env`: Key-value pairs that represent environment variables for the
  deployment.  These variables will be available in all later sections.
- `build`: Commands to build the deployment.  These should create all necessary
  files in the `{{ .Build }}` directory, which will be cleaned up afterwards.
- `deploy`: Key-value pairs describing where files in the `{{ .Build }}`
  directory should be deployed to the server.
- `post`: Commands that run after all other steps.

An example configuration is provided in the `examples/` directory.

## Git Integration

Integrad is intended for small servers, which generally don't have managed Git
instances.  The easiest way to integrate Integrad with these is to set up a
[post-receive hook][git-hooks] that triggers a deployment.  An example hook is
provided in the `examples/` directory.

## Server Configuration

All configuration is done through environment variables, as the Lord Stallman
intended.  There are only three variables to set, and each have sensible
defaults:

- `INTEGRAD_SOCKET`: The Unix socket used for communication.  Default value: `"/var/integrad/integrad.sock"`
- `INTEGRAD_DB`: The database file used to keep track of jobs.  Default value:
  `"/var/integrad/integrad.db"`
- `INTEGRAD_SHELL`: The shell used to run all `build` and `post` commands.  Default value: `"bash"`

## Future Features

- Currently, you have to figure out how to run the Integrad server yourself.
  I've been running into some issues with systemd (what a shock), but I'll
  eventually provide an `integrad.service` file so that Integrad can be run as a
  system-level service.
- Jobs and their logs are kept around forever in an in-memory key-value store,
  which might exhaust resources on the server.  It might be useful to prune jobs
  once they are older than a specified time frame.
- At some point in the future, deployment control based on VCS branch might be
  useful to incorporate into the program directly, rather than just in the
  runner script.

## Non-Features

Integrad is not intended to be a complete replacement for other CI/CD servers.
For big and complex projects that need complex deployments, other servers will
suit you well.  They can build containers, deploy using custom deployment
scripts, spin up Kubernetes pods, and what have you.  Integrad will not do these
things.  It is intended to run on resource-constrained, single-server setups
owned by one person or a small group.

[git-hooks]: https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks
