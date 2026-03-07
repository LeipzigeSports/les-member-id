#!/usr/bin/env bash
set -exo pipefail

kcadm () {
  docker compose exec keycloak bash /opt/keycloak/bin/kcadm.sh "${@}" >&2
}

CLIENT_ID="$(jq -r '[.clients[] | select(.clientId=="member-id")][0].id' import/realm-export.json)"
CLIENT_SECRET="$(openssl rand -hex 16)"
SECRET_KEY="$(openssl rand -hex 16)"

kcadm config credentials --server http://localhost:9090 --realm master --user admin --password admin
kcadm create users -r demo -s username=demo -s enabled=true -s emailVerified=true -s firstName=Max -s lastName=Mustermann -s email=max@mustermann.test
kcadm set-password -r demo --username demo --new-password demo
kcadm update "clients/$CLIENT_ID" -r demo -s "secret=$CLIENT_SECRET"


cat <<EOF
secretKey: $SECRET_KEY

oauth:
  realmUrl: http://localhost:9090/realms/demo
  clientId: member-id
  clientSecret: $CLIENT_SECRET

redis:
  url: redis://localhost:6379/0
EOF
