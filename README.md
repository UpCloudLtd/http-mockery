# http-mockery
HTTP server returning configurable responses with support for templates.

## Usage

To run http-mockery you'll need to have a config file. Example is available [here](examples/config-example.json).
Config file needs to include an `endpoints` config if you want to respond with anything else than 404 Not Found. Default listening address is "0.0.0.0:8080", but this can be changed with `listen_ip` and `listen_port`.

Endpoint config needs to have atleast `uri`, `method` and `response_code` to operate normally. If you want the endpoint to return any JSON, you'll also need to provide the name of a `template` file, and `variables` configuration if the template includes anything to replace. Replacable variables are marked with < and >, e.g. `<replace_me>`. Matching variable must then be found (see examples). Value can be either provided in the config as `value` or as an environment variable where the env var name should be included in the variable config as `env_var`. Both `value` and `env_var` can be defined, but env_var always has precedence.

Endpoint `type` is defaulted to `normal` but can also be set as `regexp`. It allows for standard regular expressions in the `uri` to match more specific use cases. Endpoints are checked in a given order and first matching endpoint (with correct `uri` and `response_code`) will be used.

All templates must be valid JSON after variable replacement. No other formats are supported at this time (PRs are welcome!)

## Example usage with Kubernetes

TODO
