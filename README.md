# Vocfaucet

Faucet backend for the Vocdoni networks.

Example:

```
go run . --auth=open --amounts=150 --waitPeriod=1m --listenPort=8080
curl localhost:8080/v2/open/claim/0x658747A3eE4cb25D47cAfA3c106BeA4d559F6341
```

Multiple authentication handlers might be supported, using different faucet amounts:

```
go run . --auth=open,oauth --amounts=200,2000
```

With docker compose:

```
docker compose build
cp .env.example .env
docker compose up -d
```

