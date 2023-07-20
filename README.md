# Vocfaucet

Faucet backend for the Vocdoni networks.

[WIP]

Example:

```
go run . --auth=open --amounts=150
curl localhost:8080/v2/open/claim/0x658747A3eE4cb25D47cAfA3c106BeA4d559F6341
```

Multiple authentication handlers might be supported, using different faucet amounts:

```
go run . --auth=open,oauth,sms --amounts=20,200,1000
```
