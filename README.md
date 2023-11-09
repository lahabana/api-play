# ApiOpsDemo

A demo service with modifiable APIs for playing with a lot of things.
You can add latency to an API, errors and call other apis or even make the healthcheck fail.

This is a great way to play with a service mesh like [Kuma](https://kuma.io) for example.

## Running it

```shell
go run ./... -config-file config.yaml
```

Where `config.yaml` is a configuration of the apis to run.
The file is monitored so if you modify it we reload automatically it to change the apis served.

You can also change the APIs by using the API directly with a POST to `/api/dynamic`.

Check the openAPI spec for full documentation of what can be done.

## Dev

Run the app:
```shell
go run ./...
```

After changing the openAPI spec:
```shell
go generate ./...
```
