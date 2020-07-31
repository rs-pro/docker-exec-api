# golang docker exec api

# Project status: pre-alpha (not yet working)

```
$ go get github.com/rs-pro/docker-exec-api
```

## Whait this is

This is a REST microservice to execute commands via docker and provide their output as a websocket connection.

Includes support passing SSH agent through to the container.

Not designed to execute untrusted user-provided commands.

Websocket connection is read-only and not protected by api key (but requires a client to know a random docker id) by design so it can be served directly to end user.

Developed for [rtrack.ru](https://rtrack.ru) as a part of deployment automation service.

## Install

```
go get github.com/rs-pro/docker-exec-api
```

## Examples:

Examples use [httpie](https://httpie.org/) and [websocat](https://github.com/vi/websocat)

Replace ```create-your-key``` with a better key for security.

```
# create config.yml (see example in this repo)
docker-exec-api
# from your backend
http POST http://localhost:12010/sessions X-Api-Key:create-your-key image=ruby:2.7 commands:='["git -C app pull || git clone git@rscz.ru:rocket-science/piudelcibo.git app", "cd app", "gem install bundler:1.17.2", "bundle install", "cap staging deploy"]' volumes:='[{"piudelcibo_data": "/data"]' pull_image="ruby:2.7"
# Example output:

HTTP/1.1 200 OK
Content-Length: 73
Content-Type: application/json; charset=utf-8
Date: Mon, 06 Jul 2020 20:39:31 GMT

{
    "ID": "your-long-id"
}

# from browser to show output:
 websocat -t "ws://localhost:12010/sessions/your-long-id/websocket" -
# or
 http get http://localhost:12010/sessions/your-long-id/output
 
```

## Production usage

Please use nginx for HTTPS connections.
Config example:

```

```

## License

MIT License
