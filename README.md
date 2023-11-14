# http-mockery
HTTP server returning configurable responses with support for templates.

## Usage

To run http-mockery you'll need to have a config file. Example is available [here](examples/config-example.json).
Config file needs to include an `endpoints` config if you want to respond with anything else than 404 Not Found. Default listening address is "0.0.0.0:8080", but this can be changed with `listen_ip` and `listen_port`.
Default config file is `config.json` in the same directory as the application, but it can also be defined with `HTTP_MOCKERY_CONFIG` env variable.

Endpoint config needs to have atleast `uri`, `method` and `response_code` to operate normally. If you want the endpoint to return any JSON, you'll also need to provide the name of a `template` file, and `variables` configuration if the template includes template tags. Replacable variables are marked with `<` and `>` tags, e.g. `<replace_me>`. Matching `env_var` and `value` variables must then be found (see examples), `header` variables are treated as optional.   
Following template tag value providers are supported (order by priority):
- `env_var` uses environment variable's value to replace tag
- `value`, replaces template tag with raw value
- `header`, uses HTTP request header field value to replace template tag
 
Endpoint `type` is defaulted to `normal` but can also be set as `regexp`. It allows for standard regular expressions in the `uri` to match more specific use cases. Endpoints are checked in a given order and first matching endpoint (with correct `uri` and `response_code`) will be used.

Request and response bodies from requests can be logged with their relevant config options under `logging`, example [here](examples/config-example.json). Request & response content logging can also be toggled with env variables `HTTP_MOCKERY_REQUEST_CONTENTS` and `HTTP_MOCKERY_RESPONSE_CONTENTS` Endpoint-specific secrets are censored from logs.

It's also possible to proxy all requests that don't match any endpoint towards a configured destination with [proxy-pass configuration](examples/config-example-proxy-pass.json).

All templates must be valid JSON after variable replacement. No other formats are supported at this time (PRs are welcome!)
