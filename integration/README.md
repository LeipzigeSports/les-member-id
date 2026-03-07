The [Compose file](compose.yaml) starts a local test environment with Redis and Keycloak.
Keycloak is preconfigured with a "demo" realm and a "demo" user.
Within the "demo" realm is a "member-id" client that can be used for testing.

| **Username** | **Password** | **Role** |
|:--|:--|:--|
| admin | admin | Keycloak instance admin |
| demo | demo | Example user |

Start up services with the following command.

```shell
docker compose up -dV
```

Run `docker compose logs -f` and wait until Keycloak is up and running.
To finish setting up the "demo" realm, run the following command.

```shell
./setup-config-yaml.sh
```

This will create the "demo" user and reset the member-id client secret.
The script will then output the contents of the config.yaml file that is needed for the member ID service to run.

```yaml
secretKey: xxx

oauth:
  realmUrl: http://localhost:9090/realms/demo
  clientId: member-id
  clientSecret: xxx

redis:
  url: redis://localhost:6379/0
```
